package node

import (
	"fmt"
	"strings"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/token"
	"github.com/kujirahand/nadesiko3go/value"
)

// NType : Nodeの種類
type NType int

const (
	// Nop : 何もしない
	Nop NType = iota
	// TypeNodeList : Nodeのリスト
	TypeNodeList
	// Const : 定数
	Const
	// Operator : 演算子
	Operator
	// Sentence : 文
	Sentence
	// Word : 変数など
	Word
	// CallFunc : 関数呼び出し
	CallFunc
	// Calc : カッコ
	Calc
	// Let : 代入文
	Let
	// If : もし
	If
	// Repeat : n回
	Repeat
	// For : 繰り返す
	For
	// While : 条件繰り返し
	While
	// Continue : 続ける
	Continue
	// Break : 抜ける
	Break
	// DefFunc : 関数定義
	DefFunc
	// Return : 戻る
	Return
	// JSONArray : array
	JSONArray
	// JSONHash : hash
	JSONHash
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
}

// Node : Node Interface
type Node interface {
	GetType() NType
	GetJosi() string
	GetFileInfo() core.TFileInfo
}

// TNodeList : Node List
type TNodeList []Node

// GetType : 型名取得
func (n TNodeList) GetType() NType { return TypeNodeList }

// GetFileInfo : ファイル情報取得
func (n TNodeList) GetFileInfo() core.TFileInfo { return core.TFileInfo{} }

// GetJosi : 助詞取得
func (n TNodeList) GetJosi() string { return "" }

// NewNodeList : 新規TNodeListを取得
func NewNodeList() TNodeList {
	return TNodeList{}
}

// UserFunc : ユーザー関数の一覧
var UserFunc = map[int]Node{}

// TNodeNop : NOP
type TNodeNop struct {
	Node
	Comment  string
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeNop) GetType() NType { return Nop }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeNop) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeNop) GetJosi() string { return n.Josi }

// NewNodeNop : NOP Node
func NewNodeNop(t *token.Token) Node {
	n := TNodeNop{
		Comment:  t.Literal,
		FileInfo: t.FileInfo,
		Josi:     t.Josi,
	}
	return n
}

// TNodeCalc : Calc
type TNodeCalc struct {
	Node
	Josi     string
	Child    Node
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeCalc) GetType() NType { return Calc }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeCalc) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeCalc) GetJosi() string { return n.Josi }

// NewNodeCalc : TNodeCalcを生成
func NewNodeCalc(t *token.Token, child Node) Node {
	n := TNodeCalc{
		FileInfo: t.FileInfo,
		Josi:     t.Josi,
		Child:    child,
	}
	return n
}

// TNodeLet :
type TNodeLet struct {
	Node
	Var      string
	VarIndex TNodeList
	Value    Node
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeLet) GetType() NType { return Let }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeLet) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeLet) GetJosi() string { return n.Josi }

// NewNodeLet : TNodeLetを返す
func NewNodeLet(t *token.Token, index TNodeList, value Node) Node {
	n := TNodeLet{
		Var:      t.Literal,
		VarIndex: index,
		Value:    value,
		FileInfo: t.FileInfo,
		Josi:     t.Josi,
	}
	return n
}

// TNodeConst : Const
type TNodeConst struct {
	Node
	Value    value.Value
	IsExtend bool
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeConst) GetType() NType { return Const }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeConst) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeConst) GetJosi() string { return n.Josi }

// NewNodeConst : TNodeConstを返す
func NewNodeConst(vtype value.Type, t *token.Token) TNodeConst {
	node := TNodeConst{
		Value:    value.NewValueByType(vtype, t.Literal),
		IsExtend: false,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// NewNodeConstInt : 整数型のConstノードを返す
func NewNodeConstInt(t *token.Token, num int) TNodeConst {
	node := TNodeConst{
		Value:    value.NewValueInt(int64(num)),
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// NewNodeConstEx : STRING_EX型に対応するノードを返す
func NewNodeConstEx(vtype value.Type, t *token.Token) TNodeConst {
	node := NewNodeConst(vtype, t)
	node.IsExtend = true
	return node
}

// TNodeOperator : 演算子型のノード
type TNodeOperator struct {
	Node
	Left     Node
	Right    Node
	Operator string
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeOperator) GetType() NType { return Operator }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeOperator) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeOperator) GetJosi() string { return n.Josi }

// NewNodeOperator : TNodeOperatorを返す
func NewNodeOperator(op *token.Token, left Node, right Node) TNodeOperator {
	p := TNodeOperator{
		Left:     left,
		Right:    right,
		Operator: op.Literal,
		Josi:     right.GetJosi(),
		FileInfo: left.GetFileInfo(),
	}
	return p
}

// TNodeSentence : TNodeSentence
type TNodeSentence struct {
	Node
	Memo     string
	List     TNodeList
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeSentence) GetType() NType { return Sentence }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeSentence) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeSentence) GetJosi() string { return n.Josi }

// NewNodeSentence : TNodeSentence型のノードを生成
func NewNodeSentence(finfo core.TFileInfo) TNodeSentence {
	node := TNodeSentence{
		FileInfo: finfo,
	}
	node.List = TNodeList{}
	return node
}

// Append : TNodeSentence に Node を追加
func (n *TNodeSentence) Append(node Node) {
	n.List = append(n.List, node)
}

// TNodeCallFunc : 関数呼び出しのためのノード
type TNodeCallFunc struct {
	Node
	Args     TNodeList
	Name     string
	Cache    *value.Value
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeCallFunc) GetType() NType { return CallFunc }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeCallFunc) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeCallFunc) GetJosi() string { return n.Josi }

// NewNodeCallFunc : 関数呼び出しのノードを返す
func NewNodeCallFunc(t *token.Token) TNodeCallFunc {
	node := TNodeCallFunc{
		Name:     t.Literal,
		FileInfo: t.FileInfo,
	}
	node.Args = []Node{}
	return node
}

// TNodeWord : 変数を表すノード
type TNodeWord struct {
	Node
	Name     string
	Cache    *value.Value
	Index    TNodeList
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeWord) GetType() NType { return Word }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeWord) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeWord) GetJosi() string { return n.Josi }

// NewNodeWord : 変数を表すノードを生成
func NewNodeWord(t *token.Token, index TNodeList) TNodeWord {
	node := TNodeWord{
		Name:     t.Literal,
		Index:    index,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// TNodeIf : もし
type TNodeIf struct {
	Node
	Expr      Node
	TrueNode  Node
	FalseNode Node
	Josi      string
	FileInfo  core.TFileInfo
}

// GetType : 型名取得
func (n TNodeIf) GetType() NType { return If }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeIf) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeIf) GetJosi() string { return n.Josi }

// NewNodeIf : TNodeIfを返す
func NewNodeIf(t *token.Token, nExpr, nTrue, nFalse Node) TNodeIf {
	node := TNodeIf{
		Expr:      nExpr,
		TrueNode:  nTrue,
		FalseNode: nFalse,
		Josi:      t.Josi,
		FileInfo:  t.FileInfo,
	}
	return node
}

// TNodeFor : 繰り返す
type TNodeFor struct {
	Node
	LoopID   int
	Word     string
	FromNode Node
	ToNode   Node
	Block    Node
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeFor) GetType() NType { return For }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeFor) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeFor) GetJosi() string { return n.Josi }

// NewNodeFor : 繰り返しノードを返す
func NewNodeFor(t *token.Token, hensu string, nFrom, nTo, block Node) TNodeFor {
	node := TNodeFor{
		Word:     hensu,
		FromNode: nFrom,
		ToNode:   nTo,
		Block:    block,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// TNodeWhile : 繰り返す
type TNodeWhile struct {
	Node
	LoopID   int
	Expr     Node
	Block    Node
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeWhile) GetType() NType { return While }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeWhile) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeWhile) GetJosi() string { return n.Josi }

// NewNodeWhile : 間のノードを生成
func NewNodeWhile(t *token.Token, expr, block Node) TNodeWhile {
	node := TNodeWhile{
		Expr:     expr,
		Block:    block,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// TNodeRepeat : 回
type TNodeRepeat struct {
	Node
	LoopID   int
	Expr     Node
	Block    Node
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeRepeat) GetType() NType { return Repeat }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeRepeat) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeRepeat) GetJosi() string { return n.Josi }

// NewNodeRepeat : TNodeRepeat を返す
func NewNodeRepeat(t *token.Token, nExpr, block Node) TNodeRepeat {
	node := TNodeRepeat{
		Expr:     nExpr,
		Block:    block,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// TNodeContinue : Continue
type TNodeContinue struct {
	Node
	LoopID   int
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeContinue) GetType() NType { return Continue }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeContinue) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeContinue) GetJosi() string { return n.Josi }

// NewNodeContinue : continue
func NewNodeContinue(t *token.Token, LoopID int) TNodeContinue {
	node := TNodeContinue{
		LoopID:   LoopID,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// TNodeBreak : Break
type TNodeBreak struct {
	Node
	LoopID   int
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeBreak) GetType() NType { return Break }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeBreak) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeBreak) GetJosi() string { return n.Josi }

// NewNodeBreak : break
func NewNodeBreak(t *token.Token, LoopID int) TNodeBreak {
	node := TNodeBreak{
		LoopID:   LoopID,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// TNodeDefFunc : TNodeDefFunc
type TNodeDefFunc struct {
	Node
	Word     string
	Args     TNodeList
	ArgNames []string
	Block    Node
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeDefFunc) GetType() NType { return DefFunc }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeDefFunc) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeDefFunc) GetJosi() string { return n.Josi }

// NewNodeDefFunc : def func
func NewNodeDefFunc(t *token.Token, args Node, block Node) TNodeDefFunc {
	word := t.Literal
	node := TNodeDefFunc{
		Word:     word,
		Args:     args.(TNodeList),
		ArgNames: []string{},
		Block:    block,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	// Analize Args
	a := [][]string{}
	m := map[string]int{}
	cnt := 0
	for _, v := range node.Args {
		nw := v.(TNodeWord)
		i, ok := m[nw.Name]
		if !ok {
			m[nw.Name] = cnt
			cnt++
			a = append(a, []string{nw.Josi})
			node.ArgNames = append(node.ArgNames, nw.Name)
		} else {
			a[i] = append(a[i], nw.Josi)
		}
	}
	// Add System
	funcID := core.GetSystem().AddUserFunc(word, a)
	UserFunc[funcID] = node
	return node
}

// TNodeReturn : Return
type TNodeReturn struct {
	Node
	Arg      Node
	LoopID   int
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeReturn) GetType() NType { return Return }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeReturn) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeReturn) GetJosi() string { return n.Josi }

// NewNodeReturn : return node
func NewNodeReturn(t *token.Token, arg Node, LoopID int) TNodeReturn {
	node := TNodeReturn{
		LoopID:   LoopID,
		Arg:      arg,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// TNodeJSONArray : TNodeJSONArray
type TNodeJSONArray struct {
	Node
	Items    TNodeList
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型名取得
func (n TNodeJSONArray) GetType() NType { return JSONArray }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeJSONArray) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeJSONArray) GetJosi() string { return n.Josi }

// NewNodeJSONArray : array node
func NewNodeJSONArray(t *token.Token, items TNodeList) TNodeJSONArray {
	node := TNodeJSONArray{
		Items:    items,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// JSONHashKeyValue : for json hash
type JSONHashKeyValue map[string]Node

// TNodeJSONHash : TNodeJSONHash
type TNodeJSONHash struct {
	Node
	Items    JSONHashKeyValue
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型情報取得
func (n TNodeJSONHash) GetType() NType { return JSONHash }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeJSONHash) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeJSONHash) GetJosi() string { return n.Josi }

// NewNodeJSONHash : json hash node
func NewNodeJSONHash(t *token.Token, items JSONHashKeyValue) TNodeJSONHash {
	node := TNodeJSONHash{
		Items:    items,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// ---

// ToString : Nodeの値をデバッグ用に出力する
func ToString(n Node, level int) string {
	indent := ""
	for i := 0; i < level; i++ {
		indent += "|-"
	}
	s := fmt.Sprintf("%03d: %s {%s}",
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
		s += " " + nl.Var
		if len(nl.VarIndex) > 0 {
			s += fmt.Sprintf("[]*%d", len(nl.VarIndex))
			for _, v := range nl.VarIndex {
				ss += ToString(v, level+1)
			}
		}
		ss += ToString(nl.Value, level+1) + "\n"
	case If:
		ni := n.(TNodeIf)
		ss += ToString(ni.Expr, level+1) + "\n"
		ss += ToString(ni.TrueNode, level+1) + "\n"
		ss += ToString(ni.FalseNode, level+1) + "\n"
	case For:
		ni := n.(TNodeFor)
		s += fmt.Sprintf("%s id=%d", ni.Word, ni.LoopID)
		ss += ToString(ni.FromNode, level+1) + "\n"
		ss += ToString(ni.ToNode, level+1) + "\n"
		ss += ToString(ni.Block, level+1) + "\n"
	case Repeat:
		nn := n.(TNodeRepeat)
		s += fmt.Sprintf(" id=%d", nn.LoopID)
		ss += ToString(nn.Expr, level+1) + "\n"
		ss += ToString(nn.Block, level+1) + "\n"
	case While:
		nn := n.(TNodeWhile)
		s += fmt.Sprintf(" id=%d", nn.LoopID)
		ss += ToString(nn.Expr, level+1) + "\n"
		ss += ToString(nn.Block, level+1) + "\n"
	case Continue:
		s += fmt.Sprintf("id=%d", n.(TNodeContinue).LoopID)
	case Break:
		s += fmt.Sprintf("id=%d", n.(TNodeBreak).LoopID)
	case DefFunc:
		nn := n.(TNodeDefFunc)
		s += fmt.Sprintf(" %s", nn.Word)
		ss += ToString(nn.Args, level+1) + "\n"
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
