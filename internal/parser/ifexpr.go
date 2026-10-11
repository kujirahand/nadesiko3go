package parser

import (
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

// --- 値を返すもし式 (本家 #2591 / nadesiko3 PR #2612) ---

// ifExprValueTypes は、分岐の末尾に置ける（値になる）ノードの種類。
var ifExprValueTypes = map[ast.NodeType]bool{
	ast.Number: true, ast.BigInt: true, ast.Bool: true, ast.Null: true, ast.String: true,
	ast.Word: true, ast.Variable: true, ast.Op: true, ast.Calc: true, ast.Not: true,
	ast.And: true, ast.Or: true, ast.Eq: true, ast.Func: true, ast.CalcFunc: true,
	ast.CallValue: true, ast.FuncPointer: true, ast.FuncObj: true, ast.JSONArray: true,
	ast.JSONObj: true, ast.RefArray: true, ast.RefProp: true, ast.RefArrayValue: true,
	ast.Renbun: true, ast.IfExpr: true,
}

// ifExprStatementTypes は、分岐の途中に置ける（値にならない）ノードの種類。
var ifExprStatementTypes = map[ast.NodeType]bool{
	ast.Let: true, ast.LetArray: true, ast.LetProp: true, ast.DefLocalVar: true,
	ast.DefLocalVarList: true, ast.Inc: true,
}

func (p *Parser) validateIfExprStatement(n *ast.Node) {
	if !ifExprValueTypes[n.Type] && !ifExprStatementTypes[n.Type] {
		p.failNode("「もし」式の分岐には代入・関数呼び出し・値の式を書いてください。", n)
	}
}

// validateIfExprBranch は分岐の各文と、末尾が値を返す式であることを検証する。
func (p *Parser) validateIfExprBranch(block *ast.Node) {
	for _, n := range block.Blocks {
		p.validateIfExprStatement(n)
	}
	var last *ast.Node
	if len(block.Blocks) > 0 {
		last = block.Blocks[len(block.Blocks)-1]
	}
	ok := last != nil && ifExprValueTypes[last.Type]
	if ok && (last.Type == ast.Func || last.Type == ast.CalcFunc) {
		if meta := p.FuncList[last.Name]; meta != nil && meta.ReturnNone {
			ok = false
		}
	}
	if !ok {
		if last == nil {
			last = block
		}
		p.failNode("「もし」式の分岐の末尾には値を返す式が必要です。", last)
	}
}

// yIFExpression は値として使う条件分岐を読む。各分岐の末尾の式が結果になる。
func (p *Parser) yIFExpression() *ast.Node {
	m := p.peekSourceMap(nil)
	startIndex := p.index
	outerReadingCalc := p.isReadingCalc
	outerStackList := append([][]*ast.Node(nil), p.stackList...)
	outerStack := p.stack
	isNested := p.ifExprDepth > 0
	p.ifExprDepth++
	p.stack = nil
	defer func() {
		p.stack = outerStack
		p.stackList = outerStackList
		p.isReadingCalc = outerReadingCalc
		p.ifExprDepth--
	}()
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		// 解析失敗は再送出し、別構文の試行には使わない。
		p.index = startIndex
		if se, ok := r.(syntaxError); ok && isNested && !strings.Contains(se.err.Msg, "入れ子") {
			line, file := se.err.Line, se.err.File
			if line == 0 {
				line = m.Line
			}
			if file == "" {
				file = m.File
			}
			panic(syntaxError{&errs.NakoError{
				Kind: errs.Syntax, File: file, Line: line,
				Msg: "入れ子の条件分岐は、両分岐で値を返す「もし」式で書いてください。\n" + se.err.Msg,
			}})
		}
		panic(r)
	}()

	p.get() // skip もし
	p.skipComma()
	cond := p.yIFCond()
	p.skipComma()
	trueMultiline := p.check(lexer.TypeEOL)
	trueBlock := p.yIFExpressionBranch(trueMultiline)
	for p.checkTypes([]lexer.TokenType{lexer.TypeComma, lexer.TypeEOL}) {
		p.get()
	}
	if !p.check("違えば") {
		msg := "「もし」式には「違えば」が必要です。"
		if isNested {
			msg = "入れ子の条件分岐は、両分岐で値を返す「もし」式で書いてください。「違えば」が必要です。"
		}
		p.failAt(msg, m)
	}
	p.get()
	p.skipComma()
	falseMultiline := p.check(lexer.TypeEOL)
	falseBlock := p.yIFExpressionBranch(falseMultiline)
	if trueMultiline || falseMultiline {
		for p.check(lexer.TypeEOL) {
			p.get()
		}
		if !p.check("ここまで") {
			p.failAt("「もし」式で「ここまで」がありません。", m)
		}
		p.get()
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.IfExpr, Blocks: []*ast.Node{cond, trueBlock, falseBlock}, SourceMap: m, End: &end}
}

func (p *Parser) skipComma() {
	for p.check(lexer.TypeComma) {
		p.get()
	}
}

func (p *Parser) yIFExpressionBranch(multiline bool) *ast.Node {
	m := p.peekSourceMap(nil)
	var blocks []*ast.Node
	for !p.isEOF() {
		if multiline {
			for p.check(lexer.TypeEOL) {
				p.get()
			}
		}
		if p.checkTypes([]lexer.TokenType{"違えば", "ここまで"}) {
			break
		}
		value := p.yLet()
		if value == nil {
			value = p.yCalc()
		}
		if value == nil && p.check(lexer.TypeFunc) {
			value = p.yCall()
		}
		if value == nil {
			p.failAt("「もし」式の分岐には代入・関数呼び出し・値の式を書いてください。", p.peekSourceMap(nil))
		}
		p.validateIfExprStatement(value)
		if len(p.stack) > 0 {
			p.failNode(p.makeStackBalanceReport(), value)
		}
		blocks = append(blocks, value)
		if !multiline {
			break
		}
		if !p.check(lexer.TypeEOL) {
			p.failAt("「もし」式の分岐では文を改行で区切ってください。", p.peekSourceMap(nil))
		}
	}
	end := p.peekSourceMap(nil)
	block := &ast.Node{Type: ast.Block, Blocks: blocks, SourceMap: m, End: &end}
	p.validateIfExprBranch(block)
	return block
}
