package guilib

import (
	"strings"
	"testing"
)

// Issue #353 の再現: HTML置換・text置換後も Go モデルが表示とずれる。
func TestIssue353HTMLReplaceSyncsModel(t *testing.T) {
	screen := NewScreen()
	screen.DisplayHTML(`<div id="parent"><span id="old">old</span></div>`)
	parent, ok := screen.queryByID("parent")
	if !ok {
		t.Fatal("parent not found")
	}
	old, ok := screen.queryByID("old")
	if !ok {
		t.Fatal("old not found before replace")
	}
	// old にイベントを結び付けておく（置換後に残ってはいけない）
	if err := screen.bind(old, "click", eventBinding{}); err != nil {
		t.Fatal(err)
	}
	// old を DOM親要素にしておく（置換で消えたら親指定も解除されること）
	screen.setParent(old)
	screen.DrainOperations()

	if err := screen.setHTML(parent, `<button id="new">new</button>`); err != nil {
		t.Fatal(err)
	}
	if _, ok := screen.queryByID("old"); ok {
		t.Error("old should not be found after HTML replace")
	}
	newHandle, ok := screen.queryByID("new")
	if !ok {
		t.Fatal("new should be found after HTML replace")
	}
	if text, err := screen.text(newHandle); err != nil || text != "new" {
		t.Errorf("new text = %q, err = %v, want new", text, err)
	}
	// 新しい部品を操作できること（注目とテキスト設定）
	if err := screen.focus(newHandle); err != nil {
		t.Errorf("new should be focusable: %v", err)
	}
	if err := screen.setText(newHandle, "new2"); err != nil {
		t.Errorf("new should accept setText: %v", err)
	}
	if _, ok := screen.events[old]; ok {
		t.Error("old event binding should be removed after HTML replace")
	}
	if got := screen.parent(); got != 0 {
		t.Errorf("domParent = %d, want 0 after the parent element was replaced away", got)
	}
	// HTML取得は、ハンドル注入済みの描画結果ではなく指定した文字列を返す
	if got, err := screen.html(parent); err != nil || got != `<button id="new">new</button>` {
		t.Errorf("html(parent) = %q, err = %v, want the original markup", got, err)
	}
	ops := screen.DrainOperations()
	found := false
	for _, op := range ops {
		if op.Type == "html" && op.Handle == parent && strings.Contains(op.HTML, `data-gonako-handle=`) && strings.Contains(op.HTML, `id="new"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("html operation should carry rendered HTML with handles, ops = %#v", ops)
	}

	// text置換でも子要素と古いイベントを残さない
	if err := screen.bind(newHandle, "click", eventBinding{}); err != nil {
		t.Fatal(err)
	}
	if err := screen.setText(parent, "hello"); err != nil {
		t.Fatal(err)
	}
	if _, ok := screen.queryByID("new"); ok {
		t.Error("new should not be found after text replace")
	}
	if _, ok := screen.events[newHandle]; ok {
		t.Error("new event binding should be removed after text replace")
	}
	if text, err := screen.text(parent); err != nil || text != "hello" {
		t.Errorf("parent text = %q, err = %v, want hello", text, err)
	}
}

// setText は select の子 option を消さない（値の設定で子要素は変わらないため）。
func TestIssue353SetTextKeepsSelectOptions(t *testing.T) {
	screen := NewScreen()
	screen.DisplayHTML(`<select id="sel"><option id="opt1" value="a">A</option><option id="opt2" value="b">B</option></select>`)
	sel, ok := screen.queryByID("sel")
	if !ok {
		t.Fatal("select not found")
	}
	if err := screen.setText(sel, "b"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"opt1", "opt2"} {
		if _, ok := screen.queryByID(id); !ok {
			t.Errorf("%s should remain after setText on select", id)
		}
	}
}

// tbody のような文脈依存の要素でも、置換した行が登録されること。
func TestIssue353HTMLReplaceInTableBody(t *testing.T) {
	screen := NewScreen()
	screen.DisplayHTML(`<table><tbody id="body"></tbody></table>`)
	body, ok := screen.queryByID("body")
	if !ok {
		t.Fatal("tbody not found")
	}
	if err := screen.setHTML(body, `<tr id="row"><td>c1</td></tr>`); err != nil {
		t.Fatal(err)
	}
	if _, ok := screen.queryByID("row"); !ok {
		t.Fatal("row should be found after HTML replace in tbody")
	}
	cell, ok := screen.query("td")
	if !ok {
		t.Fatal("td should be registered after HTML replace in tbody")
	}
	if got, err := screen.text(cell); err != nil || got != "c1" {
		t.Errorf("td text = %q, err = %v, want c1", got, err)
	}
}
