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
	IsCompile  bool
	MainFile   string
	Code       string
	RunMode    TRunMode
	Scopes     scope.TScopeList
	Global     *scope.Scope
	Sore       *value.Value
	Taisyo     *value.Value
	TaisyoKey  *value.Value
	BreakID    int
	ContinueID int
	ReturnID   int
	LoopLevel  int
	JosiList   []DefArgs // システム関数の助詞情報を記憶する
	UserFuncs  value.TArray
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
	c.Scopes = scope.NewScopeList() // 最初のスコープも自動的に得られる
	c.Global = c.Scopes.GetGlobal()
	c.JosiList = []DefArgs{}
	c.BreakID = -1
	c.ContinueID = -1
	c.ReturnID = -1
	c.LoopLevel = 0
	c.UserFuncs = value.TArray{}
	return &c
}

// SetSoreLink : よく使う変数を設定
func (p *Core) SetSoreLink() {
	g := p.Global
	p.Sore = g.Get("それ")
	p.Taisyo = g.Get("対象")
	p.TaisyoKey = g.Get("対象キー")
	g.Set("そう", p.Sore)
}

// addFuncCustom : システムに関数を登録する
func (p *Core) addFuncCustom(name string, args DefArgs, val value.Value) int {
	val.Tag = len(p.JosiList)
	p.JosiList = append(p.JosiList, args)
	p.Global.Set(name, &val)
	return val.Tag
}

// AddConst : システムに定数を登録
func (p *Core) AddConst(name, v string) int {
	val := value.NewStrPtr(v)
	val.IsConst = true
	p.Global.Set(name, val)
	return -1
}

// AddConstInt : システムに定数を登録
func (p *Core) AddConstInt(name string, v int) int {
	val := value.NewIntPtr(int(v))
	val.IsConst = true
	p.Global.Set(name, val)
	return -1
}

// AddConstValue : システムに変数を登録
func (p *Core) AddConstValue(name string, v *value.Value) int {
	p.Global.Set(name, v)
	v.IsConst = true
	return -1
}

// AddVar : システムに変数を登録
func (p *Core) AddVar(name, v string) int {
	val := value.NewStrPtr(v)
	p.Global.Set(name, val)
	return -1
}

// AddVarInt : システムに整数型の値を登録
func (p *Core) AddVarInt(name string, v int) int {
	val := value.NewIntPtr(int(v))
	p.Global.Set(name, val)
	return -1
}

// AddVarValue : システムに変数を登録
func (p *Core) AddVarValue(name string, v *value.Value) int {
	p.Global.Set(name, v)
	return -1
}

// AddFunc : システムにGo関数を登録する
func (p *Core) AddFunc(name string, args DefArgs, f value.TFunction) int {
	val := value.NewFunc(f)
	return p.addFuncCustom(name, args, val)
}

// AddUserFunc : システムにユーザー関数を登録する
func (p *Core) AddUserFunc(name string, args DefArgs, node interface{}) int {
	userFuncVal := value.NewUserFunc(node)
	tag := p.addFuncCustom(name, args, userFuncVal)
	p.UserFuncs.Append(&userFuncVal)
	return tag
}
