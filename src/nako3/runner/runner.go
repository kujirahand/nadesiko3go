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
	// println(node.NodeToString(*n, 0))
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
	case node.If:
		return runIf(n)
	case node.Repeat:
		return runRepeat(n)
	case node.While:
		return runWhile(n)
	case node.For:
		return runFor(n)
	}
	// 未定義のノードを表示
	println("system error")
	println(node.NodeToString(*n, 0))
	return nil, RuntimeError("{システム}未実装のノード", n)
}

func runFor(n *node.Node) (*value.Value, error) {
	var lastValue *value.Value = nil
	nn := (*n).(node.NodeFor)
	// eval
	vFrom, err1 := runNode(&nn.FromNode)
	if err1 != nil {
		return nil, RuntimeError(
			"繰返の『から』でエラー:"+err1.Error(), n)
	}
	vTo, err2 := runNode(&nn.ToNode)
	if err2 != nil {
		return nil, RuntimeError(
			"繰返の『まで』でエラー:"+err2.Error(), n)
	}
	i := vFrom.ToInt()
	iTo := vTo.ToInt()
	// check Word
	var loopVar *value.Value = nil
	if nn.Word == "" {
		loopVar = &sys.Sore
	} else {
		// できるだけ再利用
		loopVar = sys.Globals.Get(nn.Word)
		if loopVar == nil {
			newVar := value.NewValueInt(0)
			sys.Globals.Set(nn.Word, &newVar)
			loopVar = &newVar
		}
	}
	for ; i <= iTo; i++ {
		loopVar.SetInt(i)
		v, err := runNode(&nn.Block)
		if err != nil {
			return nil, err
		}
		lastValue = v
	}
	return lastValue, nil
}

func runWhile(n *node.Node) (*value.Value, error) {
	var lastValue *value.Value = nil
	nn := (*n).(node.NodeWhile)
	for true {
		cond, err := runNode(&nn.Expr)
		if err != nil {
			return nil, err
		}
		if cond == nil {
			break
		}
		if cond.ToBool() {
			v, err := runNode(&nn.Block)
			if err != nil {
				return nil, err
			}
			lastValue = v
			continue
		}
		break
	}
	return lastValue, nil
}

func runRepeat(n *node.Node) (*value.Value, error) {
	ni := (*n).(node.NodeRepeat)
	// 回数を評価
	expr, err := runNode(&ni.Expr)
	if err != nil {
		return nil, RuntimeError("『回』構文の式でエラー。", n)
	}
	if expr == nil {
		return nil, nil
	}
	// 繰り返し
	var lastValue *value.Value = nil
	var errNode error = nil
	kaisu := expr.ToInt()
	for i := 0; i < int(kaisu); i++ {
		// 「それ」と「対象」を更新
		sys.Sore.SetInt(int64(i))
		sys.Taisyo.SetInt(int64(i))
		// 実行
		lastValue, errNode = runNode(&ni.Block)
		if errNode != nil {
			return nil, err
		}
	}
	return lastValue, err
}

func runIf(n *node.Node) (*value.Value, error) {
	ni := (*n).(node.NodeIf)
	// 条件を評価
	expr, err := runNode(&ni.Expr)
	if err != nil {
		return nil, RuntimeError("『もし』構文の条件式でエラー。", n)
	}
	if expr == nil {
		return runNode(&ni.FalseNode)
	}
	if expr.ToBool() {
		return runNode(&ni.TrueNode)
	} else {
		return runNode(&ni.FalseNode)
	}
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
		sys.Sore.SetValue(val)
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
		sys.Sore.SetValue(result)
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
	case "^":
		v = value.Exp(l, r)
	case "==", "=":
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
	case "かつ":
		v = value.And(l, r)
	case "または":
		v = value.Or(l, r)
	default:
		println("system error : 未定義:", op.Operator)
		return nil, RuntimeError(
			"(システム)未定義の演算子。"+op.Operator, n)
	}
	return &v, nil
}

func runConst(n *node.Node) (*value.Value, error) {
	nc := (*n).(node.NodeConst)
	return &nc.Value, nil
}
