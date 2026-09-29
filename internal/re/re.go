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
	"strconv"
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
	// hasNames はパターンが名前付きキャプチャを持つか。JavaScriptでは
	// 名前付き組を持たない正規表現では置換テンプレートの $<name> が
	// リテラルになるため、その判定に使う
	hasNames bool
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
	body = normalizeJSUnicodeEscapes(body)

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
	hasNames := false
	for _, name := range compiled.SubexpNames()[1:] {
		if name != "" {
			hasNames = true
			break
		}
	}
	return &Regexp{re: compiled, Global: global, hasNames: hasNames}, nil
}

var (
	jsUnicodeEscapeRE  = regexp.MustCompile(`\\u([0-9A-Fa-f]{4})`)
	jsSurrogateRangeRE = regexp.MustCompile(`\[(?:\\uD[89ABab][0-9A-Fa-f]{2})-(?:\\uD[89ABab][0-9A-Fa-f]{2})\]\[(?:\\uD[C-Fc-f][0-9A-Fa-f]{2})-(?:\\uD[C-Fc-f][0-9A-Fa-f]{2})\]`)
)

func normalizeJSUnicodeEscapes(pattern string) string {
	// A JavaScript UTF-16 surrogate-pair range denotes non-BMP code points.
	// Go's regexp engine is rune based, so express it as one rune range.
	pattern = jsSurrogateRangeRE.ReplaceAllString(pattern, `[\x{10000}-\x{10FFFF}]`)
	return jsUnicodeEscapeRE.ReplaceAllStringFunc(pattern, func(escape string) string {
		n, err := strconv.ParseInt(escape[2:], 16, 32)
		if err != nil || n >= 0xd800 && n <= 0xdfff {
			return escape
		}
		return `\x{` + strings.ToUpper(escape[2:]) + `}`
	})
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

// FindAllSubmatches returns every match with its capture groups. Index 0 in
// each row is the whole match, matching regexp.FindAllStringSubmatch.
func (r *Regexp) FindAllSubmatches(s string) [][]string {
	return r.re.FindAllStringSubmatch(s, -1)
}

// SubexpNames reports the name of each capture group. Index 0 is always empty.
func (r *Regexp) SubexpNames() []string { return r.re.SubexpNames() }

// Find returns the first match and its capture groups, or nil when there is
// none. Index 0 is the whole match.
func (r *Regexp) Find(s string) []string { return r.re.FindStringSubmatch(s) }

// Replace substitutes matches with the template, honouring the 『g』 flag: only
// the first match changes without it.
//
// The template is written in JavaScript's style: 『$$』 is a literal dollar,
// 『$&』 the match, 『$`』 and 『$'』 the parts before and after it, 『$n』
// a capture group and 『$<name>』 a named one.
func (r *Regexp) Replace(s, template string) string {
	limit := -1
	if !r.Global {
		limit = 1
	}
	// 一致した範囲だけを切り出して再マッチすると \B や ^ のような
	// 文脈が失われるので、元文字列上の一致位置でテンプレートを展開する
	matches := r.re.FindAllStringSubmatchIndex(s, limit)
	if len(matches) == 0 {
		return s
	}
	var out strings.Builder
	out.Grow(len(s) + len(template)*len(matches))
	last := 0
	for _, m := range matches {
		out.WriteString(s[last:m[0]])
		r.expandTemplate(&out, template, s, m)
		last = m[1]
	}
	out.WriteString(s[last:])
	return out.String()
}

// expandTemplate expands a JavaScript replacement template for one match.
// match indexes src, as FindStringSubmatchIndex returns. It runs per match
// because 『$`』 and 『$'』 depend on where the match sits in src.
func (r *Regexp) expandTemplate(out *strings.Builder, template, src string, match []int) {
	groups := len(match)/2 - 1
	for i := 0; i < len(template); {
		if template[i] != '$' || i+1 >= len(template) {
			out.WriteByte(template[i])
			i++
			continue
		}
		next := template[i+1]
		switch {
		case next == '$':
			out.WriteByte('$')
			i += 2
		case next == '&':
			out.WriteString(src[match[0]:match[1]])
			i += 2
		case next == '`':
			out.WriteString(src[:match[0]])
			i += 2
		case next == '\'':
			out.WriteString(src[match[1]:])
			i += 2
		case next >= '0' && next <= '9':
			// 組番号は最大2桁を整数として読む。2桁が範囲外なら1桁に
			// フォールバックし、それも無効なら『$』をリテラルにする
			n, size := int(next-'0'), 1
			if i+2 < len(template) && template[i+2] >= '0' && template[i+2] <= '9' {
				if nn := n*10 + int(template[i+2]-'0'); nn >= 1 && nn <= groups {
					n, size = nn, 2
				}
			}
			if n < 1 || n > groups {
				out.WriteByte('$')
				i++
				continue
			}
			if match[2*n] >= 0 {
				out.WriteString(src[match[2*n]:match[2*n+1]])
			}
			i += 1 + size
		case next == '<' && r.hasNames:
			// 存在しない名前や一致しなかった組は空文字になる
			end := strings.IndexByte(template[i+2:], '>')
			if end < 0 {
				out.WriteByte('$')
				i++
				continue
			}
			if idx := r.re.SubexpIndex(template[i+2 : i+2+end]); idx >= 0 && match[2*idx] >= 0 {
				out.WriteString(src[match[2*idx]:match[2*idx+1]])
			}
			i += 3 + end
		default:
			out.WriteByte('$')
			i++
		}
	}
}

// Split cuts the string at every match.
func (r *Regexp) Split(s string) []string { return r.re.Split(s, -1) }
