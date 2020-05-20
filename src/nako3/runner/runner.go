package runner

import (
	"fmt"
	"nako3/core"
	"nako3/node"
	"nako3/value"
)

// Core
var sys *core.Core = nil

// Run : ノードを実行する
func Run(n *node.Node) (*value.Value, error) {
	if sys == nil {
		sys = core.GetSystem()
	}
	return runNode(n)
}

// RuntimeError : 実行時エラーを生成
func RuntimeError(msg string, n *node.Node) error {
	var e error
	if n != nil {
		fi := (*n).GetFileInfo()
		e = fmt.Errorf("[実行時エラー] (%d) %s", fi.Line, msg)
	} else {
		e = fmt.Errorf("[実行時エラー] " + msg)
	}
	return e
}

func runNodeList(nodes node.NodeList) (*value.Value, error) {
	var lastValue *value.Value = nil
	for _, n := range nodes {
		v, err := runNode(&n)
		if err != nil {
			return v, err
		}
		lastValue = v
	}
	return lastValue, nil
}

func runNode(n *node.Node) (*value.Value, error) {
	if n == nil {
		return nil, nil
	}
	switch (*n).GetType() {
	case node.Nop:
		return nil, nil
	case node.Calc:
		nchild := (*n).(node.NodeCalc)
		return runNode(&nchild.Child)
	case node.TypeNodeList:
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
	case node.Word:
		return runWord(n)
	case node.Let:
		return runLet(n)
	}
	return nil, RuntimeError("{システム}未実装のノード", n)
}

func runLet(n *node.Node) (*value.Value, error) {
	cl := (*n).(node.NodeLet)
	// 変数に代入する値を評価する
	val, err := runNode(&cl.Value)
	if err != nil {
		return nil, err
	}
	// 普通に変数に代入する場合
	if len(cl.VarIndex) == 0 {
		sys.Globals.Set(cl.Var, val)
		sys.Sore = *val
		return val, nil
	}
	// TODO: 配列など参照に代入する場合
	panic("開発中")
}

func runWord(n *node.Node) (*value.Value, error) {
	cw := (*n).(node.NodeWord)
	// 関数の実態を得る
	val := cw.Cache
	if val == nil {
		val = sys.Globals.Get(cw.Name)
		cw.Cache = val
	}
	return val, nil
}

func runCallFunc(n *node.Node) (*value.Value, error) {
	cf := (*n).(node.NodeCallFunc)
	// 関数の実態を得る
	funcV := cf.Cache
	if funcV == nil {
		funcV = sys.Globals.Get(cf.Name)
		cf.Cache = funcV
	}
	// 変数が見当たらない
	if funcV == nil {
		msgu := fmt.Sprintf("関数『%s』は未定義。", cf.Name)
		return nil, RuntimeError(msgu, n)
	}
	// 関数ではない？
	if funcV.Type != value.Function {
		msgn := fmt.Sprintf("『%s』は関数ではい。", cf.Name)
		return nil, RuntimeError(msgn, n)
	}
	// args
	defArgs := sys.JosiList[funcV.Tag]           // 定義
	args := make(value.ValueArray, len(defArgs)) // 関数に与える値
	nodeArgs := cf.Args                          // ノードの値
	usedArgs := make([]bool, len(nodeArgs))      // ノードを利用したか(同じ助詞が二つある場合)
	for bi := 0; bi < len(usedArgs); bi++ {
		usedArgs[bi] = false
	}
	for i, josiList := range defArgs {
		for _, josi := range josiList {
			for k, nodeJosi := range nodeArgs {
				if usedArgs[k] {
					continue
				}
				if josi != nodeJosi.GetJosi() { // 助詞が一致しない
					continue
				}
				usedArgs[k] = true
				argResult, err1 := runNode(&nodeJosi)
				if err1 != nil {
					msg := fmt.Sprintf("関数『%s』の引数でエラー。", cf.Name)
					return nil, RuntimeError(err1.Error()+msg, n)
				}
				if argResult != nil {
					args[i] = *argResult
				} else {
					args[i] = value.NewValueNull()
				}
			}
		}
	}
	// 引数のチェック (1) 漏れなくcf.Args内のノードを評価したか
	for ci, b := range usedArgs {
		if b == false {
			msgArg := fmt.Sprintf("関数『%s』の第%d引数の間違い。", cf.Name, ci)
			return nil, RuntimeError(msgArg, n)
		}
	}
	// 引数のチェック (2) 関数定義引数(defArgs)と数が合っているか？
	// 		特定として 引数-1であれば、変数「それ」の値を補う
	// fmt.Printf("args: %d=%d", len(nodeArgs), len(defArgs))
	if len(nodeArgs) != len(defArgs) {
		// 特例ルール -- 「それ」を補完する
		if len(nodeArgs) == (len(defArgs) - 1) {
			args[0] = sys.Sore
		} else {
			return nil, RuntimeError(fmt.Sprintf("関数『%s』で引数の数が違います。", cf.Name), n)
		}
	}
	// 関数を実行
	result, err2 := funcV.Value.(value.ValueFunc)(args)
	// 結果をそれに覚える
	if result != nil {
		sys.Sore = *result
		sys.Globals.Set("それ", &sys.Sore)
	}
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
		return nil, RuntimeError(err1.Error()+"演算"+op.Operator, n)
	}
	l, err2 := runNode(&op.Left)
	if err2 != nil {
		return nil, RuntimeError(err1.Error()+"演算"+op.Operator, n)
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
	case "==":
		v = value.EqEq(l, r)
	case "!=":
		v = value.NtEq(l, r)
	case ">":
		v = value.Gt(l, r)
	case ">=":
		v = value.GtEq(l, r)
	case "<":
		v = value.Lt(l, r)
	case "<=":
		v = value.LtEq(l, r)
	default:
		return nil, RuntimeError("(システム)未定義の演算子。", n)
	}
	return &v, nil
}

func runConst(n *node.Node) (*value.Value, error) {
	nc := (*n).(node.NodeConst)
	return &nc.Value, nil
}
