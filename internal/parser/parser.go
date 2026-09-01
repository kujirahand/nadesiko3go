package parser

import (
	"fmt"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

// syntaxError carries a parse failure out through the recursive descent.
//
// The TypeScript parser is written around exceptions: several rules catch a
// failure from a nested rule and rethrow it with more context (yDefFuncCommon
// and yLet especially). Threading error returns through every rule would
// change that structure, so the panic stays inside this package and Parse
// converts it back into an ordinary error.
type syntaxError struct{ err *errs.NakoError }

func (p *Parser) failAt(msg string, sm ast.SourceMap) {
	panic(syntaxError{&errs.NakoError{Kind: errs.Syntax, File: sm.File, Line: sm.Line, Msg: msg}})
}

func (p *Parser) failNode(msg string, n *ast.Node) {
	if n == nil {
		p.failAt(msg, p.peekSourceMap(nil))
		return
	}
	p.failAt(msg, n.SourceMap)
}

func (p *Parser) failToken(msg string, t *lexer.Token) {
	if t == nil {
		p.failAt(msg, p.peekSourceMap(nil))
		return
	}
	p.failAt(msg, ast.SourceMap{Line: t.Line, Column: t.Column, File: t.File, Offset: t.Offset, Length: t.Length})
}

// Parse turns a token stream into a syntax tree.
func (p *Parser) Parse(tokens []lexer.Token, filename string) (result *ast.Node, err error) {
	p.reset()
	p.originalVarNames = map[*lexer.Token]string{}
	p.tokens = tokens
	p.ModName = lexer.FilenameToModName(filename)
	p.ModList = append(p.ModList, p.ModName)

	defer func() {
		if r := recover(); r != nil {
			se, ok := r.(syntaxError)
			if !ok {
				panic(r)
			}
			result, err = nil, se.err
		}
	}()

	node := p.startParser()

	// 関数毎に非同期処理が必要かどうかを判定する。「非同期関数を呼ぶ関数もまた
	// 非同期」と伝播していくので、変化が無くなるまで繰り返す。
	for CheckAsyncFn(node, p.FuncList) {
	}
	return node, nil
}

// startParser is the outermost rule.
func (p *Parser) startParser() *ast.Node {
	b := p.ySentenceList()
	if c := p.get(); c != nil && c.Type != lexer.TypeEOF {
		p.failToken(fmt.Sprintf("構文解析でエラー。%sの使い方が間違っています。", nodeToStr(c, 1, "")), c)
	}
	return b
}

func (p *Parser) yNop() *ast.Node {
	m := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.Nop, SourceMap: m, End: &m}
}

// ySentenceList reads statements until the end of the token stream.
func (p *Parser) ySentenceList() *ast.Node {
	var blocks []*ast.Node
	m := p.peekSourceMap(nil)
	for !p.isEOF() {
		n := p.ySentence()
		if n == nil {
			break
		}
		blocks = append(blocks, n)
	}
	if len(blocks) == 0 {
		t := p.peek(0)
		if t == nil && len(p.tokens) > 0 {
			t = &p.tokens[0]
		}
		p.failToken("構文解析に失敗:"+nodeToStr(p.peek(0), 1, ""), t)
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.Block, Blocks: blocks, SourceMap: m, End: &end}
}

func (p *Parser) makeStackBalanceReport() string {
	report := makeStackBalanceReport(p.stack, p.recentlyCalledFunc)
	p.recentlyCalledFunc = nil
	return report
}

// yEOL reads a line end, and refuses to move on with values left unconsumed.
func (p *Parser) yEOL() *ast.Node {
	eol := p.get()
	if eol == nil {
		return nil
	}
	if len(p.stack) > 0 { // 余剰スタックの確認 #1009
		p.failToken(p.makeStackBalanceReport(), eol)
	}
	p.recentlyCalledFunc = nil
	comment, _ := eol.Value.(string)
	return &ast.Node{
		Type: ast.EOL, Comment: comment,
		SourceMap: ast.SourceMap{Line: eol.Line, Column: eol.Column, File: eol.File},
	}
}

// ySentence reads one statement.
func (p *Parser) ySentence() *ast.Node {
	m := p.peekSourceMap(nil)
	end := func() *ast.SourceMap { e := p.peekSourceMap(nil); return &e }

	// 最初の語句が決まっている構文
	switch {
	case p.check(lexer.TypeEOL):
		return p.yEOL()
	case p.check("もし"):
		return p.yIF()
	case p.check("後判定"):
		return p.yAtohantei()
	case p.check("エラー監視"):
		return p.yTryExcept()
	}
	if p.accept(tokenType("抜ける")) {
		return &ast.Node{Type: ast.Break, SourceMap: m, End: end()}
	}
	if p.accept(tokenType("続ける")) {
		return &ast.Node{Type: ast.Continue, SourceMap: m, End: end()}
	}
	if p.check("??") {
		return p.yDebugPrint()
	}
	// 実行モードの指定
	if p.accept(tokenType("DNCLモード")) {
		return p.yDNCLMode(1)
	}
	if p.accept(tokenType("DNCL2モード")) {
		return p.yDNCLMode(2)
	}
	if p.accept(tokenType(lexer.TypeNot), tokenType(lexer.TypeString), tokenType("モード設定")) {
		return p.ySetGenMode(p.yToken(1).StringValue())
	}
	if p.accept(tokenType(lexer.TypeNot), tokenType("モジュール公開既定値"), tokenType("eq"), tokenType(lexer.TypeString)) {
		return p.yExportDefault(p.yToken(3).StringValue())
	}
	if p.accept(tokenType(lexer.TypeNot), tokenType("厳チェック")) { // #1698
		return p.ySetMode("厳しくチェック")
	}
	// <廃止された構文>
	if p.check("逐次実行") {
		return p.yTikuji()
	}
	if p.accept(tokenType(lexer.TypeNot), tokenType("非同期モード")) {
		return p.yASyncMode()
	}
	// </廃止された構文>

	if p.check2([][]lexer.TokenType{{lexer.TypeFunc}, {"eq"}}) {
		word := p.get()
		p.failToken(fmt.Sprintf("関数『%s』に代入できません。", word.StringValue()), word)
	}

	// 先読みして初めて確定する構文
	for _, r := range []func() *ast.Node{p.ySpeedMode, p.yPerformanceMonitor, p.yLet, p.yDefTest, p.yDefFunc} {
		if p.accept(rule(r)) {
			return p.yNode(0)
		}
	}

	// 関数呼び出しの他、各種構文
	if p.accept(rule(p.yCall)) {
		c1 := p.yNode(0)
		if next := p.peek(0); next != nil && next.Type == "ならば" {
			condMap := p.peekSourceMap(nil)
			p.get() // skip ならば
			// もし文の条件として関数呼び出しがある場合
			return p.yIfThen(c1, condMap)
		}
		if containsString(RenbunJosi, c1.Josi) {
			// 連文をblockとして接続する(もし構文などのため)
			if len(p.stack) >= 1 {
				p.failNode(p.makeStackBalanceReport(), c1)
			}
			if c2 := p.ySentence(); c2 != nil {
				return &ast.Node{
					Type: ast.Block, Blocks: []*ast.Node{c1, c2}, Josi: c2.Josi,
					SourceMap: m, End: end(),
				}
			}
		}
		return c1
	}
	return nil
}

// --- 実行モードの指定と廃止された構文 ---

// yASyncMode reads the withdrawn 非同期モード statement (#11).
func (p *Parser) yASyncMode() *ast.Node {
	p.Warnings = append(p.Warnings, "『非同期モード』構文は廃止されました(https://nadesi.com/v3/doc/go.php?1028)。")
	m := p.peekSourceMap(nil)
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.EOL, SourceMap: m, End: &end}
}

func (p *Parser) yDNCLMode(ver int) *ast.Node {
	m := p.peekSourceMap(nil)
	if ver == 1 {
		p.arrayIndexFrom = 1           // 配列インデックスは1から
		p.flagReverseArrayIndex = true // 配列アクセスをJSと逆順で指定する
	}
	p.flagCheckArrayInit = true // 配列代入時に自動で初期化チェックする
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.EOL, SourceMap: m, End: &end}
}

func (p *Parser) ySetGenMode(mode string) *ast.Node {
	m := p.peekSourceMap(nil)
	p.GenMode = mode
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.EOL, SourceMap: m, End: &end}
}

func (p *Parser) yExportDefault(mode string) *ast.Node {
	m := p.peekSourceMap(nil)
	p.isExportDefault = mode == "公開"
	p.ModuleExport[p.ModName] = p.isExportDefault
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.EOL, SourceMap: m, End: &end}
}

func (p *Parser) ySetMode(mode string) *ast.Node {
	m := p.peekSourceMap(nil)
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.RunMode, Value: mode, SourceMap: m, End: &end}
}

// yTikuji reads the withdrawn 逐次実行 statement (#1611).
func (p *Parser) yTikuji() *ast.Node {
	if !p.check("逐次実行") {
		return nil
	}
	p.get()
	p.Warnings = append(p.Warnings, "『逐次実行』構文は廃止されました(https://nadesi.com/v3/doc/go.php?944)。")
	m := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.EOL, SourceMap: m, End: &m}
}

// --- ブロックと関数定義 ---

func (p *Parser) yBlock() *ast.Node {
	m := p.peekSourceMap(nil)
	var blocks []*ast.Node
	if p.check("ここから") {
		p.get()
	}
	for !p.isEOF() {
		if p.checkTypes([]lexer.TokenType{"違えば", "ここまで", "エラー"}) {
			break
		}
		if !p.accept(rule(p.ySentence)) {
			break
		}
		blocks = append(blocks, p.yNode(0))
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{Type: ast.Block, Blocks: blocks, SourceMap: m, End: &end}
}

// yDefFuncReadArgs reads a parenthesised parameter list. The lexer already
// looked at it, but the parser needs the tokens as nodes.
func (p *Parser) yDefFuncReadArgs() []*ast.Node {
	if !p.check("(") {
		return nil
	}
	var args []*ast.Node
	seen := map[string]bool{}
	p.get() // skip '('
	for !p.isEOF() {
		if p.check(")") {
			p.get()
			break
		}
		if t := p.get(); t != nil {
			if t.Type == "|" || t.Type == lexer.TypeComma {
				continue
			}
			name := t.StringValue()
			if name != "" && !seen[name] {
				seen[name] = true
				args = append(args, p.tokenToArgNode(t))
			}
		}
		if p.check(lexer.TypeComma) {
			p.get()
		}
	}
	return args
}

func (p *Parser) yDefTest() *ast.Node { return p.yDefFuncCommon(ast.DefTest, lexer.TypeDefTest) }
func (p *Parser) yDefFunc() *ast.Node { return p.yDefFuncCommon(ast.DefFunc, lexer.TypeDefFunc) }

// yDefFuncCommon reads a named function definition.
func (p *Parser) yDefFuncCommon(nodeType ast.NodeType, tokType lexer.TokenType) (result *ast.Node) {
	if !p.check(tokType) {
		return nil
	}
	m := p.peekSourceMap(nil)
	// 関数定義トークンを取得する。preDefineFunc が先読みした型が入っている。
	def := p.get()
	if def == nil {
		return nil
	}

	isExport := p.isExportDefault
	if p.check("{") {
		p.get()
		attr := p.get()
		if p.check("}") {
			p.get()
		} else {
			p.failToken("関数の属性の指定が正しくありません。『{』と『}』で囲む必要があります。", def)
		}
		if attr != nil {
			switch attr.StringValue() {
			case "公開", "エクスポート":
				isExport = true
			case "非公開":
				isExport = false
			}
		}
	}

	var defArgs []*ast.Node
	if p.check("(") { // lexerでも解析しているが再度詳しく読む
		defArgs = p.yDefFuncReadArgs()
	}

	funcName := p.get()
	if funcName == nil || funcName.Type != lexer.TypeFunc {
		p.failToken(nodeToStr(funcName, 0, "関数")+"の宣言でエラー。", def)
	}

	if p.check("(") {
		if len(defArgs) > 0 { // 関数引数の二重定義
			p.failToken(nodeToStr(funcName, 0, "関数")+
				"の宣言で、引数定義は名前の前か後に一度だけ可能です。", funcName)
		}
		defArgs = p.yDefFuncReadArgs()
	}

	if p.check("とは") {
		p.get()
	}
	block := p.yNop()
	multiline := p.check("ここから") || p.check(lexer.TypeEOL)
	asyncFn := false

	// 定義の途中で起きたエラーは、どの関数の定義かを添えて報告し直す
	func() {
		defer func() {
			if r := recover(); r != nil {
				se, ok := r.(syntaxError)
				if !ok {
					panic(r)
				}
				p.failToken(nodeToStr(funcName, 0, "関数")+
					"の定義で以下のエラーがありました。\n"+se.err.Error(), def)
			}
		}()
		p.funcLevel++
		p.usedAsyncFn = false
		backupLocalvars := p.localvars
		p.localvars = map[string]*VarInfo{"それ": {Type: "var", Value: ""}}

		if multiline {
			p.saveStack()
			// 関数の引数をローカル変数として登録する
			for _, arg := range defArgs {
				if arg == nil || arg.StringValue() == "" {
					continue
				}
				p.localvars[arg.StringValue()] = &VarInfo{Type: "var", Value: ""}
			}
			block = p.yBlock()
			if p.check("ここまで") {
				p.get()
			} else {
				nextWord := "(なし)"
				if n := p.peek(0); n != nil {
					nextWord = literalToStr(n.Value)
				}
				p.failToken(fmt.Sprintf(
					"『ここまで』がありません。関数定義の末尾に必要です。『%s』の前に『ここまで』を記述してください。",
					nextWord), def)
			}
			p.loadStack()
		} else {
			p.saveStack()
			if b := p.ySentence(); b != nil {
				block = b
			}
			p.loadStack()
		}
		p.funcLevel--
		asyncFn = p.usedAsyncFn
		p.localvars = backupLocalvars
	}()

	name := funcName.StringValue()
	meta := p.FuncList[name]
	if meta != nil && !meta.AsyncFn && asyncFn {
		meta.AsyncFn = asyncFn
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{
		Type: nodeType, Name: name, Args: defArgs, Blocks: []*ast.Node{block},
		AsyncFn: asyncFn, IsExport: isExport, Meta: meta,
		SourceMap: m, End: &end,
	}
}

// --- 条件分岐(もし文) ---

// yIFCond reads the condition of a もし statement.
func (p *Parser) yIFCond() *ast.Node {
	m := p.peekSourceMap(nil)
	a := p.yGetArg()
	if a == nil {
		p.failAt("「もし」文の条件式に間違いがあります。"+nodeToStr(p.peek(0), 1, ""), m)
	}
	// チェック : Aならば
	if a.Josi == "ならば" {
		return a
	}
	if a.Josi == "でなければ" {
		end := p.peekSourceMap(nil)
		return &ast.Node{Type: ast.Not, Operator: "not", Blocks: []*ast.Node{a}, SourceMap: m, End: &end}
	}
	// チェック : AがBならば --- 「関数B(A)」のとき
	if a.Josi != "" && p.check(lexer.TypeFunc) {
		p.stack = append(p.stack, a)
		a = p.yCall()
	} else if a.Josi == "が" {
		// チェック : AがBならば --- 「A = B」のとき
		tmpI := p.index
		b := p.yGetArg()
		if b == nil {
			p.failAt("もし文の条件「AがBならば」でBがないか条件が複雑過ぎます。"+
				nodeToStr(p.peek(0), 1, ""), m)
		}
		if p.check("ならば") {
			naraba := p.get()
			b.Josi = naraba.StringValue()
		}
		if b.Josi == "ならば" || b.Josi == "でなければ" {
			op := "eq"
			if b.Josi == "でなければ" {
				op = "noteq"
			}
			end := p.peekSourceMap(nil)
			return &ast.Node{Type: ast.Op, Operator: op, Blocks: []*ast.Node{a, b}, SourceMap: m, End: &end}
		}
		p.index = tmpI
	}
	// もし文で追加の関数呼び出しがある場合
	if !p.check("ならば") {
		p.stack = append(p.stack, a)
		a = p.yCall()
	}
	if !p.check("ならば") {
		msg := "もし文で『ならば』がないか、条件が複雑過ぎます。" +
			nodeToStr(p.peek(0), 1, "") + "の直前に『ならば』を書いてください。"
		if a != nil {
			p.failNode(msg, a)
		}
		p.failNode(msg, p.yNop())
	}
	naraba := p.get()
	if naraba != nil && naraba.StringValue() == "でなければ" { // 否定形
		end := p.peekSourceMap(nil)
		a = &ast.Node{Type: ast.Not, Operator: "not", Blocks: []*ast.Node{a}, SourceMap: m, End: &end}
	}
	if a == nil {
		p.failAt("「もし」文の条件式に間違いがあります。"+nodeToStr(p.peek(0), 1, ""), m)
	}
	return a
}

func (p *Parser) yIF() *ast.Node {
	m := p.peekSourceMap(nil)
	if !p.check("もし") {
		return nil
	}
	mosi := p.get() // skip もし
	for p.check(lexer.TypeComma) {
		p.get()
	}
	var expr *ast.Node
	func() {
		defer func() {
			if r := recover(); r != nil {
				se, ok := r.(syntaxError)
				if !ok {
					panic(r)
				}
				p.failToken("『もし』文の条件で次のエラーがあります。\n"+se.err.Error(), mosi)
			}
		}()
		expr = p.yIFCond()
	}()
	return p.yIfThen(expr, m)
}

// yIfThen reads what follows a condition. It works without a 「もし」, so that
// 『(条件)ならば』 alone is still a conditional.
func (p *Parser) yIfThen(expr *ast.Node, m ast.SourceMap) *ast.Node {
	trueBlock := p.yNop()
	falseBlock := p.yNop()
	tanbun := false

	if p.check(lexer.TypeEOL) {
		trueBlock = p.yBlock()
	} else {
		if b := p.ySentence(); b != nil {
			trueBlock = b
		}
		tanbun = true
	}

	for p.check(lexer.TypeEOL) {
		p.get()
	}

	if p.check("違えば") {
		p.get()
		for p.check(lexer.TypeComma) {
			p.get()
		}
		if p.check(lexer.TypeEOL) {
			falseBlock = p.yBlock()
		} else {
			if b := p.ySentence(); b != nil {
				falseBlock = b
			}
			tanbun = true
		}
	}

	if !tanbun {
		if p.check("ここまで") {
			p.get()
		} else {
			p.failAt("『もし』文で『ここまで』がありません。", m)
		}
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{
		Type: ast.If, Blocks: []*ast.Node{expr, trueBlock, falseBlock},
		SourceMap: m, End: &end,
	}
}

// --- 実行速度・パフォーマンスの指定 ---

// yOptionBlock reads the 「オプション/オプション」文字列that precedes a mode
// statement, then the block it applies to. Both 実行速度優先 and
// パフォーマンスモニタ適用 have the same shape.
func (p *Parser) yOptionBlock(marker lexer.TokenType, nodeType ast.NodeType, names []string, label string) *ast.Node {
	m := p.peekSourceMap(nil)
	if !p.check2([][]lexer.TokenType{{lexer.TypeString}, {marker}}) {
		return nil
	}
	optionNode := p.get()
	p.get()
	val := optionNode.StringValue()
	if val == "" {
		return nil
	}

	options := map[string]bool{}
	for _, n := range names {
		options[n] = false
	}
	for _, name := range strings.Split(val, "/") {
		if name == "全て" {
			for _, n := range names {
				options[n] = true
			}
			break
		}
		if _, ok := options[name]; ok {
			options[name] = true
		} else {
			// 互換性を考えて、警告に留める
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s文のオプション『%s』は存在しません。", label, name))
		}
	}

	multiline := false
	if p.check("ここから") {
		p.get()
		multiline = true
	} else if p.check(lexer.TypeEOL) {
		multiline = true
	}

	block := p.yNop()
	if multiline {
		block = p.yBlock()
		if p.check("ここまで") {
			p.get()
		}
	} else if b := p.ySentence(); b != nil {
		block = b
	}

	return &ast.Node{Type: nodeType, Options: options, Blocks: []*ast.Node{block}, SourceMap: m}
}

func (p *Parser) ySpeedMode() *ast.Node {
	return p.yOptionBlock("実行速度優先", ast.SpeedMode,
		[]string{"行番号無し", "暗黙の型変換無し", "強制ピュア", "それ無効"}, "実行速度優先")
}

func (p *Parser) yPerformanceMonitor() *ast.Node {
	return p.yOptionBlock("パフォーマンスモニタ適用", ast.PerformanceMonitor,
		[]string{"ユーザ関数", "システム関数本体", "システム関数"}, "パフォーマンスモニタ適用")
}

// --- 引数の取得と演算子 ---

// yGetArgOperator reads the operators and values that follow firstValue and
// rebuilds them into a tree by precedence.
func (p *Parser) yGetArgOperator(firstValue *ast.Node) *ast.Node {
	args := []*ast.Node{firstValue}
	for !p.isEOF() {
		op := p.peek(0)
		if op == nil {
			break
		}
		if _, ok := opPriority[string(op.Type)]; !ok {
			break
		}
		op = p.get()
		args = append(args, p.operatorNode(op))
		v := p.yValue()
		if v == nil {
			p.failNode(fmt.Sprintf("計算式で演算子『%s』後に値がありません", literalToStr(op.Value)), firstValue)
		}
		args = append(args, v)
	}
	if len(args) == 1 {
		return args[0]
	}
	return p.infixToAST(args)
}

// operatorNode wraps an operator token so infixToAST can read its precedence.
func (p *Parser) operatorNode(t *lexer.Token) *ast.Node {
	return &ast.Node{
		Type:      ast.NodeType(t.Type),
		Value:     t.Value,
		Josi:      t.Josi,
		SourceMap: ast.SourceMap{Line: t.Line, Column: t.Column, File: t.File, Offset: t.Offset, Length: t.Length},
	}
}

// yRange reads 『A…B』 as a call to the 範囲 function (#1704).
func (p *Parser) yRange(kara *ast.Node) *ast.Node {
	if !p.check("…") {
		return nil
	}
	m := p.peekSourceMap(nil)
	p.get() // skip '…'
	made := p.yValue()
	if kara == nil || made == nil {
		p.failAt("範囲オブジェクトの指定エラー。『A…B』の書式で指定してください。", m)
	}
	meta := p.FuncList["範囲"]
	if meta == nil {
		p.failAt("関数『範囲』が見つかりません。plugin_systemをシステムに追加してください。", m)
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{
		Type: ast.Func, Name: "範囲", Blocks: []*ast.Node{kara, made},
		Josi: made.Josi, Meta: meta, SourceMap: m, End: &end,
	}
}

// yDebugPrint reads 『??』, an alias for 表示 (#1745).
func (p *Parser) yDebugPrint() *ast.Node {
	m := p.peekSourceMap(nil)
	t := p.get() // skip '??'
	if t == nil || t.StringValue() != "??" {
		p.failAt("『??』で指定してください。", m)
	}
	arg := p.yCalc()
	if arg == nil {
		p.failAt("『??(計算式)』で指定してください。", m)
	}
	meta := p.FuncList["ハテナ関数実行"]
	if meta == nil {
		p.failAt("関数『ハテナ関数実行』が見つかりません。plugin_systemをシステムに追加してください。", m)
	}
	end := p.peekSourceMap(nil)
	return &ast.Node{
		Type: ast.Func, Name: "ハテナ関数実行", Blocks: []*ast.Node{arg},
		Meta: meta, SourceMap: m, End: &end,
	}
}

func (p *Parser) yGetArg() *ast.Node {
	value1 := p.yValue()
	if value1 == nil {
		return nil
	}
	if p.check("…") { // 範囲オブジェクト
		return p.yRange(value1)
	}
	return p.yGetArgOperator(value1)
}

// yGetArgParen reads the arguments inside a C-style call.
func (p *Parser) yGetArgParen(first *ast.Node, funcName string) []*ast.Node {
	isClose := false
	si := len(p.stack)
	for !p.isEOF() {
		if p.check(")") {
			isClose = true
			break
		}
		// カッコを用いた関数呼び出しの中でも助詞を用いた関数呼び出しを有効にする #2000
		v := p.yCalc()
		if v == nil {
			break
		}
		p.pushStack(v)
		if p.check(lexer.TypeComma) {
			p.get()
		}
	}
	if !isClose {
		name := funcName
		if name == "" && first != nil {
			name = nodeToStr(first, 0, "関数")
		}
		p.failNode(fmt.Sprintf("C風関数『%s』でカッコが閉じていません", name), first)
	}
	var a []*ast.Node
	for si < len(p.stack) {
		if v := p.popStack(nil); v != nil {
			a = append([]*ast.Node{v}, a...)
		}
	}
	return a
}

// yApplyCallValue keeps applying C-style calls to a value that is a function.
func (p *Parser) yApplyCallValue(callee *ast.Node) *ast.Node {
	node := callee
	for p.check("(") {
		p.get() // skip '('
		args := p.yGetArgParen(node, "関数呼び出しの結果")
		if !p.check(")") {
			p.failNode("C風関数呼び出しのエラー", node)
		}
		closeTok := p.get()
		end := p.peekSourceMap(nil)
		node = &ast.Node{
			Type:      ast.CallValue,
			Blocks:    append([]*ast.Node{node}, args...),
			Josi:      closeTok.Josi,
			SourceMap: node.SourceMap,
			End:       &end,
		}
	}
	return node
}
