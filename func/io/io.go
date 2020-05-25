package io

import (
	"fmt"
	"io/ioutil"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/value"
)

// RegisterFunction : 関数を登録
func RegisterFunction(sys *core.Core) {
	sys.AddFunc("表示", core.DefArgs{{"の", "を", "と"}}, Println)    /// 文字列Sを表示 | ひょうじ
	sys.AddVar("表示ログ", "")                                       /// 表示した内容 | ひょうじろぐ
	sys.AddFunc("開", core.DefArgs{{"を", "から"}}, OpenFile)        /// ファイルFの内容を全部読む | ひらく
	sys.AddFunc("読", core.DefArgs{{"を", "から"}}, OpenFile)        /// ファイルFの内容を全部読む | よむ
	sys.AddFunc("バイナリ読", core.DefArgs{{"を", "から"}}, OpenBinFile) /// ファイルFの内容をバイナリで全部読む | ばいなりよむ
}

// OpenBinFile : バイナリ読む
func OpenBinFile(args value.TArray) (*value.Value, error) {
	f := args[0].ToString()
	bin, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("ファイルが読めません。file=" + f)
	}
	vText := value.NewValueStr(string(bin))
	return &vText, nil
}

// OpenFile : 読む
func OpenFile(args value.TArray) (*value.Value, error) {
	f := args[0].ToString()
	text, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("ファイルが読めません。file=" + f)
	}
	vText := value.NewValueStr(string(text))
	return &vText, nil
}

// Println : 表示
func Println(args value.TArray) (*value.Value, error) {
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
