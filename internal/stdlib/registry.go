package stdlib

import "github.com/kujirahand/nadesiko3go/internal/lexer"

// ParserFuncList returns the plugin_system metadata needed by the lexer and
// parser. Function implementations are added in later stages; keeping the
// signatures here lets stage 1 parse the compatibility fixtures now.
// nullConst and undefinedConst distinguish the two empty values in the
// signature table, which a plain nil could not.
type nullConst struct{}
type undefinedConst struct{}

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
	addConst("永遠", true)
	addConst("オン", true)
	addConst("オフ", false)
	addConst("改行", "\n")
	addConst("タブ", "\t")
	addConst("空", "")
	addConst("NULL", nullConst{})
	addConst("undefined", undefinedConst{})
	addConst("未定義", undefinedConst{})
	addConst("エラーメッセージ", "")
	addConst("対象", "")
	addConst("対象キー", "")
	addConst("回数", "")

	addFunc("表示", [][]string{{"を", "と"}})
	list["表示"].ReturnNone = true
	addFunc("足", [][]string{{"に", "と"}, {"を"}})
	addFunc("掛", [][]string{{"に", "と"}, {"を"}})
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

	addFunc("文字数", [][]string{{"の"}})
	addFunc("何文字目", [][]string{{"で", "の"}, {"が"}})
	addFunc("CHR", [][]string{{"の"}})
	addFunc("ASC", [][]string{{"の"}})
	addFunc("文字挿入", [][]string{{"で", "の"}, {"に", "へ"}, {"を"}})
	addFunc("文字検索", [][]string{{"で", "の"}, {"から"}, {"を"}})
	addFunc("文字列連結", [][]string{{"と", "を"}})
	list["文字列連結"].IsVariableJosi = true
	addFunc("文字列分解", [][]string{{"を", "の", "で"}})
	addFunc("リフレイン", [][]string{{"を", "の"}, {"で"}})
	addFunc("出現回数", [][]string{{"で"}, {"の"}})
	addFunc("文字抜出", [][]string{{"で", "の"}, {"から"}, {"を", ""}})
	addFunc("文字左部分", [][]string{{"の", "で"}, {"だけ", ""}})
	addFunc("文字右部分", [][]string{{"の", "で"}, {"だけ", ""}})
	addFunc("区切", [][]string{{"の", "を"}, {"で"}})
	addFunc("文字削除", [][]string{{"の"}, {"から"}, {"だけ", "を", ""}})
	addFunc("文字始", [][]string{{"が"}, {"で", "から"}})
	addFunc("文字終", [][]string{{"が"}, {"で"}})
	addFunc("置換", [][]string{{"の", "で"}, {"を", "から"}, {"に", "へ"}})
	addFunc("単置換", [][]string{{"の", "で"}, {"を"}, {"に", "へ"}})
	for _, name := range []string{"トリム", "右トリム", "大文字変換", "小文字変換", "平仮名変換", "カタカナ変換", "英数全角変換", "英数半角変換"} {
		addFunc(name, [][]string{{"の", "を"}})
	}
	for _, name := range []string{"ゼロ埋", "空白埋"} {
		addFunc(name, [][]string{{"を"}, {"で"}})
	}
	addFunc("数列判定", [][]string{{"を", "が"}})

	addFunc("配列結合", [][]string{{"を"}, {"で"}})
	addFunc("配列検索", [][]string{{"の", "から"}, {"を"}})
	addFunc("要素数", [][]string{{"の"}})
	addFunc("配列挿入", [][]string{{"の"}, {"に", "へ"}, {"を"}})
	for _, name := range []string{"配列ソート", "配列数値ソート", "配列逆順"} {
		addFunc(name, [][]string{{"の", "を"}})
	}
	addFunc("配列削除", [][]string{{"の", "から"}, {"を"}})
	addFunc("配列取出", [][]string{{"の"}, {"から"}, {"を"}})
	addFunc("配列ポップ", [][]string{{"の", "から"}})
	for _, name := range []string{"配列プッシュ", "配列追加"} {
		addFunc(name, [][]string{{"に", "へ"}, {"を"}})
	}
	addFunc("配列複製", [][]string{{"を"}})
	for _, name := range []string{"配列最大値", "配列最小値", "配列合計"} {
		addFunc(name, [][]string{{"の"}})
	}
	addFunc("配列連番作成", [][]string{{"から"}, {"までの", "まで", "の"}})
	addFunc("配列要素作成", [][]string{{"を"}, {"だけ", "で"}})
	addFunc("配列マップ", [][]string{{"を"}, {"へ", "に"}})
	addFunc("配列フィルタ", [][]string{{"で", "の"}, {"を", "について"}})

	addFunc("辞書キー列挙", [][]string{{"の"}})
	addFunc("辞書キー削除", [][]string{{"から", "の"}, {"を"}})
	addFunc("辞書キー存在", [][]string{{"の", "に"}, {"が"}})
	addFunc("変数型確認", [][]string{{"の"}})
	for _, name := range []string{"文字列変換", "整数変換", "実数変換"} {
		addFunc(name, [][]string{{"を"}})
	}
	addFunc("JSONエンコード", [][]string{{"を", "の"}})
	addFunc("JSONデコード", [][]string{{"を", "の", "から"}})
	addFunc("正規表現マッチ", [][]string{{"を", "が"}, {"で", "に"}})
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
