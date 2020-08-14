package node

import (
	"fmt"
	"strings"
)

var nodeTypeNames = map[NType]string{
	Nop:          "Nop",
	TypeNodeList: "TypeNodeList",
	Const:        "値",
	Operator:     "演算",
	Sentence:     "文",
	Word:         "変数",
	CallFunc:     "関数呼出",
	Calc:         "計算",
	Let:          "代入",
	If:           "もし",
	Repeat:       "回",
	For:          "繰返",
	While:        "間",
	Continue:     "続",
	Break:        "抜",
	DefFunc:      "関数",
	Return:       "戻る",
	JSONArray:    "JSONArray",
	JSONHash:     "JSONHash",
	Foreach:      "反復",
	NodeVarIndex: "配列参照",
}

// ToString : Nodeの値をデバッグ用に出力する
func ToString(n Node, level int) string {
	indent := ""
	for i := 0; i < level; i++ {
		indent += "|  "
	}
	if n == nil {
		return indent + "(null)"
	}
	s := fmt.Sprintf("+ %03d: %s {%s}",
		n.GetFileInfo().Line,
		nodeTypeNames[n.GetType()],
		n.GetJosi())
	ss := ""
	switch n.GetType() {
	case Nop:
		np := n.(TNodeNop)
		s += " // " + np.Comment
	case Word:
		nw := n.(TNodeWord)
		s += nw.Name
		if nw.Index != nil {
			ss += ToString(*nw.Index, level+1)
		}
	case Const:
		cv := n.(TNodeConst).Value
		s += "(" + cv.ToString() + ")"
	case Operator:
		s += ":" + n.(TNodeOperator).Operator
		l := n.(TNodeOperator).Left
		r := n.(TNodeOperator).Right
		ss += ToString(l, level+1) + "\n"
		ss += ToString(r, level+1) + "\n"
	case Sentence:
		s += n.(TNodeSentence).Memo
		for _, v := range n.(TNodeSentence).List {
			ss += ToString(v, level+1) + "\n"
		}
	case NodeVarIndex:
		nlist := n.(TNodeVarIndex)
		s += fmt.Sprintf("(%d)", len(nlist.Items))
		for _, v := range nlist.Items {
			ss += ToString(v, level+1) + "\n"
		}
	case TypeNodeList:
		nlist := n.(TNodeList)
		s += fmt.Sprintf("(%d)", len(nlist))
		for _, v := range nlist {
			ss += ToString(v, level+1) + "\n"
		}
	case CallFunc:
		s += ":" + n.(TNodeCallFunc).Name
		for _, v := range n.(TNodeCallFunc).Args {
			ss += ToString(v, level+1) + "\n"
		}
	case Calc:
		nc := n.(TNodeCalc)
		ss += ToString(nc.Child, level+1) + "\n"
	case Let:
		nl := n.(TNodeLet)
		s += " " + nl.Name
		if nl.Index != nil && len(nl.Index.Items) > 0 {
			s += fmt.Sprintf("[]*%d", len(nl.Index.Items))
			for _, v := range nl.Index.Items {
				ss += ToString(v, level+1)
			}
		}
		ss += ToString(nl.ValueNode, level+1) + "\n"
	case If:
		ni := n.(TNodeIf)
		ss += ToString(ni.Expr, level+1) + "\n"
		ss += ToString(ni.TrueNode, level+1) + "\n"
		ss += ToString(ni.FalseNode, level+1) + "\n"
	case For:
		ni := n.(TNodeFor)
		s += fmt.Sprintf("%s", ni.Word)
		ss += ToString(ni.FromNode, level+1) + "\n"
		ss += ToString(ni.ToNode, level+1) + "\n"
		ss += ToString(ni.Block, level+1) + "\n"
	case Foreach:
		nn := n.(TNodeForeach)
		ss += ToString(nn.Expr, level+1) + "\n"
		ss += ToString(nn.Block, level+1) + "\n"
	case Repeat:
		nn := n.(TNodeRepeat)
		ss += ToString(nn.Expr, level+1) + "\n"
		ss += ToString(nn.Block, level+1) + "\n"
	case While:
		nn := n.(TNodeWhile)
		ss += ToString(nn.Expr, level+1) + "\n"
		ss += ToString(nn.Block, level+1) + "\n"
	case Continue:
		s += fmt.Sprintf("id=%d", n.(TNodeContinue).LoopID)
	case Break:
		s += fmt.Sprintf("id=%d", n.(TNodeBreak).LoopID)
	case Return:
		nn := n.(TNodeReturn)
		s += fmt.Sprintf("id=%d", nn.LoopID)
		ss += ToString(nn.Arg, level+1) + " --- ここまで「戻る」の引数\n"
	case DefFunc:
		nn := n.(TNodeDefFunc)
		s += fmt.Sprintf(" %s", nn.Word)
		ss += ToString(nn.Args, level+1) + " --- ここまで引数\n"
		ss += ToString(nn.Block, level+1) + " --- ここまでブロック\n"
	case JSONArray:
		nn := n.(TNodeJSONArray)
		ss += ToString(nn.Items, level+1) + "\n"
	default:
		s += " *"
	}
	if ss != "" {
		s += "\n" + strings.TrimRight(ss, " \t\n")
	}
	return indent + s
}
