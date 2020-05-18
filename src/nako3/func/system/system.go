package system

import (
	. "nako3/core"
	"nako3/value"
)

func RegisterFunction(sys *Core) {
	sys.AddFunc("足", DefArgs{Josi{"と", "に"}, Josi{"を"}}, Print)
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
