package compiler

import (
	"fmt"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/node"
	"github.com/kujirahand/nadesiko3go/value"
)

const (
	// RegSize : レジスタサイズ
	RegSize = 10
)

// TCodeLabel : ジャンプ用のラベル管理
type TCodeLabel struct {
	code *TCode
	addr int
	memo string
}

// TCompiler : コンパイラオブジェクト
type TCompiler struct {
	Codes  []*TCode
	Consts value.TArray
	Reg    []*value.Value
	Labels []*TCodeLabel
	rcount int
	index  int
	length int
	sys    *core.Core
}

// NewCompier : コンパイラオブジェクトを生成
func NewCompier(sys *core.Core) *TCompiler {
	p := TCompiler{}
	p.Codes = []*TCode{}
	p.Consts = value.TArray{}
	p.Labels = []*TCodeLabel{}
	p.rcount = 0
	p.index = 0
	p.sys = sys

	// レジスタ領域を初期化
	p.Reg = make([]*value.Value, RegSize)

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
	codes, err := p.convNode(n)
	if err != nil {
		return err
	}
	p.fixLabels(codes)
	p.Codes = codes
	return nil
}

func (p *TCompiler) convNode(n *node.Node) ([]*TCode, error) {
	if n == nil {
		return nil, nil
	}
	ntype := (*n).GetType()
	switch ntype {
	case node.Nop:
		return nil, nil
	case node.DefFunc:
		return nil, nil
	case node.Word:
		return p.convWord(n)
	case node.TypeNodeList:
		return p.convNodeList(n)
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
	}
	println("[SYSTEM ERROR] Compile " + node.ToString(*n, 0))
	// panic(-1)
	return nil, nil
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
	scope := p.sys.Scopes.GetTopScope()
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
	scope := p.sys.Scopes.GetTopScope()
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
	scope := p.sys.Scopes.GetTopScope()
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
	codes := []*TCode{labelForBegin}
	// varNo
	varName := nn.Word
	if varName == "" {
		varName = "それ"
	}

	// To
	toCodes, errTo := p.convNode(&nn.ToNode)
	if errTo != nil {
		return nil, CompileError("『繰返』構文の引数で。"+errTo.Error(), n)
	}
	toR := p.rcount - 1
	codes = append(codes, toCodes...)

	// From
	fromCodes, errFrom := p.convNode(&nn.FromNode)
	if errFrom != nil {
		return nil, CompileError("『繰返』構文の引数で。"+errTo.Error(), n)
	}
	codes = append(codes, fromCodes...)

	// WORD = fromR
	initVarCodes := p.makeSetLocal(varName)
	codes = append(codes, initVarCodes)

	// cond : IF WORD > TO then goto BlockEnd
	labelBlockEnd := p.makeLabel("FOR_BLOCK_END")
	labelCond := p.makeLabel("FOR_COND")
	localCode := p.makeGetLocal(varName)
	localR := p.rcount - 1
	codes = append(codes, []*TCode{
		labelCond,
		localCode,
		NewCode(Gt, p.rcount, localR, toR),
		p.makeJumpIfTrue(p.rcount, labelBlockEnd),
	}...)
	p.rcount -= 2

	// Block
	blockCodes, errBlock := p.convNode(&nn.Block)
	if errBlock != nil {
		return nil, CompileError("『繰返』構文にて。"+errBlock.Error(), n)
	}
	codes = append(codes, blockCodes...)
	codes = append(codes, NewCode(IncLocal, localCode.B, 0, 0)) // WORD++
	codes = append(codes, p.makeJump(labelCond))
	codes = append(codes, labelBlockEnd)
	p.fixLabels(codes)
	p.rcount = tmpRCount
	return codes, nil
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
		println("[compile]")
		println(p.CodesToString(p.Codes))
	}
	return p.Run()
}

func (p *TCompiler) makeLabel(memo string) *TCode {
	c := TCode{Type: DefLabel, Memo: memo}
	lbl := TCodeLabel{code: &c, memo: memo, addr: -1}
	c.A = len(p.Labels) // ラベル番号
	p.Labels = append(p.Labels, &lbl)
	return &c
}

func (p *TCompiler) makeJump(code *TCode) *TCode {
	c := TCode{Type: JumpLabel, A: code.A}
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
