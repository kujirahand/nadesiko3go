// Package prepare normalizes source text before lexical analysis.
//
// The main job is folding full-width symbols to their half-width form, while
// leaving the inside of string literals and comments untouched
// (nako_prepare.mts equivalent).
package prepare

import (
	"regexp"
	"strings"
)

// Result is one piece of converted text together with where it started in the
// original source. Pos is a rune offset, not a byte offset.
//
// String literals and comments are emitted as a single Result holding the whole
// body plus its terminator, so the conversion table never touches their
// contents.
type Result struct {
	Text string
	Pos  int
}

// convertTable folds single characters that have an obvious half-width
// counterpart. Anything not listed here falls through to the FF01-FF5E range
// rule in Convert1ch.
var convertTable = map[rune]rune{
	// ハイフンへの変換
	0x2010: '-', // 別のハイフン
	0x2011: '-', // 改行しないハイフン
	0x2013: '-', // ENダッシュ
	0x2014: '-', // EMダッシュ
	0x2015: '-', // 全角のダッシュ
	0x2212: '-', // 全角のマイナス
	// チルダの変換
	0x02DC: '~', // 小さなチルダ
	0x02F7: '~', // Modifier Letter Low Tilde
	0x2053: '~', // Swung Dash
	0x223C: '~', // Tilde Operator
	0x301C: '~', // Wave Dash
	0xFF5E: '~', // 全角チルダ
	// スペースの変換
	0x2000: ' ', // EN QUAD
	0x2002: ' ', // EN SPACE
	0x2003: ' ', // EM SPACE
	0x2004: ' ', // THREE-PER-EM SPACE
	0x2005: ' ', // FOUR-PER-EM SPACE
	0x2006: ' ', // SIX-PER-EM SPACE
	0x2007: ' ', // FIGURE SPACE
	0x2009: ' ', // THIN SPACE
	0x200A: ' ', // HAIR SPACE
	0x200B: ' ', // ZERO WIDTH SPACE
	0x202F: ' ', // NARROW NO-BREAK SPACE
	0x205F: ' ', // MEDIUM MATHEMATICAL SPACE
	0x3164: ' ', // HANGUL FILLER
	// 全角スペース(0x3000)は変換しない。インデント量2として扱うため。
	// その他の変換
	0x203B:  '#', // '※' --- コメント
	0x3002:  ';', // 句点
	0x3010:  '[', // '【'
	0x3011:  ']', // '】'
	0x3001:  ',', // 読点
	0xFF0C:  ',', // '，'
	0x2715:  '*', // ✕
	0x2716:  '*', // ✖
	0x2717:  '*', // ✗
	0x2718:  '*', // ✘
	0x274C:  '*', // ❌
	0x2795:  '+', // ➕
	0x2796:  '-', // ➖
	0x2797:  '÷', // ➗
	0x1F7F0: '=', // 🟰
}

// Convert1ch folds one character. It returns c unchanged when there is nothing
// to fold.
//
// 0x1F7F0 ('🟰') is outside the BMP. The TypeScript version walks the source in
// UTF-16 code units and therefore hands convert1ch a lone surrogate, so its
// table entry never fires there. Go walks runes, so the entry works as it was
// originally intended (#1781).
func Convert1ch(c rune) rune {
	if folded, ok := convertTable[c]; ok {
		return folded
	}
	if c < 0x7F {
		return c
	}
	// 全角半角単純変換可能 --- '！' - '～'
	if c >= 0xFF01 && c <= 0xFF5E {
		return c - 0xFEE0
	}
	return c
}

// normalized holds line-ending-normalized source together with a mapping back
// to rune offsets in the original text.
type normalized struct {
	runes []rune
	pos   []int // pos[i] is the original rune offset of runes[i]
}

// normalizeNewlines turns CRLF and CR into LF. The offset table replaces the
// replay-based position mapping of the TypeScript version.
func normalizeNewlines(code string) normalized {
	src := []rune(code)
	out := normalized{
		runes: make([]rune, 0, len(src)),
		pos:   make([]int, 0, len(src)),
	}
	for i := 0; i < len(src); i++ {
		if src[i] == '\r' {
			out.runes = append(out.runes, '\n')
			out.pos = append(out.pos, i)
			if i+1 < len(src) && src[i+1] == '\n' {
				i++
			}
			continue
		}
		out.runes = append(out.runes, src[i])
		out.pos = append(out.pos, i)
	}
	return out
}

// sourcePos maps an index in the normalized text back to the original source.
// An index one past the end maps just past the last character.
func (n normalized) sourcePos(i int) int {
	if i < len(n.pos) {
		return n.pos[i]
	}
	if len(n.pos) == 0 {
		return 0
	}
	return n.pos[len(n.pos)-1] + 1
}

// hasPrefixAt reports whether the normalized text contains want starting at i.
func (n normalized) hasPrefixAt(i int, want string) bool {
	for _, r := range want {
		if i >= len(n.runes) || n.runes[i] != r {
			return false
		}
		i++
	}
	return true
}

// Convert folds the source one character at a time, skipping the inside of
// string literals and comments.
func Convert(code string) []Result {
	if code == "" {
		return nil
	}
	src := normalizeNewlines(code)

	var res []Result
	inStr := false  // 文字列リテラル内かどうか
	inStr2 := false // 絵文字・記号による範囲コメント内かどうか
	endOfStr := ""  // 終了させる記号
	left := 0       // 現在処理中の部分文字列の左端
	str := ""       // 文字列リテラルの中身

	i := 0
	for i < len(src.runes) {
		c := src.runes[i]

		// 文字列のとき。終了記号は1文字。
		if inStr {
			if string(c) == endOfStr {
				inStr = false
				res = append(res, Result{Text: str + endOfStr, Pos: src.sourcePos(left)})
				i++
				left = i
				continue
			}
			str += string(c)
			i++
			continue
		}
		// 範囲コメントのとき。終了記号は複数文字になりうる。
		if inStr2 {
			if src.hasPrefixAt(i, endOfStr) {
				inStr2 = false
				closing := endOfStr
				if closing == "＊／" {
					closing = "*/" // 強制変換
				}
				res = append(res, Result{Text: str + closing, Pos: src.sourcePos(left)})
				i += len([]rune(endOfStr))
				left = i
				continue
			}
			str += string(c)
			i++
			continue
		}

		// 文字列判定
		if closing, ok := stringCloser(c); ok {
			res = append(res, Result{Text: string(c), Pos: src.sourcePos(left)})
			i++
			left = i
			inStr = true
			endOfStr = closing
			str = ""
			continue
		}
		// 絵文字による範囲コメント (#726)
		if c == '🌴' || c == '🌿' {
			res = append(res, Result{Text: string(c), Pos: src.sourcePos(left)})
			i++
			left = i
			inStr2 = true
			endOfStr = string(c)
			str = ""
			continue
		}

		c1 := Convert1ch(c)
		// クォートによる文字列。閉じ記号は変換前の文字なので、全角で開いたら全角で閉じる。
		if c1 == '"' || c1 == '\'' {
			res = append(res, Result{Text: string(c1), Pos: src.sourcePos(left)})
			i++
			left = i
			inStr = true
			endOfStr = string(c)
			str = ""
			continue
		}
		// ラインコメントを飛ばす (#725)
		if c1 == '#' {
			res = append(res, Result{Text: string(c1), Pos: src.sourcePos(left)})
			i++
			left = i
			inStr = true // 本当はコメントだが、中身を変換しない点は文字列と同じ
			endOfStr = "\n"
			str = ""
			continue
		}
		if src.hasPrefixAt(i, "//") || src.hasPrefixAt(i, "／／") {
			res = append(res, Result{Text: "//", Pos: src.sourcePos(left)}) // 強制変換
			i += 2
			left = i
			inStr = true
			endOfStr = "\n"
			str = ""
			continue
		}
		// 複数行コメントを飛ばす (#731)
		if src.hasPrefixAt(i, "／＊") {
			res = append(res, Result{Text: "/*", Pos: src.sourcePos(left)}) // 強制変換
			i += 2
			left = i
			inStr2 = true
			endOfStr = "＊／"
			str = ""
			continue
		}
		if src.hasPrefixAt(i, "/*") {
			res = append(res, Result{Text: "/*", Pos: src.sourcePos(left)})
			i += 2
			left = i
			inStr2 = true
			endOfStr = "*/"
			str = ""
			continue
		}

		res = append(res, Result{Text: string(c1), Pos: src.sourcePos(left)})
		i++
		left = i
	}

	if inStr || inStr2 {
		// 閉じられていない文字列はここではエラーにしない。字句解析側が報告する。
		// コメントなど文字列以外は自動で閉じる。
		// TS版と同じ判定にしてある。全角クォートや”で閉じられていない場合は
		// ここで自動的に閉じられる。
		switch endOfStr {
		case "\"", "'", "」", "』":
		default:
			res = append(res, Result{Text: str + endOfStr, Pos: src.sourcePos(left)})
		}
	}
	return res
}

// stringCloser reports the closing character for the quotes that open a string
// literal.
func stringCloser(c rune) (string, bool) {
	switch c {
	case '「':
		return "」", true
	case '『':
		return "』", true
	case '“':
		return "”", true
	}
	return "", false
}

// Text joins the converted pieces back into a single string.
func Text(results []Result) string {
	var b strings.Builder
	for _, r := range results {
		b.WriteString(r.Text)
	}
	return b.String()
}

var (
	nakoModeMarkRE  = regexp.MustCompile(`！|💡`)
	nakoModeBlockRE = regexp.MustCompile(`/\*.*?\*/`)
	nakoModeSplitRE = regexp.MustCompile(`[;。\n]`)
)

// CheckNakoMode reports whether the source declares one of modeNames, such as
// "!インデント構文". Only the first 256 runes are inspected.
func CheckNakoMode(code string, modeNames []string) bool {
	code = string([]rune(code)[:min(256, len([]rune(code)))])
	// 全角半角の揺れを吸収する。TS版と同じく最初の1件だけ。
	if loc := nakoModeMarkRE.FindStringIndex(code); loc != nil {
		code = code[:loc[0]] + "!" + code[loc[1]:]
	}
	code = nakoModeBlockRE.ReplaceAllString(code, "")

	lines := nakoModeSplitRE.Split(code, 30)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		for _, name := range modeNames {
			if line == name {
				return true
			}
		}
	}
	return false
}
