package parser_test

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/parser"
)

// TestParseSourceRequireBuiltinPlugin は、バイナリに組み込み済みの
// プラグイン名への取込文が読み飛ばされて正常に続行することを確認する
// (#229)。マニュアルの `!「nadesiko3-sqlite3」を取り込む` が
// そのまま動くのが狙い。
func TestParseSourceRequireBuiltinPlugin(t *testing.T) {
	funcs := lexer.FuncList{
		"表示":          {Name: "表示", Type: "func", Josi: [][]string{{"を", "と"}}},
		"SQLITE3開":    {Name: "SQLITE3開", Type: "func", Josi: [][]string{{"を", "から"}}},
		"開":           {Name: "開", Type: "func", Josi: [][]string{{"を", "から"}}},
		"CSV取得":       {Name: "CSV取得", Type: "func", Josi: [][]string{{"を", "の", "から"}}},
		"SIN":         {Name: "SIN", Type: "func", Josi: [][]string{{"の"}}},
		"TOMLデコード":    {Name: "TOMLデコード", Type: "func", Josi: [][]string{{"を", "の", "から"}}},
		"OFFICEバージョン": {Name: "OFFICEバージョン", Type: "const", Value: "?"},
	}

	for _, code := range []string{
		`!「nadesiko3-sqlite3」を取り込む。` + "\n" + `「ok」と表示。`,
		`!「nadesiko3-sqlite3.js」を取り込む。` + "\n" + `「ok」と表示。`,
		`!「nadesiko3-office」を取り込む。` + "\n" + `「ok」と表示。`,
		`!「plugin_node」を取り込む。` + "\n" + `「ok」と表示。`,
		`!「plugin_node.mjs」を取り込む。` + "\n" + `「ok」と表示。`,
		`!「plugin_csv」を取り込む。` + "\n" + `「ok」と表示。`,
		`!「plugin_math」を取り込む。` + "\n" + `「ok」と表示。`,
		`!「plugin_toml」を取り込む。` + "\n" + `「ok」と表示。`,
		`!「nadesiko3-toml」を取り込む。` + "\n" + `「ok」と表示。`,
		`!「plugin_system」を取り込む。` + "\n" + `「ok」と表示。`,
	} {
		name := code
		if i := strings.Index(code, "」"); i >= 0 {
			name = code[:i+len("」")]
		}
		t.Run(name, func(t *testing.T) {
			if _, err := parser.ParseSource(code, "main.nako3", funcs); err != nil {
				t.Fatalf("ParseSource(%q) = %v", code, err)
			}
		})
	}
}

// TestParseSourceRequireBuiltinPluginNotEmbedded は、組み込まれていない
// プラグイン名は従来どおり「ファイルが見つかりません」になることを確認する。
// （例: gonako-cui には office/pdf/image が入っていない）
func TestParseSourceRequireBuiltinPluginNotEmbedded(t *testing.T) {
	funcs := lexer.FuncList{
		"表示": {Name: "表示", Type: "func", Josi: [][]string{{"を", "と"}}},
	}
	_, err := parser.ParseSource("!「nadesiko3-office」を取り込む。", "main.nako3", funcs)
	if err == nil || !strings.Contains(err.Error(), "見つかりません") {
		t.Fatalf("組み込みのないプラグイン名: err = %v, want 見つかりません", err)
	}
	// 未知のプラグイン名も従来どおりファイル解決のエラーになる
	_, err = parser.ParseSource("!「unknown-plugin」を取り込む。", "main.nako3", funcs)
	if err == nil {
		t.Fatal("未知のプラグイン名でエラーになりませんでした")
	}
}
