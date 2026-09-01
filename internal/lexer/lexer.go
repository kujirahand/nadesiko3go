// Package lexer splits preprocessed nadesiko source into tokens
// (nako_lexer.mts equivalent).
package lexer

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Tokenize splits src into tokens. src must already have been through the
// prepare package. line is the line number of the first line, zero-based.
//
// The returned error is always *errs.NakoError with Kind errs.Lexer.
func Tokenize(src string, line int, filename string) ([]Token, error) {
	srcLen := runeLen(src)
	var result []Token
	column := 1
	isDefTest := false

	// 最初にインデントを数える
	indent, skip := countIndent(src)
	src = dropRunes(src, skip)
	column += skip

	lexErr := func(msg string, at int) error {
		return &errs.NakoError{Kind: errs.Lexer, File: filename, Line: at, Msg: msg}
	}

	for src != "" {
		ok := false
		for _, r := range rules {
			m := r.pattern.FindStringIndex(src)
			if m == nil {
				continue
			}
			matched := src[:m[1]]
			ruleName := r.name
			ok = true

			// 空白ならスキップ。TS版と同じく、ここでは while ループではなく
			// ルールのループを continue する。
			if r.name == TypeSpace {
				column += runeLen(matched)
				src = src[m[1]:]
				continue
			}

			if r.parser != nil {
				rp, err := r.parser(src, !(isDefTest && r.name == TypeWord))
				if err != nil {
					return nil, lexErr(err.Error(), line)
				}
				consumed := runeLen(src) - runeLen(rp.src)

				if r.name == TypeStringEx {
					list, listOK := splitStringEx(rp.res)
					if !listOK {
						return nil, lexErr("展開あり文字列で値の埋め込み{...}が対応していません。", line)
					}
					offsetBase := srcLen - runeLen(src)
					if len(list) == 1 {
						// 展開なし(埋め込み式なし)の場合
						result = append(result, Token{
							Type: TypeString, Value: list[0], Josi: rp.josi, Indent: indent,
							File: filename, Line: line, Column: column,
							Offset: offsetBase, Length: consumed,
						})
						line += rp.numEOL
						column += consumed
						src = rp.src
						if rp.numEOL > 0 {
							column = 1
						}
						break
					}
					// 展開あり(埋め込み式あり)の場合。連結する式に組み立てる。
					result = append(result, Token{Type: "(", Value: "(", Indent: indent, File: filename, Line: line, Column: column, Offset: offsetBase})
					offset := 0
					for i, part := range list {
						partLen := runeLen(part)
						if i%2 == 0 {
							result = append(result, Token{
								Type: TypeString, Value: part, Indent: indent, File: filename,
								Line: line, Column: column,
								Offset: offsetBase + offset, Length: partLen + 2,
							})
							// 先頭なら'"...{'、それ以外なら'}...{'、最後は何でもよい
							offset += partLen + 2
							continue
						}
						at := offsetBase + offset
						result = append(result,
							Token{Type: "&", Value: "&", Indent: indent, File: filename, Line: line, Column: column, Offset: at},
							Token{Type: "(", Value: "(", Indent: indent, File: filename, Line: line, Column: column, Offset: at},
							Token{Type: TypeCode, Value: part, Indent: indent, File: filename, Line: line, Column: column, Offset: at, Length: partLen},
							Token{Type: ")", Value: ")", Indent: indent, File: filename, Line: line, Column: column, Offset: at + partLen},
							Token{Type: "&", Value: "&", Indent: indent, File: filename, Line: line, Column: column, Offset: at + partLen},
						)
						offset += partLen
					}
					line += rp.numEOL
					column += consumed
					src = rp.src
					if rp.numEOL > 0 {
						column = 1
					}
					result = append(result, Token{Type: ")", Value: ")", Josi: rp.josi, Indent: indent, File: filename, Line: line, Column: column, Offset: srcLen - runeLen(src)})
					break
				}

				columnCurrent := column
				column += consumed
				result = append(result, Token{
					Type: r.name, Value: rp.res, Josi: rp.josi, Indent: indent,
					Line: line, Column: columnCurrent, File: filename,
					Offset: srcLen - runeLen(src), Length: consumed,
				})
				src = rp.src
				line += rp.numEOL
				if rp.numEOL > 0 {
					column = 1
				}
				break
			}

			// ソースを進める前に位置を計算する
			srcOffset := srcLen - runeLen(src)

			var value any = matched
			if r.cb != nil {
				value = r.cb(matched)
			}
			columnCurrent := column
			lineCurrent := line
			column += runeLen(matched)
			src = src[m[1]:]
			// 改行の時の処理。eolの値はその行の行番号になる。
			if (r.name == TypeEOL && matched == "\n") || r.name == TypeEOLUnderscore {
				value = line
				line++
				column = 1
			}

			// 数値なら単位を持つか？ --- #994
			if r.name == TypeNumber {
				if um := UnitRE.FindString(src); um != "" {
					src = src[len(um):]
					column += runeLen(matched)
				}
				// CSSの単位なら文字列として認識させる #1811
				if cssUnit := CSSUnitRE.FindString(src); cssUnit != "" {
					ruleName = TypeString
					src = src[len(cssUnit):]
					column += runeLen(matched)
					value = jsNumberToString(value) + cssUnit
				}
			}

			particle := ""
			if r.readJosi {
				if j, rest, found := readJosi(src); found {
					column += runeLen(src) - runeLen(rest)
					particle = normalizeJosiAfter(j)
					src = rest
				}
			}

			switch ruleName {
			case TypeDefTest:
				isDefTest = true
			case TypeEOL:
				isDefTest = false
			}

			// ここまで‰(#682) を処理
			if ruleName == TypeDecLineNo {
				line--
				break
			}

			result = append(result, Token{
				Type: ruleName, Value: value, Indent: indent,
				Line: lineCurrent, Column: columnCurrent, File: filename,
				Josi:   particle,
				Offset: srcOffset, Length: (srcLen - runeLen(src)) - srcOffset,
			})
			// 改行のとき次の行のインデントを調べる。改行の後は必ずcolumnが1になる。
			// 一行に2つ以上の文を含む場合を考慮する。(core #66)
			if ruleName == TypeEOL && column == 1 {
				var skip int
				indent, skip = countIndent(src)
				column += skip
				src = dropRunes(src, skip)
			}
			break
		}
		if !ok {
			return nil, lexErr("未知の語句: "+firstRunes(src, 3)+"...", line)
		}
	}
	return result, nil
}

// countIndent returns the indent width of the leading indent characters and how
// many characters they occupy.
func countIndent(src string) (indent, skip int) {
	for _, c := range src {
		n := isIndentChars(c)
		if n == 0 {
			return indent, skip
		}
		indent += n
		skip++
	}
	return indent, skip
}

var stringExOpenRE = regexp.MustCompile(`[{｛]`)

// splitStringEx splits an interpolated string into alternating literal and
// expression parts: "A{B}C{D}E" becomes ["A", "B", "C", "D", "E"]. It reports
// false when a `{` has no matching `}`.
func splitStringEx(code string) ([]string, bool) {
	arr := stringExOpenRE.Split(code, -1)
	list := []string{arr[0]}
	for _, s := range arr[1:] {
		end, size := indexCloseBrace(s)
		if end < 0 {
			return nil, false
		}
		list = append(list, s[:end], s[end+size:])
	}
	return list, true
}

// indexCloseBrace finds the first closing brace, half-width or full-width,
// and reports its byte offset and byte length.
func indexCloseBrace(s string) (offset, size int) {
	half := strings.Index(s, "}")
	full := strings.Index(s, "｝")
	switch {
	case half < 0 && full < 0:
		return -1, 0
	case full < 0 || (half >= 0 && half < full):
		return half, len("}")
	}
	return full, len("｝")
}

// jsNumberToString renders a rule value the way JavaScript's string conversion
// would, so a CSS unit can be appended to it (#1811).
func jsNumberToString(v any) string {
	switch n := v.(type) {
	case string:
		return n
	case float64:
		return value.NumberToString(n)
	}
	return ""
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

// dropRunes removes the first n runes of s.
func dropRunes(s string, n int) string {
	for i := 0; i < n && s != ""; i++ {
		_, size := utf8.DecodeRuneInString(s)
		s = s[size:]
	}
	return s
}

func firstRunes(s string, n int) string {
	out := make([]rune, 0, n)
	for _, r := range s {
		if len(out) == n {
			break
		}
		out = append(out, r)
	}
	return string(out)
}
