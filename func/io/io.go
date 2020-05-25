package io

import (
	"fmt"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/value"
)

// RegisterFunction : 関数を登録
func RegisterFunction(sys *core.Core) {
	sys.AddFunc("表示", core.DefArgs{{"の", "を", "と"}}, Println) /// 文字列Sを表示 | ひょうじ
	sys.AddVar("表示ログ", "")                                    /// 表示した内容 | ひょうじろぐ
}

// Println : 表示
func Println(args value.ValueArray) (*value.Value, error) {
	if len(args) == 0 {
		return nil, nil
	}
	// 引数を評価
	v := args[0]
	s := v.ToString()
	// 表示ログに追加
	sys := core.GetSystem()
	logv := sys.Global.Get("表示ログ")
	log := logv.ToString()
	if log != "" {
		log += "\n"
	}
	logv.SetStr(log + s)
	// println
	if sys.IsDebug {
		s = "[表示] " + s
	}
	fmt.Println(s)
	return nil, nil
}
