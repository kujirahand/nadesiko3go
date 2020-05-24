package io

import (
	. "github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/value"
)

func RegisterFunction(sys *Core) {
	sys.AddFunc("表示", DefArgs{Josi{"の", "を", "と"}}, Print)
}

func Print(args value.ValueArray) (*value.Value, error) {
	if len(args) == 0 {
		return nil, nil
	}
	v := args[0]
	s := v.ToString()
	sys := GetSystem()
	if sys.IsDebug {
		println("[表示]", s)
	} else {
		println(s)
	}
	return nil, nil
}
