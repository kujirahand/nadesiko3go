package core

import (
	"nako3/value"
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

// Josi : 助詞の一覧
type Josi []string

// DefArgs : 関数引数の助詞一覧
type DefArgs []Josi

// Core : なでしこのコアシステム情報
type Core struct {
	IsDebug  bool
	MainFile string
	Code     string
	RunMode  TRunMode
	Globals  *value.ValueHash
	JosiList []DefArgs // システム関数の助詞情報を記憶する
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
	c.Globals = value.NewValueHash()
	c.Globals.Set("それ", value.NewValueNull())
	c.JosiList = []DefArgs{}
	return &c
}

// AddFunc : システムに関数を登録する
func (sys *Core) AddFunc(name string, args DefArgs, f value.ValueFunc) {
	val := value.NewValueFunc(f)
	val.Tag = len(sys.JosiList)
	sys.JosiList = append(sys.JosiList, args)
	sys.Globals.Set(name, val)
}
