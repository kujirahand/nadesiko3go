package system

import (
	"fmt"
	"math"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/value"
)

// RegisterFunction : 関数を登録
func RegisterFunction(sys *core.Core) {
	/// システム定数
	sys.AddConst("ナデシコバージョン", "?")
	sys.AddConst("ナデシコエンジン", "nadesiko3go")
	sys.AddConst("ナデシコ種類", "cnako3")
	sys.AddConstInt("はい", 1)
	sys.AddConstInt("いいえ", 0)
	sys.AddConstInt("オン", 1)
	sys.AddConstInt("オフ", 0)
	sys.AddConst("改行", "\n")
	sys.AddConst("タブ", "\t")
	sys.AddConst("CR", "\r")
	sys.AddConst("LF", "\n")
	sys.AddConst("カッコ", "(")
	sys.AddConst("カッコ閉", ")")
	sys.AddConst("波カッコ", "{")
	sys.AddConst("波カッコ閉", "}")
	sys.AddConstInt("OK", 1)
	sys.AddConstInt("NG", 0)
	sys.AddVarValue("PI", value.NewValueFloat(float64(math.Pi)))
	sys.AddConst("空", "")
	sys.AddVarValue("NULL", value.NewValueNull())
	sys.AddVarValue("未定義", value.NewValueNull())
	sys.AddVar("エラーメッセージ", "")
	sys.AddVar("それ", "")
	sys.AddVar("そう", "") // alias "それ" ... SetSoreLinkで処理
	sys.AddConst("対象", "")
	sys.AddConst("対象キー", "")
	sys.AddConstInt("回数", 0)
	sys.AddConstValue("空配列", value.NewValueArray())
	sys.AddConstValue("空ハッシュ", value.NewValueHash())
	/// 四則演算
	sys.AddFunc("足", core.DefArgs{{"と", "に"}, {"を"}}, add) // AにBを足す | たす
	sys.AddFunc("引", core.DefArgs{{"から"}, {"を"}}, sub)     // AからBを引く | ひく
	sys.AddFunc("掛", core.DefArgs{{"と", "に"}, {"を"}}, mul) // AにBを掛ける | かける
	sys.AddFunc("倍", core.DefArgs{{"の"}, {""}}, mul)       // AのB倍 | ばい
	sys.AddFunc("割", core.DefArgs{{"を"}, {"で"}}, div)      // AをBで割る | わる
	sys.AddFunc("割余", core.DefArgs{{"を"}, {"で"}}, mod)     // AをBで割った余り | わったあまり
	sys.AddFunc("以上", core.DefArgs{{"が"}, {""}}, gteq)     // AがB以上か | いじょう
	sys.AddFunc("以下", core.DefArgs{{"が"}, {""}}, lteq)     // AがB以下か | いか
	sys.AddFunc("超", core.DefArgs{{"が"}, {""}}, gt)        // AがB超か | ちょう
	sys.AddFunc("未満", core.DefArgs{{"が"}, {""}}, lt)       // AがB未満か | みまん
	sys.AddFunc("等", core.DefArgs{{"が"}, {"と"}}, eqeq)     // AがBと等しいか | ひとしい
}

func add(args *value.TArray) (*value.Value, error) {
	return calc('+', args)
}
func sub(args *value.TArray) (*value.Value, error) {
	return calc('-', args)
}
func mul(args *value.TArray) (*value.Value, error) {
	return calc('*', args)
}
func div(args *value.TArray) (*value.Value, error) {
	return calc('/', args)
}
func mod(args *value.TArray) (*value.Value, error) {
	return calc('%', args)
}
func gt(args *value.TArray) (*value.Value, error) {
	return calc('>', args)
}
func gteq(args *value.TArray) (*value.Value, error) {
	return calc('≧', args)
}
func lt(args *value.TArray) (*value.Value, error) {
	return calc('<', args)
}
func lteq(args *value.TArray) (*value.Value, error) {
	return calc('≦', args)
}
func eqeq(args *value.TArray) (*value.Value, error) {
	return calc('=', args)
}
func nteq(args *value.TArray) (*value.Value, error) {
	return calc('≠', args)
}

func calc(op rune, args *value.TArray) (*value.Value, error) {
	var v value.Value
	l := args.Get(0)
	r := args.Get(1)
	switch op {
	case '+':
		v = value.Add(l, r)
	case '-':
		v = value.Sub(l, r)
	case '*':
		v = value.Mul(l, r)
	case '/':
		v = value.Div(l, r)
	case '%':
		v = value.Mod(l, r)
	case '>':
		v = value.Gt(l, r)
	case '<':
		v = value.Lt(l, r)
	case '≧':
		v = value.GtEq(l, r)
	case '≦':
		v = value.LtEq(l, r)
	case '=':
		v = value.EqEq(l, r)
	case '≠':
		v = value.NtEq(l, r)
	default:
		return nil, fmt.Errorf("system.calc link error")
	}
	return &v, nil
}
