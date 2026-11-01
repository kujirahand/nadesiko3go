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
	"sync"
	"unicode/utf8"
)

// ErrUnsupported reports a pattern the engine cannot handle. RE2 rejects
// backreferences and lookaround by design.
var ErrUnsupported = errors.New("この正規表現はまだ対応していません")

// FlagError reports a flag JavaScript rejects in the 『/pattern/flags』 form:
// a letter it does not know, one that already appeared, or a pair that cannot
// be written together. All of them are a SyntaxError in JavaScript, so they
// are a language error here too.
type FlagError struct {
	Flag      byte
	Duplicate bool
	// Conflict is the other flag of a pair JavaScript refuses. 『u』 and 『v』
	// each select Unicode mode, so only one may appear.
	Conflict byte
}

func (e *FlagError) Error() string {
	switch {
	case e.Duplicate:
		return "正規表現のフラグ『" + string(e.Flag) + "』が重複しています"
	case e.Conflict != 0:
		return "正規表現のフラグ『" + string(e.Flag) + "』と『" + string(e.Conflict) + "』は同時に使えません"
	}
	return "正規表現のフラグ『" + string(e.Flag) + "』は使えません"
}

// validFlags are the letters JavaScript accepts as 『/pattern/flags』. 『d』
// (match indices) and 『u』/『v』 (Unicode mode) do not change what a pattern
// matches in Go, but they are still valid input and are accepted.
const validFlags = "dgimsuvy"

// Regexp is a compiled pattern together with the flags it was written with.
type Regexp struct {
	re *regexp.Regexp
	// Global is the JavaScript 『g』 flag. It decides whether a replacement
	// touches every match or only the first.
	Global bool
	// Sticky is the JavaScript 『y』 flag. Every match has to start where the
	// previous one ended, so a fresh match starts at the beginning of the
	// subject instead of being searched for anywhere in it.
	Sticky bool
	// stickyPattern はフラグの接頭辞を含むGoのパターンで、stickyの位置指定
	// 照合に使う。先頭位置の照合は元のコンパイル結果で足りるので、この変種は
	// 2件目以降を探すときにはじめてコンパイルする。
	stickyPattern string
	stickyOnce    sync.Once
	stickyTail    *regexp.Regexp
	// stickyPositional は位置指定の照合が要るか。空一致をし得るパターンと、
	// 正の単語境界 『\b』 を含むパターンが該当する。\b は空の対象では成立
	// しないが、前後に文字がある位置では成立する唯一のゼロ幅アサーション
	// なので、GoのFindAllの一致列を信用できない。
	stickyPositional bool
	// hasNames はパターンが名前付きキャプチャを持つか。JavaScriptでは
	// 名前付き組を持たない正規表現では置換テンプレートの $<name> が
	// リテラルになるため、その判定に使う
	hasNames bool
}

// Group is one capture group of a match. Ok is false when the group did not
// take part in the match: JavaScript has undefined for that, while Go's
// regexp reports the same empty string as a group that matched 『』.
type Group struct {
	Text string
	Ok   bool
}

// Match is one whole match together with its capture groups. Text is the text
// the whole pattern matched; Groups holds groups 1..n in order.
type Match struct {
	Text   string
	Groups []Group
}

// SplitPart is one item of a split. Undefined marks a capture group that did
// not take part, which JavaScript puts in the array as undefined; Text is then
// empty and the language value is undefined rather than 『』.
type SplitPart struct {
	Text      string
	Undefined bool
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
	if err := validateFlags(flags); err != nil {
		return nil, err
	}

	var prefix strings.Builder
	global, sticky := false, false
	for _, f := range flags {
		switch f {
		case 'g':
			global = true
		case 'y':
			sticky = true
		case 'i':
			prefix.WriteString("(?i)")
		case 'm':
			prefix.WriteString("(?m)")
		case 's':
			prefix.WriteString("(?s)")
		case 'u', 'd', 'v':
			// u はGoでは常にrune単位なので指定不要。d は一致位置の
			// 返し方だけの違いで、パターンの意味を変えない。
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
	stickyPattern := ""
	positional := false
	if sticky {
		stickyPattern = prefix.String() + body
		// 空の対象で一致しないことと、正の単語境界を綴っていないことが、
		// 空一致しないことの保守的な証明になる（\b だけが空では偽で、
		// 文脈では真になり得るゼロ幅アサーション）。
		positional = compiled.MatchString("") || strings.Contains(stickyPattern, `\b`)
	}
	return &Regexp{
		re:               compiled,
		Global:           global,
		Sticky:           sticky,
		stickyPattern:    stickyPattern,
		stickyPositional: positional,
		hasNames:         hasNames,
	}, nil
}

// validateFlags rejects what JavaScript rejects in a flag list. A repeated flag
// is a SyntaxError there, and so is an unknown letter; ignoring either would
// run a pattern with a meaning nobody wrote. 『u』 and 『v』 both select Unicode
// mode, so writing both is refused as well.
func validateFlags(flags string) error {
	var seen [256]bool
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		if !strings.ContainsRune(validFlags, rune(f)) {
			return &FlagError{Flag: f}
		}
		if seen[f] {
			return &FlagError{Flag: f, Duplicate: true}
		}
		seen[f] = true
	}
	if iu, iv := strings.IndexByte(flags, 'u'), strings.IndexByte(flags, 'v'); iu >= 0 && iv >= 0 {
		if iu < iv {
			return &FlagError{Flag: 'u', Conflict: 'v'}
		}
		return &FlagError{Flag: 'v', Conflict: 'u'}
	}
	return nil
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

// FindAll returns every whole match. It reports nil when there is none, which
// the commands turn into 『空』. A sticky pattern walks the positions JavaScript
// visits, so a match only counts when it starts where the previous one ended.
func (r *Regexp) FindAll(s string) []string {
	if !r.Sticky {
		return r.re.FindAllString(s, -1)
	}
	indexes := r.stickyIndexes(s, false)
	if len(indexes) == 0 {
		return nil
	}
	found := make([]string, len(indexes))
	for i, m := range indexes {
		found[i] = s[m[0]:m[1]]
	}
	return found
}

// FindAllMatches returns every match with its capture groups.
func (r *Regexp) FindAllMatches(s string) []Match {
	var indexes [][]int
	if r.Sticky {
		indexes = r.stickyIndexes(s, true)
	} else {
		indexes = r.re.FindAllStringSubmatchIndex(s, -1)
	}
	matches := make([]Match, len(indexes))
	for i, m := range indexes {
		matches[i] = matchFromIndexes(s, m)
	}
	return matches
}

// SubexpNames reports the name of each capture group. Index 0 is always empty.
func (r *Regexp) SubexpNames() []string { return r.re.SubexpNames() }

// Find returns the first match and its capture groups. It reports false when
// there is none, or when a sticky pattern does not match at the beginning.
func (r *Regexp) Find(s string) (Match, bool) {
	indexes := r.re.FindStringSubmatchIndex(s)
	if indexes == nil || r.Sticky && indexes[0] != 0 {
		return Match{}, false
	}
	return matchFromIndexes(s, indexes), true
}

// matchFromIndexes builds a Match from the index pairs that the regexp methods
// return. A group that did not take part has -1 indexes, which is a missing
// value rather than an empty string.
func matchFromIndexes(s string, indexes []int) Match {
	m := Match{Text: s[indexes[0]:indexes[1]]}
	m.Groups = make([]Group, len(indexes)/2-1)
	for i := range m.Groups {
		start, end := indexes[2*i+2], indexes[2*i+3]
		if start < 0 {
			continue
		}
		m.Groups[i] = Group{Text: s[start:end], Ok: true}
	}
	return m
}

// prevRuneStart は pos の直前のruneの開始バイト位置を返す。
func prevRuneStart(s string, pos int) int {
	_, size := utf8.DecodeLastRuneInString(s[:pos])
	return pos - size
}

// stickyTailRE は位置 pos>0 から始まる一致を求めるための変種を返す。
//
// 部分文字列を切り出して再マッチすると ^ や \b のような文脈が失われるので、
// 直前の1runeを残した部分文字列に 『\A(?s:.)』 を前置する。前の1runeを消費
// させたうえで先頭に固定するので、照合位置から見た前後の文脈は元の文字列の
// ままになる。この変種は必要になった時点で作る。
func (r *Regexp) stickyTailRE() *regexp.Regexp {
	r.stickyOnce.Do(func() {
		r.stickyTail = regexp.MustCompile(`\A(?s:.)(?:` + r.stickyPattern + `)`)
	})
	return r.stickyTail
}

// matchAt returns the whole match and capture groups that start exactly at
// pos, with indexes in the coordinates of s. It reports nil when no match
// starts there.
func (r *Regexp) matchAt(s string, pos int) []int {
	if pos == 0 {
		m := r.re.FindStringSubmatchIndex(s)
		if m == nil || m[0] != 0 {
			return nil
		}
		return m
	}
	base := prevRuneStart(s, pos)
	m := r.stickyTailRE().FindStringSubmatchIndex(s[base:])
	if m == nil {
		return nil
	}
	// 変種は直前の1runeを消費するので、全体の開始は pos、終端と部分一致は
	// 残した1runeの分だけ元の座標へ戻す。
	row := make([]int, len(m))
	row[0], row[1] = pos, base+m[1]
	for i := 2; i < len(m); i += 2 {
		if m[i] < 0 {
			row[i], row[i+1] = -1, -1
			continue
		}
		row[i], row[i+1] = base+m[i], base+m[i+1]
	}
	return row
}

// matchWholeAt is matchAt for callers that only need the whole match.
func (r *Regexp) matchWholeAt(s string, pos int) []int {
	if pos == 0 {
		m := r.re.FindStringIndex(s)
		if m == nil || m[0] != 0 {
			return nil
		}
		return m
	}
	base := prevRuneStart(s, pos)
	m := r.stickyTailRE().FindStringIndex(s[base:])
	if m == nil {
		return nil
	}
	return []int{pos, base + m[1]}
}

// advanceMatch reports where the next sticky match may start. An empty match
// moves one rune on, which is how JavaScript's global search advances.
func advanceMatch(s string, m []int) int {
	if m[1] != m[0] {
		return m[1]
	}
	if m[1] >= len(s) {
		return len(s) + 1
	}
	_, size := utf8.DecodeRuneInString(s[m[1]:])
	return m[1] + size
}

// stickyWholeMatches returns the whole match positions a global sticky search
// visits, in JavaScript's order.
func (r *Regexp) stickyWholeMatches(s string) [][]int {
	var out [][]int
	for pos := 0; pos <= len(s); {
		m := r.matchWholeAt(s, pos)
		if m == nil {
			break
		}
		out = append(out, m)
		pos = advanceMatch(s, m)
	}
	return out
}

// stickyIndexes returns the index rows a global sticky search visits. A pattern
// that cannot match an empty string keeps Go's FindAll scan and only checks that
// each match starts where the previous one ended; no empty match can be dropped
// there, so the scan is complete. Everything else uses the positional search.
func (r *Regexp) stickyIndexes(s string, submatch bool) [][]int {
	if r.stickyPositional {
		if submatch {
			return r.stickySubmatches(s)
		}
		return r.stickyWholeMatches(s)
	}
	if submatch {
		return stickyFromScan(r.re.FindAllStringSubmatchIndex(s, -1))
	}
	return stickyFromScan(r.re.FindAllStringIndex(s, -1))
}

// stickyFromScan keeps the leading run of matches that starts at the beginning
// and continues without a gap. It is only used for patterns that cannot match an
// empty string, so Go's scan has every match the sticky search could reach.
func stickyFromScan(matches [][]int) [][]int {
	kept := matches[:0]
	next := 0
	for _, m := range matches {
		if m[0] != next {
			break
		}
		kept = append(kept, m)
		next = m[1]
	}
	return kept
}

// stickySubmatches is stickyWholeMatches keeping the capture groups.
func (r *Regexp) stickySubmatches(s string) [][]int {
	var out [][]int
	for pos := 0; pos <= len(s); {
		m := r.matchAt(s, pos)
		if m == nil {
			break
		}
		out = append(out, m)
		pos = advanceMatch(s, m)
	}
	return out
}

// Replace substitutes matches with the template, honouring the 『g』 flag: only
// the first match changes without it.
//
// The template is written in JavaScript's style: 『$$』 is a literal dollar,
// 『$&』 the match, 『$`』 and 『$'』 the parts before and after it, 『$n』
// a capture group and 『$<name>』 a named one.
func (r *Regexp) Replace(s, template string) string {
	var matches [][]int
	switch {
	case !r.Sticky && !r.Global:
		matches = r.re.FindAllStringSubmatchIndex(s, 1)
	case !r.Sticky:
		matches = r.re.FindAllStringSubmatchIndex(s, -1)
	case !r.Global:
		// stickyの非globalは先頭で始まる一致だけが対象になる
		if m := r.re.FindStringSubmatchIndex(s); m != nil && m[0] == 0 {
			matches = [][]int{m}
		}
	default:
		matches = r.stickyIndexes(s, true)
	}
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

// Split cuts the string at every match, the way JavaScript's
// String.prototype.split does: each capture group is inserted between the
// segments around its match, and a group that did not take part is undefined.
//
// The 『g』 and 『y』 flags do not change a split: it always walks the string
// from the beginning and stops at the end.
func (r *Regexp) Split(s string) []SplitPart {
	if s == "" {
		// JavaScript returns no items when an empty subject is separated by a
		// pattern that matches the empty string, and one empty item otherwise.
		if r.re.MatchString("") {
			return nil
		}
		return []SplitPart{{Text: ""}}
	}
	matches := r.re.FindAllStringSubmatchIndex(s, -1)
	parts := make([]SplitPart, 0, len(matches)+1)
	p := 0
	for _, m := range matches {
		// 一致の終端が区切りの開始位置と同じなら、それは空の一致で、
		// JavaScriptは区切りとして数えず走査位置だけを1つ進める。
		// 文字列の末尾で始まる一致も、走査が終わったあとにしか現れない。
		if m[1] == p || m[0] == len(s) {
			continue
		}
		parts = append(parts, SplitPart{Text: s[p:m[0]]})
		for g := 1; 2*g+1 < len(m); g++ {
			if m[2*g] < 0 {
				parts = append(parts, SplitPart{Undefined: true})
				continue
			}
			parts = append(parts, SplitPart{Text: s[m[2*g]:m[2*g+1]]})
		}
		p = m[1]
	}
	return append(parts, SplitPart{Text: s[p:]})
}
