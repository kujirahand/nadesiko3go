package system

import (
	"fmt"
	"math"
	"net/url"
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
	sys.AddVarValue("PI", value.NewFloatPtr(float64(math.Pi)))
	sys.AddConst("空", "")
	sys.AddVarValue("NULL", value.NewNullPtr())
	sys.AddVarValue("未定義", value.NewNullPtr())
	sys.AddVar("エラーメッセージ", "")
	sys.AddVar("それ", "")
	sys.AddVar("そう", "") // alias "それ" ... SetSoreLinkで処理
	sys.AddConst("対象", "")
	sys.AddConst("対象キー", "")
	sys.AddConstInt("回数", 0)
	sys.AddConstValue("空配列", value.NewArrayPtr())
	sys.AddConstValue("空ハッシュ", value.NewHashPtr())
	/// 四則演算
	sys.AddFunc("足", value.DefArgs{{"と", "に"}, {"を"}}, add) // AにBを足す | たす
	sys.AddFunc("引", value.DefArgs{{"から"}, {"を"}}, sub)     // AからBを引く | ひく
	sys.AddFunc("掛", value.DefArgs{{"と", "に"}, {"を"}}, mul) // AにBを掛ける | かける
	sys.AddFunc("倍", value.DefArgs{{"の"}, {""}}, mul)       // AのB倍 | ばい
	sys.AddFunc("割", value.DefArgs{{"を"}, {"で"}}, div)      // AをBで割る | わる
	sys.AddFunc("割余", value.DefArgs{{"を"}, {"で"}}, mod)     // AをBで割った余り | わったあまり
	sys.AddFunc("以上", value.DefArgs{{"が"}, {""}}, gteq)     // AがB以上か | いじょう
	sys.AddFunc("以下", value.DefArgs{{"が"}, {""}}, lteq)     // AがB以下か | いか
	sys.AddFunc("超", value.DefArgs{{"が"}, {""}}, gt)        // AがB超か | ちょう
	sys.AddFunc("未満", value.DefArgs{{"が"}, {""}}, lt)       // AがB未満か | みまん
	sys.AddFunc("等", value.DefArgs{{"が"}, {"と"}}, eqeq)     // AがBと等しいか | ひとしい
	// 型変換
	sys.AddFunc("変数型確認", value.DefArgs{{"の"}}, typeOf) // 値Vの型を返す | かたかくにん
	sys.AddFunc("TYPEOF", value.DefArgs{{""}}, typeOf) // 値Vの型を返す | かたかくにん
	// 文字列処理
	sys.AddFunc("置換", value.DefArgs{{"の"}, {"を", "から"}, {"へ", "に"}}, replaceStr)       // SのAをBに置換して返す | ちかん
	sys.AddFunc("単置換", value.DefArgs{{"の"}, {"を", "から"}, {"へ", "に"}}, replaceStr1time) // 一度だけSのAをBに置換して返す | たんちかん
	// CSV操作
	sys.AddFunc("CSV取得", value.DefArgs{{"を", "の", "で"}}, getCSV) // CSV形式のデータstrを強制的に二次元配列に変換して返す | CSVしゅとく
	sys.AddFunc("TSV取得", value.DefArgs{{"を", "の", "で"}}, getTSV) // TSV形式のデータstrを強制的に二次元配列に変換して返す | TSVしゅとく
	sys.AddFunc("表CSV変換", value.DefArgs{{"を"}}, convToCSV)       // 二次元配列をCSVデータに変換して返す | ひょうCSVへんかん
	sys.AddFunc("表TSV変換", value.DefArgs{{"を"}}, convToTSV)       // 二次元配列をCSVデータに変換して返す | ひょうCSVへんかん
	// JSON
	sys.AddFunc("JSONエンコード", value.DefArgs{{"を", "の"}}, jsonEncode)         // 値VのJSONをエンコードして文字列を返す | JSONえんこーど
	sys.AddFunc("JSONエンコード整形", value.DefArgs{{"を", "の"}}, jsonEncodeFormat) // 値VのJSONをエンコードして整形した文字列を返す | JSONえんこーど
	sys.AddFunc("JSONデコード", value.DefArgs{{"を", "の", "から"}}, jsonDecode)    // JSON文字列Sをデコードしてオブジェクトを返す | JSONでこーど
	// 日時
	sys.AddFunc("今", value.DefArgs{}, getNow)    // 現在時刻を返す | いま
	sys.AddFunc("今日", value.DefArgs{}, getToday) // 今日の日付を返す | きょう
	// 配列
	sys.AddFunc("要素数", value.DefArgs{{"の"}}, countV) // Sの要素数を得る | ようそすう
	// URLエンコードとパラメータ
	sys.AddFunc("URLエンコード", value.DefArgs{{"を", "の", "から"}}, urlEncode)          // 文字列SをURLエンコードして返す | URLえんこーど
	sys.AddFunc("URLデコード", value.DefArgs{{"を", "の", "から"}}, urlDecode)           // 文字列SをURLデコードして返す | URLでこーど
	sys.AddFunc("URLパラメータ解析", value.DefArgs{{"を", "の", "から"}}, urlAnalizeParams) // URLパラメータを解析してハッシュで返す| URLぱらめーたかいせき
	// ハッシュ
	sys.AddFunc("ハッシュキー列挙", value.DefArgs{{"の"}}, hashKeys)                   // ハッシュAのキー一覧を配列で返す。 | はっしゅきーれっきょ
	sys.AddFunc("ハッシュ内容列挙", value.DefArgs{{"の"}}, hashValues)                 // ハッシュAの内容一覧を配列で返す。 | はっしゅないようれっきょ
	sys.AddFunc("ハッシュキー削除", value.DefArgs{{"の", "から"}, {"を"}}, hashRemoveKey) // ハッシュAからキーKEYを削除 | はっしゅきーさくじょ
	sys.AddFunc("ハッシュキー存在", value.DefArgs{{"の", "に"}, {"が"}}, hashExists)     // ハッシュAにキーKEYがあるか調べる | はっしゅきーそんざい
	// ビット演算
	sys.AddFunc("OR", value.DefArgs{{"と"}, {"の"}}, bitOR)            // OR | OR
	sys.AddFunc("AND", value.DefArgs{{"と"}, {"の"}}, bitAND)          // AND | AND
	sys.AddFunc("XOR", value.DefArgs{{"と"}, {"の"}}, bitXOR)          // XOR | XOR
	sys.AddFunc("NOT", value.DefArgs{{"の"}}, bitNOT)                 // NOT | NOT
	sys.AddFunc("SHIFT_L", value.DefArgs{{"を"}, {"で"}}, bitShiftL)   // SHIFT_L | SHIFT_L
	sys.AddFunc("SHIFT_R", value.DefArgs{{"を"}, {"で"}}, bitShiftR)   // SHIFT_R | SHIFT_R
	sys.AddFunc("SHIFT_UR", value.DefArgs{{"を"}, {"で"}}, bitShiftUR) // SHIFT_UR | SHIFT_UR
	// 三角関数
	sys.AddFunc("SIN", value.DefArgs{{"の"}}, sin)         // Vの三角関数sinを返す | SIN
	sys.AddFunc("COS", value.DefArgs{{"の"}}, cos)         // Vの三角関数cosを返す | COS
	sys.AddFunc("TAN", value.DefArgs{{"の"}}, tan)         // Vの三角関数tanを返す | TAN
	sys.AddFunc("ARCSIN", value.DefArgs{{"の"}}, arcsin)   // Vの三角関数ArcSinを返す | ARCSIN
	sys.AddFunc("ARCCOS", value.DefArgs{{"の"}}, arccos)   // Vの三角関数ArcCosを返す | ARCCOS
	sys.AddFunc("ARCTAN", value.DefArgs{{"の"}}, arctan)   // Vの三角関数ArcTanを返す | ARCTAN
	sys.AddFunc("RAD2DEG", value.DefArgs{{"を"}}, rad2deg) // ラジアンを度に変換 | RAD2DEG
	sys.AddFunc("DEG2RAD", value.DefArgs{{"を"}}, deg2rad) // 度をラジアンに変換 | DEG2RAD
	sys.AddFunc("度変換", value.DefArgs{{"を"}}, rad2deg)     // ラジアンを度に変換 | どへんかん
	sys.AddFunc("ラジアン変換", value.DefArgs{{"を"}}, deg2rad)  // 度をラジアンに変換 | らじあんへんかん
}

func deg2rad(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	v := (a.ToFloat() / 180) * math.Pi
	return value.NewFloatPtr(v), nil
}

func rad2deg(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	v := a.ToFloat() / math.Pi * 180
	return value.NewFloatPtr(v), nil
}

func arctan(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	v := math.Atan(a.ToFloat())
	return value.NewFloatPtr(v), nil
}

func arccos(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	v := math.Acos(a.ToFloat())
	return value.NewFloatPtr(v), nil
}

func arcsin(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	v := math.Asin(a.ToFloat())
	return value.NewFloatPtr(v), nil
}

func tan(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	v := math.Tan(a.ToFloat())
	return value.NewFloatPtr(v), nil
}

func cos(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	v := math.Cos(a.ToFloat())
	return value.NewFloatPtr(v), nil
}

func sin(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	v := math.Sin(a.ToFloat())
	return value.NewFloatPtr(v), nil
}

func bitShiftUR(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	b := args.Get(1)
	c := uint(a.ToInt()) >> uint(b.ToInt())
	return value.NewIntPtr(int(c)), nil
}

func bitShiftR(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	b := args.Get(1)
	c := a.ToInt() >> b.ToInt()
	return value.NewIntPtr(c), nil
}

func bitShiftL(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	b := args.Get(1)
	c := a.ToInt() << b.ToInt()
	return value.NewIntPtr(c), nil
}

func bitNOT(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	return value.NewBoolPtr(!a.ToBool()), nil
}

func bitXOR(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	b := args.Get(1)
	c := a.ToInt() ^ b.ToInt()
	return value.NewIntPtr(c), nil
}
func bitAND(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	b := args.Get(1)
	c := a.ToInt() & b.ToInt()
	return value.NewIntPtr(c), nil
}
func bitOR(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	b := args.Get(1)
	c := a.ToInt() | b.ToInt()
	return value.NewIntPtr(c), nil
}

func hashExists(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	k := args.Get(1)
	b := a.HashExists(k.ToString())
	return value.NewBoolPtr(b), nil
}

func hashRemoveKey(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	k := args.Get(1)
	a.HashDeleteKey(k.ToString())
	return a, nil
}

func hashKeys(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	keys := v.HashKeys()
	a := value.NewArrayPtrFromStrings(keys)
	return a, nil
}
func hashValues(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	a := value.NewArrayPtr()
	keys := v.HashKeys()
	for _, k := range keys {
		v := v.HashGet(k)
		a.Append(v)
	}
	return a, nil
}

func urlAnalizeParams(args *value.TArray) (*value.Value, error) {
	uri := args.Get(0).ToString()
	params, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	res := value.NewHashPtr()
	for key, val := range params.Query() {
		if len(val) == 1 {
			res.HashSet(key, value.NewStrPtr(val[0]))
		} else {
			a := value.NewArrayPtrFromStrings(val)
			res.HashSet(key, a)
		}
	}
	return res, nil
}

func urlEncode(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	ve := url.QueryEscape(v.ToString())
	return value.NewStrPtr(ve), nil
}

func urlDecode(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	ve, _ := url.QueryUnescape(v.ToString())
	return value.NewStrPtr(ve), nil
}

func countV(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	sz := v.Length()
	return value.NewIntPtr(sz), nil
}
func getCSV(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	vv := GetCSVToValue(v.ToString(), ',')
	return vv, nil
}
func getTSV(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	vv := GetCSVToValue(v.ToString(), '\t')
	return vv, nil
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
	return value.NewStrPtr(toCsv(v, ",")), nil
}

func convToTSV(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	return value.NewStrPtr(toCsv(v, "\t")), nil
}

func jsonEncode(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	js := v.ToJSONString()
	return value.NewStrPtr(js), nil
}

func jsonEncodeFormat(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	js := v.ToJSONStringFormat(0)
	return value.NewStrPtr(js), nil
}

func jsonDecode(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	s = strings.TrimSpace(s)
	if s == "" {
		return value.NewNullPtr(), nil
	}
	return JSONDecode(s)
}

func typeOf(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	res := value.TypeToStr(v.Type)
	return value.NewStrPtr(res), nil
}

func replaceStr1time(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	a := args.Get(1).ToString()
	b := args.Get(2).ToString()
	s2 := strings.Replace(s, a, b, 1)
	return value.NewStrPtr(s2), nil
}

func replaceStr(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	a := args.Get(1).ToString()
	b := args.Get(2).ToString()
	s2 := strings.Replace(s, a, b, -1)
	return value.NewStrPtr(s2), nil
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
	var v *value.Value
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
	return v, nil
}

func getNow(args *value.TArray) (*value.Value, error) {
	t := time.Now()
	s := t.Format("15:04:05")
	return value.NewStrPtr(s), nil
}
func getToday(args *value.TArray) (*value.Value, error) {
	t := time.Now()
	s := t.Format("2006/01/02")
	return value.NewStrPtr(s), nil
}
