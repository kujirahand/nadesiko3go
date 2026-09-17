package format

import (
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// srcToken は、字句解析したトークンに元のソース上の位置を添えたもの。
// start・endは元のソースでのrune位置(半開区間)、startLine・endLineなどは
// ast.Node.Lineと同じ数え方の物理行番号(0始まり)と行内の桁(0始まり)。
type srcToken struct {
	lexer.Token
	start, end          int
	startLine, startCol int
	endLine, endCol     int
}

// significant は、トークンが改行・コメント以外(つまりプログラムの中身)か
// どうかを返す。
func (t srcToken) significant() bool {
	switch t.Type {
	case lexer.TypeEOL, lexer.TypeEOLUnderscore, lexer.TypeEOF,
		lexer.TypeLineComment, lexer.TypeRangeComment:
		return false
	}
	return true
}

// isComment は、トークンが行コメント・範囲コメントかどうかを返す。
func (t srcToken) isComment() bool {
	return t.Type == lexer.TypeLineComment || t.Type == lexer.TypeRangeComment
}

// scanTokens は、ParseSourceが構文解析の前段で通す字句解析の最初の段階
// (prepareしてからTokenize)だけを再実行し、各トークンの元のソース上の位置を
// 求める。インデント構文の変換やrequireの解決は通さない。それらは失敗したり
// トークン列を書き換えたりする可能性があり、ここでは無関係だからである。
//
// Tokenizeはprepare後のテキストに対して行うので、トークンのOffsetはcode
// ではなくそのテキストへの添字である。prepare.NewlineOriginalOffsetsは、
// その添字を元のcode自身のrune offsetへ戻す(全角記号の折り畳みは文字の
// 位置を変えないが、改行の正規化だけは変える。このマッピングはちょうど
// それを打ち消す)。
//
// 展開あり文字列(「{A}です」など)は、Tokenizeが長さ0の括弧や連結演算子を
// 補った式に組み立てるため、そのままでは元のソース上の範囲を表さない。
// ここでは1つの文字列トークン(TypeStringEx)にまとめ直す。長さ0のトークンは
// ソース上に実体がないので捨てる。
//
// Tokenizeは、フルパイプラインなら受け付けるはずの入力でも失敗することが
// ある(フルパイプラインは先にインデント構文の変換を通し、一部のソースは
// それに依存している)。失敗した場合はokにfalseを返す。
func scanTokens(code, filename string) (tokens []srcToken, ok bool) {
	prepared := prepare.Text(prepare.Convert(code))
	raw, err := lexer.Tokenize(prepared, 0, filename)
	if err != nil {
		return nil, false
	}
	raw = mergeStringEx(raw)
	preparedLen := len([]rune(prepared))
	toOriginal := prepare.NewlineOriginalOffsets(code)
	lineStarts := originalLineStarts([]rune(code))

	for _, tok := range raw {
		start, end := tok.Offset, tok.Offset+tok.Length
		if start < 0 || end > preparedLen || start >= end {
			continue
		}
		st := srcToken{Token: tok}
		st.start = originalOffset(toOriginal, start)
		st.end = originalOffset(toOriginal, end)
		st.startLine, st.startCol = lineColOf(lineStarts, st.start)
		st.endLine, st.endCol = lineColOf(lineStarts, st.end)
		tokens = append(tokens, st)
	}
	return tokens, true
}

// mergeStringEx は、Tokenizeが展開あり文字列から組み立てた
// 「( 文字列 & ( コード ) & 文字列 ... )」の並びを、元のソース上の範囲を
// 持つ1つのTypeStringExトークンに戻す。組み立てた括弧はどれも長さ0で、
// 最後の閉じ括弧だけが文字列の直後(助詞を含む)の位置と助詞を持つ。
// 埋め込み式の閉じ括弧はコードの直後に来るので、「文字列の直後にある長さ0の
// 閉じ括弧」を探せば全体の終わりが分かる。
func mergeStringEx(tokens []lexer.Token) []lexer.Token {
	out := make([]lexer.Token, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.Type == "(" && t.Length == 0 && i+1 < len(tokens) && tokens[i+1].Type == lexer.TypeString {
			closeAt := -1
			for j := i + 2; j < len(tokens); j++ {
				if tokens[j].Type == ")" && tokens[j].Length == 0 && tokens[j-1].Type == lexer.TypeString {
					closeAt = j
					break
				}
			}
			if closeAt >= 0 {
				merged := t
				merged.Type = lexer.TypeStringEx
				merged.Value = ""
				merged.Josi = tokens[closeAt].Josi
				merged.RawJosi = tokens[closeAt].RawJosi
				merged.Length = tokens[closeAt].Offset - t.Offset
				out = append(out, merged)
				i = closeAt
				continue
			}
		}
		out = append(out, t)
	}
	return out
}
