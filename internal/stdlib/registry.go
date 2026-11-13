package stdlib

import (
	"math"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/version"
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

	addConst("はい", true)                      // @真を表す論理値で、条件判定に使う定数です // @はい
	addConst("いいえ", false)                    // @偽を表す論理値で、条件判定に使う定数です // @いいえ
	addConst("真", true)                       // @真を表す論理値で、条件判定に使う定数です // @しん
	addConst("偽", false)                      // @偽を表す論理値で、条件判定に使う定数です // @ぎ
	addConst("TRUE", true)                    // @真を表す論理値で、条件判定に使う定数です // @TRUE
	addConst("FALSE", false)                  // @偽を表す論理値で、条件判定に使う定数です // @FALSE
	addConst("true", true)                    // @真を表す論理値で、条件判定に使う定数です // @true
	addConst("false", false)                  // @偽を表す論理値で、条件判定に使う定数です // @false
	addConst("永遠", true)                      // @常に真であり、無限ループの条件などに使う定数です // @えいえん
	addConst("オン", true)                      // @有効状態を表す真の論理値定数です // @オン
	addConst("オフ", false)                     // @無効状態を表す偽の論理値定数です // @オフ
	addConst("OK", true)                      // @成功または肯定を表す真の論理値定数です // @OK
	addConst("NG", false)                     // @失敗または否定を表す偽の論理値定数です // @NG
	addConst("キャンセル", 0)                      // @ダイアログなどでの取消を表す数値定数です // @キャンセル
	addConst("PI", math.Pi)                   // @円周率を表す math.Pi の数値定数です // @パイ
	addConst("改行", "\n")                      // @改行文字である LF 文字列を表す定数です // @かいぎょう
	addConst("CR", "\r")                      // @キャリッジリターン文字を表す文字列定数です // @CR
	addConst("LF", "\n")                      // @ラインフィード文字を表す文字列定数です // @LF
	addConst("タブ", "\t")                      // @タブ文字を表す文字列定数です // @タブ
	addConst("空", "")                         // @長さが零の空文字列を表す定数です // @から
	addConst("非数", math.NaN())                // @数値ではない値である NaN を表す定数です // @ひすう
	addConst("無限大", math.Inf(1))              // @正の無限大である +Inf を表す数値定数です // @むげんだい
	addConst("NULL", nullConst{})             // @null 相当の値を表す定数です // @NULL
	addConst("undefined", undefinedConst{})   // @未定義値を表す定数です // @undefined
	addConst("未定義", undefinedConst{})         // @undefined と同じ未定義値を表す定数です // @みていぎ
	addConst("エラーメッセージ", "")                  // @発生したエラーのメッセージを保持する変数です // @エラーメッセージ
	addConst("対象", "")                        // @繰り返しや命令で対象となる値を保持する変数です // @たいしょう
	addConst("対象キー", "")                      // @辞書反復中の現在のキーを保持する変数です // @たいしょうキー
	addConst("回数", "")                        // @繰り返し処理の現在回数を保持する変数です // @かいすう
	addConst("カッコ", "「")                      // @開始かぎ括弧「を表す文字列定数です // @カッコ
	addConst("カッコ閉", "」")                     // @終了かぎ括弧」を表す文字列定数です // @カッコとじ
	addConst("カッコ閉じ", "」")                    // @終了かぎ括弧」を表すカッコ閉の別名定数です // @カッコとじ
	addConst("波カッコ", "{")                     // @開始波括弧 { を表す文字列定数です // @なみカッコ
	addConst("波カッコ閉", "}")                    // @終了波括弧 } を表す文字列定数です // @なみカッコとじ
	addConst("ナデシコエンジン", "nadesi.com/v3")     // @なでしこエンジン識別子を表す文字列定数です // @ナデシコエンジン
	addConst("ナデシコバージョン", version.Nadesiko)   // @なでしこのバージョン番号を表す文字列定数です // @ナデシコバージョン
	addConst("ナデシコ言語バージョン", version.Nadesiko) // @ナデシコバージョンと同じ値を表す定数です // @ナデシコげんごバージョン
	addConst("ナデシコ種類", "?")                   // @実行環境の種類を保持する初期値 ? の変数です // @ナデシコしゅるい
	addConst("プラグイン名", "メイン")                 // @現在のプラグイン名を保持するシステム変数です // @プラグインめい
	addConst("名前空間", "")                      // @現在使用中の名前空間を保持するシステム変数です // @なまえくうかん
	addConst("表示ログ", "")                      // @表示命令で出力した内容のログを保持する変数です // @ひょうじログ
	addConst("全角カナ一覧", fullWidthKana)         // @全角カタカナを並べた一覧文字列の定数です // @ぜんかくカナいちらん
	addConst("全角カナ濁音一覧", fullWidthVoicedKana) // @全角カタカナ濁音を並べた一覧文字列の定数です // @ぜんかくカナだくおんいちらん
	addConst("半角カナ一覧", halfWidthKana)         // @半角カタカナを並べた一覧文字列の定数です // @はんかくカナいちらん
	addConst("半角カナ濁音一覧", halfWidthVoicedKana) // @半角カタカナ濁音を並べた一覧文字列の定数です // @はんかくカナだくおんいちらん
	addConst("元号データ", eraDataConst{})         // @和暦への変換に使用する元号データの定数です // @げんごうデータ
	addConst("戻値無", 0)                        // @関数の戻り値がないことを表す数値定数です // @もどりちなし
	addConst("戻値有", 1)                        // @関数の戻り値があることを表す数値定数です // @もどりちあり
	addFunc("空配列", nil)
	addFunc("空辞書", nil)
	addFunc("空ハッシュ", nil)
	addFunc("空オブジェクト", nil)
	// 『正規表現マッチ』が部分マッチを入れる先。初期値は空の配列。
	addConst("抽出文字列", emptyArrayConst{}) // @正規表現マッチの部分一致を格納する変数です // @ちゅうしゅつもじれつ

	addFunc("表示", [][]string{{"を", "と"}})
	list["表示"].ReturnNone = true
	addFunc("言", [][]string{{"を", "と"}})
	list["言"].ReturnNone = true
	addFunc("コンソール表示", [][]string{{"を", "と"}})
	list["コンソール表示"].ReturnNone = true
	addFunc("継続表示", [][]string{{"を", "と"}})
	list["継続表示"].ReturnNone = true
	addFunc("表示ログクリア", nil)
	list["表示ログクリア"].ReturnNone = true
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
	for _, name := range []string{"システム関数一覧取得", "グローバル関数一覧取得", "プラグイン一覧取得", "モジュール一覧取得", "予約語一覧取得", "助詞一覧取得"} {
		addFunc(name, nil)
	}
	addFunc("システム関数存在", [][]string{{"が", "の"}})
	addFunc("実行", [][]string{{"を", "に", "で"}})
	addFunc("JSオブジェクト取得", [][]string{{"の"}})
	addFunc("実行時間計測", [][]string{{"の"}})
	addFunc("デバッグ表示", [][]string{{"と", "を", "の"}})
	list["デバッグ表示"].ReturnNone = true
	addFunc("ハテナ関数設定", [][]string{{"を", "の"}})
	list["ハテナ関数設定"].ReturnNone = true
	addFunc("プラグイン名設定", [][]string{{"に", "へ"}})
	list["プラグイン名設定"].ReturnNone = true
	addFunc("名前空間設定", [][]string{{"に", "へ"}})
	list["名前空間設定"].ReturnNone = true
	addFunc("名前空間ポップ", nil)
	list["名前空間ポップ"].ReturnNone = true
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
	addFunc("拡張子抽出", [][]string{{"から", "の"}})
	addFunc("終端パス追加", [][]string{{"に", "へ"}})
	addFunc("終端パス除去", [][]string{{"の", "から"}})
	addFunc("終端パス削除", [][]string{{"の", "から"}})
	addFunc("パス抽出", [][]string{{"の", "から"}})
	addFunc("ファイル名抽出", [][]string{{"の", "から"}})

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
	addFunc("RGB", [][]string{{"と"}, {"の"}, {"で"}})
	addFunc("JSONエンコード", [][]string{{"を", "の"}})
	addFunc("JSONデコード", [][]string{{"を", "の", "から"}})
	addFunc("JSON変換", [][]string{{"を", "の", "から"}})
	addFunc("JSON取得", [][]string{{"を", "の", "から"}})
	addFunc("JSONエンコード整形", [][]string{{"を", "の"}})

	for _, name := range []string{"今", "今日", "明日", "昨日", "今年", "来年", "去年", "今月", "来月", "先月", "システム時間", "システム時間ミリ秒", "時間ミリ秒取得"} {
		addFunc(name, nil)
	}
	addFunc("曜日", [][]string{{"の"}})
	addFunc("曜日番号取得", [][]string{{"の"}})
	for _, name := range []string{"UNIXTIME変換", "UNIX時間変換"} {
		addFunc(name, [][]string{{"の", "を", "から"}})
	}
	addFunc("日時変換", [][]string{{"を", "から"}})
	addFunc("日時書式変換", [][]string{{"を"}, {"で"}})
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

	for _, name := range []string{"秒待", "秒待機"} {
		addFunc(name, [][]string{{""}})
		list[name].AsyncFn = true
		list[name].ReturnNone = true
	}
	for _, name := range []string{"秒後", "秒毎", "秒タイマー開始時"} {
		addFunc(name, [][]string{{"を"}, {""}})
	}
	addFunc("タイマー停止", [][]string{{"の", "で"}})
	addFunc("全タイマー停止", nil)
	list["全タイマー停止"].ReturnNone = true

	return list
}
