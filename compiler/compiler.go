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
	rcount        int
	index         int
	length        int
	Line          int
	sys           *core.Core
}

// NewCompier : コンパイラオブジェクトを生成
func NewCompier(sys *core.Core) *TCompiler {
	p := TCompiler{}
	p.Codes = []*TCode{}
	p.Consts = value.TArray{}
	p.Labels = []*TCodeLabel{}
	p.UserFuncLabel = map[string]int{}
	p.rcount = 0
	p.index = 0
	p.sys = sys
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
	for _, v := range p.sys.UserFuncs {
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
	tmpRCount := p.rcount
	p.rcount = scope.Index
	cBlock, errBlock := p.convNode(&userNode.Block)
	if errBlock != nil {
		return nil, errBlock
	}
	c = append(c, cBlock...)
	c = append(c, p.makeGetLocal("それ"))
	c = append(c, NewCode(Return, p.rcount-1, 0, 0))
	c = append(c, labelEnd)
	// Close Local Scope
	p.sys.Scopes.Close()
	p.scope = p.sys.Scopes.GetTopScope()
	p.rcount = tmpRCount
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
	arrayIndex := p.rcount
	c := []*TCode{
		NewCodeMemo(NewArray, arrayIndex, 0, 0, "配列生成←関数の引数:"+fname),
	}
	p.rcount++
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
				argIndex := p.rcount - 1
				c = append(c, NewCodeMemo(AppendArray, arrayIndex, argIndex, 0, "引数追加"))
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
			c = append(c, NewCode(AppendArray, arrayIndex, p.rcount-1, 0))
			p.rcount--
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

	tmpRcount := p.rcount

	// 引数を得る
	argIndex, cArgs, err := p.getFuncArgs(cf.Name, funcV, cf.Args)
	if err != nil {
		return nil, err
	}
	c = append(c, cArgs...)
	// 関数を実行
	// システム関数
	funcRes := p.rcount
	p.rcount++
	fconstI := p.appendConsts(funcV)
	c = append(c, NewCodeMemo(CallFunc, funcRes, fconstI, argIndex, cf.Name))
	p.rcount = tmpRcount
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
	c = append(c, NewCodeMemo(CallUserFunc, p.rcount, funcLabel, argIndex, funcName))
	p.rcount++
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
	c = append(c, NewCodeMemo(Return, p.rcount-1, 0, 0, "戻る"))
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
	c = append(c, NewCode(NotReg, p.rcount-1, 0, 0))
	c = append(c, p.makeJumpIfTrue(p.rcount-1, labelEnd))
	p.rcount--
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
	c = append(c, p.makeJumpIfTrue(p.rcount-1, labelTrueBegin))
	p.rcount--

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
	// 現在のスコープに変数があるか
	scope := p.scope
	A := p.rcount
	p.rcount++
	B := scope.GetIndexByName(nn.Name)
	if B < 0 {
		v := value.NewValueNullPtr()
		B = scope.Set(nn.Name, v)
	}
	return []*TCode{NewCodeMemo(GetLocal, A, B, 0, nn.Name)}, nil
}

func (p *TCompiler) makeSetLocal(name string) *TCode {
	scope := p.scope
	A := scope.GetIndexByName(name)
	if A < 0 {
		scope.Set(name, value.NewValueNullPtr())
		A = scope.GetIndexByName(name)
	}
	B := p.rcount - 1
	p.rcount--
	return NewCodeMemo(SetLocal, A, B, 0, name)
}

func (p *TCompiler) makeGetLocal(name string) *TCode {
	scope := p.scope
	B := scope.GetIndexByName(name)
	if B < 0 {
		scope.Set(name, value.NewValueNullPtr())
		B = scope.GetIndexByName(name)
	}
	A := p.rcount
	p.rcount++
	return NewCodeMemo(GetLocal, A, B, 0, name)
}

func (p *TCompiler) convLet(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeLet)
	// value
	codes, err := p.convNode(&nn.ValueNode)
	if err != nil {
		return nil, CompileError("『"+nn.Name+"』の代入でエラー", n)
	}
	// SetLocal
	if nn.Index == nil || len(nn.Index) == 0 {
		codes = append(codes, p.makeSetLocal(nn.Name))
		return codes, nil
	}
	// TODO : index
	return nil, nil
}

func (p *TCompiler) convFor(n *node.Node) ([]*TCode, error) {
	nn := (*n).(node.TNodeFor)
	tmpRCount := p.rcount
	labelForBegin := p.makeLabel("FOR_BEGIN")
	c := []*TCode{labelForBegin}
	// varNo
	varName := nn.Word
	if varName == "" {
		varName = "対象"
	}

	// To
	toCodes, errTo := p.convNode(&nn.ToNode)
	if errTo != nil {
		return nil, CompileError("『繰返』構文の引数で。"+errTo.Error(), n)
	}
	c = append(c, toCodes...)
	toR := p.rcount - 1

	// From
	fromCodes, errFrom := p.convNode(&nn.FromNode)
	if errFrom != nil {
		return nil, CompileError("『繰返』構文の引数で。"+errTo.Error(), n)
	}
	c = append(c, fromCodes...)

	// WORD = fromR
	initVarCodes := p.makeSetLocal(varName)
	c = append(c, initVarCodes)

	// cond : IF WORD > TO then goto BlockEnd
	labelBlockEnd := p.makeLabel("FOR_BLOCK_END")
	labelCond := p.makeLabel("FOR_COND")
	cGetLocal := p.makeGetLocal(varName)

	c = append(c, labelCond)
	c = append(c, cGetLocal)
	varR := p.rcount - 1
	c = append(c, NewCodeMemo(Gt, p.rcount, varR, toR, "VAR > TO"))
	c = append(c, p.makeJumpIfTrue(p.rcount, labelBlockEnd))
	p.rcount--

	// Block
	blockCodes, errBlock := p.convNode(&nn.Block)
	if errBlock != nil {
		return nil, CompileError("『繰返』構文にて。"+errBlock.Error(), n)
	}
	c = append(c, blockCodes...)
	c = append(c, NewCode(IncLocal, cGetLocal.B, 0, 0)) // WORD++
	c = append(c, p.makeJump(labelCond))
	c = append(c, labelBlockEnd)
	p.fixLabels(c)
	p.rcount = tmpRCount
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

func (p *TCompiler) appendConsts(val *value.Value) int {
	// 同じ値があるか調べる
	for i, v := range p.Consts {
		if v.Type == val.Type {
			switch v.Type {
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
	idx := len(p.Consts)
	p.Consts = append(p.Consts, val)
	return idx
}

func (p *TCompiler) convConst(n *node.Node) ([]*TCode, error) {
	op := (*n).(node.TNodeConst)
	v := op.Value
	// push const
	cindex := len(p.Consts)
	p.Consts = append(p.Consts, &v)
	constO := NewCodeMemo(ConstO, p.rcount, cindex, 0, "="+v.ToString())
	p.rcount++
	codes := []*TCode{constO}
	return codes, nil
}

func (p *TCompiler) convOperator(n *node.Node) ([]*TCode, error) {
	op := (*n).(node.TNodeOperator)
	tmpRCount := p.rcount
	// Right node
	r, errR := p.convNode(&op.Right)
	if errR != nil {
		return nil, CompileError("演算エラー", n)
	}
	pcR := p.rcount - 1
	// Left node
	l, errL := p.convNode(&op.Left)
	if errL != nil {
		return nil, CompileError("演算エラー", n)
	}
	pcL := p.rcount - 1
	res := []*TCode{}
	res = append(res, r...)
	res = append(res, l...)
	//
	toindex := tmpRCount
	p.rcount = toindex + 1
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
	for i, v := range p.Consts {
		if v.Type == value.Str {
			if v.ToString() == id {
				return i
			}
		}
	}
	if !canCreate {
		return -1
	}
	resIndex := len(p.Consts)
	vv := value.NewValueStr(id)
	p.Consts = append(p.Consts, &vv)
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
			println("label[", v.A, "]=", lbl.memo, lbl.addr)
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
