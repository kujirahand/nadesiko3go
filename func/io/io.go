package io

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/value"
)

// RegisterFunction : 関数を登録
func RegisterFunction(sys *core.Core) {
	// コマンドラインと標準入出力
	sys.AddConstValue("コマンドライン", getCommandline())            // コマンドライン引数を保持 | こまんどらいん
	sys.AddFunc("表示", core.DefArgs{{"の", "を", "と"}}, Println) // 文字列Sを表示 | ひょうじ
	sys.AddVar("表示ログ", "")                                    // 表示した内容 | ひょうじろぐ
	sys.AddFunc("尋", core.DefArgs{{"と", "を"}}, ask)           // 標準入力から入力を得る | たずねる
	// ファイル読み書き
	sys.AddFunc("開", core.DefArgs{{"を", "から"}}, OpenFile)         // ファイルFの内容を全部読む | ひらく
	sys.AddFunc("読", core.DefArgs{{"を", "から"}}, OpenFile)         // ファイルFの内容を全部読む | よむ
	sys.AddFunc("バイナリ読", core.DefArgs{{"を", "から"}}, OpenBinFile)  // ファイルFの内容をバイナリで全部読む | ばいなりよむ
	sys.AddFunc("保存", core.DefArgs{{"を"}, {"に", "へ"}}, WriteFile) // SをファイルFに保存 | ほぞん
	// プロセス
	sys.AddFunc("OS取得", core.DefArgs{}, getOS)  // OSの種類を返す | OSしゅとく
	sys.AddFunc("秒待", core.DefArgs{{""}}, wait) // N秒待つ | びょうまつ
	// パス操作
	sys.AddConst("母艦パス", getBokanPath())                           // スクリプトのパスを保持 | ぼかんぱす
	sys.AddFunc("ファイル名抽出", core.DefArgs{{"から", "の"}}, basename)    // N秒待つ | びょうまつ
	sys.AddFunc("パス抽出", core.DefArgs{{"から", "の"}}, dirname)        // Fのパスを返す | ぱすちゅうしゅつ
	sys.AddFunc("相対パス展開", core.DefArgs{{"を"}, {"で"}}, relatepath)  // 相対パス展開 | そうたいぱすてんかい
	sys.AddFunc("絶対パス変換", core.DefArgs{{"から", "を", "の"}}, abspath) // 絶対パス変換 | ぜったいぱすへんかん
}

func abspath(args *value.TArray) (*value.Value, error) {
	f := args.Get(0).ToString()
	path, err := filepath.Abs(f)
	if err != nil {
		path = f
	}
	return value.NewStrPtr(path), nil
}

func getBokanPath() string {
	s := filepath.Dir(core.GetSystem().MainFile)
	return s
}

func basename(args *value.TArray) (*value.Value, error) {
	f := args.Get(0).ToString()
	path := filepath.Base(f)
	return value.NewStrPtr(path), nil
}
func dirname(args *value.TArray) (*value.Value, error) {
	f := args.Get(0).ToString()
	path := filepath.Dir(f)
	return value.NewStrPtr(path), nil
}
func relatepath(args *value.TArray) (*value.Value, error) {
	a := args.Get(0).ToString()
	b := args.Get(1).ToString()
	a = filepath.FromSlash(a)
	b = filepath.FromSlash(b)
	alist := strings.Split(a, "/")
	blist := strings.Split(b, "/")
	result := []string{}
	// 階層を昇る場合
	for _, v := range blist {
		if v == ".." {
			alist = alist[:len(alist)-1] // 末尾削除
			continue
		}
		if v == "." {
			continue
		}
		break
	}
	// aのパスを追加
	for _, v := range alist {
		result = append(result, v)
	}
	// bの追加文を追加
	for _, v := range blist {
		if v == "" || v == "." || v == ".." {
			continue
		}
		result = append(result, v)
	}
	r := strings.Join(result, "/")
	return value.NewStrPtr(r), nil
}

func wait(args *value.TArray) (*value.Value, error) {
	sec := args.Get(0).ToFloat()
	msec := int64(sec * 1000)
	time.Sleep(time.Duration(msec) * time.Millisecond)
	return nil, nil
}

func getOS(args *value.TArray) (*value.Value, error) {
	return value.NewStrPtr(runtime.GOOS), nil
}

func ask(args *value.TArray) (*value.Value, error) {
	msg := args.Get(0).ToString()
	fmt.Print(msg + " ")
	stdin := bufio.NewScanner(os.Stdin)
	stdin.Scan()
	text := stdin.Text()
	return value.NewStrPtr(text), nil
}

// コマンドライン
func getCommandline() *value.Value {
	v := value.NewArrayPtr()
	for _, arg := range os.Args {
		v.Append(value.NewStrPtr(arg))
	}
	return v
}

// WriteFile : ファイルを保存する
func WriteFile(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	f := args.Get(1).ToString()
	err := ioutil.WriteFile(f, []byte(s), os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("『保存』命令でファイルが書き込めません。file=" + f)
	}
	return nil, nil
}

// OpenBinFile : バイナリ読む
// TODO: バイナリ読む - 実装途中
func OpenBinFile(args *value.TArray) (*value.Value, error) {
	f := args.Get(0).ToString()
	bin, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("ファイルが読めません。file=" + f)
	}
	vText := value.NewBytes(bin) // TODO
	return &vText, nil
}

// OpenFile : 読む
func OpenFile(args *value.TArray) (*value.Value, error) {
	f := args.Get(0).ToString()
	text, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("ファイルが読めません。file=" + f)
	}
	vText := value.NewStrPtr(string(text))
	return vText, nil
}

// Println : 表示
func Println(args *value.TArray) (*value.Value, error) {
	if args.Length() == 0 {
		return nil, nil
	}
	// 引数を評価
	v := args.Get(0)
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
