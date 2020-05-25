package runner

import (
	"fmt"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/lexer"
	"github.com/kujirahand/nadesiko3go/node"
	"github.com/kujirahand/nadesiko3go/value"
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

func runNodeList(nodes node.TNodeList) (*value.Value, error) {
	var lastValue *value.Value = nil
	for _, n := range nodes {
		if sys.BreakID >= 0 || sys.ContinueID >= 0 {
			break
		}
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
	case node.DefFunc:
		return nil, nil
	case node.Calc:
		nchild := (*n).(node.TNodeCalc)
		return runNode(&nchild.Child)
	case node.TypeNodeList:
		nlist := (*n).(node.TNodeList)
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
	case node.Continue:
		return runContinue(n)
	case node.Break:
		return runBreak(n)
	case node.Return:
		return runReturn(n)
	case node.JSONArray:
		return runJSONArray(n)
	case node.JSONHash:
		return runJSONHash(n)
	}
	// 未定義のノードを表示
	println("system error")
	println(node.ToString(*n, 0))
	return nil, RuntimeError("{システム}未実装のノード", n)
}

func runFor(n *node.Node) (*value.Value, error) {
	var lastValue *value.Value = nil
	nn := (*n).(node.TNodeFor)
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
		loopVar = sys.Global.Get(nn.Word)
		if loopVar == nil {
			newVar := value.NewValueInt(0)
			sys.Global.Set(nn.Word, &newVar)
			loopVar = &newVar
		}
	}
	// LOOP
	sys.LoopLevel++
	for ; i <= iTo; i++ {
		loopVar.SetInt(i)
		v, err := runNode(&nn.Block)
		if err != nil {
			return nil, err
		}
		lastValue = v
		// check break
		if sys.BreakID >= 0 {
			sys.BreakID = -1
			break
		}
	}
	if sys.ContinueID == sys.LoopLevel {
		sys.ContinueID = -1
	}
	sys.LoopLevel--
	return lastValue, nil
}

func runWhile(n *node.Node) (*value.Value, error) {
	var lastValue *value.Value = nil
	nn := (*n).(node.TNodeWhile)
	sys.LoopLevel++
	for true {
		// break ?
		if sys.BreakID >= 0 || sys.ReturnID >= 0 {
			if sys.BreakID == sys.LoopLevel {
				sys.BreakID = -1
			}
			break
		}
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
	if sys.ContinueID == sys.LoopLevel {
		sys.ContinueID = -1
	}
	sys.LoopLevel--
	return lastValue, nil
}

func runRepeat(n *node.Node) (*value.Value, error) {
	ni := (*n).(node.TNodeRepeat)
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
	sys.LoopLevel++
	kaisu := expr.ToInt()
	for i := 0; i < int(kaisu); i++ {
		if sys.BreakID >= 0 || sys.ReturnID >= 0 {
			if sys.BreakID == sys.LoopLevel {
				sys.BreakID = -1
			}
			break
		}
		// 「それ」と「対象」を更新
		sys.Sore.SetInt(int64(i))
		sys.Taisyo.SetInt(int64(i))
		// 実行
		lastValue, errNode = runNode(&ni.Block)
		if errNode != nil {
			sys.LoopLevel--
			return nil, err
		}
	}
	if sys.ContinueID >= 0 {
		sys.ContinueID = -1
	}
	sys.LoopLevel--
	return lastValue, err
}

func runIf(n *node.Node) (*value.Value, error) {
	ni := (*n).(node.TNodeIf)
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
	}
	return runNode(&ni.FalseNode)
}

func runLet(n *node.Node) (*value.Value, error) {
	cl := (*n).(node.TNodeLet)
	// 変数に代入する値を評価する
	val, err := runNode(&cl.Value)
	if err != nil {
		return nil, err
	}

	// 普通に変数に代入する場合
	if cl.VarIndex == nil || len(cl.VarIndex) == 0 {
		// 既にこのレベル以下に変数がある？
		vv := sys.Scopes.Get(cl.Var)
		if vv != nil {
			vv.SetValue(val)
		} else {
			sys.Scopes.SetTopVars(cl.Var, val)
		}
		return val, nil
	}

	// 配列など参照に代入する場合
	vv := sys.Scopes.Get(cl.Var)
	if vv == nil {
		vv = value.NewValueNullPtr()
		sys.Scopes.SetTopVars(cl.Var, vv)
	}
	// 添字へのアクセス
	for i, nIndex := range cl.VarIndex {
		iv, err := runNode(&nIndex)
		if err != nil {
			return nil, RuntimeError("代入の添字の評価でエラー:"+err.Error(), &nIndex)
		}
		if vv == nil {
			return nil, RuntimeError("代入時NULLに対する添字アクセス", n)
		}
		if vv.Type == value.Array {
			idx := int(iv.ToInt())
			if i == len(cl.VarIndex)-1 {
				vv.ArraySet(idx, val)
			} else {
				vv = vv.ArrayGet(idx)
			}
			continue
		}
		if vv.Type == value.Hash {
			key := iv.ToString()
			if i == len(cl.VarIndex)-1 {
				vv.HashSet(key, val)
			} else {
				vv = vv.HashGet(key)
			}
		}
	}
	return val, nil
}

func runWord(n *node.Node) (*value.Value, error) {
	cw := (*n).(node.TNodeWord)
	// 関数の実態を得る
	val := cw.Cache
	if val == nil {
		val = sys.Scopes.Get(cw.Name)
		cw.Cache = val
	}
	// 配列アクセスが不要な時
	if cw.Index == nil || len(cw.Index) == 0 {
		return val, nil
	}
	// 添字を一つずつ取り出していく
	for _, nIndex := range cw.Index {
		i, err := runNode(&nIndex)
		if err != nil {
			return nil, RuntimeError(fmt.Sprintf("配列添字の値参照でエラー:%s", err.Error()), &nIndex)
		}
		if val == nil {
			return nil, RuntimeError("NULLに対する配列アクセス", n)
		}
		if val.Type == value.Array {
			val = val.ArrayGet(int(i.ToInt()))
			continue
		}
		if val.Type == value.Hash {
			val = val.HashGet(i.ToString())
		}
	}
	return val, nil
}

func runCallFunc(n *node.Node) (*value.Value, error) {
	cf := (*n).(node.TNodeCallFunc)
	// 関数の実態を得る
	funcV := cf.Cache
	if funcV == nil {
		funcV = sys.Scopes.Get(cf.Name)
		cf.Cache = funcV
	}
	// 変数が見当たらない
	if funcV == nil {
		msgu := fmt.Sprintf("関数『%s』は未定義。", cf.Name)
		return nil, RuntimeError(msgu, n)
	}
	// 関数ではない？
	if !funcV.IsFunction() {
		msgn := fmt.Sprintf("『%s』は関数ではい。", cf.Name)
		return nil, RuntimeError(msgn, n)
	}
	// args
	defArgs := sys.JosiList[funcV.Tag]       // 定義
	args := make(value.TArray, len(defArgs)) // 関数に与える値
	nodeArgs := cf.Args                      // ノードの値
	usedArgs := make([]bool, len(nodeArgs))  // ノードを利用したか(同じ助詞が二つある場合)
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
					args[i] = argResult
				} else {
					args[i] = value.NewValueNullPtr()
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
			args[0] = &sys.Sore
		} else {
			return nil, RuntimeError(fmt.Sprintf("関数『%s』で引数の数が違います。", cf.Name), n)
		}
	}
	// 関数を実行
	var result *value.Value = nil
	var err2 error = nil
	if funcV.Type == value.UserFunc {
		// User func
		index := funcV.Tag
		userNode := node.UserFunc[index].(node.TNodeDefFunc)
		sys.Scopes.Open()
		// スコープにローカル変数を挿入
		scope := sys.Scopes.GetTopScope()
		for i, v := range userNode.ArgNames {
			scope.Set(v, args[i])
		}
		sys.LoopLevel++
		result, err2 = runNode(&userNode.Block)
		sys.LoopLevel--
		sys.Scopes.Close()
	} else {
		// Go func
		f := funcV.Value.(value.TFunction)
		result, err2 = f(args)
	}
	// 結果をそれに覚える
	if result != nil {
		sys.Sore.SetValue(result)
	}
	return result, err2
}

func runSentence(n *node.Node) (*value.Value, error) {
	se := (*n).(node.TNodeSentence)
	return runNodeList(se.List)
}

func runOperator(n *node.Node) (*value.Value, error) {
	op := (*n).(node.TNodeOperator)
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
	case "&":
		v = value.AddStr(l, r)
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

func runJSONArray(n *node.Node) (*value.Value, error) {
	nn := (*n).(node.TNodeJSONArray)
	res := value.NewValueArray()
	for i, vNode := range nn.Items {
		val, err := runNode(&vNode)
		if err != nil {
			return nil, RuntimeError(fmt.Sprintf("JSONArray[%d]の評価: %s", i, err.Error()), n)
		}
		res.Append(val)
	}
	return &res, nil
}

func runJSONHash(n *node.Node) (*value.Value, error) {
	nn := (*n).(node.TNodeJSONHash)
	res := value.NewValueHash()
	for k, vNode := range nn.Items {
		val, err := runNode(&vNode)
		if err != nil {
			return nil, RuntimeError(fmt.Sprintf("JSONHash[%s]の評価: %s", k, err.Error()), n)
		}
		res.HashSet(k, val)
	}
	return &res, nil
}

func runBreak(n *node.Node) (*value.Value, error) {
	sys.BreakID = sys.LoopLevel
	return nil, nil
}

func runContinue(n *node.Node) (*value.Value, error) {
	sys.ContinueID = sys.LoopLevel
	return nil, nil
}

func runReturn(n *node.Node) (*value.Value, error) {
	var result *value.Value = nil
	var err error = nil
	nn := (*n).(node.TNodeReturn)
	if nn.Arg != nil {
		result, err = runNode(&nn.Arg)
	}
	sys.ReturnID = sys.LoopLevel
	return result, err
}

func runConst(n *node.Node) (*value.Value, error) {
	nc := (*n).(node.TNodeConst)
	if !nc.IsExtend {
		return &nc.Value, nil
	}
	// 拡張
	result := ""
	runes := nc.Value.ToRunes()
	i := 0
	for i < len(runes) {
		c := runes[i]
		if c != '{' {
			result += string(c)
			i++
			continue
		}
		i++
		e := ""
		for i < len(runes) {
			c = runes[i]
			if c != '}' {
				e += string(c)
				i++
				continue
			}
			i++
			break
		}
		e = lexer.DeleteOkurigana(e)
		ev := sys.Scopes.Get(e)
		if ev != nil {
			result += ev.ToString()
		}
	}
	resultValue := value.NewValueStr(result)
	return &resultValue, nil
}
