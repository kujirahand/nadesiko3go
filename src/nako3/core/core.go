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

// Core : なでしこのコアシステム情報
type Core struct {
	IsDebug    bool
	MainFile   string
	Code       string
	RunMode    TRunMode
	GlobalVars *value.ValueHash
}

var sys *Core = nil

// GetSystem : なでしこのコア情報インスタンスを取得する
func GetSystem() *Core {
	if sys != nil {
		return sys
	}
	sys = newCore()
	return sys
}

func newCore() *Core {
	c := Core{}
	c.IsDebug = false
	c.RunMode = MainFile
	c.GlobalVars = value.NewValueHash()
	c.GlobalVars.Set("それ", value.NewValueNull())
	return &c
}
