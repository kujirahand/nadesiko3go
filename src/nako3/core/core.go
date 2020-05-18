package core

import (
	"nako3/value"
)

type Core struct {
	GlobalVars *value.ValueHash
}

func New() *Core {
	c := Core{}
	c.GlobalVars = value.NewValueHash()
	c.GlobalVars.Set("それ", value.NewValueNull())
	return &c
}
