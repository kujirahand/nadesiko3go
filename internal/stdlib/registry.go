package stdlib

import (
	"math"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

// ParserFuncList returns the plugin_system metadata needed by the lexer and
// parser. Function implementations are added in later stages; keeping the
// signatures here lets stage 1 parse the compatibility fixtures now.
// nullConst and undefinedConst distinguish the two empty values in the
// signature table, which a plain nil could not.
type nullConst struct{}
type undefinedConst struct{}

// emptyArrayConst marks a constant whose value is a fresh empty array.
type emptyArrayConst struct{}
type eraDataConst struct{}

func ParserFuncList() lexer.FuncList {
	list := lexer.FuncList{}
	addConst := func(name string, value any) {
		list[name] = &lexer.FuncItem{Name: name, Type: "const", Value: value}
	}
	addFunc := func(name string, josi [][]string) {
		list[name] = &lexer.FuncItem{Name: name, Type: "func", Josi: josi, Pure: true}
	}

	addConst("はい", true)
	addConst("いいえ", false)
	addConst("真", true)
	addConst("偽", false)
	addConst("TRUE", true)
	addConst("FALSE", false)
	addConst("true", true)
	addConst("false", false)
	addConst("永遠", true)
	addConst("オン", true)
	addConst("オフ", false)
	addConst("OK", true)
	addConst("NG", false)
	addConst("キャンセル", 0)
	addConst("PI", math.Pi)
	addConst("改行", "\n")
	addConst("CR", "\r")
	addConst("LF", "\n")
	addConst("タブ", "\t")
	addConst("空", "")
	addConst("非数", math.NaN())
	addConst("無限大", math.Inf(1))
	addConst("NULL", nullConst{})
	addConst("undefined", undefinedConst{})
	addConst("未定義", undefinedConst{})
	addConst("エラーメッセージ", "")
	addConst("対象", "")
	addConst("対象キー", "")
	addConst("回数", "")
	addConst("カッコ", "「")
	addConst("カッコ閉", "」")
	addConst("カッコ閉じ", "」")
	addConst("波カッコ", "{")
	addConst("波カッコ閉", "}")
	addConst("ナデシコエンジン", "nadesi.com/v3")
	addConst("ナデシコバージョン", "3.8.1")
	addConst("ナデシコ言語バージョン", "3.8.1")
	addConst("ナデシコ種類", "?")
	addConst("プラグイン名", "メイン")
	addConst("名前空間", "")
	addConst("表示ログ", "")
	addConst("全角カナ一覧", "アイウエオカキクケコサシスセソタチツテトナニヌネノハヒフヘホマミムメモヤユヨラリルレロワヲンァィゥェォャュョッ、。ー「」")
	addConst("全角カナ濁音一覧", "ガギグゲゴザジズゼゾダヂヅデドバビブベボパピプペポ")
	addConst("半角カナ一覧", "ｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄﾅﾆﾇﾈﾉﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜｦﾝｧｨｩｪｫｬｭｮｯ､｡ｰ｢｣ﾞﾟ")
	addConst("半角カナ濁音一覧", "ｶﾞｷﾞｸﾞｹﾞｺﾞｻﾞｼﾞｽﾞｾﾞｿﾞﾀﾞﾁﾞﾂﾞﾃﾞﾄﾞﾊﾞﾋﾞﾌﾞﾍﾞﾎﾞﾊﾟﾋﾟﾌﾟﾍﾟﾎﾟ")
	addConst("元号データ", eraDataConst{})
	addFunc("空配列", nil)
	// 『正規表現マッチ』が部分マッチを入れる先。初期値は空の配列。
	addConst("抽出文字列", emptyArrayConst{})

	addFunc("表示", [][]string{{"を", "と"}})
	list["表示"].ReturnNone = true
	addFunc("継続表示", [][]string{{"を", "と"}})
	list["継続表示"].ReturnNone = true
	for _, name := range []string{"連続表示", "連続無改行表示"} {
		addFunc(name, [][]string{{"と", "を"}})
		list[name].IsVariableJosi = true
		list[name].ReturnNone = true
	}
	addFunc("足", [][]string{{"に", "と"}, {"を"}})
	addFunc("掛", [][]string{{"に", "と"}, {"を"}})
	addFunc("合計", [][]string{{"と", "を", "の"}})
	list["合計"].IsVariableJosi = true
	addFunc("引", [][]string{{"から"}, {"を"}})
	addFunc("倍", [][]string{{"の", "を"}, {""}})
	addFunc("割", [][]string{{"を"}, {"で"}})
	addFunc("割余", [][]string{{"を"}, {"で"}})
	addFunc("偶数", [][]string{{"が"}})
	addFunc("奇数", [][]string{{"が"}})
	addFunc("二乗", [][]string{{"の", "を"}})
	addFunc("べき乗", [][]string{{"の"}, {"の"}})
	for _, name := range []string{"以上", "以下", "未満", "超"} {
		addFunc(name, [][]string{{"が"}, {""}})
	}
	for _, name := range []string{"等", "等無", "一致", "不一致"} {
		addFunc(name, [][]string{{"が"}, {"と"}})
	}
	addFunc("範囲内", [][]string{{"が"}, {"から"}, {"の", "までの"}})
	addFunc("連続加算", [][]string{{"を"}, {"に", "と"}})
	list["連続加算"].IsVariableJosi = true
	for _, name := range []string{"MAX", "最大値", "MIN", "最小値"} {
		addFunc(name, [][]string{{"の"}, {"と"}})
		list[name].IsVariableJosi = true
	}
	addFunc("CLAMP", [][]string{{"の", "を"}, {"から"}, {"までの", "で"}})
	for _, name := range []string{"論理OR", "論理AND"} {
		addFunc(name, [][]string{{"と"}, {"の"}})
	}
	addFunc("論理NOT", [][]string{{"の"}})
	addFunc("NOT", [][]string{{"の"}})
	for _, name := range []string{"SHIFT_L", "SHIFT_R", "SHIFT_UR"} {
		addFunc(name, [][]string{{"を"}, {"で"}})
	}
	addFunc("OR", [][]string{{"と"}, {"の"}})
	addFunc("AND", [][]string{{"と"}, {"の"}})
	addFunc("XOR", [][]string{{"と"}, {"の"}})
	addFunc("真偽判定", [][]string{{"の", "を"}})
	// 『??』と『A…B』は構文側から名前で引かれるので、署名が要る
	addFunc("ハテナ関数実行", [][]string{{"の", "を", "と"}})
	list["ハテナ関数実行"].ReturnNone = true
	addFunc("範囲", [][]string{{"から"}, {"の", "までの"}})
	addFunc("エラー発生", [][]string{{"の", "で"}})
	list["エラー発生"].ReturnNone = true
	for _, name := range []string{"システム関数一覧取得", "プラグイン一覧取得", "モジュール一覧取得", "予約語一覧取得", "助詞一覧取得"} {
		addFunc(name, nil)
	}
	addFunc("システム関数存在", [][]string{{"が", "の"}})
	addFunc("実行", [][]string{{"を", "に", "で"}})
	addFunc("JSオブジェクト取得", [][]string{{"の"}})
	addFunc("ASYNC", nil)
	list["ASYNC"].AsyncFn = true
	list["ASYNC"].ReturnNone = true
	addFunc("AWAIT実行", [][]string{{"を"}, {"で"}})
	list["AWAIT実行"].AsyncFn = true
	for _, name := range []string{"お願", "ください", "です", "拝啓", "敬具", "礼節レベル取得"} {
		addFunc(name, nil)
	}
	for _, name := range []string{"お願", "ください", "です", "拝啓", "敬具"} {
		list[name].ReturnNone = true
	}

	addFunc("文字数", [][]string{{"の"}})
	addFunc("LEN", [][]string{{"の"}})
	addFunc("何文字目", [][]string{{"で", "の"}, {"が"}})
	addFunc("CHR", [][]string{{"の"}})
	addFunc("ASC", [][]string{{"の"}})
	addFunc("文字挿入", [][]string{{"で", "の"}, {"に", "へ"}, {"を"}})
	addFunc("文字検索", [][]string{{"で", "の"}, {"から"}, {"を"}})
	addFunc("文字列連結", [][]string{{"と", "を"}})
	list["文字列連結"].IsVariableJosi = true
	addFunc("連結", [][]string{{"と", "を"}})
	list["連結"].IsVariableJosi = true
	addFunc("追加", [][]string{{"で", "に", "へ"}, {"を"}})
	addFunc("一行追加", [][]string{{"で", "に", "へ"}, {"を"}})
	addFunc("文字列分解", [][]string{{"を", "の", "で"}})
	addFunc("リフレイン", [][]string{{"を", "の"}, {"で"}})
	addFunc("出現回数", [][]string{{"で"}, {"の"}})
	addFunc("文字抜出", [][]string{{"で", "の"}, {"から"}, {"を", ""}})
	addFunc("文字抜き出", [][]string{{"で", "の"}, {"から"}, {"を", ""}})
	addFunc("MID", [][]string{{"で", "の"}, {"から"}, {"を", ""}})
	addFunc("文字左部分", [][]string{{"の", "で"}, {"だけ", ""}})
	addFunc("LEFT", [][]string{{"の", "で"}, {"だけ", ""}})
	addFunc("文字右部分", [][]string{{"の", "で"}, {"だけ", ""}})
	addFunc("RIGHT", [][]string{{"の", "で"}, {"だけ", ""}})
	addFunc("区切", [][]string{{"の", "を"}, {"で"}})
	addFunc("文字列分割", [][]string{{"を"}, {"で"}})
	addFunc("出現", [][]string{{"に", "で"}, {"が"}})
	addFunc("切取", [][]string{{"から", "の"}, {"まで", "を"}})
	addFunc("範囲切取", [][]string{{"で", "の"}, {"から"}, {"まで", "を"}})
	addFunc("文字削除", [][]string{{"の"}, {"から"}, {"だけ", "を", ""}})
	addFunc("文字始", [][]string{{"が"}, {"で", "から"}})
	addFunc("文字終", [][]string{{"が"}, {"で"}})
	addFunc("置換", [][]string{{"の", "で"}, {"を", "から"}, {"に", "へ"}})
	addFunc("単置換", [][]string{{"の", "で"}, {"を"}, {"に", "へ"}})
	for _, name := range []string{"トリム", "右トリム", "大文字変換", "小文字変換", "平仮名変換", "カタカナ変換", "英数全角変換", "英数半角変換"} {
		addFunc(name, [][]string{{"の", "を"}})
	}
	for _, name := range []string{"英数記号全角変換", "英数記号半角変換", "カタカナ全角変換", "カタカナ半角変換", "全角変換", "半角変換"} {
		addFunc(name, [][]string{{"の", "を"}})
	}
	addFunc("末尾空白除去", [][]string{{"の", "を"}})
	addFunc("空白除去", [][]string{{"の", "を"}})
	addFunc("左トリム", [][]string{{"の", "を"}})
	for _, name := range []string{"ゼロ埋", "空白埋"} {
		addFunc(name, [][]string{{"を"}, {"で"}})
	}
	addFunc("数列判定", [][]string{{"を", "が"}})
	addFunc("数字判定", [][]string{{"を", "が"}})
	addFunc("かなか判定", [][]string{{"を", "の", "が"}})
	addFunc("カタカナ判定", [][]string{{"を", "の", "が"}})
	addFunc("通貨形式", [][]string{{"を", "の"}})
	for _, name := range []string{"URLエンコード", "BASE64エンコード"} {
		addFunc(name, [][]string{{"を", "から"}})
	}
	for _, name := range []string{"URLデコード", "BASE64デコード"} {
		addFunc(name, [][]string{{"を", "へ", "に"}})
	}
	addFunc("URLパラメータ解析", [][]string{{"を", "の", "から"}})
	addFunc("拡張子変更", [][]string{{"の", "を", "から"}, {"に", "へ"}})
	addFunc("終端パス追加", [][]string{{"に", "へ"}})
	addFunc("終端パス除去", [][]string{{"の", "から"}})
	addFunc("終端パス削除", [][]string{{"の", "から"}})
	addFunc("パス抽出", [][]string{{"の", "から"}})

	addFunc("配列結合", [][]string{{"を"}, {"で"}})
	addFunc("配列只結合", [][]string{{"を"}})
	addFunc("配列検索", [][]string{{"の", "から"}, {"を"}})
	addFunc("要素数", [][]string{{"の"}})
	addFunc("配列要素数", [][]string{{"の"}})
	addFunc("配列挿入", [][]string{{"の"}, {"に", "へ"}, {"を"}})
	addFunc("配列一括挿入", [][]string{{"の"}, {"に", "へ"}, {"を"}})
	for _, name := range []string{"配列ソート", "配列数値ソート", "配列逆順"} {
		addFunc(name, [][]string{{"の", "を"}})
	}
	addFunc("配列数値変換", [][]string{{"の", "を"}})
	addFunc("配列シャッフル", [][]string{{"の", "を"}})
	addFunc("配列削除", [][]string{{"の", "から"}, {"を"}})
	addFunc("配列切取", [][]string{{"の", "から"}, {"を"}})
	addFunc("配列取出", [][]string{{"の"}, {"から"}, {"を"}})
	addFunc("配列ポップ", [][]string{{"の", "から"}})
	for _, name := range []string{"配列プッシュ", "配列追加"} {
		addFunc(name, [][]string{{"に", "へ"}, {"を"}})
	}
	addFunc("配列複製", [][]string{{"を"}})
	addFunc("配列範囲コピー", [][]string{{"の", "から"}, {"を"}})
	addFunc("参照", [][]string{{"から", "の"}, {"を"}})
	addFunc("配列参照", [][]string{{"の", "から"}, {"を"}})
	addFunc("配列足", [][]string{{"に", "へ", "と"}, {"を"}})
	addFunc("配列入替", [][]string{{"の"}, {"と"}, {"を"}})
	for _, name := range []string{"配列最大値", "配列最小値", "配列合計"} {
		addFunc(name, [][]string{{"の"}})
	}
	addFunc("配列連番作成", [][]string{{"から"}, {"までの", "まで", "の"}})
	addFunc("配列要素作成", [][]string{{"を"}, {"だけ", "で"}})
	addFunc("配列マップ", [][]string{{"を"}, {"へ", "に"}})
	addFunc("配列関数適用", [][]string{{"を"}, {"へ", "に"}})
	addFunc("配列フィルタ", [][]string{{"で", "の"}, {"を", "について"}})
	addFunc("配列カスタムソート", [][]string{{"で"}, {"の", "を"}})
	for _, name := range []string{"表ソート", "表数値ソート"} {
		addFunc(name, [][]string{{"の"}, {"を"}})
	}
	addFunc("表ピックアップ", [][]string{{"の"}, {"から"}, {"を", "で"}})
	addFunc("表完全一致ピックアップ", [][]string{{"の"}, {"から"}, {"を", "で"}})
	addFunc("表検索", [][]string{{"の"}, {"で", "に"}, {"から"}, {"を"}})
	for _, name := range []string{"表列数", "表行数", "表行列交換", "表右回転"} {
		addFunc(name, [][]string{{"の", "を"}})
	}
	addFunc("表重複削除", [][]string{{"の"}, {"を", "で"}})
	addFunc("表列取得", [][]string{{"の"}, {"を"}})
	addFunc("表列挿入", [][]string{{"の"}, {"に", "へ"}, {"を"}})
	addFunc("表列削除", [][]string{{"の"}, {"を"}})
	addFunc("表列合計", [][]string{{"の"}, {"を", "で"}})
	addFunc("表曖昧検索", [][]string{{"の"}, {"から"}, {"で"}, {"を"}})
	addFunc("表正規表現ピックアップ", [][]string{{"の", "で"}, {"から"}, {"を"}})

	addFunc("辞書キー列挙", [][]string{{"の"}})
	addFunc("ハッシュキー列挙", [][]string{{"の"}})
	addFunc("ハッシュ内容列挙", [][]string{{"の"}})
	addFunc("辞書キー削除", [][]string{{"から", "の"}, {"を"}})
	addFunc("ハッシュキー削除", [][]string{{"から", "の"}, {"を"}})
	addFunc("辞書キー存在", [][]string{{"の", "に"}, {"が"}})
	addFunc("ハッシュキー存在", [][]string{{"の", "に"}, {"が"}})
	addFunc("変数型確認", [][]string{{"の"}})
	for _, name := range []string{"文字列変換", "整数変換", "実数変換"} {
		addFunc(name, [][]string{{"を"}})
	}
	addFunc("TOSTR", [][]string{{"を"}})
	addFunc("TOINT", [][]string{{"を"}})
	addFunc("TOFLOAT", [][]string{{"を"}})
	addFunc("INT", [][]string{{"の"}})
	addFunc("FLOAT", [][]string{{"の"}})
	addFunc("TYPEOF", [][]string{{"の"}})
	addFunc("NAN判定", [][]string{{"を"}})
	addFunc("非数判定", [][]string{{"を"}})
	addFunc("HEX", [][]string{{"の"}})
	addFunc("進数変換", [][]string{{"を", "の"}, {"", "へ"}})
	addFunc("二進", [][]string{{"を", "の", "から"}})
	addFunc("二進表示", [][]string{{"を", "の", "から"}})
	addFunc("JSONエンコード", [][]string{{"を", "の"}})
	addFunc("JSONデコード", [][]string{{"を", "の", "から"}})
	addFunc("JSON変換", [][]string{{"を", "の", "から"}})
	addFunc("JSON取得", [][]string{{"を", "の", "から"}})
	addFunc("JSONエンコード整形", [][]string{{"を", "の"}})

	for _, name := range []string{"今", "今日", "明日", "昨日", "今年", "来年", "去年", "今月", "来月", "先月", "システム時間", "システム時間ミリ秒"} {
		addFunc(name, nil)
	}
	addFunc("曜日", [][]string{{"の"}})
	addFunc("曜日番号取得", [][]string{{"の"}})
	for _, name := range []string{"UNIXTIME変換", "UNIX時間変換"} {
		addFunc(name, [][]string{{"の", "を", "から"}})
	}
	addFunc("日時変換", [][]string{{"を", "から"}})
	addFunc("和暦変換", [][]string{{"を"}})
	for _, name := range []string{"年数差", "月数差", "日数差", "時間差", "分差", "秒差"} {
		addFunc(name, [][]string{{"と", "から"}, {"の", "までの"}})
	}
	addFunc("日時差", [][]string{{"と", "から"}, {"の", "までの"}, {"による"}})
	for _, name := range []string{"時間加算", "日付加算", "日時加算"} {
		addFunc(name, [][]string{{"に"}, {"を"}})
	}
	addFunc("正規表現マッチ", [][]string{{"を", "が"}, {"で", "に"}})
	addFunc("正規表現抽出", [][]string{{"から", "を"}, {"で"}})
	addFunc("正規表現置換", [][]string{{"の"}, {"を", "から"}, {"で", "に", "へ"}})
	addFunc("正規表現区切", [][]string{{"を"}, {"で"}})

	addFunc("秒待", [][]string{{""}})
	list["秒待"].AsyncFn = true
	list["秒待"].ReturnNone = true
	for _, name := range []string{"秒後", "秒毎"} {
		addFunc(name, [][]string{{"を"}, {""}})
	}
	addFunc("タイマー停止", [][]string{{"の", "で"}})

	return list
}
