package io

import (
	"nako3/core"
	"nako3/value"
)

func RegisterFunction(hash *value.ValueHash) {
	hash.Set("表示", value.NewValueFunc(Print))
}

func Print(args value.ValueArray) (*value.Value, error) {
	if len(args) == 0 {
		return nil, nil
	}
	v := args[0]
	s := v.ToString()
	sys := core.GetSystem()
	if sys.IsDebug {
		println("[表示]", s)
	} else {
		println(s)
	}
	return nil, nil
}
