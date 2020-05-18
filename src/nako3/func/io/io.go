package io

import (
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
	println("[表示]", v.ToString())
	return nil, nil
}
