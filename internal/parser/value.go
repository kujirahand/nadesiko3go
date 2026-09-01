package parser

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

// tokenToArgNode converts a function argument token to the small AST node kept
// in DefFunc.Args. Function arguments are names even when preDefineFunc has
// classified one as a function token.
func (p *Parser) tokenToArgNode(t *lexer.Token) *ast.Node {
	if t == nil {
		return nil
	}
	m := p.peekSourceMap(t)
	return &ast.Node{Type: ast.Word, Value: t.Value, Josi: t.Josi, RawJosi: t.RawJosi, SourceMap: m, NameToken: t}
}

// yCalc reads one expression. A calculation may contain a nadesiko-style
// function call, so it temporarily lets yCall consume the value stack.
func (p *Parser) yCalc() *ast.Node {
	old := p.flagNoPostfixIndex
	p.flagNoPostfixIndex = false
	defer func() { p.flagNoPostfixIndex = old }()
	return p.yCalcMain()
}

func (p *Parser) yCalcMain() *ast.Node {
	m := p.peekSourceMap(nil)
	if p.check(lexer.TypeEOL) {
		return nil
	}
	t := p.yGetArg()
	if t == nil {
		return nil
	}
	if t.Josi == "" && !p.canNextFuncTakeNoJosiArg() {
		return t
	}
	oldReading := p.isReadingCalc
	p.isReadingCalc = true
	p.pushStack(t)
	t1 := p.yCall()
	p.isReadingCalc = oldReading
	if t1 == nil {
		return p.popStack(nil)
	}
	fCalc := t1
	if containsString(RenbunJosi, t1.Josi) {
		if p.check("戻る") {
			p.failAt("式の中で『戻す(戻る)』文は使えません。『(値)を(変数)に代入』などで一度値を受け取ってから『(変数)を戻す』と書いてください。", m)
		}
		if t2 := p.yCalc(); t2 != nil {
			end := p.peekSourceMap(nil)
			fCalc = &ast.Node{Type: ast.Renbun, Operator: "renbun", Blocks: []*ast.Node{t1, t2}, Josi: t2.Josi, SourceMap: m, End: &end}
		}
	}
	if op := p.peek(0); op != nil {
		if _, ok := opPriority[string(op.Type)]; ok {
			return p.yGetArgOperator(fCalc)
		}
	}
	return fCalc
}

func (p *Parser) canNextFuncTakeNoJosiArg() bool {
	if !p.check(lexer.TypeFunc) {
		return false
	}
	t := p.peek(0)
	f := p.metaOf(t)
	if f == nil {
		return false
	}
	for _, js := range f.Josi {
		if containsString(js, "") {
			return true
		}
	}
	return false
}

func (p *Parser) yValueKakko() *ast.Node {
	if !p.check("(") {
		return nil
	}
	open := p.get()
	p.saveStack()
	v := p.yCalc()
	if v == nil {
		v = p.ySentence()
	}
	if v == nil {
		near := p.get()
		p.failToken("(...)の解析エラー。"+nodeToStr(near, 1, "")+"の近く", open)
	}
	if !p.check(")") {
		p.failToken("(...)の解析エラー。"+nodeToStr(v, 1, "")+"の近く", open)
	}
	closeTok := p.get()
	p.loadStack()
	v.Josi = closeTok.Josi
	return p.yRefArrayValue(v)
}

func (p *Parser) yConst(t *lexer.Token, m ast.SourceMap) *ast.Node {
	if t == nil {
		return nil
	}
	return &ast.Node{Type: ast.NodeType(t.Type), Value: t.Value, Josi: t.Josi, RawJosi: t.RawJosi, SourceMap: m}
}

func (p *Parser) yValue() *ast.Node {
	m := p.peekSourceMap(nil)
	if p.check(lexer.TypeComma) { // #877
		p.get()
	}
	if p.checkTypes([]lexer.TokenType{lexer.TypeNumber, lexer.TypeBigInt, lexer.TypeString}) {
		return p.yConst(p.get(), m)
	}
	if p.check("(") {
		return p.yValueKakko()
	}
	if p.check2([][]lexer.TokenType{{"-"}, {lexer.TypeNumber, lexer.TypeWord, lexer.TypeFunc}}) {
		minus := p.get()
		v := p.yValue()
		josi := ""
		if v != nil {
			josi = v.Josi
		}
		left := &ast.Node{Type: ast.Number, Value: -1.0, SourceMap: p.peekSourceMap(minus)}
		if v == nil {
			v = p.yNop()
		}
		end := p.peekSourceMap(nil)
		return &ast.Node{Type: ast.Op, Operator: "*", Blocks: []*ast.Node{left, v}, Josi: josi, SourceMap: m, End: &end}
	}
	if p.check(lexer.TypeNot) {
		p.get()
		v := p.yValue()
		josi := ""
		if v != nil {
			josi = v.Josi
		}
		end := p.peekSourceMap(nil)
		return &ast.Node{Type: ast.Not, Operator: "not", Blocks: []*ast.Node{v}, Josi: josi, SourceMap: m, End: &end}
	}
	if a := p.yJSONArray(); a != nil {
		return a
	}
	if o := p.yJSONObject(); o != nil {
		return o
	}

	// A one-word function either ends the expression or carries a particle.
	if p.check(lexer.TypeFunc) {
		t := p.peek(0)
		nextIsSplit := false
		if next := p.peek(1); next != nil {
			split := append(append([]string{}, operatorList...), "eol", ")", "]", "ならば", "回", "間", "反復", "条件分岐")
			nextIsSplit = containsString(split, string(next.Type))
		}
		if nextIsSplit || t.Josi != "" {
			p.get()
			f := p.metaOf(t)
			if f == nil {
				p.failToken(fmt.Sprintf("一語関数『%s』は存在しません。", t.StringValue()), t)
			}
			var args []*ast.Node
			if len(f.Josi) == 1 {
				args = append(args, &ast.Node{Type: ast.Word, Value: "それ"})
			} else if len(f.Josi) >= 2 {
				p.failToken(fmt.Sprintf("関数『%s』で引数が指定されていません。%d個の引数を指定してください。", t.StringValue(), len(f.Josi)), t)
			}
			p.UsedFuncs[t.StringValue()] = true
			end := p.peekSourceMap(nil)
			return &ast.Node{Type: ast.Func, Name: t.StringValue(), Blocks: args, Josi: t.Josi, Meta: f, AsyncFn: f.AsyncFn, SourceMap: m, End: &end}
		}
	}

	// C-style call: FUNC(...) or a function value held in a variable.
	if p.check2([][]lexer.TokenType{{lexer.TypeFunc, lexer.TypeWord}, {"("}}) && p.peekDef().Josi == "" {
		nameTok := p.get()
		p.get() // '('
		// 名前空間を解決してから呼び出す。変数に入った関数を『F()』と
		// 呼ぶときも、変数名と同じ名前で引けるようにするため。
		nameNode := p.getVarNameRef(p.wordNode(nameTok))
		funcName := nameNode.StringValue()
		args := p.yGetArgParen(nameNode, funcName)
		if !p.check(")") {
			p.failToken("C風関数呼び出しのエラー", nameTok)
		}
		closeTok := p.get()
		f := p.metaOf(nameTok)
		if f != nil && len(f.Josi) != len(args) && !f.IsVariableJosi {
			p.failToken(fmt.Sprintf("関数『%s』で引数%d個が指定されましたが、%d個の引数を指定してください。", funcName, len(args), len(f.Josi)), nameTok)
		}
		p.UsedFuncs[funcName] = true
		end := p.peekSourceMap(nil)
		n := &ast.Node{Type: ast.Func, Name: funcName, Blocks: args, Josi: closeTok.Josi, Meta: f, AsyncFn: f != nil && f.AsyncFn, SourceMap: m, End: &end}
		return p.yApplyCallValue(n)
	}
	if p.check(lexer.TypeDefFunc) {
		return p.yMumeiFunc()
	}
	if w := p.yValueWord(); w != nil {
		return w
	}
	if p.check("func_pointer") {
		t := p.get()
		end := p.peekSourceMap(nil)
		return &ast.Node{Type: ast.FuncPointer, Name: t.StringValue(), Josi: t.Josi, SourceMap: m, End: &end}
	}
	return nil
}

func (p *Parser) wordNode(t *lexer.Token) *ast.Node {
	if t == nil {
		return nil
	}
	m := p.peekSourceMap(t)
	return &ast.Node{Type: ast.Word, Value: t.Value, Josi: t.Josi, RawJosi: t.RawJosi, NameToken: t, SourceMap: m}
}

func (p *Parser) yValueArrayIndex() *ast.Node {
	old := p.flagNoPostfixIndex
	p.flagNoPostfixIndex = true
	defer func() { p.flagNoPostfixIndex = old }()
	return p.yValue()
}

func (p *Parser) isInsideGroup() bool {
	depth := 0
	for i := 0; i < p.index && i < len(p.tokens); i++ {
		switch p.tokens[i].Type {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		}
	}
	return depth > 0
}

func (p *Parser) checkRefArrayComma(n *ast.Node) {
	if n.Josi == "" && p.check(lexer.TypeComma) && !p.isInsideGroup() {
		p.failNode("配列アクセス『@』でカンマ区切りの多次元指定は使えません。『A[1,2]』または『A@1@2』のように書いてください。", n)
	}
}

func (p *Parser) checkArrayIndex(n *ast.Node) *ast.Node {
	if p.arrayIndexFrom == 0 {
		return n
	}
	minus := &ast.Node{Type: ast.Number, Value: float64(p.arrayIndexFrom), SourceMap: n.SourceMap}
	return &ast.Node{Type: ast.Op, Operator: "-", Blocks: []*ast.Node{n, minus}, Josi: n.Josi, SourceMap: n.SourceMap}
}

func (p *Parser) checkArrayReverse(a []*ast.Node) []*ast.Node {
	if p.flagReverseArrayIndex && len(a) > 1 {
		slices.Reverse(a)
	}
	return a
}

func (p *Parser) yValueWordGetIndex(n *ast.Node) bool {
	if p.check("@") {
		p.get()
		idx := p.yValueArrayIndex()
		if idx == nil {
			p.failNode("変数の後ろの『@要素』の指定が不正です。", n)
		}
		n.Index = append(n.Index, p.checkArrayIndex(idx))
		n.Josi = idx.Josi
		p.checkRefArrayComma(n)
		for n.Josi == "" && p.check2([][]lexer.TokenType{{"$"}, {lexer.TypeWord, lexer.TypeString}}) {
			if !p.yValueWordGetProp(n) {
				break
			}
		}
		return true
	}
	if !p.check("[") {
		return false
	}
	saved := p.index
	p.get()
	var indexes []*ast.Node
	for {
		idx := p.yCalc()
		if idx == nil {
			p.index = saved
			return false
		}
		indexes = append(indexes, p.checkArrayIndex(idx))
		if !p.check(lexer.TypeComma) {
			break
		}
		p.get()
	}
	if !p.check("]") {
		p.index = saved
		return false
	}
	closeTok := p.get()
	n.Index = append(n.Index, p.checkArrayReverse(indexes)...)
	n.Josi = closeTok.Josi
	for n.Josi == "" && p.check2([][]lexer.TokenType{{"$"}, {lexer.TypeWord, lexer.TypeString}}) {
		if !p.yValueWordGetProp(n) {
			break
		}
	}
	return true
}

func (p *Parser) yValueWordGetProp(n *ast.Node) bool {
	if !p.check("$") {
		return false
	}
	saved := p.index
	p.get()
	var prop *ast.Node
	if p.checkTypes([]lexer.TokenType{lexer.TypeWord, lexer.TypeString}) {
		t := p.get()
		prop = p.wordNode(t)
		prop.Type = ast.String
	} else {
		prop = p.yValue()
	}
	if prop == nil {
		p.index = saved
		return false
	}
	n.Index = append(n.Index, prop)
	n.Josi = prop.Josi
	return n.Josi == ""
}

func (p *Parser) yValueWord() *ast.Node {
	if !p.check(lexer.TypeWord) {
		return nil
	}
	t := p.get()
	w := p.wordNode(t)
	p.getVarNameRef(w)
	if p.flagNoPostfixIndex {
		return w
	}
	if (w.Josi == "" && p.checkTypes([]lexer.TokenType{"[", "@"})) || (w.Josi != "" && p.check("@")) {
		end := p.peekSourceMap(nil)
		r := &ast.Node{Type: ast.RefArray, Name: w.StringValue(), NameToken: w.NameToken, Index: []*ast.Node{}, SourceMap: w.SourceMap, End: &end}
		for !p.isEOF() && p.yValueWordGetIndex(r) {
		}
		if len(r.Index) == 0 {
			p.failNode(fmt.Sprintf("配列『%s』アクセスで指定ミス", w.StringValue()), w)
		}
		return p.yRefArrayValue(r)
	}
	if p.check2([][]lexer.TokenType{{"$"}, {lexer.TypeWord, lexer.TypeString}}) {
		end := p.peekSourceMap(nil)
		r := &ast.Node{Type: ast.RefProp, Name: w.StringValue(), NameToken: w.NameToken, SourceMap: w.SourceMap, End: &end}
		for p.check("$") {
			if !p.yValueWordGetProp(r) {
				break
			}
		}
		return r
	}
	return w
}

func (p *Parser) yRefArrayValue(value *ast.Node) *ast.Node {
	if p.flagNoPostfixIndex {
		return value
	}
	val := value
	for {
		if val.Josi == "" && p.checkTypes([]lexer.TokenType{"@", "["}) {
			start := p.index
			r := &ast.Node{Type: ast.RefArrayValue, Name: "@", Index: []*ast.Node{val}, SourceMap: val.SourceMap}
			for !p.isEOF() && p.yValueWordGetIndex(r) {
			}
			if p.index == start {
				p.failNode("配列の直後にある『[...]』を配列アクセスとして解析できません。配列の要素を区切る『,』(カンマ)を忘れていませんか。", val)
			}
			val = r
			continue
		}
		if p.check("$") {
			start := p.index
			r := &ast.Node{Type: ast.RefArrayValue, Name: "$", Index: []*ast.Node{val}, SourceMap: val.SourceMap}
			for !p.isEOF() && p.yValueWordGetProp(r) {
			}
			if p.index == start {
				p.failNode("配列の直後にある『$プロパティ』の指定が不正です。", val)
			}
			val = r
			continue
		}
		return val
	}
}

func (p *Parser) yJSONArray() *ast.Node {
	if !p.check("[") {
		return nil
	}
	m := p.peekSourceMap(nil)
	p.get()
	var values []*ast.Node
	for !p.isEOF() {
		for p.check(lexer.TypeEOL) {
			p.get()
		}
		if p.check("]") {
			break
		}
		v := p.yCalc()
		if v == nil {
			break
		}
		values = append(values, v)
		if p.check(lexer.TypeComma) {
			p.get()
		}
	}
	for p.check(lexer.TypeEOL) {
		p.get()
	}
	if !p.check("]") {
		p.failAt("配列変数の初期化が『]』で閉じられていません。", m)
	}
	closeTok := p.get()
	end := p.peekSourceMap(nil)
	return p.yRefArrayValue(&ast.Node{Type: ast.JSONArray, Blocks: values, Josi: closeTok.Josi, SourceMap: m, End: &end})
}

func (p *Parser) yJSONObject() *ast.Node {
	if !p.check("{") {
		return nil
	}
	m := p.peekSourceMap(nil)
	p.get()
	var values []*ast.Node
	for !p.isEOF() {
		for p.check(lexer.TypeEOL) {
			p.get()
		}
		if p.check("}") {
			break
		}
		if !p.checkTypes([]lexer.TokenType{lexer.TypeWord, lexer.TypeString, lexer.TypeNumber}) {
			p.failAt("辞書オブジェクトの宣言で末尾の『}』がありません。", m)
		}
		keyTok := p.get()
		key := p.yConst(keyTok, p.peekSourceMap(keyTok))
		if keyTok.Type == lexer.TypeWord {
			key.Type = ast.String
		}
		var v *ast.Node
		if p.check(":") {
			p.get()
			v = p.yCalc()
			if v == nil {
				p.failAt("辞書オブジェクトの宣言で値がありません。", m)
			}
		} else {
			copyKey := *key
			if keyTok.Type == lexer.TypeWord {
				copyKey.Type = ast.Word
			}
			v = &copyKey
		}
		values = append(values, key, v)
		if p.check(lexer.TypeComma) {
			p.get()
		}
	}
	if !p.check("}") {
		p.failAt("辞書型変数の初期化が『}』で閉じられていません。", m)
	}
	closeTok := p.get()
	end := p.peekSourceMap(nil)
	return p.yRefArrayValue(&ast.Node{Type: ast.JSONObj, Blocks: values, Josi: closeTok.Josi, SourceMap: m, End: &end})
}

// --- variable name resolution ---

func (p *Parser) createVar(t *lexer.Token, name string, isConst, isExport bool) string {
	kind := "var"
	if isConst {
		kind = "const"
	}
	if p.funcLevel == 0 {
		if !strings.Contains(name, "__") {
			name = p.ModName + "__" + name
		}
		p.FuncList[name] = &lexer.FuncItem{Name: name, Type: kind, IsExport: boolPtr(isExport)}
		if t != nil {
			t.Value = name
		}
		return name
	}
	p.localvars[name] = &VarInfo{Type: kind, Value: ""}
	if t != nil {
		t.Value = name
	}
	return name
}

func boolPtr(v bool) *bool { return &v }

func (p *Parser) resolveAssignVarName(n *ast.Node, separateFileScope bool) *ast.Node {
	if n == nil {
		return nil
	}
	name := n.StringValue()
	if n.NameToken != nil {
		if original, ok := p.originalVarNames[n.NameToken]; ok {
			name = original
		}
	}
	f := p.FindVar(name)
	if f == nil {
		name = p.createVar(n.NameToken, name, false, p.isExportDefault)
		n.Value = name
		return n
	}
	if separateFileScope && p.funcLevel == 0 && f.Scope == ScopeGlobal && f.Func != nil && f.Func.Type != "const" && !strings.Contains(name, "__") && f.Name != p.ModName+"__"+name {
		name = p.createVar(n.NameToken, name, false, p.isExportDefault)
		n.Value = name
		return n
	}
	if f.Scope == ScopeGlobal {
		n.Value = f.Name
		if n.NameToken != nil {
			n.NameToken.Value = f.Name
		}
	}
	return n
}

func (p *Parser) getVarNameRef(n *ast.Node) *ast.Node {
	if n == nil {
		return nil
	}
	name := n.StringValue()
	f := p.FindVar(name)
	if f == nil {
		if p.funcLevel == 0 && !strings.Contains(name, "__") {
			if n.NameToken != nil {
				p.originalVarNames[n.NameToken] = name
				n.NameToken.Value = p.ModName + "__" + name
			}
			n.Value = p.ModName + "__" + name
		}
	} else if f.Scope == ScopeGlobal {
		if n.NameToken != nil && name != f.Name {
			p.originalVarNames[n.NameToken] = name
			n.NameToken.Value = f.Name
		}
		n.Value = f.Name
	}
	return n
}

func (p *Parser) getAssignmentVarName(n *ast.Node) *ast.Node {
	if n == nil {
		return nil
	}
	if n.Type == ast.Word {
		return p.resolveAssignVarName(n, true)
	}
	if n.Type == ast.RefArray || n.Type == ast.RefProp {
		base := &ast.Node{Type: ast.Word, Value: n.Name, NameToken: n.NameToken, SourceMap: n.SourceMap}
		base = p.resolveAssignVarName(base, false)
		n.Name = base.StringValue()
	}
	return n
}
