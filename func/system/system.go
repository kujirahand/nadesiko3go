package system

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/runeutil"
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
	// 二次元配列処理 TODO
	sys.AddFunc("表ソート", value.DefArgs{{"の"}, {"を", "で"}}, csvSort) // 二次元配列Aの列NOで表ソート | ひょうそーと
	// 型変換 TODO
	sys.AddFunc("変数型確認", value.DefArgs{{"の"}}, typeOf)    // 値Vの型を返す | かたかくにん
	sys.AddFunc("TYPEOF", value.DefArgs{{"の"}}, typeOf)   // 値Vの型を返す | TYPEOF
	sys.AddFunc("文字列変換", value.DefArgs{{"を"}}, toStr)     // 値Vを文字列に変換 | もじれつへんかん
	sys.AddFunc("TOSTR", value.DefArgs{{"を"}}, toStr)     // 値Vを文字列に変換 | TOSTR
	sys.AddFunc("整数変換", value.DefArgs{{"を"}}, toInt)      // 値Vを整数に変換 | せいすうへんかん
	sys.AddFunc("TOINT", value.DefArgs{{"を"}}, toInt)     // 値Vを整数に変換 | TOINT
	sys.AddFunc("INT", value.DefArgs{{"を"}}, toInt)       // 値Vを整数に変換 | INT
	sys.AddFunc("実数変換", value.DefArgs{{"を"}}, toFloat)    // 値Vを実数に変換 | じっすうへんかん
	sys.AddFunc("TOFLOAT", value.DefArgs{{"を"}}, toFloat) // 値Vを実数に変換 | TOFLOAT
	sys.AddFunc("FLOAT", value.DefArgs{{"を"}}, toFloat)   // 値Vを実数に変換 | FLOAT
	sys.AddFunc("HEX", value.DefArgs{{"の"}}, toHex)       // 値Vを16進数に変換 | HEX
	// 指定形式 TODO
	// 文字列処理
	sys.AddFunc("文字数", value.DefArgs{{"の"}}, countStr)                           // 文字列Sの文字数を返す | もじすう
	sys.AddFunc("何文字目", value.DefArgs{{"で", "の"}, {"が"}}, indexOf)               // 文字列SでAが何文字目にあるか返す | なんもじめ
	sys.AddFunc("CHR", value.DefArgs{{"の"}}, chr)                                // 文字コードから文字を返す | CHR
	sys.AddFunc("ASC", value.DefArgs{{"の"}}, asc)                                // 文字からコードを返す | ASC
	sys.AddFunc("文字挿入", value.DefArgs{{"で", "の"}, {"に", "へ"}, {"を"}}, strInsert) // SのI番目にAを文字挿入 || もじそうにゅう
	sys.AddFunc("文字検索", value.DefArgs{{"で", "の"}, {"から"}, {"を"}}, strFind)       // 文字列Sで文字列A文字目からBを検索。見つからなければ0を返す。(類似命令に『何文字目』がある)(v1非互換) || もじけんさく
	sys.AddFunc("追加", value.DefArgs{{"で", "に", "へ"}, {"を"}}, strAdd)             // 文字列Sに文字列Aを追加 || ついか
	sys.AddFunc("一行追加", value.DefArgs{{"で", "に", "へ"}, {"を"}}, strAddLine)       // 文字列Sに文字列Aを追加 || いちぎょうついか
	sys.AddFunc("文字列分解", value.DefArgs{{"を", "の", "で"}}, strSplitChar)           // 文字列Vを一文字ずつに分解して返す || もじれつぶんかい
	sys.AddFunc("リフレイン", value.DefArgs{{"を", "の"}, {"で"}}, strRepeat)            // 文字列VをCNT回繰り返す(v1非互換) || りふれいん
	sys.AddFunc("出現回数", value.DefArgs{{"で"}, {"の"}}, strCountStr)                // 文字列SにAが何回出現するか数える || しゅつげんかいすう
	sys.AddFunc("MID", value.DefArgs{{"で", "の"}, {"から"}, {"を"}}, mid)            // 文字列SのA文字目からCNT文字を抽出する || MID
	sys.AddFunc("文字抜出", value.DefArgs{{"で", "の"}, {"から"}, {"を"}}, mid)           // 文字列SのA文字目からCNT文字を抽出する || もじぬきだす
	sys.AddFunc("LEFT", value.DefArgs{{"で", "の"}, {"だけ"}}, left)                 // 文字列Sの左からCNT文字を抽出する || LEFT
	sys.AddFunc("文字左部分", value.DefArgs{{"で", "の"}, {"だけ"}}, left)                // 文字列Sの左からCNT文字を抽出する || もじひだりぶぶん
	sys.AddFunc("RIGHT", value.DefArgs{{"で", "の"}, {"だけ"}}, right)               // 文字列Sの右からCNT文字を抽出する || LEFT
	sys.AddFunc("文字右部分", value.DefArgs{{"で", "の"}, {"だけ"}}, right)               // 文字列Sの右からCNT文字を抽出する || もじみぎぶぶん
	sys.AddFunc("区切", value.DefArgs{{"を", "の"}, {"で"}}, strSplit)                // 文字列Sを区切り文字Aで区切って配列で返す || くぎる
	sys.AddFunc("切取", value.DefArgs{{"から", "の"}, {"まで", "を"}}, strCut)           // 文字列Sから文字列Aまでの部分を抽出する || きりとる
	sys.AddFunc("文字削除", value.DefArgs{{"の"}, {"から"}, {"を", "だけ"}}, strDelete)    // 文字列SのA文字目からB文字分を削除して返す || もじさくじょ
	// 文字変換 TODOテスト
	sys.AddFunc("大文字変換", value.DefArgs{{"の", "を"}}, toUpper)              // 文字列Sを大文字変換して返す || おおもじへんかん
	sys.AddFunc("小文字変換", value.DefArgs{{"の", "を"}}, toLower)              // 文字列Sを小文字変換して返す || こもじへんかん
	sys.AddFunc("平仮名変換", value.DefArgs{{"の", "を"}}, toHiragana)           // 文字列Sのカタカナをひらがなに変換して返す || ひらがなへんかん
	sys.AddFunc("カタカナ変換", value.DefArgs{{"の", "を"}}, toKatakana)          // 文字列Sのひらがなをカタカナに変換して返す || かたかなへんかん
	sys.AddFunc("英数全角変換", value.DefArgs{{"の", "を"}}, toZenkaku)           // 文字列Sの半角英数を全角に変換して返す || えいすうぜんかくへんかん
	sys.AddFunc("英数半角変換", value.DefArgs{{"の", "を"}}, toHankaku)           // 文字列Sの全角英数を半角に変換して返す || えいすうはんかくへんかん
	sys.AddFunc("英数記号全角変換", value.DefArgs{{"の", "を"}}, toZenkakuAndKigou) // 文字列Sの半角英数記号を全角に変換して返す || えいすうきごうぜんかくへんかん
	sys.AddFunc("英数記号半角変換", value.DefArgs{{"の", "を"}}, toHankakuAndKigou) // 文字列Sの全角英数記号を半角に変換して返す || えいすうきごうはんかくへんかん
	sys.AddFunc("カタカナ全角変換", value.DefArgs{{"の", "を"}}, toZenKana)         // 文字列Sの半角カタカナを全角カタカナに変換して返す || かたかなぜんかくへんかん
	sys.AddFunc("カタカナ半角変換", value.DefArgs{{"の", "を"}}, toHanKana)         // 文字列Sの全角カタカナを半角カタカナに変換して返す || かたかなはんかくへんかん
	sys.AddFunc("全角変換", value.DefArgs{{"の", "を"}}, toZenAll)              // 文字列Sを全角に変換して返す || ぜんかくへんかん
	sys.AddFunc("半角変換", value.DefArgs{{"の", "を"}}, toHanAll)              // 文字列Sを半角に変換して返す || はんかくへんかん
	// 置換・トリム TODO
	sys.AddFunc("置換", value.DefArgs{{"の"}, {"を", "から"}, {"へ", "に"}}, replaceStr)       // SのAをBに置換して返す | ちかん
	sys.AddFunc("単置換", value.DefArgs{{"の"}, {"を", "から"}, {"へ", "に"}}, replaceStr1time) // 一度だけSのAをBに置換して返す | たんちかん
}
func toZenAll(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	su := runeutil.ToZenkakuAndKigou(s)
	su2 := runeutil.ToZenkakuKatakana(su)
	return value.NewStrPtr(su2), nil
}
func toHanAll(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	su := runeutil.ToHankakuAndKigou(s)
	su2 := runeutil.ToHankakuKatakana(su)
	return value.NewStrPtr(su2), nil
}
func toZenKana(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	su := runeutil.ToZenkakuKatakana(s)
	return value.NewStrPtr(su), nil
}
func toHanKana(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	su := runeutil.ToHankakuKatakana(s)
	return value.NewStrPtr(su), nil
}
func toZenkakuAndKigou(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	su := runeutil.ToZenkakuAndKigou(s)
	return value.NewStrPtr(su), nil
}
func toHankakuAndKigou(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	su := runeutil.ToHankakuAndKigou(s)
	return value.NewStrPtr(su), nil
}
func toZenkaku(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	su := runeutil.ToZenkaku(s)
	return value.NewStrPtr(su), nil
}
func toHankaku(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	su := runeutil.ToHankaku(s)
	return value.NewStrPtr(su), nil
}

func toKatakana(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	su := runeutil.ToKatakana(s)
	return value.NewStrPtr(su), nil
}
func toHiragana(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	su := runeutil.ToHiragana(s)
	return value.NewStrPtr(su), nil
}

func toUpper(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	su := strings.ToUpper(s)
	return value.NewStrPtr(su), nil
}
func toLower(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	su := strings.ToLower(s)
	return value.NewStrPtr(su), nil
}

func strDelete(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	a := args.Get(1).ToInt() - 1
	b := args.Get(2).ToInt()
	sr := []rune(s)
	subStr := string(sr[0:a]) + string(sr[a+b:])
	return value.NewStrPtr(subStr), nil
}

func strCut(args *value.TArray) (*value.Value, error) {
	s := args.Get(0)
	a := args.Get(1).ToString()
	ss := s.ToString()
	i := strings.Index(ss, a)
	if i < 0 {
		s.SetStr("")
		return value.NewStrPtr(ss), nil
	}
	head := ss[:i]
	foot := ss[i+len(a):]
	s.SetStr(foot)
	return value.NewStrPtr(head), nil
}

func strSplit(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	a := args.Get(1).ToString()

	res := value.NewArrayPtr()
	sp := strings.Split(s, a)
	for _, v := range sp {
		res.Append(value.NewStrPtr(v))
	}
	return res, nil
}

func right(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	cnt := args.Get(1).ToInt()
	sr := []rune(s)
	i := len(sr) - cnt
	sub := sr[i:]
	return value.NewStrPtr(string(sub)), nil
}

func left(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	cnt := args.Get(1).ToInt()
	sr := []rune(s)
	if cnt > len(sr) {
		cnt = len(sr)
	}
	sub := sr[0:cnt]
	return value.NewStrPtr(string(sub)), nil
}

func mid(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	a := args.Get(1).ToInt() - 1
	cnt := args.Get(2).ToInt()
	sr := []rune(s)
	sub := sr[a : a+cnt]
	return value.NewStrPtr(string(sub)), nil
}

func strCountStr(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	a := args.Get(1).ToString()
	n := 0
	for {
		j := strings.Index(s, a)
		if j < 0 {
			break
		}
		j += len(a)
		s = s[j:]
		n++
	}
	return value.NewIntPtr(n), nil
}
func strRepeat(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	v := args.Get(1).ToInt()
	n := ""
	for i := 0; i < v; i++ {
		n += s
	}
	return value.NewStrPtr(n), nil
}

func strSplitChar(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	runes := []rune(s)
	a := value.NewArrayPtr()
	for _, v := range runes {
		a.Append(value.NewStrPtr(string([]rune{v})))
	}
	return a, nil
}

func strAddLine(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	a := args.Get(1).ToString()
	b := s + "\n" + a
	return value.NewStrPtr(b), nil
}

func strAdd(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	a := args.Get(1).ToString()
	b := s + a
	return value.NewStrPtr(b), nil
}

func strFind(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	a := args.Get(1).ToInt() - 1
	b := args.Get(2).ToString()
	subrune := []rune(s)[a:]
	substr := string(subrune)
	n := strings.Index(substr, b)
	if n >= 0 {
		n += a
		multiI := utf8.RuneCountInString(s[:n]) + 1
		return value.NewIntPtr(multiI), nil
	}
	return value.NewIntPtr(0), nil
}

func strInsert(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	i := args.Get(1).ToInt() - 1
	a := args.Get(2).ToString()
	// マイナス値であれば後ろから
	if i < 0 {
		i = len(s) - i
	}
	s2 := s[:i] + a + s[i:]
	return value.NewStrPtr(s2), nil
}

func chr(args *value.TArray) (*value.Value, error) {
	code := args.Get(0).ToInt()
	runes := []rune{rune(code)}
	s := string(runes)
	return value.NewStrPtr(s), nil
}
func asc(args *value.TArray) (*value.Value, error) {
	ch := args.Get(0).ToString()
	runes := []rune(ch)
	if len(runes) == 1 {
		return value.NewIntPtr(int(runes[0])), nil
	}
	a := value.NewArrayPtr()
	for _, v := range runes {
		a.Append(value.NewIntPtr(int(v)))
	}
	return a, nil
}

func indexOf(args *value.TArray) (*value.Value, error) {
	s := args.Get(0).ToString()
	a := args.Get(1).ToString()
	i := strings.Index(s, a)
	if i > 0 {
		multiI := utf8.RuneCountInString(s[:i]) + 1
		return value.NewIntPtr(multiI), nil
	}
	return value.NewIntPtr(0), nil
}

func countStr(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	s := v.ToString()
	return value.NewIntPtr(len(s)), nil
}

func toHex(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	s := v.ToHexString()
	return value.NewStrPtr(s), nil
}

func toFloat(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	return value.NewFloatPtr(v.ToFloat()), nil
}
func toInt(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	return value.NewIntPtr(v.ToInt()), nil
}
func toStr(args *value.TArray) (*value.Value, error) {
	v := args.Get(0)
	return value.NewStrPtr(v.ToString()), nil
}

func csvSort(args *value.TArray) (*value.Value, error) {
	a := args.Get(0)
	col := args.Get(1)
	ta := a.ToArray()
	ta.SortCsv(col.ToInt())
	return a, nil
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
