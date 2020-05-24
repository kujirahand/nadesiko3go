package io

import (
	"fmt"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/value"
)

// RegisterFunction : 関数を登録
func RegisterFunction(sys *core.Core) {
	sys.AddFunc("表示", core.DefArgs{{"の", "を", "と"}}, Print)
}

// Print : 表示
func Print(args value.ValueArray) (*value.Value, error) {
	if len(args) == 0 {
		return nil, nil
	}
	v := args[0]
	s := v.ToString()
	sys := core.GetSystem()
	if sys.IsDebug {
		s = "[表示] " + s
	}
	fmt.Println(s)
	return nil, nil
}
