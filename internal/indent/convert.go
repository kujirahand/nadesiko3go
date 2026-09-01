// Package indent converts indentation syntax into parser input.
package indent

import (
	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

var modeKeywords = map[string]bool{
	"!インデント構文": true,
	"!ここまでだるい": true,
	"💡インデント構文": true,
	"💡ここまでだるい": true,
}

// ConvertSyntax inserts ここまで tokens at indentation boundaries when the
// source enables indentation syntax. It operates before comments are removed.
func ConvertSyntax(tokens []lexer.Token) ([]lexer.Token, error) {
	enabled := false
	for i, token := range tokens {
		if i > 100 {
			break
		}
		if token.Type == lexer.TypeLineComment && modeKeywords[token.StringValue()] {
			enabled = true
			break
		}
	}
	if !enabled {
		return convertInline(tokens), nil
	}
	for _, token := range tokens {
		if token.Type == lexer.TypeKokomade {
			return nil, &errs.NakoError{Kind: errs.Syntax, File: token.File, Line: token.Line, Msg: "インデント構文が有効化されているときに『ここまで』を使うことはできません。"}
		}
	}

	lines := splitLines(tokens)
	var levels [][2]int // [current indent, parent indent]
	lastIndent := 0
	jsonObjectLevel := 0
	jsonArrayLevel := 0

	for i, line := range lines {
		left := firstSignificant(line)
		if left == nil {
			continue
		}
		if jsonObjectLevel > 0 || jsonArrayLevel > 0 {
			countJSON(line, &jsonObjectLevel, &jsonArrayLevel)
			continue
		}
		current := left.Indent
		if current != lastIndent {
			for lastIndent > current && len(levels) > 0 {
				top := levels[len(levels)-1]
				if !skipEnd(left) || top[1] != current {
					appendEnd(&lines[i-1], lineTemplate(lines[i-1], *left))
				}
				levels = levels[:len(levels)-1]
				if len(levels) > 0 {
					lastIndent = levels[len(levels)-1][0]
				} else {
					lastIndent = 0
				}
			}
			if current > lastIndent {
				levels = append(levels, [2]int{current, lastIndent})
				lastIndent = current
			}
		}
		countJSON(line, &jsonObjectLevel, &jsonArrayLevel)
	}

	if len(lines) == 0 {
		return tokens, nil
	}
	template := lexer.Token{File: "main.nako3"}
	for i := len(lines) - 1; i >= 0; i-- {
		if len(lines[i]) > 0 {
			template = lines[i][len(lines[i])-1]
			break
		}
	}
	for range levels {
		appendEnd(&lines[len(lines)-1], template)
	}

	result := make([]lexer.Token, 0, len(tokens)+len(levels)*2)
	for _, line := range lines {
		result = append(result, line...)
	}
	return convertInline(result), nil
}

// convertInline implements the always-available trailing-colon indentation
// syntax. A colon at the end of a line opens a block; dedenting inserts the
// corresponding ここまで token. This is separate from !インデント構文 and
// matches nako_indent_inline.mts.
func convertInline(tokens []lexer.Token) []lexer.Token {
	lines := splitLines(tokens)
	var levels []int
	checkIndent := -1
	previousLine := -1
	for i := range lines {
		left := firstSignificant(lines[i])
		if left == nil {
			continue
		}
		current := left.Indent
		for checkIndent >= 0 && checkIndent >= current {
			if !(skipEnd(left) && checkIndent == current) && previousLine >= 0 {
				appendEnd(&lines[previousLine], lineTemplate(lines[previousLine], *left))
			}
			levels = levels[:len(levels)-1]
			if len(levels) == 0 {
				checkIndent = -1
			} else {
				checkIndent = levels[len(levels)-1]
			}
		}
		if at := lastSignificantIndex(lines[i]); at >= 0 && lines[i][at].Type == ":" {
			indent := lines[i][at].Indent
			lines[i] = append(lines[i][:at], lines[i][at+1:]...)
			levels = append(levels, indent)
			checkIndent = indent
		}
		previousLine = i
	}
	if previousLine >= 0 {
		for range levels {
			template := lineTemplate(lines[previousLine], lexer.Token{File: "main.nako3"})
			appendEnd(&lines[previousLine], template)
		}
	}
	result := make([]lexer.Token, 0, len(tokens)+len(levels)*2)
	for _, line := range lines {
		result = append(result, line...)
	}
	return result
}

func lastSignificantIndex(line []lexer.Token) int {
	for i := len(line) - 1; i >= 0; i-- {
		switch line[i].Type {
		case lexer.TypeEOL, lexer.TypeLineComment, lexer.TypeRangeComment:
			continue
		default:
			return i
		}
	}
	return -1
}

func splitLines(tokens []lexer.Token) [][]lexer.Token {
	var lines [][]lexer.Token
	var line []lexer.Token
	level := 0
	for _, token := range tokens {
		line = append(line, token)
		switch token.Type {
		case "{":
			level++
		case "}":
			level--
		case lexer.TypeEOL:
			if level == 0 {
				lines = append(lines, line)
				line = nil
			}
		}
	}
	if len(line) > 0 {
		lines = append(lines, line)
	}
	return lines
}

func firstSignificant(line []lexer.Token) *lexer.Token {
	for i := range line {
		switch line[i].Type {
		case lexer.TypeEOL, lexer.TypeLineComment, lexer.TypeRangeComment:
			continue
		default:
			return &line[i]
		}
	}
	return nil
}

func skipEnd(token *lexer.Token) bool {
	return token.Type == lexer.TypeChigaeba || (token.Type == lexer.TypeWord && token.StringValue() == "エラー" && token.Josi == "ならば")
}

func countJSON(line []lexer.Token, objectLevel, arrayLevel *int) {
	for _, token := range line {
		switch token.Type {
		case "{":
			*objectLevel++
		case "}":
			*objectLevel--
		case "[":
			*arrayLevel++
		case "]":
			*arrayLevel--
		}
	}
}

func lineTemplate(line []lexer.Token, fallback lexer.Token) lexer.Token {
	for i := len(line) - 1; i >= 0; i-- {
		if line[i].Type != lexer.TypeEOL {
			return line[i]
		}
	}
	return fallback
}

func appendEnd(line *[]lexer.Token, template lexer.Token) {
	end := template
	end.Type = lexer.TypeKokomade
	end.Value = "ここまで"
	end.Josi = ""
	end.RawJosi = ""
	eol := template
	eol.Type = lexer.TypeEOL
	eol.Value = template.Line
	eol.Josi = ""
	eol.RawJosi = ""
	*line = append(*line, end, eol)
}
