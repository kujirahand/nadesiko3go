package compiler

import (
	"fmt"

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
	Line          int
	sys           *core.Core
	forNest       int
}

// NewCompier : コンパイラオブジェクトを生成
func NewCompier(sys *core.Core) *TCompiler {
	p := TCompiler{}
	p.Codes = []*TCode{}
	p.Consts = value.NewTArrayDef(value.TArrayItems{
		value.NewValueIntPtr(0),
		value.NewValueIntPtr(1),
		value.NewValueIntPtr(2),
		value.NewValueIntPtr(3),
		value.NewValueIntPtr(4),
		value.NewValueIntPtr(5),
		value.NewValueIntPtr(6),
		value.NewValueIntPtr(7),
		value.NewValueIntPtr(8),
		value.NewValueIntPtr(9),
	})
	p.Labels = []*TCodeLabel{}
	p.UserFuncLabel = map[string]int{}
	p.index = 0
	p.sys = sys
	p.scope = sys.Scopes.GetTopScope()
	p.reg = p.scope.Reg
	return &p
}

// CompileError : コンパイルエラー
func CompileError(msg string, n *node.Node) error {
	var e error
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
	for _, v := range p.sys.UserFuncs.Items {
		funcID := v.IValue
		println("compile=", funcID)
		nodeDef := node.UserFunc[funcID]
		cDef, eDef := p.convDefFunc(&nodeDef)
		if eDef != nil {
			return CompileError(eDef.Error(), &nodeDef)
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
		return nil, nil
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
	case node.DefFunc:
		return nil, nil // 関数定義は Compile で最初に行う
	}
	println("[SYSTEM ERROR] Compile " + node.ToString(*n, 0))
	// panic(-1)
	return nil, nil
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

	// 関数名を取得
	funcV, err := p.getFunc(funcName)
	if err != nil {
		return nil, err
	}
	// User func
	userFuncIndex := funcV.Tag
	userNode := node.UserFunc[userFuncIndex].(node.TNodeDefFunc)
	// Open Local Scope
	scope := p.sys.Scopes.Open()
	p.scope = scope
	// 変数の登録(順番に注意)
	scope.Set("それ", value.NewValueNullPtr())
	scope.Reg.Set(metaRegReturnAddr, value.NewValueIntPtr(-1))
	scope.Reg.Set(metaRegReturnValue, value.NewValueIntPtr(-1))
	scope.Index = 2
	// スコープにローカル変数を挿入 (順番が重要)
	for _, name := range userNode.ArgNames {
		scope.Set(name, value.NewValueNullPtr())
		args = append(args, name)
	}
	codeLabel.argNames = args
	// ローカルスコープに「それ」を配置
	localSore := value.NewValueNullPtr()
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

func (p *TCompiler) getFuncArgs(fname string, funcV *value.Value, nodeArgs node.TNodeList) (int, []*TCode, error) {
	// 関数の引数を得る
	defArgs := p.sys.JosiList[funcV.Tag]    // 定義
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
				if josi != nodeJosi.GetJosi() { // 助詞が一致しない
					continue
				}
				usedArgs[k] = true
				cArg, err1 := p.convNode(&nodeJosi)
				if err1 != nil {
					msg := fmt.Errorf("関数『%s』引数でエラー。%s", fname, err1.Error())
					return -1, nil, msg
				}
				c = append(c, cArg...)
				// argIndex := p.regBack()
				// c = append(c, NewCodeMemo(AppendArray, arrayIndex, argIndex, 0, fname+"の引数追加"))
			}
		}
	}
	// 引数のチェック (1) 漏れなくcf.Args内のノードを評価したか
	for ci, b := range usedArgs {
		if b == false {
			msgArg := fmt.Errorf("関数『%s』の第%d引数の間違い。", fname, ci)
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
			// c = append(c, NewCode(AppendArray, arrayIndex, p.regBack(), 0))
		} else {
			return -1, nil, fmt.Errorf("関数『%s』で引数の数が違います。", fname)
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
		return nil, err
	}
	// ユーザー関数の場合
	if funcV.Type == value.UserFunc {
		return p.callUserFunc(cf, funcV)
	}

	tmpRcount := p.regTop()

	// 引数を得る
	argIndex, cArgs, err := p.getFuncArgs(cf.Name, funcV, cf.Args)
	if err != nil {
		return nil, err
	}
	c = append(c, cArgs...)
	// 関数を実行
	// システム関数
	funcRes := p.regTop()
	fconstI := p.appendConsts(funcV)
	c = append(c, NewCodeMemo(CallFunc, funcRes, fconstI, argIndex, cf.Name))
	p.scope.Index = tmpRcount
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
	argIndex, cArgs, err := p.getFuncArgs(cf.Name, funcV, cf.Args)
	if err != nil {
		return nil, err
	}
	c = append(c, cArgs...)
	c = append(c, NewCodeMemo(CallUserFunc, p.regNext(), funcLabel, argIndex, funcName))
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
	labelEnd := p.makeLabel("FOREACH_END")
	// expr
	cExpr, errExpr := p.convNode(&nn.Expr)
	if errExpr != nil {
		return nil, CompileError("反復の条件式で。"+errExpr.Error(), n)
	}
	c = append(c, cExpr...)
	rExpr := p.regTop() - 1
	// len(expr)
	// rLenExpr := p.regNext()
	// c = append(c, NewCodeMemo(Length, rLenExpr, rExpr, 0, "LEN_EXPR"))
	// $N=0
	rI := p.regNext()
	c = append(c, p.makeConstInt(rI, 0))
	// COND
	labelCond := p.makeLabel("FOREACH_COND")
	c = append(c, labelCond)
	// $N >= len(expr) IfTrue=>END
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

func (p *TCompiler) convCalc(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeCalc)
	return p.convNode(&nn.Child)
}

func (p *TCompiler) convWhile(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeWhile)
	labelBegin := p.makeLabel("WHILE_BEGIN")
	labelEnd := p.makeLabel("WHILE_END")
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
		v := value.NewValueNullPtr()
		varNo = scope.Set(nn.Name, v)
	}
	c = append(c, NewCodeMemo(GetLocal, toReg, varNo, 0, nn.Name))
	// 配列アクセス
	if nn.Index != nil {
		for _, vNode := range nn.Index {
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

func (p *TCompiler) makeSetLocal(name string) *TCode {
	scope := p.scope
	A := scope.GetIndexByName(name)
	if A < 0 {
		scope.Set(name, value.NewValueNullPtr())
		A = scope.GetIndexByName(name)
	}
	B := p.regBack()
	return NewCodeMemo(SetLocal, A, B, 0, name)
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
		idxNull := p.appendConsts(value.NewValueNullPtr())
		return NewCodeMemo(ConstO, A, idxNull, 0, "未定義の変数:"+name)
	}
	return NewCodeMemo(GetLocal, A, B, 0, name) // ローカル変数
}

func (p *TCompiler) convLet(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeLet)
	// value
	c, err := p.convNode(&nn.ValueNode)
	if err != nil {
		return nil, CompileError("『"+nn.Name+"』の代入でエラー", n)
	}
	valueR := p.regTop() - 1

	// SetLocal (Indexがない場合)
	if nn.Index == nil || len(nn.Index) == 0 {
		c = append(c, p.makeSetLocal(nn.Name))
		return c, nil
	}

	// Indexがある場合
	c = append(c, p.makeGetLocal(nn.Name))
	varR := p.regTop() - 1
	for _, exprNode := range nn.Index {
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
	labelForBegin := p.makeLabel("FOR_BEGIN")
	c := []*TCode{labelForBegin}

	// varNo
	varName := nn.Word
	if varName == "" {
		varName = "__FOR_I" + value.IntToStr(p.forNest)
		p.forNest++
	}

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
	labelBlockEnd := p.makeLabel("FOR_BLOCK_END")
	labelCond := p.makeLabel("FOR_COND")
	c = append(c, labelCond)

	getLoopVar := p.makeGetLocal(varName)
	c = append(c, getLoopVar)
	varR := p.regTop() - 1

	varExpr := p.regTop()
	c = append(c, NewCodeMemo(Gt, varExpr, varR, toR, "VAR > TO"))
	c = append(c, p.makeJumpIfTrue(varExpr, labelBlockEnd))

	// それに値を設定
	c = append(c, p.makeSetSore(varR))

	// Block
	blockCodes, errBlock := p.convNode(&nn.Block)
	if errBlock != nil {
		return nil, CompileError("『繰返』構文にて。"+errBlock.Error(), n)
	}
	c = append(c, blockCodes...)
	c = append(c, NewCode(IncLocal, getLoopVar.B, 0, 0)) // WORD++
	c = append(c, p.makeJump(labelCond))
	c = append(c, labelBlockEnd)
	p.scope.Index = tmpRCount
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
	for i, v := range p.Consts.Items {
		if v.Type == value.Int && v.ToInt() == num {
			return i
		}
	}
	// なければ追加
	val := value.NewValueInt(num)
	idx := p.Consts.Length()
	p.Consts.Append(&val)
	return idx
}

func (p *TCompiler) appendConstsStr(s string) int {
	// 同じ値があるか調べる
	for i, v := range p.Consts.Items {
		if v.Type == value.Str && v.ToString() == s {
			return i
		}
	}
	// なければ追加
	val := value.NewValueStr(s)
	idx := p.Consts.Length()
	p.Consts.Append(&val)
	return idx
}

func (p *TCompiler) appendConsts(val *value.Value) int {
	// 同じ値があるか調べる
	for i, v := range p.Consts.Items {
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
				if v.Tag == val.Tag {
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
		ci := p.appendConstsStr(v.ToString())
		return []*TCode{NewCodeMemo(ConstO, regI, ci, 0, "="+v.ToString())}, nil
	default:
		ci := p.appendConsts(&v)
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
	}
	return res, nil
}

func (p *TCompiler) getConstNoByID(id string, canCreate bool) int {
	for i, v := range p.Consts.Items {
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
	vv := value.NewValueStr(id)
	p.Consts.Append(&vv)
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
		println(p.CodesToString(p.Codes))
		println("[Run Code]")
	}
	return p.Run()
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
		case JumpLabelIfTrue:
			v.Type = JumpIfTrue
			lbl := p.Labels[v.B]
			v.B = lbl.addr - i
		}
	}
}
