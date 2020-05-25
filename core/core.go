package core

import (
	"github.com/kujirahand/nadesiko3go/scope"
	"github.com/kujirahand/nadesiko3go/value"
)

const (
	// NadesikoVersion : ナデシコバージョン
	NadesikoVersion = "0.0.1"
)

// TRunMode : コマンドラインの実行モード
type TRunMode string

const (
	// EvalCode : -e オプションで実行したとき
	EvalCode TRunMode = "evalcode"
	// MainFile : ファイルから実行
	MainFile TRunMode = "mainfile"
)

// DefArgs : 関数引数の助詞一覧
type DefArgs [][]string

// Core : なでしこのコアシステム情報
type Core struct {
	IsDebug    bool
	MainFile   string
	Code       string
	RunMode    TRunMode
	Scopes     *scope.ScopeObj
	Global     *scope.Scope
	Sore       value.Value
	Taisyo     value.Value
	BreakID    int
	ContinueID int
	ReturnID   int
	LoopLevel  int
	JosiList   []DefArgs // システム関数の助詞情報を記憶する
}

var sys *Core = nil

// GetSystem : なでしこのコア情報インスタンスを取得する
func GetSystem() *Core {
	if sys != nil {
		return sys
	}
	sys = NewCore()
	return sys
}

// NewCore : 新規コア情報インスタンスを作成
func NewCore() *Core {
	c := Core{}
	c.IsDebug = false
	c.RunMode = MainFile
	c.Scopes = scope.NewScopeObj()
	c.Global = c.Scopes.GetGlobal()
	c.Sore = value.NewValueNull()
	c.Taisyo = value.NewValueNull()
	g := c.Global
	g.Set("それ", &c.Sore)
	g.Set("そう", &c.Sore) // Alias "それ"
	g.Set("対象", &c.Taisyo)
	c.JosiList = []DefArgs{}
	c.BreakID = -1
	c.ContinueID = -1
	c.ReturnID = -1
	c.LoopLevel = 0
	return &c
}

// addFuncCustom : システムに関数を登録する
func (sys *Core) addFuncCustom(name string, args DefArgs, val value.Value) int {
	val.Tag = len(sys.JosiList)
	sys.JosiList = append(sys.JosiList, args)
	sys.Global.Set(name, &val)
	return val.Tag
}

// AddVar : システムに変数を登録
func (sys *Core) AddVar(name, v string) int {
	val := value.NewValueStr(v)
	sys.Global.Set(name, &val)
	return -1
}

// AddFunc : システムにGo関数を登録する
func (sys *Core) AddFunc(name string, args DefArgs, f value.ValueFunc) int {
	val := value.NewValueFunc(f)
	return sys.addFuncCustom(name, args, val)
}

// AddUserFunc : システムにユーザー関数を登録する
func (sys *Core) AddUserFunc(name string, args DefArgs) int {
	val := value.NewValueUserFunc(-1)
	tag := sys.addFuncCustom(name, args, val)
	val.Value = tag
	return tag
}
