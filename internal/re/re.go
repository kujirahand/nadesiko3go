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
	"regexp/syntax"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ErrUnsupported reports a pattern the engine cannot handle. RE2 rejects
// backreferences and lookaround by design.
var ErrUnsupported = errors.New("この正規表現はまだ対応していません")

// FlagError は 『/pattern/flags』 の形でJavaScriptが拒むフラグを表す。知らない
// 文字、すでに現れた文字、同時に書けない組み合わせのいずれか。JavaScriptでは
// どれもSyntaxErrorなので、ここでも言語のエラーにする。
type FlagError struct {
	// Flag は問題のフラグ。ASCII英字以外の文字もそのまま報告できるよう
	// runeで持つ。
	Flag      rune
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
// (一致位置) と 『u』 はGoでパターンの意味を変えない。『v』は文字として受理
// するだけで、UnicodeSetsの集合演算は未実装で、Goの構文解釈になりJSのvと等価でない。
const validFlags = "dgimsuvy"

// stickyMode は sticky の照合がどの経路を使うかを表す。位置指定の照合が
// 要らないパターンはGoの走査をそのまま使い、要るパターンは、前後の文字を
// 見るアサーション（『^』『$』『\b』など）を含むかどうかで分ける。
type stickyMode uint8

const (
	// stickyScan はGoのFindAll走査をそのまま使い、先頭から連続する一致だけを
	// 残す。空一致し得ず、文脈を見るアサーションを含まないパターンだけが
	// ここに入る。
	stickyScan stickyMode = iota
	// stickySuffix は文脈を見ない位置指定照合。元のパターンを s[pos:] に
	// 当てて先頭一致だけを採る。一致の可否が後ろの文字だけで決まるので、
	// 派生パターンを作らなくても意味が変わらない。
	stickySuffix
	// stickyContext は前後の文字を見るアサーションを含む位置指定照合。
	// 直前の1runeを残した部分文字列に派生パターンを当てる。
	stickyContext
)

// StickyVariantError は、文脈を見る位置指定照合用の派生パターンを作れな
// かったことを表す。元のパターン自体はGoの正規表現として正しいので、
// 『パターンが正しくない』とは区別する。
type StickyVariantError struct {
	// Depth は入れ子の深さがGoの上限を超えたことを表す。
	Depth bool
	Cause error
}

func (e *StickyVariantError) Error() string {
	if e.Depth {
		return "文脈を見る位置指定照合用の派生パターンが深すぎます"
	}
	return "文脈を見る位置指定照合用の派生パターンを作れません"
}

func (e *StickyVariantError) Unwrap() error { return e.Cause }

// Regexp はコンパイル済みのパターンと、そのとき書かれたフラグをまとめたもの。
type Regexp struct {
	re *regexp.Regexp
	// tail は文脈を見る位置指定照合用の派生パターン。stickyContextのときだけ
	// 非nilで、Compileで作り切る。照合の途中でコンパイルしない。
	tail *regexp.Regexp
	// mode はstickyの照合経路。stickyでないパターンではstickyScanのまま。
	mode stickyMode
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
	// textScan は一致文字列だけで連続を確かめられるパターンか。文脈を見る
	// ゼロ幅アサーションを含まず、空一致もし得ない場合だけtrueになる。
	textScan bool
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

// slashForm は 『/pattern/flags』 の形を本体とフラグ列に分ける。最後の
// 『/』 より後を常にフラグ列として扱うので、フラグ列にASCII英字以外が来ても
// 取りこぼさずに検証へ回せる。JavaScriptの正規表現リテラル読み取りは
// 『[a-zA-Z]*』 で終わる形だけをこの形式とみなすため、『/a/é』 をパターン
// そのものとして受理してしまう。ここではそれを言語のエラーにする。
//
// 先頭が 『/』 でないものと、本体が空になるもの（『//』 など）は、JavaScript
// と同じく裸のパターンとして扱う。
func slashForm(pattern string) (body, flags string, ok bool) {
	if !strings.HasPrefix(pattern, "/") {
		return "", "", false
	}
	last := strings.LastIndexByte(pattern, '/')
	if last <= 1 {
		return "", "", false
	}
	return pattern[1:last], pattern[last+1:], true
}

// Compile builds a pattern written the way nadesiko spells it: either
// 『/pattern/flags』, or a bare pattern, which is treated as if it carried the
// 『g』 flag.
func Compile(pattern string) (*Regexp, error) {
	body, flags := pattern, "g"
	if b, f, ok := slashForm(pattern); ok {
		body, flags = b, f
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
	mode, tail, context, err := stickySearch(compiled, sticky)
	if err != nil {
		return nil, err
	}
	textScan := sticky && mode == stickyScan && !context
	return &Regexp{
		re:       compiled,
		Global:   global,
		Sticky:   sticky,
		tail:     tail,
		mode:     mode,
		hasNames: hasNames,
		textScan: textScan,
	}, nil
}

// stickySearch はstickyの照合経路を決める。位置指定の照合が要らないなら
// 走査のまま。要るなら、前後の文字を見るアサーションの有無で、元のパターンを
// そのまま当てる経路と、派生パターンを当てる経路に分ける。派生パターンは
// errorを返せるCompileの中で作る。
func stickySearch(compiled *regexp.Regexp, sticky bool) (stickyMode, *regexp.Regexp, bool, error) {
	if !sticky {
		return stickyScan, nil, false, nil
	}
	context := hasRealContextAssertion(compiled)
	if !needsPositionalSearch(compiled) {
		return stickyScan, nil, context, nil
	}
	if !context {
		return stickySuffix, nil, false, nil
	}
	tail, err := compileStickyTail(compiled)
	if err != nil {
		return stickyScan, nil, context, err
	}
	return stickyContext, tail, context, nil
}

// needsPositionalSearch は位置指定の照合が要るかを判定する。空一致し得る
// パターンと、正の単語境界 『\b』 を綴るパターンが該当する。空の対象で一致
// しないこと自体は、正の幅の要素を要求している証拠になる。ただし 『\b』 は
// 空の対象では偽になり、前後に文字がある位置では真になり得る唯一のゼロ幅
// アサーションなので、それを綴るパターンはGoのFindAllの一致列を信用できない。
func needsPositionalSearch(re *regexp.Regexp) bool {
	return re.MatchString("") || strings.Contains(re.String(), `\b`)
}

// hasContextAssertion は、一致の前後の文字や絶対位置を見るゼロ幅アサーションを
// 綴っているかを保守的に判定する。文字列として探すだけなので、文字クラスや
// エスケープされたリテラルを拾うことがあるが、その場合は遅い経路へ倒れるだけで
// 安全側になる。
func hasContextAssertion(pattern string) bool {
	for _, token := range []string{"^", "$", `\b`, `\B`, `\A`, `\z`} {
		if strings.Contains(pattern, token) {
			return true
		}
	}
	return false
}

// hasRealContextAssertion は、そのゼロ幅アサーションが本当に構文木に現れるかを
// 確かめる。文字列の走査だけでは、文字クラスの中の 『$』 や、エスケープした
// リテラル、『\Q...\E』 の中の文字まで拾ってしまう。拾い過ぎたまま派生パターンを
// 作ると、元が正しくても深さの都合で拒むことになる。
func hasRealContextAssertion(compiled *regexp.Regexp) bool {
	src := compiled.String()
	if !hasContextAssertion(src) {
		return false
	}
	expr, err := syntax.Parse(src, syntax.Perl)
	if err != nil {
		// 文字列としては疑わしいのに構文木へ落とせない。安全側に倒して
		// 文脈ありとして扱う。
		return true
	}
	return hasContextAssertionNode(expr)
}

// hasContextAssertionNode は、一致の前後の文字や絶対位置を見るアサーションが
// 構文木にあるかを返す。対象は 『^』『$』『\A』『\z』『\b』『\B』 だけ。
func hasContextAssertionNode(expr *syntax.Regexp) bool {
	switch expr.Op {
	case syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText,
		syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return true
	}
	for _, sub := range expr.Sub {
		if hasContextAssertionNode(sub) {
			return true
		}
	}
	return false
}

// stickyFromTexts は一致文字列の並びから、先頭に連続する分だけを残す。文脈を
// 見るアサーションを含まないパターンでは、一致の可否は消費する文字だけで決まる
// ので、次の一致が現在位置の前置きであることを確かめれば連続と言える。
func stickyFromTexts(s string, matches []string) []string {
	kept := matches[:0]
	cursor := 0
	for _, m := range matches {
		if !strings.HasPrefix(s[cursor:], m) {
			break
		}
		kept = append(kept, m)
		cursor += len(m)
	}
	switch {
	case len(kept) == 0:
		return nil
	case len(kept) == len(matches):
		// 全件が連続していたら走査結果をそのまま使う
		return kept
	}
	// 途中で途切れたときは、残りの一致列が確保したbackingを返り値に引きずら
	// ないよう、先頭の連続分だけを詰め直す。
	out := make([]string, len(kept))
	copy(out, kept)
	return out
}

// validateFlags はJavaScriptがフラグ列で拒むものを拒む。重複したフラグも、
// 知らない文字も、JavaScriptではSyntaxError。無言で受理すると、書いた意図と
// 違うパターンが動いてしまう。『u』と『v』はどちらもUnicodeモードを選ぶので、
// 同時に書くことも拒む。
func validateFlags(flags string) error {
	var seen [256]bool
	for _, f := range flags {
		if !strings.ContainsRune(validFlags, f) {
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
	if r.textScan {
		return stickyFromTexts(s, r.re.FindAllString(s, -1))
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

// compileStickyTail は、文脈を見る位置指定照合用の派生パターンを作る。
//
// 部分文字列を切り出して再マッチすると ^ や \b のような文脈が失われるので、
// 直前の1runeを残した部分文字列に 『\A(?s:.)』 を前置する。前の1runeを消費
// させたうえで先頭に固定するので、照合位置から見た前後の文脈は元の文字列の
// ままになる。
//
// 派生パターンは元のパターンより1段深い。元がvalidでもGoの入れ子の上限を
// 超えることがあるので、作れなかったことはerrorで返す。まず素の包みを試し、
// Goの 『\Q』 が対応する 『\E』 を欠いて後ろを飲み込んでいる場合だけ、包みの
// 中で引用を閉じる。
func compileStickyTail(compiled *regexp.Regexp) (*regexp.Regexp, error) {
	src := compiled.String()
	tail, err := regexp.Compile(`\A(?s:.)(?:` + src + `)`)
	if err == nil {
		return tail, nil
	}
	if closed, closeErr := regexp.Compile(`\A(?s:.)(?:` + src + `\E)`); closeErr == nil {
		return closed, nil
	}
	return nil, &StickyVariantError{Depth: isNestingDepth(err), Cause: err}
}

// isNestingDepth は、Goの正規表現が入れ子の深さの上限で失敗したかを返す。
func isNestingDepth(err error) bool {
	var syntaxErr *syntax.Error
	return errors.As(err, &syntaxErr) && syntaxErr.Code == syntax.ErrNestingDepth
}

// matchAt は pos からちょうど始まる一致全体と捕獲を、s の座標で返す。そこか
// ら始まる一致がなければnilを返す。
func (r *Regexp) matchAt(s string, pos int) []int {
	if r.mode == stickySuffix {
		m := r.re.FindStringSubmatchIndex(s[pos:])
		if m == nil || m[0] != 0 {
			return nil
		}
		return shiftIndexes(m, pos)
	}
	if pos == 0 {
		m := r.re.FindStringSubmatchIndex(s)
		if m == nil || m[0] != 0 {
			return nil
		}
		return m
	}
	base := prevRuneStart(s, pos)
	m := r.tail.FindStringSubmatchIndex(s[base:])
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

// shiftIndexes は部分文字列の座標を元の文字列の座標へ移す。参加していない
// 組の-1はそのまま残す。
func shiftIndexes(m []int, offset int) []int {
	row := make([]int, len(m))
	row[0], row[1] = offset+m[0], offset+m[1]
	for i := 2; i < len(m); i += 2 {
		if m[i] < 0 {
			row[i], row[i+1] = -1, -1
			continue
		}
		row[i], row[i+1] = offset+m[i], offset+m[i+1]
	}
	return row
}

// matchWholeAt は一致全体だけを要る呼び出し元のための matchAt。
func (r *Regexp) matchWholeAt(s string, pos int) []int {
	if r.mode == stickySuffix {
		m := r.re.FindStringIndex(s[pos:])
		if m == nil || m[0] != 0 {
			return nil
		}
		return []int{pos, pos + m[1]}
	}
	if pos == 0 {
		m := r.re.FindStringIndex(s)
		if m == nil || m[0] != 0 {
			return nil
		}
		return m
	}
	base := prevRuneStart(s, pos)
	m := r.tail.FindStringIndex(s[base:])
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

// stickyIndexes はグローバルなsticky検索が訪れる一致の位置列を返す。位置指定
// の照合が要らないパターンはGoのFindAll走査をそのまま使い、各一致が前の一致の
// 終端から始まることだけを確かめる。空一致が落ちる心配がないので走査は完全で
// ある。位置指定の照合が要るパターンは、派生パターンを持つかどうかではなく
// 経路そのもので分ける。派生パターンを持たない後ろ向きの経路でも空一致は
// 走査から落ちるので、位置を指定して1件ずつ確かめる。
func (r *Regexp) stickyIndexes(s string, submatch bool) [][]int {
	if r.mode != stickyScan {
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

// Split は一致のたびに文字列を区切る。JavaScriptの String.prototype.split と
// 同じで、一致の前後の断片の間に、その一致の捕獲を順に差し込む。参加しな
// かった組はundefinedとして印を付ける。
//
// 『g』 と 『y』 のフラグは区切りの結果を変えない。区切りは常に文字列の先頭
// から末尾まで歩く。
func (r *Regexp) Split(s string) []SplitPart {
	if s == "" {
		// 空の対象を、空文字に一致するパターンで区切ると、JavaScriptは
		// 要素を1つも返さない。一致しないパターンなら空の要素を1つ返す。
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
