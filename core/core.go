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

// Core : なでしこのコアシステム情報
type Core struct {
	IsDebug    bool
	IsCompile  bool
	IsOptimze  bool
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
	c.IsOptimze = true
	c.Scopes = scope.NewScopeList() // 最初のスコープも自動的に得られる
	c.Global = c.Scopes.GetGlobal()
	// c.JosiList = []value.DefArgs{}
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
func (p *Core) addFuncCustom(name string, val value.Value) int {
	p.Global.Set(name, &val)
	tag := p.Global.GetIndexByName(name)
	fv := val.Value.(value.TFuncValue)
	fv.Tag = tag
	return fv.Tag
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
func (p *Core) AddFunc(name string, args value.DefArgs, f value.TFunction) int {
	val := value.NewFunc(name, args, f)
	return p.addFuncCustom(name, val)
}

// AddUserFunc : システムにユーザー関数を登録する
func (p *Core) AddUserFunc(name string, args value.DefArgs, node interface{}) int {
	userFuncVal := value.NewUserFunc(name, args, node)
	p.UserFuncs.Append(&userFuncVal)
	return p.addFuncCustom(name, userFuncVal)
}
