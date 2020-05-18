package runner

import (
	"fmt"
	"nako3/core"
	"nako3/node"
	"nako3/value"
)

// Core
var sys *core.Core = nil

func SetCore(co *core.Core) {
	sys = co
}

func Run(n *node.Node) (*value.Value, error) {
	if sys == nil {
		panic("need call SetCore")
	}
	return runNode(n)
}

func RunError(msg string, n *node.Node) error {
	e := fmt.Errorf("[実行時]" + msg)
	return e
}

func runNodeList(nodes node.NodeList) (*value.Value, error) {
	var lastValue *value.Value = nil
	for _, n := range nodes {
		v, err := runNode(&n)
		if err != nil {
			return v, err
		}
		if v != nil {
			lastValue = v
		}
	}
	return lastValue, nil
}

func runNode(n *node.Node) (*value.Value, error) {
	switch (*n).GetType() {
	case node.Nop:
		return nil, nil
	case node.TNodeList:
		nlist := (*n).(node.NodeList)
		return runNodeList(nlist)
	case node.Const:
		return runConst(n)
	case node.Operator:
		return runOperator(n)
	case node.Sentence:
		return runSentence(n)
	case node.CallFunc:
		return runCallFunc(n)
	}
	return nil, RunError("{システム}未実装のノード", n)
}

func runCallFunc(n *node.Node) (*value.Value, error) {
	cf := (*n).(node.NodeCallFunc)
	// args
	args := make(value.ValueArray, len(cf.Args))
	for i, v := range cf.Args {
		argResult, err1 := runNode(&v)
		if err1 != nil {
			msg := fmt.Sprintf("関数『%s』の引数でエラー。", cf.Name)
			return nil, RunError(err1.Error()+msg, n)
		}
		args[i] = *argResult
	}
	// find
	funcV := cf.Cache
	if funcV == nil {
		funcV = sys.GlobalVars.Get(cf.Name)
		cf.Cache = funcV
	}
	if funcV == nil {
		msgu := fmt.Sprintf("関数『%s』は未定義。", cf.Name)
		return nil, RunError(msgu, n)
	}
	result, err2 := funcV.Value.(value.ValueFunc)(args)
	return result, err2
}

func runSentence(n *node.Node) (*value.Value, error) {
	se := (*n).(node.NodeSentence)
	return runNodeList(se.List)
}

func runOperator(n *node.Node) (*value.Value, error) {
	op := (*n).(node.NodeOperator)
	var v value.Value
	r, err1 := runNode(&op.Right)
	if err1 != nil {
		return nil, RunError(err1.Error()+"演算"+op.Operator, n)
	}
	l, err2 := runNode(&op.Left)
	if err2 != nil {
		return nil, RunError(err1.Error()+"演算"+op.Operator, n)
	}
	if r == nil {
		rNull := value.NewValueNull()
		r = &rNull
	}
	if l == nil {
		rNull := value.NewValueNull()
		l = &rNull
	}
	switch op.Operator {
	case "+":
		v = value.Add(l, r)
	case "-":
		v = value.Sub(l, r)
	case "*":
		v = value.Mul(l, r)
	case "/":
		v = value.Div(l, r)
	case "%":
		v = value.Mod(l, r)
	default:
		return nil, RunError("(システム)未定義の演算子。", n)
	}
	return &v, nil
}

func runConst(n *node.Node) (*value.Value, error) {
	nc := (*n).(node.NodeConst)
	return &nc.Value, nil
}
