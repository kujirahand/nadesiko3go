package guilib

import (
	"fmt"
	"strings"
	"testing"
)

// Issue #357: fragment 要素の HTML取得・テキスト取得の結果が、登録時に
// 子孫 markup を全ノードへ保持していた従来実装と一致することの固定。
func TestIssue357FragmentHTMLAndTextGet(t *testing.T) {
	screen := NewScreen()
	wrapper := screen.DisplayHTML(`<div id="a"><div id="b"><span id="c">text</span></div></div>`)

	a, ok := screen.queryByID("a")
	if !ok {
		t.Fatal("a not found")
	}
	b, ok := screen.queryByID("b")
	if !ok {
		t.Fatal("b not found")
	}
	c, ok := screen.queryByID("c")
	if !ok {
		t.Fatal("c not found")
	}

	if got, err := screen.html(wrapper); err != nil ||
		got != `<div id="a" data-gonako-handle="2"><div id="b" data-gonako-handle="3"><span id="c" data-gonako-handle="4">text</span></div></div>` {
		t.Errorf("html(wrapper) = %q, err = %v", got, err)
	}
	if got, err := screen.html(a); err != nil ||
		got != `<div id="b" data-gonako-handle="3"><span id="c" data-gonako-handle="4">text</span></div>` {
		t.Errorf("html(a) = %q, err = %v", got, err)
	}
	if got, err := screen.html(b); err != nil ||
		got != `<span id="c" data-gonako-handle="4">text</span>` {
		t.Errorf("html(b) = %q, err = %v", got, err)
	}
	if got, err := screen.html(c); err != nil || got != "text" {
		t.Errorf("html(c) = %q, err = %v, want text", got, err)
	}
	if got, err := screen.text(c); err != nil || got != "text" {
		t.Errorf("text(c) = %q, err = %v, want text", got, err)
	}
	if got, err := screen.text(b); err != nil ||
		got != `<span id="c" data-gonako-handle="4">text</span>` {
		t.Errorf("text(b) = %q, err = %v", got, err)
	}
}

// 子要素への変更が祖先の HTML取得へ伝播しないこと（従来実装でも祖先の保持
// 文字列は古いままだった。同等の範囲であることを固定する）。
// 一方、変更した要素自身の取得は新しい状態を返すこと。
func TestIssue357DescendantChangeAndAncestorStaleness(t *testing.T) {
	screen := NewScreen()
	screen.DisplayHTML(`<div id="a"><div id="b"><span id="c">text</span></div></div>`)
	b, ok := screen.queryByID("b")
	if !ok {
		t.Fatal("b not found")
	}
	c, ok := screen.queryByID("c")
	if !ok {
		t.Fatal("c not found")
	}

	if err := screen.setText(c, "changed"); err != nil {
		t.Fatal(err)
	}
	if got, err := screen.text(c); err != nil || got != "changed" {
		t.Errorf("text(c after setText) = %q, err = %v, want changed", got, err)
	}
	if got, err := screen.html(c); err != nil || got != "changed" {
		t.Errorf("html(c after setText) = %q, err = %v, want changed", got, err)
	}
	// 祖先は従来どおり古い内容のまま。
	if got, err := screen.html(b); err != nil ||
		got != `<span id="c" data-gonako-handle="4">text</span>` {
		t.Errorf("html(b after child setText) = %q, err = %v", got, err)
	}

	// 親への setText は子孫を破棄し、自身の取得は新しいテキストになる。
	if err := screen.setText(b, "plain"); err != nil {
		t.Fatal(err)
	}
	if _, ok := screen.queryByID("c"); ok {
		t.Error("c should not be found after setText on parent")
	}
	if got, err := screen.html(b); err != nil || got != "plain" {
		t.Errorf("html(b after setText) = %q, err = %v, want plain", got, err)
	}
	if got, err := screen.text(b); err != nil || got != "plain" {
		t.Errorf("text(b after setText) = %q, err = %v, want plain", got, err)
	}
}

// setHTML 後: 対象の HTML取得は元の文字列、新規子要素の取得は描画結果。
func TestIssue357SetHTMLThenGet(t *testing.T) {
	screen := NewScreen()
	screen.DisplayHTML(`<div id="a">old</div>`)
	a, ok := screen.queryByID("a")
	if !ok {
		t.Fatal("a not found")
	}
	if err := screen.setHTML(a, `<p id="n"><b>bold</b></p>`); err != nil {
		t.Fatal(err)
	}
	if got, err := screen.html(a); err != nil || got != `<p id="n"><b>bold</b></p>` {
		t.Errorf("html(a after setHTML) = %q, err = %v, want original markup", got, err)
	}
	n, ok := screen.queryByID("n")
	if !ok {
		t.Fatal("n not found after setHTML")
	}
	if got, err := screen.html(n); err != nil ||
		got != fmt.Sprintf(`<b data-gonako-handle="%d">bold</b>`, n+1) {
		t.Errorf("html(n) = %q, err = %v", got, err)
	}
	if got, err := screen.text(n); err != nil ||
		got != fmt.Sprintf(`<b data-gonako-handle="%d">bold</b>`, n+1) {
		t.Errorf("text(n) = %q, err = %v", got, err)
	}
	// setHTML の対象へ setText すると、元の文字列は消えてテキストが返る。
	if err := screen.setText(a, "after"); err != nil {
		t.Fatal(err)
	}
	if got, err := screen.html(a); err != nil || got != "after" {
		t.Errorf("html(a after setText) = %q, err = %v, want after", got, err)
	}
}

// 空文字の HTML変更: 保持値は空で、取得は空を返す（従来実装と同結果）。
// 続けて HTML変更を呼んだ場合は、後の内容だけが残る。
func TestIssue357SetHTMLEmptyAndRepeated(t *testing.T) {
	screen := NewScreen()
	screen.DisplayHTML(`<div id="a"><span id="old">old</span></div>`)
	a, ok := screen.queryByID("a")
	if !ok {
		t.Fatal("a not found")
	}
	if err := screen.setHTML(a, ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := screen.queryByID("old"); ok {
		t.Error("old should not be found after setHTML empty")
	}
	if got, err := screen.html(a); err != nil || got != "" {
		t.Errorf("html(a after setHTML empty) = %q, err = %v, want empty", got, err)
	}
	if got, err := screen.text(a); err != nil || got != "" {
		t.Errorf("text(a after setHTML empty) = %q, err = %v, want empty", got, err)
	}
	if err := screen.setHTML(a, `<i id="one">1</i>`); err != nil {
		t.Fatal(err)
	}
	if err := screen.setHTML(a, `<i id="two">2</i>`); err != nil {
		t.Fatal(err)
	}
	if _, ok := screen.queryByID("one"); ok {
		t.Error("one should not be found after second setHTML")
	}
	two, ok := screen.queryByID("two")
	if !ok {
		t.Fatal("two not found after second setHTML")
	}
	if got, err := screen.html(a); err != nil || got != `<i id="two">2</i>` {
		t.Errorf("html(a after second setHTML) = %q, err = %v, want second markup", got, err)
	}
	// 新しい子へのテキスト変更は子自身の取得に反映され、対象の保持値（元の文字列）は不変。
	if err := screen.setText(two, "2nd"); err != nil {
		t.Fatal(err)
	}
	if got, err := screen.text(two); err != nil || got != "2nd" {
		t.Errorf("text(two after setText) = %q, err = %v, want 2nd", got, err)
	}
	if got, err := screen.html(a); err != nil || got != `<i id="two">2</i>` {
		t.Errorf("html(a after child setText) = %q, err = %v, want original markup", got, err)
	}
}

// fragment の select に選択肢を追加・置換しても、select 自身の HTML取得は
// 登録時の内容のまま（従来実装でも保持文字列は更新されなかった）。
func TestIssue357SelectOptionsDoNotRewriteHTMLGet(t *testing.T) {
	screen := NewScreen()
	screen.DisplayHTML(`<select id="sel"><option value="a">A</option></select>`)
	sel, ok := screen.queryByID("sel")
	if !ok {
		t.Fatal("sel not found")
	}
	want := `<option value="a" data-gonako-handle="3">A</option>`
	if got, err := screen.html(sel); err != nil || got != want {
		t.Errorf("html(sel) = %q, err = %v, want %q", got, err, want)
	}
	screen.appendOptions(sel, []string{"B"})
	if got, err := screen.html(sel); err != nil || got != want {
		t.Errorf("html(sel after appendOptions) = %q, err = %v, want %q", got, err, want)
	}
	if err := screen.replaceOptions(sel, []string{"C"}); err != nil {
		t.Fatal(err)
	}
	if got, err := screen.html(sel); err != nil || got != want {
		t.Errorf("html(sel after replaceOptions) = %q, err = %v, want %q", got, err, want)
	}
}

// プログラム生成部品の HTML取得は従来どおりで、子要素を含む形には変わらない
// （受け入れ条件: 変わる場合は差分明記。ここでは「変わらない」ことを固定する）。
func TestIssue357ProgrammaticContainerHTMLGetUnchanged(t *testing.T) {
	screen := NewScreen()
	parent := screen.create("div", "", `<b>固定</b>`, "", 0)
	screen.create("span", "子", "", "", parent)
	if got, err := screen.html(parent); err != nil || got != `<b>固定</b>` {
		t.Errorf("html(parent) = %q, err = %v, want the stored html only", got, err)
	}
	plain := screen.create("div", "本文", "", "", 0)
	screen.create("span", "子", "", "", plain)
	if got, err := screen.html(plain); err != nil || got != "本文" {
		t.Errorf("html(plain) = %q, err = %v, want text only", got, err)
	}
}

// 要素の削除後: 削除された要素は取得できず、祖先の取得は従来どおり古いまま。
func TestIssue357RemoveKeepsAncestorSnapshot(t *testing.T) {
	screen := NewScreen()
	screen.DisplayHTML(`<div id="a"><div id="b">B</div></div>`)
	a, ok := screen.queryByID("a")
	if !ok {
		t.Fatal("a not found")
	}
	b, ok := screen.queryByID("b")
	if !ok {
		t.Fatal("b not found")
	}
	if _, err := screen.remove(b, "DOM削除"); err != nil {
		t.Fatal(err)
	}
	if _, err := screen.html(b); err == nil {
		t.Error("html(removed) should return an error")
	}
	want := `<div id="b" data-gonako-handle="3">B</div>`
	if got, err := screen.html(a); err != nil || got != want {
		t.Errorf("html(a after remove) = %q, err = %v, want %q", got, err, want)
	}
}

// HTML変更で入れた子孫を削除しても、対象の HTML取得は元の文字列のまま。
func TestIssue357SetHTMLThenRemoveChild(t *testing.T) {
	screen := NewScreen()
	screen.DisplayHTML(`<div id="a">old</div>`)
	a, ok := screen.queryByID("a")
	if !ok {
		t.Fatal("a not found")
	}
	if err := screen.setHTML(a, `<p id="n">N</p>`); err != nil {
		t.Fatal(err)
	}
	n, ok := screen.queryByID("n")
	if !ok {
		t.Fatal("n not found after setHTML")
	}
	if _, err := screen.remove(n, "DOM削除"); err != nil {
		t.Fatal(err)
	}
	if got, err := screen.html(a); err != nil || got != `<p id="n">N</p>` {
		t.Errorf("html(a after removing setHTML child) = %q, err = %v, want original markup", got, err)
	}
}

// textarea のテキスト初期化（value 属性が無ければ textContent）。
func TestIssue357TextareaTextInit(t *testing.T) {
	screen := NewScreen()
	screen.DisplayHTML(`<textarea id="t">hello</textarea><textarea id="v" value="preset"></textarea>`)
	th, ok := screen.queryByID("t")
	if !ok {
		t.Fatal("t not found")
	}
	if got, err := screen.text(th); err != nil || got != "hello" {
		t.Errorf("text(textarea) = %q, err = %v, want hello", got, err)
	}
	vh, ok := screen.queryByID("v")
	if !ok {
		t.Fatal("v not found")
	}
	if got, err := screen.text(vh); err != nil || got != "preset" {
		t.Errorf("text(textarea[value]) = %q, err = %v, want preset", got, err)
	}
}

// --- ベンチマーク（Issue #357 の受け入れ条件の計測） ---

// Issue #357 の計測用: div を depth 段入れ子にした markup。
func nestedDivMarkup(depth int) string {
	var b strings.Builder
	for i := 0; i < depth; i++ {
		b.WriteString("<div>")
	}
	b.WriteString("x")
	for i := 0; i < depth; i++ {
		b.WriteString("</div>")
	}
	return b.String()
}

func BenchmarkDisplayHTMLNested(b *testing.B) {
	for _, depth := range []int{100, 200, 400} {
		markup := nestedDivMarkup(depth)
		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				screen := NewScreen()
				screen.DisplayHTML(markup)
			}
		})
	}
}

// 単純な wrapper 取得（DisplayHTML のラッパーに対する HTML取得）のコスト。
// ラッパーは描画済み文字列を保持するため、深さによらず定数時間のはず。
func BenchmarkScreenHTMLGetWrapper(b *testing.B) {
	for _, depth := range []int{10, 400} {
		screen := NewScreen()
		wrapper := screen.DisplayHTML(nestedDivMarkup(depth))
		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := screen.html(wrapper); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// 深い位置の要素に対する HTML取得（取得時にその範囲だけを描画する）。
func BenchmarkScreenHTMLGetDeepest(b *testing.B) {
	const depth = 400
	screen := NewScreen()
	wrapper := screen.DisplayHTML(nestedDivMarkup(depth))
	deepest := wrapper + depth
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := screen.html(deepest); err != nil {
			b.Fatal(err)
		}
	}
}
