// Package re abstracts the regular-expression engine used by the nadesiko
// regexp commands.
//
// It is deliberately not part of the compatibility guarantee: the commands in
// internal/stdlib are. This package only has to compile a JavaScript-style
// pattern and run it, so that the engine behind it can change (AGENTS.md §3).
//
// The engine is Go's standard regexp, which is RE2. RE2 has no backreferences
// and no lookaround; a pattern using them fails to compile and is reported as
// ErrUnsupported, which is where dlclark/regexp2 would slot in later
// (AGENTS.md §15).
package re

import (
	"errors"
	"regexp"
	"strings"
)

// ErrUnsupported reports a pattern the engine cannot handle. RE2 rejects
// backreferences and lookaround by design.
var ErrUnsupported = errors.New("この正規表現はまだ対応していません")

// Regexp is a compiled pattern together with the flags it was written with.
type Regexp struct {
	re *regexp.Regexp
	// Global is the JavaScript 『g』 flag. It decides whether a replacement
	// touches every match or only the first.
	Global bool
}

// jsPatternRE splits the 『/pattern/flags』 form nadesiko writes patterns in.
var jsPatternRE = regexp.MustCompile(`^/(.+)/([a-zA-Z]*)$`)

// Compile builds a pattern written the way nadesiko spells it: either
// 『/pattern/flags』, or a bare pattern, which is treated as if it carried the
// 『g』 flag.
func Compile(pattern string) (*Regexp, error) {
	body, flags := pattern, "g"
	if m := jsPatternRE.FindStringSubmatch(pattern); m != nil {
		body, flags = m[1], m[2]
	}

	var prefix strings.Builder
	global := false
	for _, f := range flags {
		switch f {
		case 'g':
			global = true
		case 'i':
			prefix.WriteString("(?i)")
		case 'm':
			prefix.WriteString("(?m)")
		case 's':
			prefix.WriteString("(?s)")
		case 'u', 'y', 'd', 'v':
			// u はGoでは常にrune単位なので指定不要。y と d は
			// 検索位置の扱いだけの違いで、パターンの意味を変えない。
		}
	}

	compiled, err := regexp.Compile(prefix.String() + body)
	if err != nil {
		if unsupportedByRE2(body) {
			return nil, ErrUnsupported
		}
		return nil, err
	}
	return &Regexp{re: compiled, Global: global}, nil
}

// unsupportedByRE2 reports whether a pattern uses a construct RE2 leaves out
// on purpose, rather than being malformed.
func unsupportedByRE2(pattern string) bool {
	return strings.Contains(pattern, "(?=") || strings.Contains(pattern, "(?!") ||
		strings.Contains(pattern, "(?<=") || strings.Contains(pattern, "(?<!") ||
		backreferenceRE.MatchString(pattern)
}

// backreferenceRE matches 『\1』 and friends, but not an escaped backslash.
var backreferenceRE = regexp.MustCompile(`(^|[^\\])(\\\\)*\\[1-9]`)

// FindAll returns every match. It reports nil when there is none, which the
// commands turn into 『空』.
func (r *Regexp) FindAll(s string) []string { return r.re.FindAllString(s, -1) }

// Find returns the first match and its capture groups, or nil when there is
// none. Index 0 is the whole match.
func (r *Regexp) Find(s string) []string { return r.re.FindStringSubmatch(s) }

// Replace substitutes matches with the template, honouring the 『g』 flag: only
// the first match changes without it.
//
// The template is written in JavaScript's style, where 『$1』 refers to a group.
func (r *Regexp) Replace(s, template string) string {
	expanded := jsTemplate(template)
	if r.Global {
		return r.re.ReplaceAllString(s, expanded)
	}
	loc := r.re.FindStringIndex(s)
	if loc == nil {
		return s
	}
	first := r.re.ReplaceAllString(s[loc[0]:loc[1]], expanded)
	return s[:loc[0]] + first + s[loc[1]:]
}

// Split cuts the string at every match.
func (r *Regexp) Split(s string) []string { return r.re.Split(s, -1) }

// groupRefRE finds a JavaScript group reference in a replacement template.
var groupRefRE = regexp.MustCompile(`\$(\d+|&)`)

// jsTemplate rewrites a JavaScript replacement template into Go's form.
//
// Go reads 『$1年』 as a group named 「1年」, so every reference is wrapped in
// braces. A literal 『$』 that is not a reference is escaped as 『$$』.
func jsTemplate(template string) string {
	var out strings.Builder
	last := 0
	for _, loc := range groupRefRE.FindAllStringSubmatchIndex(template, -1) {
		out.WriteString(strings.ReplaceAll(template[last:loc[0]], "$", "$$"))
		ref := template[loc[2]:loc[3]]
		if ref == "&" {
			ref = "0" // $& は一致した全体
		}
		out.WriteString("${" + ref + "}")
		last = loc[1]
	}
	out.WriteString(strings.ReplaceAll(template[last:], "$", "$$"))
	return out.String()
}
