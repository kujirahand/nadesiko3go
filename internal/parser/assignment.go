package parser

import (
	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

// yLet reads assignment and the common local-variable declarations. More
// elaborate declaration forms are deliberately handled after ordinary and
// indexed assignment because those are the language foundation.
func (p *Parser) yLet() *ast.Node {
	m := p.peekSourceMap(nil)
	if p.check2([][]lexer.TokenType{{lexer.TypeWord}, {"eq"}}) {
		wordTok := p.get()
		p.get() // eq
		value := p.yCalc()
		if value == nil || value.Type == ast.EOL {
			p.failAt(nodeToStr(wordTok, 1, "")+"への代入文で計算式に書き間違いがあります。", m)
		}
		if p.check(lexer.TypeComma) {
			p.get()
		}
		word := p.resolveAssignVarName(p.wordNode(wordTok), true)
		end := p.peekSourceMap(nil)
		return &ast.Node{Type: ast.Let, Name: word.StringValue(), Blocks: []*ast.Node{value}, SourceMap: m, End: &end}
	}

	if p.check2([][]lexer.TokenType{{lexer.TypeWord}, {"@"}}) || p.check2([][]lexer.TokenType{{lexer.TypeWord}, {"["}}) {
		if n := p.yLetArrayChain(m); n != nil {
			if p.check(lexer.TypeComma) {
				p.get()
			}
			return n
		}
	}

	// 『名前とは変数/定数』
	if p.check2([][]lexer.TokenType{{lexer.TypeWord}, {"とは"}}) {
		wordTok := p.get()
		p.get()
		if !p.checkTypes([]lexer.TokenType{"変数", "定数"}) {
			p.failToken("ローカル変数『"+wordTok.StringValue()+"』の定義エラー", wordTok)
		}
		vtype := p.get()
		isExport := p.readVarAttribute(p.isExportDefault, "変数")
		name := p.createVar(wordTok, wordTok.StringValue(), vtype.Type == "定数", isExport)
		value := p.yNop()
		if p.check("eq") {
			p.get()
			if v := p.yCalc(); v != nil {
				value = v
			}
		}
		if p.check(lexer.TypeComma) {
			p.get()
		}
		end := p.peekSourceMap(nil)
		return &ast.Node{Type: ast.DefLocalVar, Name: name, VarType: string(vtype.Type), IsExport: isExport, Blocks: []*ast.Node{value}, SourceMap: m, End: &end}
	}
	return nil
}

func (p *Parser) yLetArrayChain(m ast.SourceMap) *ast.Node {
	saved := p.index
	rollback := func() *ast.Node { p.index = saved; return nil }
	if !p.check(lexer.TypeWord) {
		return nil
	}
	wordTok := p.get()
	var indexes []*ast.Node
	for {
		if p.check("@") {
			p.get()
			idx := p.yValueArrayIndex()
			if idx == nil {
				return rollback()
			}
			indexes = append(indexes, p.checkArrayIndex(idx))
			p.checkRefArrayComma(idx)
			continue
		}
		if p.check("[") {
			p.get()
			var group []*ast.Node
			for {
				idx := p.yCalc()
				if idx == nil {
					return rollback()
				}
				group = append(group, p.checkArrayIndex(idx))
				if !p.check(lexer.TypeComma) {
					break
				}
				p.get()
			}
			if !p.check("]") {
				return rollback()
			}
			p.get()
			indexes = append(indexes, p.checkArrayReverse(group)...)
			continue
		}
		break
	}
	if len(indexes) == 0 {
		return rollback()
	}
	var props []*ast.Node
	for p.check2([][]lexer.TokenType{{"$"}, {lexer.TypeWord, lexer.TypeString}}) {
		p.get()
		t := p.get()
		prop := p.wordNode(t)
		prop.Type = ast.String
		props = append(props, prop)
	}
	if !p.check("eq") {
		return rollback()
	}
	p.get()
	value := p.yCalc()
	if value == nil {
		return rollback()
	}
	word := p.resolveAssignVarName(p.wordNode(wordTok), false)
	end := p.peekSourceMap(nil)
	if len(props) > 0 {
		return &ast.Node{Type: ast.LetProp, Name: word.StringValue(), Blocks: append([]*ast.Node{value}, indexes...), Index: props, SourceMap: m, End: &end}
	}
	return &ast.Node{Type: ast.LetArray, Name: word.StringValue(), Blocks: append([]*ast.Node{value}, indexes...), Index: indexes, CheckInit: p.flagCheckArrayInit, SourceMap: m, End: &end}
}

func (p *Parser) yTryExcept() *ast.Node {
	if !p.check("エラー監視") {
		return nil
	}
	m := p.peekSourceMap(nil)
	watch := p.get()
	block := p.yBlock()
	if !p.check2([][]lexer.TokenType{{"エラー"}, {"ならば"}}) {
		p.failToken("エラー構文で『エラーならば』がありません。『エラー監視..エラーならば..ここまで』を対で記述します。", watch)
	}
	p.get()
	p.get()
	errBlock := p.yBlock()
	if !p.check("ここまで") {
		p.failAt("『ここまで』がありません。『エラー監視』...『エラーならば』...『ここまで』を対応させてください。", m)
	}
	p.get()
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.TryExcept, Blocks: []*ast.Node{block, errBlock}, SourceMap: m, End: &end}
}
