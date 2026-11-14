package commanddoc

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/nodelib"
	"github.com/kujirahand/nadesiko3go/internal/officelib"
	"github.com/kujirahand/nadesiko3go/internal/sqlitelib"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

func TestCommandsLoadsEmbeddedList(t *testing.T) {
	list, err := Commands()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 100 {
		t.Fatalf("命令一覧が少なすぎます: %d件", len(list))
	}
	for _, cmd := range list {
		if cmd.Name == "" {
			t.Fatal("名前のない命令があります")
		}
		if cmd.DocURL == "" {
			t.Fatalf("『%s』のマニュアルURLがありません", cmd.Name)
		}
	}
}

// TestCommandsCoverRuntimeConsts は #474 の回帰防止。
// ランタイムに登録された定数・システム変数 (FuncItem の Type が const/var) が、
// 埋め込みの命令一覧にも同じ種別で載っていることを確認する。
// 落ちた場合は `just gen-const-list` の実行漏れを疑うこと。
// guilib は cgo (GTK/WebKit) 依存でこのパッケージから import できないため、
// 代表として『母艦』の収録だけを別途確認する。
func TestCommandsCoverRuntimeConsts(t *testing.T) {
	list, err := Commands()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Command{}
	for _, cmd := range list {
		byName[cmd.Name] = cmd
	}
	lists := map[string]lexer.FuncList{
		"stdlib":    stdlib.ParserFuncList(),
		"nodelib":   nodelib.New().FuncList(),
		"officelib": officelib.New().FuncList(),
		"sqlitelib": sqlitelib.New().FuncList(),
	}
	expected := map[string]string{}
	for _, funcs := range lists {
		for name, item := range funcs {
			if item.Type == "const" || item.Type == "var" {
				expected[name] = item.Type
			}
		}
	}
	// guilib は import できないため、定数名を明示して期待値に加える。
	for _, name := range []string{
		"ドロップファイル", "フォーム値", "フォームチェック", "DOM親要素",
		"DOMスキン", "DOMスキン辞書", "DOM部品オプション", "母艦",
	} {
		expected[name] = "const"
	}
	checked := 0
	for name, typ := range expected {
		checked++
		cmd, ok := byName[name]
		if !ok {
			t.Errorf("定数『%s』が命令一覧にありません (#474)", name)
			continue
		}
		if cmd.Type != typ {
			t.Errorf("『%s』の種別が違います: 一覧=%s ランタイム=%s", name, cmd.Type, typ)
		}
	}
	if checked == 0 {
		t.Fatal("検査対象の定数が0件でした")
	}
	// 逆方向: 一覧にある const/var が、ランタイム側に実在することも確認する。
	// ソースで削除・改名した定数が一覧に残り続ける事故を検知するため。
	for _, cmd := range list {
		if cmd.Type != "const" && cmd.Type != "var" {
			continue
		}
		if _, ok := expected[cmd.Name]; !ok {
			t.Errorf("一覧の『%s』(%s) がランタイム登録にありません。削除・改名の反映漏れではありませんか (#474)", cmd.Name, cmd.Type)
		}
	}
	// PI は Issue #474 の発端。種別・分類・読みまで固定で確認する。
	pi, ok := byName["PI"]
	if !ok {
		t.Fatal("『PI』が命令一覧にありません (#474)")
	}
	if pi.Type != "const" || pi.Category != "システム定数" || pi.Yomi != "パイ" {
		t.Errorf("『PI』の内容が違います: %+v", pi)
	}
	if cmd, ok := byName["母艦"]; !ok || cmd.Type != "const" {
		t.Errorf("guilib の定数『母艦』が命令一覧にありません (#474): %+v", cmd)
	}
}

func TestDocURL(t *testing.T) {
	got := DocURL("秒待")
	want := "https://nadesi.com/v3/doc/index.php?gonako%2F%E7%A7%92%E5%BE%85"
	if got != want {
		t.Fatalf("URLが違います: %s", got)
	}
}

var testList = []Command{
	{Name: "秒待", Yomi: "びょうまつ", Desc: "N秒の間待機する", Category: "タイマー", Plugin: "plugin_system"},
	{Name: "秒待機", Yomi: "びょうたいき", Desc: "N秒の間待機する(『秒待』と同じ)", Category: "タイマー"},
	{Name: "表示", Yomi: "ひょうじ", Desc: "Sを表示して改行する", Category: "システム"},
	{Name: "テキスト分割", Yomi: "てきすとぶんかつ", Desc: "文字列を区切り文字で分割する", Category: "文字列処理"},
}

// 完全一致・前方一致・部分一致の順に並ぶこと。
func TestSearchOrder(t *testing.T) {
	found := Search(testList, []string{"秒待"})
	if len(found) != 2 {
		t.Fatalf("件数が違います: %d", len(found))
	}
	if found[0].Name != "秒待" || found[0].Score != 100 {
		t.Fatalf("1件目が違います: %#v", found[0])
	}
	if found[1].Name != "秒待機" || found[1].Score != 80 {
		t.Fatalf("2件目が違います: %#v", found[1])
	}
}

// 命令名以外（読み・説明・分類）にも一致すること。
func TestSearchMatchesYomiAndDesc(t *testing.T) {
	for _, keyword := range []string{"ひょうじ", "改行", "タイマー"} {
		if len(Search(testList, []string{keyword})) == 0 {
			t.Errorf("『%s』が見つかりません", keyword)
		}
	}
	if got := Search(testList, []string{"改行"}); len(got) != 1 || got[0].Name != "表示" {
		t.Errorf("説明での検索結果が違います: %#v", got)
	}
}

// キーワードを複数与えたらAND検索になること。
func TestSearchMultipleKeywordsAreAND(t *testing.T) {
	if got := Search(testList, []string{"秒待", "びょうまつ"}); len(got) != 1 || got[0].Name != "秒待" {
		t.Fatalf("AND検索の結果が違います: %#v", got)
	}
	if got := Search(testList, []string{"秒待", "文字列"}); len(got) != 0 {
		t.Fatalf("一致しないはずです: %#v", got)
	}
}

func TestSearchNoMatch(t *testing.T) {
	if got := Search(testList, []string{"存在しない命令"}); len(got) != 0 {
		t.Fatalf("一致しないはずです: %#v", got)
	}
}

// マニュアルサイトの検索結果HTMLから、リンクだけを取り出せること。
// メニュー部分のリンクを拾わないことも確かめる。
func TestParseWebResults(t *testing.T) {
	body := `<div id="menu"><ul><li><a href="https://example.com/menu">メニュー</a></li></ul></div>
<div class="box">
    <!-- result -->
    <div><ul>
          <li><a href="https://nadesi.com/v3/doc/index.php?gonako%2F%E7%A7%92%E5%BE%85&amp;show">gonako/秒待</a></li>
          <li><a href="https://nadesi.com/v3/doc/index.php?plugin_system%2F%E7%A7%92%E5%BE%85&amp;show">plugin_system/秒待</a></li>
        </ul></div>
  </div>
</div>`
	got := parseWebResults(body)
	if len(got) != 2 {
		t.Fatalf("件数が違います: %#v", got)
	}
	if got[0].Title != "gonako/秒待" {
		t.Errorf("タイトルが違います: %q", got[0].Title)
	}
	if !strings.HasSuffix(got[0].URL, "&show") || strings.Contains(got[0].URL, "&amp;") {
		t.Errorf("URLのエスケープが戻っていません: %q", got[0].URL)
	}
}

func TestParseWebResultsWithoutResultBlock(t *testing.T) {
	if got := parseWebResults(`<html><li><a href="x">y</a></li></html>`); len(got) != 0 {
		t.Fatalf("結果欄が無ければ空のはずです: %#v", got)
	}
}

// wnakoの命令一覧も埋め込まれていて、マニュアルへのリンクが張れること（#101）。
func TestWNakoCommandsLoadsEmbeddedList(t *testing.T) {
	list, err := WNakoCommands()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 500 {
		t.Fatalf("wnakoの命令一覧が少なすぎます: %d件", len(list))
	}
	var hyouji Command
	for _, cmd := range list {
		if cmd.Name == "" {
			t.Fatal("名前のない命令があります")
		}
		if cmd.DocURL == "" {
			t.Fatalf("『%s』のマニュアルURLがありません", cmd.Name)
		}
		if cmd.Name == "表示" {
			hyouji = cmd
		}
	}
	if hyouji.Name == "" {
		t.Fatal("wnakoの命令一覧に『表示』がありません")
	}
	if hyouji.Plugin != "plugin_system" {
		t.Fatalf("『表示』のプラグインが違います: %q", hyouji.Plugin)
	}
	if hyouji.Template != "【S】を表示" {
		t.Fatalf("『表示』の書式が違います: %q", hyouji.Template)
	}
	// wnakoの命令だけを検索できること。
	if found := Search(list, []string{"カメ"}); len(found) == 0 {
		t.Fatal("wnakoの命令一覧からタートル系の命令が見つかりません")
	}
}

func TestWNakoDocURL(t *testing.T) {
	got := WNakoDocURL("plugin_system", "表示")
	want := "https://nadesi.com/v3/doc/index.php?plugin_system%2F%E8%A1%A8%E7%A4%BA"
	if got != want {
		t.Fatalf("URLが違います: %s", got)
	}
	if got := WNakoDocURL("", "表示"); !strings.HasSuffix(got, "%E8%A1%A8%E7%A4%BA") {
		t.Fatalf("プラグイン名が無い場合のURLが違います: %s", got)
	}
}
