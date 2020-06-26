package system

import (
	"fmt"
	"math"
	"strings"
	"time"

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
	// 型変換
	sys.AddFunc("変数型確認", core.DefArgs{{"の"}}, typeOf) // 値Vの型を返す | かたかくにん
	sys.AddFunc("TYPEOF", core.DefArgs{{""}}, typeOf) // 値Vの型を返す | かたかくにん
	// 文字列処理
	sys.AddFunc("置換", core.DefArgs{{"の"}, {"を", "から"}, {"へ", "に"}}, replaceStr)       // SのAをBに置換して返す | ちかん
	sys.AddFunc("単置換", core.DefArgs{{"の"}, {"を", "から"}, {"へ", "に"}}, replaceStr1time) // 一度だけSのAをBに置換して返す | たんちかん
	// CSV操作
	sys.AddFunc("CSV取得", core.DefArgs{{"を", "の", "で"}}, getCSV) // CSV形式のデータstrを強制的に二次元配列に変換して返す | CSVしゅとく
	sys.AddFunc("TSV取得", core.DefArgs{{"を", "の", "で"}}, getTSV) // TSV形式のデータstrを強制的に二次元配列に変換して返す | TSVしゅとく
	sys.AddFunc("表CSV変換", core.DefArgs{{"を"}}, convToCSV)       // 二次元配列をCSVデータに変換して返す | ひょうCSVへんかん
	sys.AddFunc("表TSV変換", core.DefArgs{{"を"}}, convToTSV)       // 二次元配列をCSVデータに変換して返す | ひょうCSVへんかん
	// JSON
	sys.AddFunc("JSONエンコード", core.DefArgs{{"を", "の"}}, jsonEncode)         // 値VのJSONをエンコードして文字列を返す | JSONえんこーど
	sys.AddFunc("JSONエンコード整形", core.DefArgs{{"を", "の"}}, jsonEncodeFormat) // 値VのJSONをエンコードして整形した文字列を返す | JSONえんこーど
	sys.AddFunc("JSONデコード", core.DefArgs{{"を", "の", "から"}}, jsonDecode)    // JSON文字列Sをデコードしてオブジェクトを返す | JSONでこーど
	// 日時
	sys.AddFunc("今", core.DefArgs{}, getNow)    // 現在時刻を返す | いま
	sys.AddFunc("今日", core.DefArgs{}, getToday) // 今日の日付を返す | きょう
	// 配列
	sys.AddFunc("要素数", core.DefArgs{{"の"}}, countV) // Sの要素数を得る | ようそすう
}

func countV(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	sz := v.Length()
	return value.NewValueIntPtr(sz), nil
}
func getCSV(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	vv := GetCSVToValue(v.ToString(), ',')
	return &vv, nil
}
func getTSV(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	vv := GetCSVToValue(v.ToString(), '\t')
	return &vv, nil
}

func csvQuote(s string) string {
	s = strings.Replace(s, "\"", "\"\"", 0)
	s = "\"" + s + "\""
	return s
}

func toCsv(v *value.Value, splitter string) string {
	if v.Type != value.Array {
		return csvQuote(v.ToString())
	}
	csv := ""
	for i := 0; i < v.Length(); i++ {
		row := v.ArrayGet(i)
		if row.Type != value.Array {
			csv += csvQuote(row.ToString()) + "\r\n"
		}
		maxCols := 0
		for j := 0; j < row.Length(); j++ {
			col := row.ArrayGet(j)
			if col.Length() > maxCols {
				maxCols = col.Length()
			}
		}
		for j := 0; j < maxCols; j++ {
			col := row.ArrayGet(j)
			csv += col.ToString()
			if j != (maxCols - 1) {
				csv += splitter
			}
		}
		csv += "\r\n"
	}
	return csv
}

func convToCSV(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	return value.NewValueStrPtr(toCsv(v, ",")), nil
}

func convToTSV(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	return value.NewValueStrPtr(toCsv(v, "\t")), nil
}

func jsonEncode(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	js := v.ToJSONString()
	return value.NewValueStrPtr(js), nil
}

func jsonEncodeFormat(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	js := v.ToJSONStringFormat(0)
	return value.NewValueStrPtr(js), nil
}

func jsonDecode(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	s = strings.TrimSpace(s)
	if s == "" {
		return value.NewValueNullPtr(), nil
	}
	return JSONDecode(s)
}

func typeOf(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	res := value.TypeToStr(v.Type)
	return value.NewValueStrPtr(res), nil
}

func replaceStr1time(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	a := args.Get(1).ToString()
	b := args.Get(2).ToString()
	s2 := strings.Replace(s, a, b, 1)
	return value.NewValueStrPtr(s2), nil
}

func replaceStr(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	a := args.Get(1).ToString()
	b := args.Get(2).ToString()
	s2 := strings.Replace(s, a, b, 0)
	return value.NewValueStrPtr(s2), nil
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

func getNow(args *value.TArray) (*value.Value, error) {
	t := time.Now()
	s := t.Format("15:04:05")
	return value.NewValueStrPtr(s), nil
}
func getToday(args *value.TArray) (*value.Value, error) {
	t := time.Now()
	s := t.Format("2006/01/02")
	return value.NewValueStrPtr(s), nil
}
