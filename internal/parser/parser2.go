package parser

import (
	"fmt"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

// --- 繰り返しと条件分岐の各構文 ---

// yRepeatTime reads 『N回…ここまで』.
func (p *Parser) yRepeatTime() *ast.Node {
	m := p.peekSourceMap(nil)
	if !p.check("回") {
		return nil
	}
	p.get() // skip '回'
	if p.check(lexer.TypeComma) {
		p.get()
	}
	if p.check("繰返") { // 『N回、繰り返す』 (#924)
		p.get()
	}
	num := p.popStack([]string{})
	if num == nil {
		end := p.peekSourceMap(nil)
		num = &ast.Node{Type: ast.Word, Value: "それ", SourceMap: m, End: &end}
	}
	block := p.yNop()
	if p.check(lexer.TypeComma) {
		p.get()
	}
	multiline := false
	if p.check("ここから") {
		p.get()
		multiline = true
	} else if p.check(lexer.TypeEOL) {
		multiline = true
	}
	if multiline {
		block = p.yBlock()
		if p.check("ここまで") {
			p.get()
		} else {
			p.failAt("『ここまで』がありません。『回』...『ここまで』を対応させてください。", m)
		}
	} else if b := p.ySentence(); b != nil {
		block = b
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.RepeatTimes, Blocks: []*ast.Node{num, block}, SourceMap: m, End: &end}
}

// yWhile reads 『(条件)の間…ここまで』.
func (p *Parser) yWhile() *ast.Node {
	m := p.peekSourceMap(nil)
	if !p.check("間") {
		return nil
	}
	p.get() // skip '間'
	for p.check(lexer.TypeComma) {
		p.get()
	}
	if p.check("繰返") { // #927
		p.get()
	}
	expr := p.popStack(nil)
	if expr == nil {
		p.failAt("『間』で条件がありません。", m)
	}
	if p.check(lexer.TypeComma) {
		p.get()
	}
	if !p.checkTypes([]lexer.TokenType{"ここから", lexer.TypeEOL}) {
		p.failAt("『間』の直後は改行が必要です", m)
	}
	block := p.yBlock()
	if p.check("ここまで") {
		p.get()
	} else {
		p.failAt("『ここまで』がありません。『間』...『ここまで』を対応させてください。", m)
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.While, Blocks: []*ast.Node{expr, block}, SourceMap: m, End: &end}
}

// yAtohantei reads the loop that tests its condition after the body (#1147).
func (p *Parser) yAtohantei() *ast.Node {
	m := p.peekSourceMap(nil)
	if p.check("後判定") {
		p.get()
	}
	if p.check("繰返") {
		p.get()
	}
	if p.check("ここから") {
		p.get()
	}
	block := p.yBlock()
	if p.check("ここまで") {
		p.get()
	}
	if p.check(lexer.TypeComma) {
		p.get()
	}
	cond := p.yGetArg()
	bUntil := false
	if t := p.peek(0); t != nil && literalToStr(t.Value) == "なる" && (t.Josi == "まで" || t.Josi == "までの") {
		p.get() // skip なるまで
		bUntil = true
	}
	if p.check("間") {
		p.get()
	}
	end := p.peekSourceMap(nil)
	if bUntil { // 条件を反転する
		cond = &ast.Node{Type: ast.Not, Operator: "not", Blocks: []*ast.Node{cond}, SourceMap: m, End: &end}
	}
	if cond == nil {
		cond = &ast.Node{Type: ast.Number, Value: 1.0, SourceMap: m, End: &end}
	}
	return &ast.Node{Type: ast.Atohantei, Blocks: []*ast.Node{cond, block}, SourceMap: m, End: &end}
}

// yFor reads 『AからBまで繰り返す』 and its counting variants.
func (p *Parser) yFor() *ast.Node {
	const errorForArguments = "『繰り返す』文でAからBまでの指定がありません。"
	flagDown := true // AからBまでの時、A>=Bを許容するかどうか
	flagUp := true   // AからBまでの時、A<=Bを許容するかどうか
	loopDirection := ""
	m := p.peekSourceMap(nil)
	if !p.check("繰返") && !p.check("増繰返") && !p.check("減繰返") {
		return nil
	}
	kurikaesu := p.get() // skip 繰り返す
	kurikaesuType := kurikaesu.Type

	// スタックに(増や|減ら)してがある？
	if n := len(p.stack); n > 0 {
		incdec := p.stack[n-1]
		if incdec.Type == ast.Word && (incdec.StringValue() == "増" || incdec.StringValue() == "減") {
			p.stack = p.stack[:n-1]
			if incdec.StringValue() == "増" {
				flagDown = false
				kurikaesuType = "増繰返"
			} else {
				flagUp = false
				kurikaesuType = "減繰返"
			}
		}
	}

	vInc := p.yNop()
	if kurikaesuType == "増繰返" || kurikaesuType == "減繰返" {
		if v := p.popStack([]string{"ずつ"}); v != nil {
			vInc = v
		}
		if kurikaesuType == "増繰返" {
			flagDown = false
			loopDirection = "up"
		} else {
			flagUp = false
			loopDirection = "down"
		}
	}
	vTo := p.popStack([]string{"まで", "を"}) // 範囲オブジェクトの場合もある
	vFrom := p.popStack([]string{"から"})
	if vFrom == nil {
		vFrom = p.yNop()
	}
	vWord := p.popStack([]string{"を", "で"})
	wordStr := ""
	if vWord != nil {
		if vWord.Type != ast.Word {
			p.failNode("『(変数名)をAからBまで繰り返す』で指定してください。", vWord)
		}
		wordStr = vWord.StringValue()
	}
	if vTo == nil {
		// 『AからBの範囲を繰り返す』構文のとき (#1704)
		p.failToken(errorForArguments, kurikaesu)
	}

	if p.check(lexer.TypeComma) {
		p.get()
	}
	multiline := false
	if p.check("ここから") {
		multiline = true
		p.get()
	} else if p.check(lexer.TypeEOL) {
		multiline = true
		p.get()
	}
	block := p.yNop()
	if multiline {
		block = p.yBlock()
		if p.check("ここまで") {
			p.get()
		} else {
			p.failAt("『ここまで』がありません。『繰り返す』...『ここまで』を対応させてください。", m)
		}
	} else if b := p.ySentence(); b != nil {
		block = b
	}

	end := p.peekSourceMap(nil)
	return &ast.Node{
		Type: ast.For, Blocks: []*ast.Node{vFrom, vTo, vInc, block},
		FlagDown: flagDown, FlagUp: flagUp, LoopDirection: loopDirection, Word: wordStr,
		SourceMap: m, End: &end,
	}
}

func (p *Parser) yReturn() *ast.Node {
	m := p.peekSourceMap(nil)
	if !p.check("戻る") {
		return nil
	}
	p.get() // skip '戻る'
	v := p.popStack([]string{"で", "を"})
	if v == nil {
		v = p.yNop()
	}
	if len(p.stack) > 0 {
		p.failAt("『戻』文の直前に未解決の引数があります。『(式)を戻す』のように式をカッコで括ってください。", m)
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.Return, Blocks: []*ast.Node{v}, SourceMap: m, End: &end}
}

// yForEach reads 『(配列)を反復』.
func (p *Parser) yForEach() *ast.Node {
	m := p.peekSourceMap(nil)
	if !p.check("反復") {
		return nil
	}
	p.get() // skip '反復'
	for p.check(lexer.TypeComma) {
		p.get()
	}
	target := p.popStack([]string{"を"})
	if target == nil {
		target = p.yNop() // nop なら「それ」の値が使われる
	}
	name := p.popStack([]string{"で"})
	wordStr := ""
	if name != nil {
		if name.Type != ast.Word {
			p.failAt("『(変数名)で(配列)を反復』で指定してください。", m)
		}
		wordStr = name.StringValue()
	}
	block := p.yNop()
	multiline := false
	if p.check("ここから") {
		multiline = true
		p.get()
	} else if p.check(lexer.TypeEOL) {
		multiline = true
	}
	if multiline {
		block = p.yBlock()
		if p.check("ここまで") {
			p.get()
		} else {
			p.failAt("『ここまで』がありません。『反復』...『ここまで』を対応させてください。", m)
		}
	} else if b := p.ySentence(); b != nil {
		block = b
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{
		Type: ast.Foreach, Word: wordStr, Blocks: []*ast.Node{target, block},
		SourceMap: m, End: &end,
	}
}

// ySwitch reads 『(値)で条件分岐』.
//
// blocks[0] is the value, blocks[1] the 違えば block, then each case
// contributes its condition and body as a pair.
func (p *Parser) ySwitch() *ast.Node {
	m := p.peekSourceMap(nil)
	if !p.check("条件分岐") {
		return nil
	}
	joukenbunki := p.get() // skip '条件分岐'
	eol := p.get()
	if eol == nil {
		return nil
	}
	expr := p.popStack([]string{"で"})
	if expr == nil {
		p.failToken("『(値)で条件分岐』のように記述してください。", joukenbunki)
	}
	if eol.Type != lexer.TypeEOL {
		p.failToken("『条件分岐』の直後は改行してください。", joukenbunki)
	}

	blocks := []*ast.Node{expr, p.yNop()} // blocks[1] は後で 違えば に差し替える
	for !p.isEOF() {
		if p.check(lexer.TypeEOL) {
			p.get()
			continue
		}
		if p.check("ここまで") {
			p.get()
			break
		}
		if t := p.peek(0); t != nil && t.Type == "違えば" {
			p.get() // skip 違えば
			if p.check(lexer.TypeComma) {
				p.get()
			}
			defaultBlock := p.yBlock()
			if p.check("ここまで") {
				p.get() // 違えばとペアの ここまで
			}
			for p.check(lexer.TypeEOL) {
				p.get()
			}
			if p.check("ここまで") {
				p.get() // 条件分岐の ここまで
			}
			blocks[1] = defaultBlock
			break
		}
		cond := p.yValue()
		if cond == nil {
			p.failToken("『条件分岐』は『(条件)ならば〜ここまで』と記述してください。", joukenbunki)
		}
		naraba := p.get()
		if naraba == nil || naraba.Type != "ならば" {
			p.failToken("『条件分岐』で条件は＊＊ならばと記述してください。", joukenbunki)
		}
		if p.check(lexer.TypeComma) {
			p.get()
		}
		condBlock := p.yBlock()
		if t := p.peek(0); t != nil && t.Type == "ここまで" {
			p.get()
		}
		blocks = append(blocks, cond, condBlock)
	}

	end := p.peekSourceMap(nil)
	return &ast.Node{
		Type: ast.Switch, Blocks: blocks, CaseCount: len(blocks)/2 - 1,
		SourceMap: m, End: &end,
	}
}

// --- 無名関数 ---

func (p *Parser) yMumeiFunc() *ast.Node {
	m := p.peekSourceMap(nil)
	if !p.check(lexer.TypeDefFunc) {
		return nil
	}
	def := p.get()
	var args []*ast.Node
	if p.check(lexer.TypeComma) {
		p.get()
	}
	if p.check("(") { // 関数の引数定義は省略できる
		args = p.yDefFuncReadArgs()
	}
	if p.check(lexer.TypeComma) {
		p.get()
	}

	block := p.yNop()
	isAsyncFn := false
	p.funcLevel++
	p.saveStack()
	backupAsyncFn := p.usedAsyncFn
	p.usedAsyncFn = false
	backupLocalvars := p.localvars // #1746
	p.localvars = map[string]*VarInfo{"それ": {Type: "var", Value: ""}}
	for _, arg := range args {
		if arg == nil || arg.StringValue() == "" {
			continue
		}
		p.localvars[arg.StringValue()] = &VarInfo{Type: "var", Value: ""}
	}
	func() {
		defer func() {
			p.loadStack()
			p.usedAsyncFn = backupAsyncFn
			p.localvars = backupLocalvars
			p.funcLevel--
		}()
		block = p.yBlock()
		isAsyncFn = p.usedAsyncFn
		if !p.check("ここまで") { // #1045
			p.failAt("『ここまで』がありません。『には』構文か無名関数の末尾に『ここまで』が必要です。", m)
		}
		p.get() // skip ここまで
	}()

	end := p.peekSourceMap(nil)
	return &ast.Node{
		Type: ast.FuncObj, Name: "", Args: args, Blocks: []*ast.Node{block},
		Meta: p.metaOf(def), IsExport: false, AsyncFn: isAsyncFn,
		SourceMap: m, End: &end,
	}
}

// --- 代入文と増減 ---

// yDainyu reads 『(変数名)に(値)を代入』.
func (p *Parser) yDainyu() *ast.Node {
	m := p.peekSourceMap(nil)
	dainyu := p.get() // 代入
	if dainyu == nil {
		return nil
	}
	value := p.popStack([]string{"を"})
	if value == nil {
		value = &ast.Node{Type: ast.Word, Value: "それ", Josi: "を", SourceMap: m}
	}
	word := p.popStack([]string{"へ", "に"})
	if word == nil || (word.Type != ast.Word && word.Type != ast.Func && word.Type != ast.RefArray) {
		p.failToken("代入文で代入先の変数が見当たりません。『(変数名)に(値)を代入』のように使います。", dainyu)
	}
	if word.Type == ast.Func {
		p.failToken("関数『"+word.Name+"』に代入できません。『(変数名)に(値)を代入』のように使います。", dainyu)
	}
	target := p.getAssignmentVarName(word)
	end := p.peekSourceMap(nil)
	if target.Type == ast.RefArray { // 配列への代入
		return &ast.Node{
			Type: ast.LetArray, Name: target.Name,
			Blocks:    append([]*ast.Node{value}, target.Index...),
			Index:     target.Index,
			CheckInit: p.flagCheckArrayInit,
			SourceMap: m, End: &end,
		}
	}
	return &ast.Node{
		Type: ast.Let, Name: target.StringValue(), Blocks: []*ast.Node{value},
		SourceMap: m, End: &end,
	}
}

// ySadameru reads 『(定数名)を(値)に定める』.
func (p *Parser) ySadameru() *ast.Node {
	m := p.peekSourceMap(nil)
	sadameru := p.get() // 定める
	if sadameru == nil {
		return nil
	}
	end0 := p.peekSourceMap(nil)
	word := p.popStack([]string{"を"})
	if word == nil {
		word = &ast.Node{Type: ast.Word, Value: "それ", Josi: "を", SourceMap: m, End: &end0}
	}
	if word.Type != ast.Word && word.Type != ast.Func && word.Type != ast.RefArray {
		p.failToken("『定める』文で定数が見当たりません。『(定数名)を(値)に定める』のように使います。", sadameru)
	}
	value := p.popStack([]string{"へ", "に", "と"})
	if value == nil {
		value = p.yNop()
	}
	isExport := p.readVarAttribute(p.isExportDefault, "変数")
	name := p.createVar(word.NameToken, word.StringValue(), true, isExport)
	end := p.peekSourceMap(nil)
	return &ast.Node{
		Type: ast.DefLocalVar, Name: name, VarType: "定数", IsExport: isExport,
		Blocks: []*ast.Node{value}, SourceMap: m, End: &end,
	}
}

// readVarAttribute reads an optional 『{公開}』 attribute after a declaration.
func (p *Parser) readVarAttribute(def bool, label string) bool {
	if !p.check2([][]lexer.TokenType{{"{"}, {lexer.TypeWord}, {"}"}}) {
		return def
	}
	p.get() // skip '{'
	attrNode := p.get()
	p.get() // skip '}'
	switch attrNode.StringValue() {
	case "公開", "エクスポート":
		return true
	case "非公開":
		return false
	}
	p.Warnings = append(p.Warnings,
		fmt.Sprintf("不明な%s属性『%s』が指定されています。", label, attrNode.StringValue()))
	return def
}

// yIncDec reads 『(変数)を(値)だけ増やす』 and its 減らす counterpart.
func (p *Parser) yIncDec() *ast.Node {
	m := p.peekSourceMap(nil)
	action := p.get() // (増|減)
	if action == nil {
		return nil
	}

	// 『Nずつ増やして繰り返す』文か？
	if p.check("繰返") {
		end := p.peekSourceMap(nil)
		p.pushStack(&ast.Node{
			Type: ast.Word, Value: action.Value, Josi: action.Josi, SourceMap: m, End: &end,
		})
		return p.yFor()
	}

	word := p.popStack([]string{"を"})
	if word == nil || (word.Type != ast.Word && word.Type != ast.RefArray && word.Type != ast.RefProp) {
		p.failToken(fmt.Sprintf(
			"『%s』文で定数が見当たりません。『(変数名)を(値)だけ%s』のように使います。",
			action.Type, action.Type), action)
	}
	value := p.popStack([]string{"だけ", ""})
	if value == nil {
		end := p.peekSourceMap(nil)
		value = &ast.Node{Type: ast.Number, Value: 1.0, Josi: "だけ", SourceMap: m, End: &end}
	}
	if action.StringValue() == "減" { // 減らすなら-1をかける
		minusOne := &ast.Node{Type: ast.Number, Value: -1.0, SourceMap: ast.SourceMap{Line: action.Line}}
		value = &ast.Node{Type: ast.Op, Operator: "*", Blocks: []*ast.Node{value, minusOne}, SourceMap: m}
	}

	target := p.getAssignmentVarName(word)
	// 単なる変数なら名前は Value に、添字付きなら Name に入っている
	name := target.Name
	if name == "" {
		name = target.StringValue()
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{
		Type: ast.Inc, Name: name, NameToken: target.NameToken,
		Index: target.Index, Blocks: []*ast.Node{value}, Josi: action.Josi,
		SourceMap: m, End: &end,
	}
}

// --- 関数呼び出し ---

// yCall reads values onto the stack until a statement-level construct or a
// function call consumes them.
func (p *Parser) yCall() *ast.Node {
	if p.isEOF() {
		return nil
	}

	for !p.isEOF() {
		if p.check("ここから") {
			p.get()
		}
		switch {
		case p.check("代入"):
			return p.yDainyu()
		case p.check("定める"):
			return p.ySadameru()
		case p.check("回"):
			return p.yRepeatTime()
		case p.check("間"):
			return p.yWhile()
		case p.check("繰返"), p.check("増繰返"), p.check("減繰返"):
			return p.yFor()
		case p.check("反復"):
			return p.yForEach()
		case p.check("条件分岐"):
			return p.ySwitch()
		case p.check("戻る"):
			return p.yReturn()
		case p.check("増"), p.check("減"):
			return p.yIncDec()
		}

		// C言語風関数
		if p.check2([][]lexer.TokenType{{lexer.TypeFunc, lexer.TypeWord}, {"("}}) {
			if cur := p.peek(0); cur != nil && cur.Josi == "" {
				if t := p.yValue(); t != nil { // yValueでC言語風呼び出しをパースする
					if t.Type == ast.Func && (t.Josi == "" || containsString(RenbunJosi, t.Josi)) {
						t.Josi = ""
						return t // 関数なら値とする
					}
					p.pushStack(t)
				}
				if p.check(lexer.TypeComma) {
					p.get()
				}
				continue
			}
		}

		// なでしこ式関数
		if p.check(lexer.TypeFunc) {
			r := p.yCallFunc()
			if r == nil {
				continue
			}
			if p.check("間") { // 「〜する間」の形ならスタックに積む
				p.pushStack(r)
				continue
			}
			if !p.checkTypes(toTokenTypes(operatorList)) {
				return r // 関数呼び出しの後に演算子がないのでそのまま戻す
			}
			// 四則演算があった場合、計算してスタックに載せる
			p.pushStack(p.yGetArgOperator(r))
			continue
		}

		// 値のとき → スタックに載せる
		if t := p.yGetArg(); t != nil {
			p.pushStack(t)
			continue
		}
		break
	}

	// 助詞が余ってしまった場合
	if len(p.stack) > 0 {
		if p.isReadingCalc {
			return p.popStack(nil)
		}
		parts := make([]string, 0, len(p.stack))
		for _, n := range p.stack {
			parts = append(parts, nodeToStr(n, 0, ""))
		}
		msg := fmt.Sprintf("不完全な文です。%sが解決していません。", joinStrings(parts, "、"))
		// 各ノードについて、更に詳細な情報があるなら表示する
		for _, n := range p.stack {
			d0 := nodeToStr(n, 0, "")
			d1 := nodeToStr(n, 1, "")
			if d0 != d1 {
				msg += fmt.Sprintf("%sは%sとして使われています。", d0, d1)
			}
		}
		p.failNode(msg, p.stack[0])
	}
	return p.popStack([]string{})
}

// yCallFunc reads a nadesiko-style function call, taking its arguments off the
// stack by their particles.
func (p *Parser) yCallFunc() *ast.Node {
	m := p.peekSourceMap(nil)
	t := p.get()
	if t == nil {
		return nil
	}
	f := p.metaOf(t)
	funcName := t.StringValue()

	// (関数)には ... 構文 (#66)
	var funcObj *ast.Node
	if t.Josi == "には" {
		func() {
			defer func() {
				if r := recover(); r != nil {
					se, ok := r.(syntaxError)
					if !ok {
						panic(r)
					}
					p.failToken(fmt.Sprintf(
						"『%sには...』で無名関数の定義で以下の間違いがあります。\n%s", funcName, se.err.Error()), t)
				}
			}()
			funcObj = p.yMumeiFunc()
		}()
		if funcObj == nil {
			p.failToken("『Fには』構文がありましたが、関数定義が見当たりません。", t)
		}
	}
	if f == nil {
		p.failToken("関数の定義でエラー。", t)
	}

	// 最近使った関数を記録する(余剰エラーの報告に使う)
	recent := *f
	recent.Name = funcName
	p.recentlyCalledFunc = append(p.recentlyCalledFunc, &recent)

	if f.AsyncFn {
		p.usedAsyncFn = true
	}

	// 関数の引数を取り出す
	var args []*ast.Node
	nullCount, valueCount := 0, 0
	for i := len(f.Josi) - 1; i >= 0; i-- {
		for {
			// スタックから任意の助詞を持つ値を一つ取り出す。助詞がなければ末尾から得る。
			popArg := p.popStack(f.Josi[i])
			if popArg != nil {
				valueCount++
			} else if i < len(f.Josi)-1 || !f.IsVariableJosi {
				nullCount++
				popArg = funcObj
			} else {
				break
			}
			// 参照渡しの引数なら func_pointer に変える
			if popArg != nil && i < len(f.FuncPointers) && f.FuncPointers[i] != "" {
				if popArg.Type == ast.Func {
					popArg.Type = ast.FuncPointer
				} else {
					varname := fmt.Sprintf("%d番目の引数", i+1)
					if i < len(f.VarNames) {
						varname = f.VarNames[i]
					}
					p.failToken(fmt.Sprintf(
						"関数『%s』の引数『%s』には関数オブジェクトが必要です。", funcName, varname), t)
				}
			}
			// 引数がなければ変数「それ」で補完する
			if popArg == nil {
				popArg = &ast.Node{Type: ast.Word, Value: "それ", SourceMap: m, End: &m}
			}
			args = append([]*ast.Node{popArg}, args...)
			if i < len(f.Josi)-1 || !f.IsVariableJosi {
				break
			}
		}
	}
	// 補完が2つ以上必要ならエラーにする
	if nullCount >= 2 && (valueCount > 0 || t.Josi == "" || containsString(RenbunJosi, t.Josi)) {
		p.failToken(fmt.Sprintf("関数『%s』の引数が不足しています。", funcName), t)
	}
	p.UsedFuncs[funcName] = true

	end := p.peekSourceMap(nil)
	funcNode := &ast.Node{
		Type: ast.Func, Name: funcName, Blocks: args, Meta: f, Josi: t.Josi,
		AsyncFn: f.AsyncFn, SourceMap: m, End: &end,
	}

	switch funcNode.Name {
	case "プラグイン名設定": // ここでスコープが変わる (#1112)
		if len(args) > 0 && args[0] != nil {
			fname := literalToStr(args[0].Value)
			if fname == "メイン" {
				fname = args[0].File
			}
			p.NamespaceStack = append(p.NamespaceStack, p.ModName)
			p.isExportStack = append(p.isExportStack, p.isExportDefault)
			p.ModName = lexer.FilenameToModName(fname)
			p.ModList = append(p.ModList, p.ModName)
		}
	case "名前空間ポップ": // (#1409)
		if n := len(p.NamespaceStack); n > 0 {
			p.ModName = p.NamespaceStack[n-1]
			p.NamespaceStack = p.NamespaceStack[:n-1]
		}
		if n := len(p.isExportStack); n > 0 {
			p.isExportDefault = p.isExportStack[n-1]
			p.isExportStack = p.isExportStack[:n-1]
		}
	}

	if t.Josi == "" { // 言い切りならそこで一度切る
		return funcNode
	}
	if containsString(RenbunJosi, t.Josi) { // 「**して、**」の場合も一度切る
		if p.isReadingCalc && t.Josi == "には" {
			funcNode.Josi = ""
		} else {
			funcNode.Josi = "して"
		}
		return funcNode
	}
	// 続き
	p.pushStack(funcNode)
	return nil
}

// metaOf returns the function definition a token refers to.
//
// A name can also be a variable, and a variable holding a function has no
// signature to check arguments against, so only a function entry counts.
func (p *Parser) metaOf(t *lexer.Token) *lexer.FuncItem {
	if t == nil {
		return nil
	}
	if item, ok := p.FuncList[t.StringValue()]; ok && item.Type == "func" {
		return item
	}
	return nil
}

func toTokenTypes(names []string) []lexer.TokenType {
	out := make([]lexer.TokenType, 0, len(names))
	for _, n := range names {
		out = append(out, lexer.TokenType(n))
	}
	return out
}

func joinStrings(parts []string, sep string) string {
	out := ""
	for i, s := range parts {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}
