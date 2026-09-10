package commanddoc

import (
	"strings"
	"testing"
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
