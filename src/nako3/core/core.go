package core

import (
	"nako3/value"
)

const (
	NADESIKO_VERSION = "0.0.1"
)

type TRunMode string

const (
	EvalCode TRunMode = "evalcode"
	MainFile TRunMode = "mainfile"
)

type Core struct {
	IsDebug    bool
	MainFile   string
	Code       string
	RunMode    TRunMode
	GlobalVars *value.ValueHash
}

var sys *Core = nil

func GetSystem() *Core {
	if sys != nil {
		return sys
	}
	sys = NewCore()
	return sys
}

func NewCore() *Core {
	c := Core{}
	c.IsDebug = false
	c.RunMode = MainFile
	c.GlobalVars = value.NewValueHash()
	c.GlobalVars.Set("それ", value.NewValueNull())
	return &c
}
