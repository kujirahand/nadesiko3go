package compiler

import (
	"fmt"
	"strings"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/node"
	"github.com/kujirahand/nadesiko3go/scope"
	"github.com/kujirahand/nadesiko3go/value"
)

// TCodeLabel : ジャンプ用のラベル管理
type TCodeLabel struct {
	code     *TCode
	addr     int
	argNames []string // 関数のとき引数のリストを保持
	memo     string
}

// TCompiler : コンパイラオブジェクト
type TCompiler struct {
	Codes         []*TCode
	Consts        value.TArray
	Labels        []*TCodeLabel
	UserFuncLabel map[string]int // 何番目のLabelsにリンクするか
	reg           *value.TArray  // 実行時に使うレジスタ
	scope         *scope.Scope   // メインスコープ
	index         int
	length        int
	FileNo        int
	Line          int
	sys           *core.Core
	forNest       int
	breakLabel    *TCode   // for Break
	continueLabel *TCode   // for Continue
	loopLabels    []*TCode // for Break / Continue
}

// NewCompier : コンパイラオブジェクトを生成
func NewCompier(sys *core.Core) *TCompiler {
	p := TCompiler{}
	p.Codes = []*TCode{}
	p.Consts = value.NewTArrayDef(value.TArrayItems{
		value.NewIntPtr(0),
		value.NewIntPtr(1),
		value.NewIntPtr(2),
		value.NewIntPtr(3),
		value.NewIntPtr(4),
		value.NewIntPtr(5),
		value.NewIntPtr(6),
		value.NewIntPtr(7),
		value.NewIntPtr(8),
		value.NewIntPtr(9),
	})
	p.Labels = []*TCodeLabel{}
	p.UserFuncLabel = map[string]int{}
	p.index = 0
	p.sys = sys
	p.scope = sys.Scopes.GetTopScope()
	p.reg = p.scope.Reg
	p.loopLabels = []*TCode{}
	return &p
}

// CompileError : コンパイルエラー
func CompileError(msg string, n *node.Node) error {
	var e error
	msg = strings.Replace(msg, "[コンパイルエラー] ", "", 0)
	if n != nil {
		fi := (*n).GetFileInfo()
		e = fmt.Errorf("[コンパイルエラー] (%d) %s", fi.Line, msg)
	} else {
		e = fmt.Errorf("[コンパイルエラー] " + msg)
	}
	return e
}

// Compile : コンパイル
func (p *TCompiler) Compile(n *node.Node) error {
	labelMainBegin := p.makeLabel("MAIN_BEGIN")
	c := []*TCode{p.makeJump(labelMainBegin)}
	// 最初にユーザー関数を定義する
	for _, v := range p.sys.UserFuncs.GetItems() {
		vf := v.Value.(value.TFuncValue)
		// println("@DEF_FUNC:", vf.Name)
		nlink := vf.LinkNode
		nodeDef := nlink.(node.TNodeDefFunc)
		var n node.Node = nodeDef
		cDef, eDef := p.convDefFunc(&n)
		if eDef != nil {
			return CompileError(eDef.Error(), &n)
		}
		c = append(c, cDef...)
	}
	// MAIN
	c = append(c, labelMainBegin)
	codes, err := p.convNode(n)
	if err != nil {
		return err
	}
	c = append(c, codes...)
	// Optimize
	if p.sys.IsOptimze {
		p.Codes = c
		p.Optimize()
		c = p.Codes
	}
	p.fixLabels(c)
	p.Codes = c
	return nil
}

func (p *TCompiler) convNode(n *node.Node) ([]*TCode, error) {
	if n == nil {
		return nil, nil
	}
	switch (*n).GetType() {
	case node.Nop:
		return p.convNop(n)
	case node.Word:
		return p.convWord(n)
	case node.TypeNodeList:
		return p.convNodeList(n)
	case node.Sentence:
		return p.convSentence(n)
	case node.Operator:
		return p.convOperator(n)
	case node.Const:
		return p.convConst(n)
	case node.Let:
		return p.convLet(n)
	case node.DefVar:
		return p.convDefVar(n)
	case node.For:
		return p.convFor(n)
	case node.If:
		return p.convIf(n)
	case node.While:
		return p.convWhile(n)
	case node.Calc:
		return p.convCalc(n)
	case node.CallFunc:
		return p.convCallFunc(n)
	case node.Return:
		return p.convReturn(n)
	case node.JSONArray:
		return p.convJSONArray(n)
	case node.JSONHash:
		return p.convJSONHash(n)
	case node.Foreach:
		return p.convForeach(n)
	case node.Continue:
		return p.convContinue(n)
	case node.Break:
		return p.convBreak(n)
	case node.DefFunc:
		return nil, nil // 関数定義は Compile で最初に行う
	case node.Repeat:
		return p.convRepeat(n)
	}
	println("[SYSTEM ERROR] Compile " + node.ToString(*n, 0))
	// panic(-1)
	return nil, nil
}

func (p *TCompiler) convNop(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeNop)
	return []*TCode{
		NewCode(FileInfo, nn.FileInfo.FileNo, nn.FileInfo.Line, 0),
	}, nil
}

func (p *TCompiler) convRepeat(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeRepeat)
	labelCond := p.makeLabel("REPEAT_COND")
	labelEnd := p.makeLabel("REPEAT_END")
	labelCotinue := p.makeLabel("REPEAT_CONTINUE")
	p.loopBegin(labelCotinue, labelEnd)
	c := []*TCode{p.makeLabel("REPEAT_BEGIN")}
	tmpRegIndex := p.scope.Index
	// init
	cExpr, errExpr := p.convNode(&nn.Expr)
	if errExpr != nil {
		return nil, CompileError("『回』の回数式。"+errExpr.Error(), n)
	}
	regExpr := p.regTop() - 1
	c = append(c, cExpr...)
	regLoop := p.regNext()
	c = append(c, p.makeConstInt(regLoop, 1))
	// cond : expr < regLoop
	c = append(c, labelCond)
	regResult := p.regNext()
	c = append(c, NewCode(Lt, regResult, regExpr, regLoop))
	c = append(c, p.makeJumpIfTrue(regResult, labelEnd))
	// set 回数
	c = append(c, p.makeSetLocalReg("回数", regLoop))
	// block
	cBlock, errBlock := p.convNode(&nn.Block)
	if errBlock != nil {
		return nil, errBlock
	}
	c = append(c, cBlock...)
	// inc
	c = append(c, labelCotinue) // continue point
	c = append(c, NewCode(IncReg, regLoop, 0, 0))
	c = append(c, p.makeJump(labelCond))
	c = append(c, labelEnd)
	p.scope.Index = tmpRegIndex
	p.loopEnd()
	return c, nil
}

func (p *TCompiler) convDefFunc(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeDefFunc)
	funcName := nn.Word
	// 既に定義済みであれば戻る
	if _, exists := p.UserFuncLabel[funcName]; exists {
		return nil, nil
	}
	// 関数の定義
	labelBegin := p.makeLabel("DEF_FUNC_BEGIN:" + funcName)
	labelEnd := p.makeLabel("DEF_FUNC_END:" + funcName)
	gotoEnd := p.makeJump(labelEnd)
	c := []*TCode{gotoEnd, labelBegin}
	p.UserFuncLabel[funcName] = labelBegin.A // call時に参照
	codeLabel := p.Labels[labelBegin.A]
	args := []string{}

	// User func
	userNode := nn
	// Open Local Scope
	scope := p.sys.Scopes.Open()
	p.scope = scope
	// 変数の登録(順番に注意)
	scope.Set("それ", value.NewNullPtr())
	scope.Reg.Set(metaRegReturnAddr, value.NewIntPtr(-1))
	scope.Reg.Set(metaRegReturnValue, value.NewIntPtr(-1))
	scope.Index = 2
	// スコープにローカル変数を挿入 (順番が重要)
	for _, name := range userNode.ArgNames {
		scope.Set(name, value.NewNullPtr())
		args = append(args, name)
	}
	codeLabel.argNames = args
	// ローカルスコープに「それ」を配置
	localSore := value.NewNullPtr()
	scope.Set("それ", localSore)
	// Block
	cBlock, errBlock := p.convNode(&userNode.Block)
	if errBlock != nil {
		return nil, errBlock
	}
	c = append(c, cBlock...)
	c = append(c, p.makeGetLocal("それ"))
	c = append(c, NewCode(Return, p.regBack(), 0, 0))
	c = append(c, labelEnd)
	// Close Local Scope
	p.sys.Scopes.Close()
	p.scope = p.sys.Scopes.GetTopScope()
	return c, nil
}

func (p *TCompiler) getFunc(name string) (*value.Value, error) {
	// 関数を得る
	funcV := p.sys.Scopes.Get(name)
	// 変数が見当たらない
	if funcV == nil {
		msgu := fmt.Errorf("関数『%s』は未定義。", name)
		return nil, msgu
	}
	// 関数ではない？
	if !funcV.IsFunction() {
		msgn := fmt.Errorf("『%s』は関数ではい。", name)
		return nil, msgn
	}
	return funcV, nil
}

func (p *TCompiler) getFuncArgs(fname string, funcV *value.Value, nodeArgs node.TNodeList, useJosi bool) (int, []*TCode, error) {
	// 関数の引数を得る
	fv := funcV.Value.(value.TFuncValue)
	defArgs := fv.Args                      // 定義
	usedArgs := make([]bool, len(nodeArgs)) // ノードを利用したか(同じ助詞が二つある場合)
	// 引数を取得する
	arrayIndex := p.regTop()
	c := []*TCode{
		// NewCodeMemo(NewArray, arrayIndex, 0, 0, "配列生成←関数の引数:"+fname),
	}
	for _, josiList := range defArgs {
		for _, josi := range josiList {
			for k, nodeJosi := range nodeArgs {
				if usedArgs[k] {
					continue
				}
				if useJosi {
					if josi != nodeJosi.GetJosi() { // 助詞が一致しない
						continue
					}
				}
				usedArgs[k] = true
				cArg, err1 := p.convNode(&nodeJosi)
				if err1 != nil {
					msg := fmt.Errorf("関数『%s』引数でエラー。%s", fname, err1.Error())
					return -1, nil, msg
				}
				c = append(c, cArg...)
				// c = append(c, NewCodeMemo(AppendArray, arrayIndex, argIndex, 0, fname+"の引数追加"))
			}
		}
	}
	// 引数のチェック (1) 漏れなくcf.Args内のノードを評価したか
	for ci, b := range usedArgs {
		if b == false {
			msgArg := fmt.Errorf("関数『%s』の第%d引数の間違い。", fname, (ci + 1))
			return -1, nil, msgArg
		}
	}
	// 引数のチェック (2) 関数定義引数(defArgs)と数が合っているか？
	// 		特定として 引数-1であれば、変数「それ」の値を補う
	// fmt.Printf("args: %d=%d", len(nodeArgs), len(defArgs))
	if len(nodeArgs) != len(defArgs) {
		// 特例ルール -- 「それ」を補完する
		if len(nodeArgs) == (len(defArgs) - 1) {
			c = append(c, p.makeGetLocal("それ"))
		} else {
			if len(nodeArgs) > len(defArgs) {
				return -1, nil, fmt.Errorf(
					"関数『%s』で引数の数が多すぎます。"+
						"引数が%d個あります。"+
						"あるいは、関数に複文が指定されている可能性があります。", fname, len(nodeArgs))
			}
			return -1, nil, fmt.Errorf(
				"関数『%s』で引数の数が不足しています。%d個必要です。",
				fname,
				len(defArgs))
		}
	}
	return arrayIndex, c, nil
}

func (p *TCompiler) convCallFunc(n *node.Node) ([]*TCode, error) {
	cf := (*n).(node.TNodeCallFunc)
	c := []*TCode{}

	// 関数を得る
	funcV, err := p.getFunc(cf.Name)
	if err != nil {
		return nil, CompileError(err.Error(), n)
	}
	// ユーザー関数の場合
	if funcV.Type == value.UserFunc {
		return p.callUserFunc(cf, funcV)
	}

	tmpRcount := p.regNext()

	// 引数を得る
	argIndex, cArgs, err := p.getFuncArgs(cf.Name, funcV, cf.Args, cf.UseJosi)
	if err != nil {
		return nil, CompileError(err.Error(), n)
	}
	c = append(c, cArgs...)
	// 関数を実行
	// システム関数
	funcRes := tmpRcount
	fconstI := p.appendConsts(funcV)
	c = append(c, NewCodeMemo(CallFunc, funcRes, fconstI, argIndex, cf.Name))
	p.scope.Index = tmpRcount + 1
	return c, nil
}

// callUserFunc : ユーザー関数の呼び出しを生成
func (p *TCompiler) callUserFunc(cf node.TNodeCallFunc, funcV *value.Value) ([]*TCode, error) {
	c := []*TCode{}
	funcName := cf.Name
	funcLabel, funcDefined := p.UserFuncLabel[funcName]
	if !funcDefined {
		n := node.Node(cf)
		return nil, CompileError("[SYSTEM] 関数定義に失敗している", &n)
	}
	// 関数呼び出し
	tmpRC := p.regNext()
	argIndex, cArgs, err := p.getFuncArgs(cf.Name, funcV, cf.Args, cf.UseJosi)
	if err != nil {
		return nil, err
	}
	c = append(c, cArgs...)
	funcR := tmpRC
	c = append(c, NewCodeMemo(CallUserFunc, funcR, funcLabel, argIndex, funcName))
	p.scope.Index = tmpRC + 1
	return c, nil
}

func (p *TCompiler) convJSONArray(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeJSONArray)
	c := []*TCode{}
	arrayIndex := p.regNext()
	c = append(c, NewCode(NewArray, arrayIndex, 0, 0))
	for _, vNode := range nn.Items {
		cVal, eVal := p.convNode(&vNode)
		if eVal != nil {
			return nil, CompileError("JSONArray:"+eVal.Error(), n)
		}
		c = append(c, cVal...)
		c = append(c, NewCode(AppendArray, arrayIndex, p.regBack(), 0))
	}
	return c, nil
}

func (p *TCompiler) makeConstInt(reg, v int) *TCode {
	ci := p.appendConstsInt(v)
	return &TCode{Type: ConstO, A: reg, B: ci, Memo: "=" + value.IntToStr(v)}
}

func (p *TCompiler) convForeach(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeForeach)
	tmpRCount := p.regTop()
	c := []*TCode{p.makeLabel("FOREACH_BEGIN")}
	labelCond := p.makeLabel("FOREACH_COND")
	labelEnd := p.makeLabel("FOREACH_END")
	p.loopBegin(labelCond, labelEnd)
	// expr
	cExpr, errExpr := p.convNode(&nn.Expr)
	if errExpr != nil {
		return nil, CompileError("反復の条件式で。"+errExpr.Error(), n)
	}
	c = append(c, cExpr...)
	rExpr := p.regTop() - 1
	// Init Loop Counter
	rI := p.regNext()
	c = append(c, p.makeConstInt(rI, 0))
	// COND
	c = append(c, labelCond)
	rCond := p.regTop()
	// FOREACH isContinue:A expr:B counter:C
	c = append(c, NewCodeMemo(Foreach, rCond, rExpr, rI, "反復"))
	c = append(c, p.makeJumpIfTrue(rCond, labelEnd))
	// body
	cBody, errBody := p.convNode(&nn.Block)
	if errBody != nil {
		return nil, CompileError("『反復』ブロックで。"+errBody.Error(), n)
	}
	c = append(c, cBody...)
	c = append(c, p.makeJump(labelCond))
	c = append(c, labelEnd)
	p.scope.Index = tmpRCount
	p.loopEnd()
	return c, nil
}

func (p *TCompiler) convJSONHash(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeJSONHash)
	c := []*TCode{}
	arrayIndex := p.regNext()
	c = append(c, NewCode(NewHash, arrayIndex, 0, 0))
	for name, vNode := range nn.Items {
		cVal, eVal := p.convNode(&vNode)
		if eVal != nil {
			return nil, CompileError("JSONHash:"+eVal.Error(), n)
		}
		ci := p.appendConstsStr(name)
		c = append(c, cVal...)
		c = append(c, NewCode(SetHash, arrayIndex, ci, p.regBack()))
	}
	return c, nil
}

func (p *TCompiler) convReturn(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeReturn)
	c := []*TCode{}
	if nn.Arg != nil {
		cArg, errArg := p.convNode(&nn.Arg)
		if errArg != nil {
			return nil, CompileError("『戻る』の引数にて。"+errArg.Error(), n)
		}
		c = append(c, cArg...)
	} else {
		c = append(c, p.makeGetLocal("それ"))
	}
	c = append(c, NewCodeMemo(Return, p.regBack(), 0, 0, "戻る"))
	return c, nil
}

func (p *TCompiler) convContinue(n *node.Node) ([]*TCode, error) {
	if p.continueLabel == nil {
		return nil, CompileError("突然『続ける』が指定されました。繰り返しの中で使ってください。", n)
	}
	c := []*TCode{
		p.makeJumpWithMemo(p.continueLabel, "(CONTINUE)"),
	}
	return c, nil
}

func (p *TCompiler) convBreak(n *node.Node) ([]*TCode, error) {
	if p.breakLabel == nil {
		return nil, CompileError("突然『抜ける』が指定されました。繰り返しの中で使ってください。", n)
	}
	c := []*TCode{
		p.makeJump(p.breakLabel),
	}
	return c, nil
}

func (p *TCompiler) convCalc(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeCalc)
	return p.convNode(&nn.Child)
}

func (p *TCompiler) convWhile(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeWhile)
	labelBegin := p.makeLabel("WHILE_BEGIN")
	labelEnd := p.makeLabel("WHILE_END")
	p.loopBegin(labelBegin, labelEnd)
	c := []*TCode{labelBegin}
	// expr
	cExpr, errExpr := p.convNode(&nn.Expr)
	if errExpr != nil {
		return nil, CompileError("『間』構文の条件文で。"+errExpr.Error(), n)
	}
	c = append(c, cExpr...)
	whileExprReg := p.regBack()
	c = append(c, NewCode(NotReg, whileExprReg, 0, 0))
	c = append(c, p.makeJumpIfTrue(whileExprReg, labelEnd))
	// block
	cBlock, errBlock := p.convNode(&nn.Block)
	if errBlock != nil {
		return nil, CompileError("『間』構文にて。"+errBlock.Error(), n)
	}
	c = append(c, cBlock...)
	c = append(c, p.makeJump(labelBegin))
	c = append(c, labelEnd)
	p.loopEnd()
	return c, nil
}

func (p *TCompiler) convIf(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeIf)
	c := []*TCode{}
	cExpr, errExpr := p.convNode(&nn.Expr)
	if errExpr != nil {
		return nil, CompileError("『もし』の条件文で。"+errExpr.Error(), n)
	}
	labelEndIF := p.makeLabel("IF_END")
	labelTrueBegin := p.makeLabel("IF_TRUE_BEGIN")
	c = append(c, cExpr...)
	c = append(c, p.makeJumpIfTrue(p.regBack(), labelTrueBegin))

	if nn.FalseNode != nil {
		cFalse, errFalse := p.convNode(&nn.FalseNode)
		if errFalse != nil {
			return nil, CompileError("『もし』構文の偽ブロックで。"+errFalse.Error(), n)
		}
		if cFalse != nil {
			c = append(c, cFalse...)
		}
	}
	c = append(c, p.makeJump(labelEndIF))

	cTrue, errTrue := p.convNode(&nn.TrueNode)
	if errTrue != nil {
		return nil, CompileError("『もし』構文の真ブロックで。"+errTrue.Error(), n)
	}
	c = append(c, labelTrueBegin)
	c = append(c, cTrue...)
	c = append(c, labelEndIF)
	return c, nil
}

func (p *TCompiler) convWord(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeWord)
	c := []*TCode{}
	// 現在のスコープに変数があるか
	scope := p.scope
	toReg := p.regNext()
	varNo := scope.GetIndexByName(nn.Name)
	if varNo < 0 {
		v := value.NewNullPtr()
		varNo = scope.Set(nn.Name, v)
	}
	c = append(c, NewCodeMemo(GetLocal, toReg, varNo, 0, nn.Name))
	// 配列アクセス
	if nn.Index != nil {
		var index node.TNodeList = nn.Index.Items
		for _, vNode := range index {
			cExpr, eExpr := p.convNode(&vNode)
			if eExpr != nil {
				return nil, CompileError("添字の評価。"+eExpr.Error(), n)
			}
			c = append(c, cExpr...)
			exprReg := p.regTop() - 1
			code := NewCode(GetArrayElem, toReg, toReg, exprReg)
			c = append(c, code)
		}
	}
	return c, nil
}

func (p *TCompiler) makeSetLocalReg(name string, reg int) *TCode {
	scope := p.scope
	A := scope.GetIndexByName(name)
	if A < 0 {
		scope.Set(name, value.NewNullPtr())
		A = scope.GetIndexByName(name)
	}
	return NewCodeMemo(SetLocal, A, reg, 0, name)
}

func (p *TCompiler) makeSetLocal(name string) *TCode {
	B := p.regBack()
	if B <= 0 {
		B = 0
		p.scope.Index = 0
	}
	return p.makeSetLocalReg(name, B)
}

func (p *TCompiler) makeSetSore(reg int) *TCode {
	return NewCode(SetSore, reg, 0, 0)
}

func (p *TCompiler) makeGetLocal(name string) *TCode {
	// (1) ローカルスコープを探す
	scope := p.scope
	A := p.regNext()
	B := scope.GetIndexByName(name)
	if B < 0 {
		// (2) 現在のスコープよりも上のスコープを探す
		scopeVar, level := p.sys.Scopes.Find(name)
		if scopeVar != nil { // 上の階層にある
			if scopeVar != nil && level == 0 { // Globalにある
				B := p.sys.Global.GetIndexByName(name)
				return NewCodeMemo(GetGlobal, A, B, 0, name)
			}
			ci := p.appendConstsStr(name)
			return NewCodeMemo(FindVar, A, ci, 0, name)
		}
		// (3) NULLを値として戻す
		idxNull := p.appendConsts(value.NewNullPtr())
		return NewCodeMemo(ConstO, A, idxNull, 0, "未定義の変数:"+name)
	}
	return NewCodeMemo(GetLocal, A, B, 0, name) // ローカル変数
}

func (p *TCompiler) convDefVar(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeDefVar)
	varName := nn.Name
	c := []*TCode{}
	regExpr := -1
	// Value
	if nn.Expr != nil {
		cExpr, err := p.convNode(&nn.Expr)
		if err != nil {
			return nil, CompileError("『"+varName+"』の定義でエラー", n)
		}
		c = append(c, cExpr...)
		regExpr = p.regTop() - 1
	} else {
		c = append(c, p.makeConstInt(0, p.regTop()))
		regExpr = p.regTop()
	}
	// var
	varV := p.scope.Get(varName)
	if varV != nil {
		return nil, CompileError(fmt.Sprintf("定数『%s』の宣言で既に変数が存在します。", varName), n)
	}
	val := value.NewNullPtr()
	p.scope.Set(varName, val)
	noVar := p.scope.GetIndexByName(varName)
	meta := p.scope.GetMetaByIndex(noVar)
	meta.IsConst = nn.IsConst // Immutable?
	c = append(c, NewCodeMemo(SetLocal, noVar, regExpr, 0, "定数:"+varName))
	p.regBack()
	return c, nil
}

func (p *TCompiler) convLet(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeLet)
	varName := nn.Name
	// value
	c, err := p.convNode(&nn.ValueNode)
	if err != nil {
		return nil, CompileError("『"+varName+"』の代入でエラー", n)
	}
	valueR := p.regTop() - 1
	// println("valueR=", valueR)
	if valueR < 0 {
		valueR = 0
	}

	// SetLocal (Indexがない場合)
	if nn.Index == nil || len(nn.Index.Items) == 0 {
		varNo := p.scope.GetIndexByName(varName)
		if varNo < 0 {
			varV := p.scope.Get(varName)
			if varV == nil {
				varV = value.NewNullPtr()
				p.scope.Set(varName, varV)
				varNo = p.scope.GetIndexByName(varName)
			}
		}
		// 定数チェック
		meta := p.scope.GetMetaByIndex(varNo)
		if meta.IsConst {
			return nil, CompileError(fmt.Sprintf("定数『%s』には代入できません。", varName), n)
		}
		c = append(c, p.makeSetLocalReg(varName, valueR))
		return c, nil
	}

	// Indexがある場合
	c = append(c, p.makeGetLocal(varName))
	varR := p.regTop() - 1
	for _, exprNode := range nn.Index.Items {
		cExpr, errExpr := p.convNode(&exprNode)
		if errExpr != nil {
			return nil, CompileError("変数『"+nn.Name+"』への代入で添字の評価。"+errExpr.Error(), n)
		}
		c = append(c, cExpr...)
		idxR := p.regTop() - 1
		c = append(c, NewCodeMemo(GetArrayElem, varR, varR, idxR, "代入における要素取得"))
	}
	c = append(c, NewCodeMemo(SetArrayElem, varR, valueR, 0, nn.Name+"への代入"))

	// TODO : index
	return c, nil
}

func (p *TCompiler) convFor(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeFor)
	tmpRCount := p.regTop()

	// varNo
	varName := nn.Word
	if varName == "" {
		varName = "__FOR_I" + value.IntToStr(p.forNest)
		p.forNest++
	}

	labelForBegin := p.makeLabel("FOR_BEGIN:" + varName)
	c := []*TCode{labelForBegin}

	// WORD = From
	fromCodes, errFrom := p.convNode(&nn.FromNode)
	if errFrom != nil {
		return nil, CompileError("『繰返』構文の引数で。"+errFrom.Error(), n)
	}
	c = append(c, fromCodes...)
	c = append(c, p.makeSetLocal(varName))

	// To
	toCodes, errTo := p.convNode(&nn.ToNode)
	if errTo != nil {
		return nil, CompileError("『繰返』構文の引数で。"+errTo.Error(), n)
	}
	c = append(c, toCodes...)
	toR := p.regTop() - 1

	// cond : IF WORD > TO then goto BlockEnd
	labelCond := p.makeLabel("FOR_COND:" + varName)
	labelContinue := p.makeLabel("FOR_CONTINUE:" + varName)
	labelBlockEnd := p.makeLabel("FOR_BLOCK_END:" + varName)
	p.loopBegin(labelContinue, labelBlockEnd)
	c = append(c, labelCond)

	getLoopVar := p.makeGetLocal(varName)
	c = append(c, getLoopVar)
	varR := p.regTop() - 1

	varExpr := p.regTop()
	c = append(c, NewCodeMemo(Gt, varExpr, varR, toR, "VAR > TO"))
	c = append(c, p.makeJumpIfTrue(varExpr, labelBlockEnd))

	// それに値を設定
	c = append(c, p.makeGetLocal(varName))
	c = append(c, p.makeSetSore(p.regTop()-1))
	p.regBack()

	// Block
	p.breakLabel = labelBlockEnd
	p.continueLabel = labelContinue
	blockCodes, errBlock := p.convNode(&nn.Block)
	if errBlock != nil {
		return nil, CompileError("『繰返』構文にて。"+errBlock.Error(), n)
	}
	c = append(c, blockCodes...)
	c = append(c, labelContinue)
	c = append(c, NewCodeMemo(IncLocal, getLoopVar.B, 1, 0, "FOR_INC:"+varName)) // WORD++
	c = append(c, p.makeJump(labelCond))
	c = append(c, labelBlockEnd)
	p.scope.Index = tmpRCount
	p.loopEnd()
	return c, nil
}

func (p *TCompiler) convSentence(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeSentence)
	nl := (node.Node)(nn.List)
	return p.convNode(&nl)
}

func (p *TCompiler) convNodeList(n *node.Node) ([]*TCode, error) {
	codes := []*TCode{}
	nn := (*n).(node.TNodeList)
	for _, nv := range nn {
		res, err := p.convNode(&nv)
		if err != nil {
			return nil, err
		}
		codes = append(codes, res...)
	}
	return codes, nil
}

func (p *TCompiler) appendConstsInt(num int) int {
	// 同じ値があるか調べる
	if num < 10 { // 10以下なら最初に強制的に作成した
		return num
	}
	for i, v := range p.Consts.GetItems() {
		if v.Type == value.Int && v.ToInt() == num {
			return i
		}
	}
	// なければ追加
	val := value.NewIntPtr(num)
	idx := p.Consts.Length()
	p.Consts.Append(val)
	return idx
}

func (p *TCompiler) appendConstsStr(s string) int {
	// 同じ値があるか調べる
	for i, v := range p.Consts.GetItems() {
		if v.Type == value.Str && v.ToString() == s {
			return i
		}
	}
	// なければ追加
	val := value.NewStrPtr(s)
	idx := p.Consts.Length()
	p.Consts.Append(val)
	return idx
}

func (p *TCompiler) appendConsts(val *value.Value) int {
	// 同じ値があるか調べる
	for i, v := range p.Consts.GetItems() {
		if v.Type == val.Type {
			switch v.Type {
			case value.Null:
				return i
			case value.Int:
				if v.ToInt() == val.ToInt() {
					return i
				}
			case value.Float:
				if v.ToFloat() == val.ToFloat() {
					return i
				}
			case value.Str:
				if v.ToString() == val.ToString() {
					return i
				}
			case value.Function:
				if val.Type == value.Function && v.ToString() == val.ToString() {
					return i
				}
			}
		}
	}
	// なければ追加
	idx := p.Consts.Length()
	p.Consts.Append(val)
	return idx
}

func (p *TCompiler) convConst(n *node.Node) ([]*TCode, error) {
	op := (*n).(node.TNodeConst)
	v := op.Value
	// push const
	regI := p.regNext()
	switch v.Type {
	case value.Int:
		return []*TCode{p.makeConstInt(regI, v.ToInt())}, nil
	case value.Str:
		codeType := ConstO
		if op.IsExtend {
			codeType = ExString
		}
		ci := p.appendConstsStr(v.ToString())
		return []*TCode{NewCodeMemo(codeType, regI, ci, 0, "="+v.ToString())}, nil
	default:
		ci := p.appendConsts(v)
		return []*TCode{NewCodeMemo(ConstO, regI, ci, 0, "="+v.ToString())}, nil
	}
}

func (p *TCompiler) convOperator(n *node.Node) ([]*TCode, error) {
	op := (*n).(node.TNodeOperator)
	tmpRCount := p.regTop()
	// Right node
	r, errR := p.convNode(&op.Right)
	if errR != nil {
		return nil, CompileError("演算エラー", n)
	}
	pcR := p.regTop() - 1
	// Left node
	l, errL := p.convNode(&op.Left)
	if errL != nil {
		return nil, CompileError("演算エラー", n)
	}
	pcL := p.regTop() - 1
	res := []*TCode{}
	res = append(res, r...)
	res = append(res, l...)
	//
	toindex := tmpRCount
	p.scope.Index = toindex + 1
	switch op.Operator {
	case "+":
		res = append(res, NewCode(Add, toindex, pcL, pcR))
	case "-":
		res = append(res, NewCode(Sub, toindex, pcL, pcR))
	case "*":
		res = append(res, NewCode(Mul, toindex, pcL, pcR))
	case "/":
		res = append(res, NewCode(Div, toindex, pcL, pcR))
	case "%":
		res = append(res, NewCode(Mod, toindex, pcL, pcR))
	case "^":
		res = append(res, NewCode(Exp, toindex, pcL, pcR))
	case "==":
		res = append(res, NewCode(EqEq, toindex, pcL, pcR))
	case "!=":
		res = append(res, NewCode(NtEq, toindex, pcL, pcR))
	case ">":
		res = append(res, NewCode(Gt, toindex, pcL, pcR))
	case ">=":
		res = append(res, NewCode(GtEq, toindex, pcL, pcR))
	case "<":
		res = append(res, NewCode(Lt, toindex, pcL, pcR))
	case "<=":
		res = append(res, NewCode(LtEq, toindex, pcL, pcR))
	case "かつ":
		res = append(res, NewCode(And, toindex, pcL, pcR))
	case "または":
		res = append(res, NewCode(Or, toindex, pcL, pcR))
	}
	return res, nil
}

func (p *TCompiler) getConstNoByID(id string, canCreate bool) int {
	for i, v := range p.Consts.GetItems() {
		if v.Type == value.Str {
			if v.ToString() == id {
				return i
			}
		}
	}
	if !canCreate {
		return -1
	}
	resIndex := p.Consts.Length()
	vv := value.NewStrPtr(id)
	p.Consts.Append(vv)
	return resIndex
}

// Compile : バイトコードに変換
func Compile(sys *core.Core, n *node.Node) (*value.Value, error) {
	p := NewCompier(sys)
	err := p.Compile(n)
	if err != nil {
		return nil, err
	}
	if sys.IsDebug {
		fmt.Println(p.CodesToString(p.Codes))
		fmt.Println("[Run Code]")
	}
	if sys.IsCompile {
		return nil, nil
	}
	return p.Run()
}

func (p *TCompiler) rmCodes(i int) {
	if i < 0 || i >= len(p.Codes) {
		return
	}
	p.Codes = append(p.Codes[:i], p.Codes[i+1:]...)
}

// Optimize : 最適化処理
func (p *TCompiler) Optimize() {
	if p.sys.IsDebug {
		println("[Optimize]")
	}
	optCount := 0
	// 連続するFileInfoを削除
	lastType := NOP
	i := 0
	for i < len(p.Codes) {
		c := p.Codes[i]
		if c.Type == FileInfo && lastType == FileInfo {
			p.rmCodes(i - 1)
			continue
		}
		lastType = c.Type
		i++
	}
	// Const + Const + Add/Sub/Mul/Div = 最初に計算しておく
	i = 0
	for i < len(p.Codes) {
		c := p.Codes[i]
		if (c.Type == Add || c.Type == Sub || c.Type == Mul || c.Type == Div) && i >= 2 {
			c1 := p.Codes[i-2]
			c2 := p.Codes[i-1]
			if c1.Type == ConstO && c2.Type == ConstO {
				v1 := p.Consts.Get(c1.B)
				v2 := p.Consts.Get(c2.B)
				var v3 *value.Value
				if c.Type == Add {
					v3 = value.Add(v1, v2)
				} else if c.Type == Sub {
					v3 = value.Sub(v2, v1)
				} else if c.Type == Mul {
					v3 = value.Mul(v2, v1)
				} else if c.Type == Div {
					v3 = value.Div(v2, v1)
				}
				c3 := p.appendConsts(v3)
				c.Type = ConstO
				c.B = c3
				c.Memo = "=" + v3.ToString()
				p.rmCodes(i - 2) // c1
				p.rmCodes(i - 2) // c2
				i -= 2
				optCount++
				continue
			}
		}
		i++
	}
	// Const + GetLocal + Add/Sub + SetLocal = 短いOpcodeに変換
	// => IncLocal / DecLocal
	i = 0
	for i < len(p.Codes) {
		c := p.Codes[i]
		if (c.Type == SetLocal) && i >= 3 {
			c1 := p.Codes[i-3]
			c2 := p.Codes[i-2]
			c3 := p.Codes[i-1]
			if c1.Type == ConstO &&
				c2.Type == GetLocal &&
				(c3.Type == Add || c3.Type == Sub) {
				v1 := p.Consts.Get(c1.B)
				if v1.Type == value.Int {
					typ := IncLocal
					val := v1.ToInt()
					vno := c2.B
					varName := c2.Memo
					if c3.Type == Add {
					} else if c3.Type == Sub {
						typ = DecLocal
					}
					// IncLocal / DecLocal
					c1.Type = typ
					c1.A = vno
					c1.B = val
					c1.C = 0
					c1.Memo = varName
					i -= 2
					p.rmCodes(i) //
					p.rmCodes(i) //
					p.rmCodes(i) //
					optCount++
					continue
				}
			}
		}
		i++
	}
	// ConstO + GetLocal + Add/Sub = 短いOpcodeに変換
	// => GetLocalNAddInt / GetLocalNSubInt
	i = 0
	for i < len(p.Codes) {
		c := p.Codes[i]
		if (c.Type == Add || c.Type == Sub) && i >= 2 {
			c1 := p.Codes[i-2]
			c2 := p.Codes[i-1]
			if c1.Type == ConstO && c2.Type == GetLocal {
				v1 := p.Consts.Get(c1.B)
				if v1.Type == value.Int {
					typ := GetLocalNAdd
					val := v1.ToInt()
					vno := c2.B
					res := c.A
					varName := c2.Memo
					if c.Type == Add {
					} else if c.Type == Sub {
						typ = GetLocalNSub
					}
					// IncLocal / DecLocal
					c1.Type = typ
					c1.A = res
					c1.B = vno
					c1.C = val
					c1.Memo = varName + ":" + value.IntToStr(val)
					i--
					p.rmCodes(i)
					p.rmCodes(i)
					optCount++
					continue
				}
			}
		}
		i++
	}
	if p.sys.IsDebug {
		println("- count=", optCount)
	}
}

func (p *TCompiler) makeLabel(memo string) *TCode {
	c := TCode{Type: DefLabel, Memo: "■ " + memo}
	lbl := TCodeLabel{code: &c, memo: memo, addr: -1}
	c.A = len(p.Labels) // ラベル番号
	p.Labels = append(p.Labels, &lbl)
	return &c
}

func (p *TCompiler) makeJump(code *TCode) *TCode {
	c := TCode{Type: JumpLabel, A: code.A, Memo: "GoTo:" + p.Labels[code.A].memo}
	return &c
}

func (p *TCompiler) makeJumpWithMemo(code *TCode, memo string) *TCode {
	c := p.makeJump(code)
	c.Memo = memo + ">" + c.Memo
	return c
}

func (p *TCompiler) makeJumpIfTrue(exprR int, code *TCode) *TCode {
	c := TCode{Type: JumpLabelIfTrue, A: exprR, B: code.A}
	return &c
}

func (p *TCompiler) fixLabels(codes []*TCode) {
	// check label address
	for i, v := range codes {
		if v.Type == DefLabel {
			lbl := p.Labels[v.A]
			lbl.addr = i
		}
	}
	// ラベルジャンプから相対ジャンプへ変更
	for i, v := range codes {
		switch v.Type {
		case JumpLabel:
			v.Type = Jump
			lbl := p.Labels[v.A]
			v.A = lbl.addr - i
			v.Memo += ">Goto:" + lbl.memo
		case JumpLabelIfTrue:
			v.Type = JumpIfTrue
			lbl := p.Labels[v.B]
			v.B = lbl.addr - i
			v.Memo += ">Goto:" + lbl.memo
		}
	}
}
