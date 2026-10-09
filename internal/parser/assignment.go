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
	// 括弧付きの配列要素への代入。式の中の比較は従来どおり yCalc で読む。(本家 #2583)
	if p.check("(") && p.hasParenthesizedAssignment() {
		v := p.yValue()
		var target *ast.Node
		if v != nil {
			target = normalizeArrayAssignmentTarget(v)
		}
		if target != nil && target.Type == ast.RefArray && p.check("eq") {
			p.get() // = / は
			value := p.yCalc()
			if value == nil {
				p.failAt("配列への代入文で値がありません。", m)
			}
			target = p.getAssignmentVarName(target)
			if p.check(lexer.TypeComma) {
				p.get()
			}
			end := p.peekSourceMap(nil)
			return &ast.Node{
				Type: ast.LetArray, Name: target.Name,
				Blocks:    append([]*ast.Node{value}, target.Index...),
				Index:     target.Index,
				CheckInit: p.flagCheckArrayInit,
				SourceMap: m, End: &end,
			}
		}
		p.failAt("括弧付きの代入先は、変数を起点とする配列要素で指定してください。", m)
	}
	if p.check2([][]lexer.TokenType{{lexer.TypeWord}, {"eq"}}) {
		wordTok := p.get()
		p.get() // eq
		// 右辺の解析でエラーが起きたら、どの変数への代入かを添えて報告し直す。
		// 元のエラー文面をそのまま連ねるので、2行の文面になる。
		var value *ast.Node
		func() {
			defer func() {
				if r := recover(); r != nil {
					se, ok := r.(syntaxError)
					if !ok {
						panic(r)
					}
					p.failAt(nodeToStr(wordTok, 1, "")+
						"への代入文で計算式に以下の書き間違いがあります。\n"+se.err.Error(), m)
				}
			}()
			valueStart := p.index
			stackStart := append([]*ast.Node(nil), p.stack...)
			recentStart := append([]*lexer.FuncItem(nil), p.recentlyCalledFunc...)
			value = p.yCalc()
			// 代入の右辺は計算式でなければ文としても解析する。本家と同様、
			// 計算式の規則が値を返さなかった場合に限ってカーソルを戻す。
			if value == nil {
				p.index = valueStart
				p.stack = stackStart
				p.recentlyCalledFunc = recentStart
				value = p.ySentence()
			}
		}()
		if value == nil || value.Type == ast.EOL {
			p.failAt(nodeToStr(wordTok, 1, "")+"への代入文で計算式に書き間違いがあります。", m)
		}
		if value.Type == ast.Func && value.Meta != nil && value.Meta.ReturnNone {
			p.failNode("関数『"+value.Name+"』は戻り値がないので結果を代入できません。", value)
		}
		if p.check(lexer.TypeComma) {
			p.get()
		}
		word := p.resolveAssignVarName(p.wordNode(wordTok), true)
		end := p.peekSourceMap(nil)
		return &ast.Node{Type: ast.Let, Name: word.StringValue(), Blocks: []*ast.Node{value}, SourceMap: m, End: &end}
	}

	// 『A,B,C=配列』: 配列の各要素を複数の変数へ順に代入する。
	// 通常のカンマ区切り文と区別するため、2個以上の単語の直後に = が
	// あるところまで先読みしてからトークンを消費する。
	if n := p.yLetVarList(m); n != nil {
		return n
	}

	if p.check2([][]lexer.TokenType{{lexer.TypeWord}, {"@"}}) ||
		p.check2([][]lexer.TokenType{{lexer.TypeWord}, {"["}}) ||
		p.check2([][]lexer.TokenType{{lexer.TypeWord}, {"$"}}) {
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

	// 『変数 名前=値』『定数の名前=値』および複数宣言。
	// 先頭の予約語が持つ助詞（『の』など）はトークン型に影響しないため、
	// 空白区切りの書式と同じ規則で扱える。
	if p.checkTypes([]lexer.TokenType{"変数", "定数"}) {
		return p.yPrefixedDeclaration(m)
	}
	return nil
}

// yPrefixedDeclaration は『変数 A=1』『変数のA=1』と、
// 『変数[A,B]=[1,2]』『定数[A,B]=[1,2]』を読む。
func (p *Parser) yPrefixedDeclaration(m ast.SourceMap) *ast.Node {
	saved := p.index
	vtype := p.get()
	if vtype == nil {
		return nil
	}
	isConst := vtype.Type == "定数"

	if p.check("[") {
		return p.yPrefixedVarList(m, vtype, isConst)
	}
	if !p.check(lexer.TypeWord) {
		return nil
	}
	wordTok := p.get()
	hasAttribute := p.check2([][]lexer.TokenType{{"{"}, {lexer.TypeWord}, {"}"}})
	isExport := p.readVarAttribute(p.isExportDefault, string(vtype.Type))
	hasValue := p.check("eq")
	if (isConst || hasAttribute) && !hasValue {
		p.index = saved
		return nil
	}
	name := p.createVar(wordTok, wordTok.StringValue(), isConst, isExport)
	value := p.yNop()
	if hasValue {
		p.get()
		value = p.yCalc()
		if value == nil || value.Type == ast.EOL {
			p.failToken("『"+wordTok.StringValue()+"』への代入文で右辺の値がありません。", wordTok)
		}
	}
	if p.check(lexer.TypeComma) {
		p.get()
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{
		Type: ast.DefLocalVar, Name: name, VarType: string(vtype.Type), IsExport: isExport,
		Blocks: []*ast.Node{value}, SourceMap: m, End: &end,
	}
}

func (p *Parser) yPrefixedVarList(m ast.SourceMap, vtype *lexer.Token, isConst bool) *ast.Node {
	p.get() // skip '['
	var words []*lexer.Token
	for !p.isEOF() && !p.check("]") {
		if !p.check(lexer.TypeWord) {
			label := "複数変数"
			if isConst {
				label = "複数定数"
			}
			p.failToken(label+"の代入文でエラー。『"+string(vtype.Type)+"[A,B,C]=[1,2,3]』の書式で記述してください。", vtype)
		}
		words = append(words, p.get())
		if !p.check(lexer.TypeComma) {
			break
		}
		p.get()
	}
	if !p.check("]") {
		p.failToken("複数変数の宣言が『]』で閉じられていません。", vtype)
	}
	p.get()
	if !p.check("eq") {
		p.failToken("『"+string(vtype.Type)+"[A,B,C]=[1,2,3]』の書式で記述してください。", vtype)
	}
	p.get()
	names := make([]*ast.Node, 0, len(words))
	for _, word := range words {
		name := p.createVar(word, word.StringValue(), isConst, p.isExportDefault)
		node := p.wordNode(word)
		node.Value = name
		names = append(names, node)
	}
	rhs := p.yCalc()
	if rhs == nil || rhs.Type == ast.EOL {
		p.failToken("複数変数への代入文で右辺の値がありません。", vtype)
	}

	end := p.peekSourceMap(nil)
	return &ast.Node{
		Type: ast.DefLocalVarList, Names: names, VarType: string(vtype.Type),
		Blocks: []*ast.Node{rhs}, SourceMap: m, End: &end,
	}
}

func (p *Parser) yLetVarList(m ast.SourceMap) *ast.Node {
	saved := p.index
	var words []*lexer.Token
	for {
		if !p.check(lexer.TypeWord) {
			p.index = saved
			return nil
		}
		words = append(words, p.get())
		if !p.check(lexer.TypeComma) {
			break
		}
		p.get()
	}
	if len(words) < 2 || !p.check("eq") {
		p.index = saved
		return nil
	}
	p.get()
	rhs := p.yCalc()
	if rhs == nil || rhs.Type == ast.EOL {
		p.failAt("複数変数への代入文で右辺の値がありません。", m)
	}
	names := make([]*ast.Node, 0, len(words))
	for _, word := range words {
		name := p.createVar(word, word.StringValue(), false, p.isExportDefault)
		node := p.wordNode(word)
		node.Value = name
		names = append(names, node)
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.DefLocalVarList, Names: names, VarType: "変数", Blocks: []*ast.Node{rhs}, SourceMap: m, End: &end}
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
	// プロパティ代入『A$キー=値』や連鎖『A@0$キー=値』を受理する。
	// indexes が空でも props があれば LetProp として受理する。
	var props []*ast.Node
	for p.check2([][]lexer.TokenType{{"$"}, {lexer.TypeWord, lexer.TypeString}}) {
		p.get()
		t := p.get()
		prop := p.wordNode(t)
		prop.Type = ast.String
		props = append(props, prop)
	}
	if len(indexes) == 0 && len(props) == 0 {
		return rollback()
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

// hasParenthesizedAssignment は括弧付き代入の候補かどうかをトークンだけで確認する。(本家 #2583)
// yValue での先読みは変数名や関数の使用記録も更新するため、解析後の巻き戻しはしない。
// 括弧内の比較は無視し、外側の助詞・演算子・文末で探索を終える。
func (p *Parser) hasParenthesizedAssignment() bool {
	var closings []lexer.TokenType
	for i := p.index; i < len(p.tokens); i++ {
		t := p.tokens[i]
		if len(closings) == 0 {
			if t.Type == "eq" {
				return true
			}
			// @直後の単項演算子は添字の一部。値の後ろの二項演算子とは区別する。
			unaryIndex := (t.Type == "-" || t.Type == lexer.TypeNot) && i > 0 &&
				(p.tokens[i-1].Type == "@" || p.tokens[i-1].Type == lexer.TypeNot)
			if !unaryIndex && !containsType([]lexer.TokenType{"(", "[", "{", "@", lexer.TypeWord, lexer.TypeFunc,
				"func_pointer", lexer.TypeNumber, lexer.TypeBigInt, lexer.TypeString}, t.Type) {
				return false
			}
		}
		switch t.Type {
		case "(":
			closings = append(closings, ")")
		case "[":
			closings = append(closings, "]")
		case "{":
			closings = append(closings, "}")
		case ")", "]", "}":
			if len(closings) == 0 || closings[len(closings)-1] != t.Type {
				return false
			}
			closings = closings[:len(closings)-1]
		}
		if len(closings) == 0 && t.Josi != "" {
			return false
		}
	}
	return false
}

// normalizeArrayAssignmentTarget は括弧付きの配列参照を、変数を起点とする
// 代入先（RefArray）へ変換する。(本家 #2583)
// 添字は内側から順に連結し、参照用のASTは書き換えない。
// 関数の戻り値やリテラルなど、変数を起点としない参照は nil を返す。
func normalizeArrayAssignmentTarget(n *ast.Node) *ast.Node {
	switch {
	case n == nil:
		return nil
	case n.Type == ast.Word:
		return n
	case n.Type == ast.RefArray:
		c := *n
		c.Index = append([]*ast.Node(nil), n.Index...)
		return &c
	case n.Type != ast.RefArrayValue || n.Name != "@" || len(n.Index) == 0:
		return nil
	}
	target := normalizeArrayAssignmentTarget(n.Index[0])
	if target == nil {
		return nil
	}
	r := &ast.Node{Type: ast.RefArray, Josi: n.Josi, RawJosi: n.RawJosi, SourceMap: n.SourceMap, End: n.End}
	if target.Type == ast.Word {
		r.Name = target.StringValue()
		r.NameToken = target.NameToken
	} else {
		r.Name = target.Name
		r.NameToken = target.NameToken
		r.Index = append(r.Index, target.Index...)
	}
	r.Index = append(r.Index, n.Index[1:]...)
	return r
}
