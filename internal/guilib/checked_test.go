package guilib

import (
	"strconv"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// Issue #355: checkbox・radioの選択状態をVMへ届ける。
// 契約は DispatchEvent のコメントを参照 (フォーム値は選択中だけ、フォームチェックは真偽、
// イベント外は チェック取得 / チェック設定)。

// textFields は文字列だけの入力欄(ハンドル→値)をpayloadにする。
func textFields(values map[string]string) EventPayload {
	fields := make(map[string]EventField, len(values))
	for handle, text := range values {
		fields[handle] = EventField{Value: text}
	}
	return EventPayload{Values: fields}
}

// checkField はcheckbox・radio1つ分のフィールドを作る。
func checkField(text string, checked bool) EventField {
	return EventField{Value: text, Checked: &checked}
}

// runGUIProgram はコードを実行して画面モデルと出力バッファを返す。
func runGUIProgram(t *testing.T, code string) (*Screen, *strings.Builder) {
	t.Helper()
	screen := NewScreen()
	registry := stdlib.NewRegistry(NewWithScreen(screen))
	out := &strings.Builder{}
	host := vm.NewCUIHost(out, strings.NewReader(""), nil)
	if err := vm.RunWithHostAndRegistry(code, "gui.nako3", registry, host); err != nil {
		t.Fatal(err)
	}
	return screen, out
}

func TestCheckboxCheckedIsDistinguishedInEvent(t *testing.T) {
	// 親(span)=1、input=2、label=3。
	screen, out := runGUIProgram(t, `箱=「同意」のチェックボックス作成
箱を変更した時には
　もし、フォームチェック["2"]ならば
　　「ON」を表示
　違えば
　　「OFF」を表示
　ここまで
　フォーム値["2"]を表示
ここまで`)

	// 未選択: フォームチェックは偽、フォーム値には載らない("on"が残らない)。
	if err := screen.DispatchEvent(2, "change", EventPayload{Values: map[string]EventField{"2": checkField("on", false)}}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if lines[0] != "OFF" || (len(lines) > 1 && lines[1] == "on") {
		t.Fatalf("未選択の出力 = %q", out.String())
	}

	// 選択済み: 真になり、フォーム値にも値が載る。
	out.Reset()
	if err := screen.DispatchEvent(2, "change", EventPayload{Values: map[string]EventField{"2": checkField("on", true)}}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "ON\non" {
		t.Fatalf("選択済みの出力 = %q, want ON\\non", got)
	}
	if on, err := screen.isChecked(2); err != nil || !on {
		t.Fatalf("Go側のモデルに選択状態が反映されていません: %v, %v", on, err)
	}
}

func TestCheckedCommandsReadAndWriteOutsideEvents(t *testing.T) {
	screen, out := runGUIProgram(t, `箱=「同意」のチェックボックス作成
もし、箱のチェック取得ならば
　「初期ON」を表示
違えば
　「初期OFF」を表示
ここまで
箱にオンをチェック設定
もし、箱のチェック取得ならば
　「設定後ON」を表示
ここまで
箱にオフをチェック設定
もし、箱のチェック取得ならば
　「解除後ON」を表示
違えば
　「解除後OFF」を表示
ここまで`)
	if got := strings.TrimSpace(out.String()); got != "初期OFF\n設定後ON\n解除後OFF" {
		t.Fatalf("output = %q", got)
	}

	// 画面へ送る操作は、選択と解除の両方が載る(falseも省略されない)。
	var sent []bool
	for _, op := range screen.DrainOperations() {
		if op.Type == "checked" {
			if op.Handle != 2 || op.Checked == nil {
				t.Fatalf("checked operation = %#v", op)
			}
			sent = append(sent, *op.Checked)
		}
	}
	if len(sent) != 2 || !sent[0] || sent[1] {
		t.Fatalf("checked operations = %v, want [true false]", sent)
	}
}

func TestCheckedCommandsRejectNonCheckable(t *testing.T) {
	screen := NewScreen()
	editor := screen.create("input", "x", "", "", 0)
	if _, err := screen.isChecked(editor); err == nil || !strings.Contains(err.Error(), "チェック取得") {
		t.Fatalf("isChecked error = %v", err)
	}
	if err := screen.setChecked(editor, true); err == nil || !strings.Contains(err.Error(), "チェック設定") {
		t.Fatalf("setChecked error = %v", err)
	}
	if _, err := screen.isChecked(99); err == nil || !strings.Contains(err.Error(), "99") {
		t.Fatalf("invalid handle error = %v", err)
	}
}

func TestInitialCheckedFromAttributes(t *testing.T) {
	// HTML表示: wrapper=1、input=2(checked属性なし)、input=3(checked属性あり)。
	screen, _ := runGUIProgram(t, `「<input type="checkbox"><input type="checkbox" checked>」をHTML表示`)
	if on, _ := screen.isChecked(2); on {
		t.Fatal("checked属性の無いcheckboxが選択済みで始まっています")
	}
	if on, _ := screen.isChecked(3); !on {
		t.Fatal("checked属性のあるcheckboxが選択済みで始まっていません")
	}

	// DOM属性一括設定でchecked属性を渡した場合も選択済みになり、画面へ属性が届く。
	screen, _ = runGUIProgram(t, `箱=「同意」のチェックボックス作成
箱に{"checked":"checked"}をDOM属性一括設定`)
	if on, _ := screen.isChecked(2); !on {
		t.Fatal("DOM属性一括設定のcheckedが反映されていません")
	}

	// createWithAttributes にchecked属性を渡すと、作成操作にも選択済みが載る。
	screen = NewScreen()
	h := screen.createWithAttributes("input", "on", "", "", 0, map[string]string{"type": "checkbox", "checked": ""})
	if on, _ := screen.isChecked(h); !on {
		t.Fatal("createWithAttributesのcheckedが反映されていません")
	}
	ops := screen.DrainOperations()
	if len(ops) != 1 || ops[0].Checked == nil || !*ops[0].Checked {
		t.Fatalf("作成操作に選択済みが載っていません: %#v", ops)
	}
}

const radioProgram = `「<form><input type="radio" name="色" value="赤"><input type="radio" name="色" value="青" checked><input type="radio" name="色" value="緑"></form>」をHTML表示
枠=「form」のDOM要素取得
枠をフォーム送信した時には
　フォーム値["色"]を表示
　もし、フォームチェック["色"]ならば
　　「選択あり」を表示
　違えば
　　「選択なし」を表示
　ここまで
ここまで`

func TestRadioGroupSelectedValueIsDeterministic(t *testing.T) {
	// wrapper=1、form=2、radio=3(赤)・4(青, checked属性)・5(緑)。
	screen, out := runGUIProgram(t, radioProgram)
	if on, _ := screen.isChecked(4); !on {
		t.Fatal("checked属性のradioが選択済みで始まっていません")
	}
	if on, _ := screen.isChecked(3); on {
		t.Fatal("checked属性の無いradioが選択済みで始まっています")
	}

	values := map[int]string{3: "赤", 4: "青", 5: "緑"}
	// 辞書の反復順に依存していないことを、繰り返しと選択の入れ替えで確かめる。
	for round := 0; round < 20; round++ {
		for selected, want := range values {
			fields := map[string]EventField{}
			for handle, text := range values {
				fields[strconv.Itoa(handle)] = checkField(text, handle == selected)
			}
			out.Reset()
			if err := screen.DispatchEvent(2, "submit", EventPayload{Values: fields}); err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(out.String()); got != want+"\n選択あり" {
				t.Fatalf("選択=%d の出力 = %q, want %q", selected, got, want+"\n選択あり")
			}
		}
	}

	// どれも選ばれていなければ、フォーム値に載らず、フォームチェックは偽。
	fields := map[string]EventField{}
	for handle, text := range values {
		fields[strconv.Itoa(handle)] = checkField(text, false)
	}
	out.Reset()
	if err := screen.DispatchEvent(2, "submit", EventPayload{Values: fields}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if lines[len(lines)-1] != "選択なし" {
		t.Fatalf("未選択の出力 = %q", out.String())
	}
	for _, name := range values {
		if strings.Contains(strings.Join(lines[:len(lines)-1], ""), name) {
			t.Fatalf("未選択なのにフォーム値へ値が載っています: %q", out.String())
		}
	}
}

func TestSetCheckedOnRadioClearsSameNameInSameForm(t *testing.T) {
	// 1つ目のformの同名radioは排他、別formの同名radioには影響しない。
	// wrapper=1、form=2、radio=3・4、form=5、radio=6。
	screen, _ := runGUIProgram(t, `「<form><input type="radio" name="x" checked><input type="radio" name="x"></form><form><input type="radio" name="x" checked></form>」をHTML表示`)
	if err := screen.setChecked(4, true); err != nil {
		t.Fatal(err)
	}
	for handle, want := range map[int]bool{3: false, 4: true, 6: true} {
		if on, _ := screen.isChecked(handle); on != want {
			t.Fatalf("radio %d の選択状態 = %v, want %v", handle, on, want)
		}
	}
}

func TestSubmitTreatsUncheckedCheckboxLikeChange(t *testing.T) {
	// wrapper=1、form=2、checkbox=3、text=4。送信でも変更でも同じ規則(未選択は載らない)。
	screen, out := runGUIProgram(t, `「<form><input type="checkbox" name="agree" value="y"><input name="n" value="x"></form>」をHTML表示
枠=「form」のDOM要素取得
枠をフォーム送信した時には
　もし、フォームチェック["agree"]ならば
　　「同意」を表示
　違えば
　　「未同意」を表示
　ここまで
　フォーム値["n"]を表示
　フォーム値["agree"]を表示
ここまで`)

	send := func(agree bool) []string {
		t.Helper()
		out.Reset()
		err := screen.DispatchEvent(2, "submit", EventPayload{Values: map[string]EventField{
			"3": checkField("y", agree),
			"4": {Value: "太郎"},
		}})
		if err != nil {
			t.Fatal(err)
		}
		return strings.Split(strings.TrimSpace(out.String()), "\n")
	}

	// 未選択: 読み方は変更イベントと同じ(偽)。テキスト入力は従来どおり文字列で読める。
	lines := send(false)
	if lines[0] != "未同意" || lines[1] != "太郎" || (len(lines) > 2 && lines[2] == "y") {
		t.Fatalf("未選択の送信 = %q", lines)
	}
	lines = send(true)
	if len(lines) != 3 || lines[0] != "同意" || lines[1] != "太郎" || lines[2] != "y" {
		t.Fatalf("選択済みの送信 = %q", lines)
	}
}

func TestDropFilesArriveInDedicatedPayloadField(t *testing.T) {
	screen, out := runGUIProgram(t, `ファイルドロップ時には
　ドロップファイルを表示
ここまで`)
	files := []string{"/a/日本語.txt", "/b/c d.png"}
	// 入力欄の値と同じ辞書には混ぜず、DropFiles だけで届く。
	if err := screen.DispatchEvent(0, "drop", EventPayload{DropFiles: files}); err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if !strings.Contains(out.String(), name) {
			t.Fatalf("ドロップファイル = %q, %q が含まれません", out.String(), name)
		}
	}
}
