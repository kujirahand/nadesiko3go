package node

import (
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
	// Foreach : 反復
	Foreach
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
	// DefVar : 変数宣言
	DefVar
)

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
	Name      string
	Index     TNodeList
	ValueNode Node
	Josi      string
	FileInfo  core.TFileInfo
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
		Name:      t.Literal,
		Index:     index,
		ValueNode: value,
		FileInfo:  t.FileInfo,
		Josi:      t.Josi,
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
		Value:    value.NewValueInt(int(num)),
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

// NewNodeOperatorStr : TNodeOperatorを返す
func NewNodeOperatorStr(op string, left Node, right Node) TNodeOperator {
	p := TNodeOperator{
		Left:     left,
		Right:    right,
		Operator: op,
		Josi:     right.GetJosi(),
		FileInfo: right.GetFileInfo(),
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
	UseJosi  bool
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
func NewNodeCallFunc(t *token.Token, args TNodeList) TNodeCallFunc {
	node := TNodeCallFunc{
		Name:     t.Literal,
		Args:     args,
		FileInfo: t.FileInfo,
		UseJosi:  true,
		Josi:     t.Josi,
	}
	return node
}

// NewNodeCallFuncCStyle : 関数呼び出しのノードを返す(Cスタイル)
func NewNodeCallFuncCStyle(t *token.Token, args TNodeList, t2 *token.Token) TNodeCallFunc {
	n := NewNodeCallFunc(t, args)
	n.UseJosi = false
	n.Josi = t2.Josi
	return n
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
	sys := core.GetSystem()
	sys.AddUserFunc(word, a, node)
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
func NewNodeJSONArray(t *token.Token, items TNodeList, t2 *token.Token) TNodeJSONArray {
	node := TNodeJSONArray{
		Items:    items,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	node.Josi = t2.Josi
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
func NewNodeJSONHash(t *token.Token, items JSONHashKeyValue, t2 *token.Token) TNodeJSONHash {
	node := TNodeJSONHash{
		Items:    items,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	node.Josi = t2.Josi
	return node
}

// TNodeForeach : TNodeForeach
type TNodeForeach struct {
	Node
	Expr     Node
	Block    Node
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型情報取得
func (n TNodeForeach) GetType() NType { return Foreach }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeForeach) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeForeach) GetJosi() string { return n.Josi }

// NewNodeForeach : 反復ノードの生成
func NewNodeForeach(t *token.Token, expr Node, block Node) TNodeForeach {
	node := TNodeForeach{
		Expr:     expr,
		Block:    block,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// TNodeDefVar : 変数の宣言
type TNodeDefVar struct {
	Node
	Name     string
	Expr     Node
	IsConst  bool
	Josi     string
	FileInfo core.TFileInfo
}

// GetType : 型情報取得
func (n TNodeDefVar) GetType() NType { return DefVar }

// GetFileInfo : 行番号やファイル番号の情報取得
func (n TNodeDefVar) GetFileInfo() core.TFileInfo { return n.FileInfo }

// GetJosi : 助詞を取得
func (n TNodeDefVar) GetJosi() string { return n.Josi }

// NewNodeDefVar : 変数宣言
func NewNodeDefVar(t *token.Token, expr Node) TNodeDefVar {
	node := TNodeDefVar{
		Name:     t.Literal,
		Expr:     expr,
		IsConst:  false,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}

// NewNodeDefConst : 定数宣言
func NewNodeDefConst(t *token.Token, expr Node) TNodeDefVar {
	node := TNodeDefVar{
		Name:     t.Literal,
		Expr:     expr,
		IsConst:  true,
		Josi:     t.Josi,
		FileInfo: t.FileInfo,
	}
	return node
}
