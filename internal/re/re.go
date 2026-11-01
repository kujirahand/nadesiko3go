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

// FlagError は 『/pattern/flags』 の形でJavaScriptが拒むフラグを表す。知らない
// 文字、すでに現れた文字、同時に書けない組み合わせのいずれか。JavaScriptでは
// どれもSyntaxErrorなので、ここでも言語のエラーにする。
type FlagError struct {
	Flag      byte
	Duplicate bool
	// Conflict はJavaScriptが同時指定を拒む相手のフラグ。『u』と『v』は
	// どちらもUnicodeモードを選ぶので、片方しか書けない。
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

// validFlags はJavaScriptが 『/pattern/flags』 として受け付ける文字。『d』
// (一致位置) と 『u』/『v』 (Unicodeモード) はGoでパターンの意味を変えないが、
// JavaScriptでは正しい入力なので受理する。
const validFlags = "dgimsuvy"

// stickyTail は位置指定照合に使う派生パターンを、必要になった時点で1度だけ
// 作って保持する。空一致し得るstickyと、正の単語境界を綴るstickyだけがこれ
// を持つ。
type stickyTail struct {
	once sync.Once
	re   *regexp.Regexp
}

// Regexp はコンパイル済みのパターンと、そのとき書かれたフラグをまとめたもの。
type Regexp struct {
	re *regexp.Regexp
	// tail は位置指定照合の派生パターン。空一致し得ないstickyではnilで、
	// GoのFindAll走査と連続開始filterだけで済ませる。
	tail *stickyTail
	// Global はJavaScriptの 『g』 フラグ。置換がすべての一致を対象にするか、
	// 最初の一致だけかを決める。
	Global bool
	// Sticky はJavaScriptの 『y』 フラグ。一致は前の一致が終わった位置から
	// 始まる必要があり、最初の一致は対象の先頭から探す。
	Sticky bool
	// hasNames はパターンが名前付きキャプチャを持つか。JavaScriptでは
	// 名前付き組を持たない正規表現では置換テンプレートの $<name> が
	// リテラルになるため、その判定に使う
	hasNames bool
}

// Group は一致の捕獲1つ分。Okがfalseならその組は一致に参加していない。
// JavaScriptではこれがundefinedになるが、Goの正規表現は 『』 に一致した組と
// 同じ空文字で返すため、区別できるようにしている。
type Group struct {
	Text string
	Ok   bool
}

// Match は一致全体とその捕獲をまとめたもの。Textはパターン全体が一致した
// 文字列で、Groupsは1番目以降の組を順に持つ。
type Match struct {
	Text   string
	Groups []Group
}

// SplitPart は区切りの結果1つ分。Undefinedがtrueなら、その捕獲は参加して
// おらず、JavaScriptでは配列にundefinedとして入る。そのときTextは空で、
// 言語の値は 『』 ではなくundefinedになる。
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
	var tail *stickyTail
	if sticky && needsPositionalSearch(compiled) {
		tail = &stickyTail{}
	}
	return &Regexp{
		re:       compiled,
		Global:   global,
		Sticky:   sticky,
		tail:     tail,
		hasNames: hasNames,
	}, nil
}

// needsPositionalSearch は位置指定の照合が要るかを判定する。空一致し得る
// パターンと、正の単語境界 『\b』 を綴るパターンが該当する。空の対象で一致
// しないこと自体は、正の幅の要素を要求している証拠になる。ただし 『\b』 は
// 空の対象では偽になり、前後に文字がある位置では真になり得る唯一のゼロ幅
// アサーションなので、それを綴るパターンはGoのFindAllの一致列を信用できない。
func needsPositionalSearch(re *regexp.Regexp) bool {
	return re.MatchString("") || strings.Contains(re.String(), `\b`)
}

// validateFlags はJavaScriptがフラグ列で拒むものを拒む。重複したフラグも、
// 知らない文字も、JavaScriptではSyntaxError。無言で受理すると、書いた意図と
// 違うパターンが動いてしまう。『u』と『v』はどちらもUnicodeモードを選ぶので、
// 同時に書くことも拒む。
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

// FindAll は一致全体をすべて返す。一致がなければnilを返し、命令側で 『空』 に
// なる。stickyのパターンはJavaScriptが訪れる位置をたどるので、前の一致が
// 終わった位置から始まる一致だけを数える。
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

// FindAllMatches は一致とその捕獲をすべて返す。
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

// Find は最初の一致とその捕獲を返す。一致がないときと、stickyのパターンが
// 先頭で一致しないときはfalseを返す。
func (r *Regexp) Find(s string) (Match, bool) {
	indexes := r.re.FindStringSubmatchIndex(s)
	if indexes == nil || r.Sticky && indexes[0] != 0 {
		return Match{}, false
	}
	return matchFromIndexes(s, indexes), true
}

// matchFromIndexes は正規表現が返す位置の組からMatchを組み立てる。参加しな
// かった組は-1の位置を持ち、空文字ではなく「値がない」として扱う。
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
	r.tail.once.Do(func() {
		// 元のコンパイル結果からパターンを引き直すので、パターン文字列を
		// 別に持ち回る必要はない。
		r.tail.re = regexp.MustCompile(`\A(?s:.)(?:` + r.re.String() + `)`)
	})
	return r.tail.re
}

// matchAt は pos からちょうど始まる一致全体と捕獲を、s の座標で返す。そこか
// ら始まる一致がなければnilを返す。
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

// matchWholeAt は一致全体だけを要る呼び出し元のための matchAt。
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

// advanceMatch は次のstickyの一致を探し始める位置を返す。空の一致は1rune
// 進める。JavaScriptのグローバル検索も同じ進み方をする。
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

// stickyWholeMatches はグローバルなsticky検索が訪れる一致全体の位置を、
// JavaScriptと同じ順に返す。
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

// stickyIndexes はグローバルなsticky検索が訪れる一致の位置列を返す。空一致
// し得ないパターンはGoのFindAll走査をそのまま使い、各一致が前の一致の終端
// から始まることだけを確かめる。空一致が落ちる心配がないので走査は完全で
// ある。それ以外は位置指定の照合を使う。
func (r *Regexp) stickyIndexes(s string, submatch bool) [][]int {
	if r.tail != nil {
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

// stickyFromScan は先頭から途切れずに続く一致だけを残す。空一致し得ない
// パターンにだけ使い、そのときGoの走査はsticky検索が到達し得る一致をすべて
// 含んでいる。
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

// stickySubmatches は stickyWholeMatches の捕獲付き版。
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
