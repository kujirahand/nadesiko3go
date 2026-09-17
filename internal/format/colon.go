package format

import (
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

// colonBlock は、コロン記法に書き換えられる1つのブロック本文を表す。
// headerLineの行末に『:』を付け、endLineが『ここまで』だけの行なら
// (removeEnd)その行を消す。本文が『違えば』『エラーならば』で終わる場合は、
// インデントを戻すだけで本文が閉じるので、終端の行はそのまま残す。
type colonBlock struct {
	headerLine int
	colonAt    int // 『:』を挿入する元のソース上のrune位置
	endLine    int
	removeEnd  bool
}

// colonBlocks は、『ここまで』などの閉じるキーワードで終わるブロック本文の
// うち、コロン記法(行末の『:』とインデント)で書き換えられるものを集める。
//
// 対象にするのは、次をすべて満たす本文だけである。
//   - 開始行(もし〜ならば・N回など)の行末で本文が始まり、少なくとも1つの
//     文を持ち、開始行とは別の行にある本物の閉じるキーワードで終わる。
//   - 閉じるキーワードが『ここまで』なら、その行に他のコード・コメントが無い。
//   - 『条件分岐』の各分岐ではない(条件分岐はコロン記法の対象外)。
//
// 『!インデント構文』などのモード指定があるファイルは、ブロックの表し方が
// 異なるので何も書き換えない。
func colonBlocks(code, filename string, tree *ast.Node) []colonBlock {
	tokens, ok := scanTokens(code, filename)
	if !ok {
		return nil
	}
	lines := map[int][]srcToken{} // 行 -> その行で始まる改行以外のトークン
	lastEnd := map[int]int{}      // 行 -> その行で終わる最後のコードの終端位置
	lastType := map[int]lexer.TokenType{}
	for _, t := range tokens {
		if t.Type == lexer.TypeLineComment {
			v := t.StringValue()
			if strings.HasPrefix(v, "!") || strings.HasPrefix(v, "💡") {
				return nil // 『!インデント構文』『!DNCLモード』など
			}
		}
		if t.Type != lexer.TypeEOL {
			lines[t.startLine] = append(lines[t.startLine], t)
		}
		if t.significant() && t.end >= lastEnd[t.endLine] {
			lastEnd[t.endLine] = t.end
			lastType[t.endLine] = t.Type
		}
	}

	var result []colonBlock
	var visit func(n *ast.Node, inSwitch bool)
	visit = func(n *ast.Node, inSwitch bool) {
		if n == nil || n.File != filename {
			return
		}
		if n.Type == ast.Block && !inSwitch {
			if b, ok := colonCandidate(n, filename, lines, lastEnd, lastType); ok {
				result = append(result, b)
			}
		}
		for _, child := range n.Blocks {
			visit(child, n.Type == ast.Switch)
		}
	}
	visit(tree, false)
	return result
}

// colonCandidate は、ブロックnがコロン記法に書き換えられるかどうかを調べる。
func colonCandidate(n *ast.Node, filename string, lines map[int][]srcToken,
	lastEnd map[int]int, lastType map[int]lexer.TokenType) (colonBlock, bool) {
	end := n.End
	if end == nil || end.File != filename || end.Length == 0 || end.Line <= n.Line {
		return colonBlock{}, false
	}
	// 本文は開始行の改行から始まり、少なくとも1つの文を持つ。
	if len(n.Blocks) == 0 || n.Blocks[0] == nil || n.Blocks[0].Type != ast.EOL || n.Blocks[0].Line != n.Line {
		return colonBlock{}, false
	}
	hasStmt := false
	for _, stmt := range n.Blocks {
		if stmt != nil && stmt.Type != ast.EOL && stmt.File == filename {
			hasStmt = true
			break
		}
	}
	if !hasStmt {
		return colonBlock{}, false
	}
	// 開始行が既にコロンで終わっていれば、それはコロン記法の本文である
	// (その終端は合成されたものなので、ここまでには来ないはずだが念のため)。
	colonAt, ok := lastEnd[n.Line]
	if !ok || lastType[n.Line] == ":" {
		return colonBlock{}, false
	}
	// 終端の行を調べる。
	endTokens := lines[end.Line]
	if len(endTokens) == 0 {
		return colonBlock{}, false
	}
	first := endTokens[0]
	switch {
	case first.Type == lexer.TypeKokomade:
		if len(endTokens) != 1 || first.Josi != "" {
			return colonBlock{}, false // 『ここまで』の行に他の何かがある
		}
		return colonBlock{headerLine: n.Line, colonAt: colonAt, endLine: end.Line, removeEnd: true}, true
	case first.Type == lexer.TypeChigaeba,
		first.Type == lexer.TypeWord && first.StringValue() == "エラー" && first.Josi == "ならば":
		return colonBlock{headerLine: n.Line, colonAt: colonAt, endLine: end.Line}, true
	}
	return colonBlock{}, false
}

// toColon は、blocksに挙げたブロックをコロン記法に書き換えた結果を返す。
func toColon(code string, blocks []colonBlock) string {
	var edits []textEdit
	removed := map[int]bool{}
	for _, b := range blocks {
		edits = append(edits, textEdit{b.colonAt, b.colonAt, ":"})
		if b.removeEnd {
			removed[b.endLine] = true
		}
	}
	if len(removed) > 0 {
		// 行ごと(改行を含めて)消す。
		runes := []rune(code)
		starts := originalLineStarts(runes)
		for l := range removed {
			if l >= len(starts) {
				continue
			}
			end := len(runes)
			if l+1 < len(starts) {
				end = starts[l+1]
			}
			edits = append(edits, textEdit{starts[l], end, ""})
		}
	}
	return applyEdits(code, edits)
}
