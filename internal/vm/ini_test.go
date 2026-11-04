package vm_test

// Issue #388: INI取得・INI変換の、なでしこプログラム経由の結合テスト。

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/japanese"
)

func TestINIGetViaProgram(t *testing.T) {
	code := "A=『top=1\n[a]\nx=hello ; world\ny=\"  q  \"\n[b]\nz=\n』のINI取得\nAをJSONエンコードして表示"
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	want := `{"top":"1","a":{"x":"hello ; world","y":"  q  "},"b":{"z":""}}`
	if got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

func TestINIGetAccessAndAlias(t *testing.T) {
	code := "A=『[設定]\n名前=けもの\n』からINI取得\nA[\"設定\"][\"名前\"]を表示"
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != "けもの" {
		t.Errorf("got=%q", got)
	}
}

func TestINIGetError(t *testing.T) {
	_, err := runCUICommands(t, "A=『[sec\nk=v』のINI取得\nAを表示")
	if err == nil {
		t.Fatal("エラーになるはず")
	}
}

func TestINIRoundTripViaProgram(t *testing.T) {
	code := "A={\"top\":\"t\", \"sec\":{\"k\":\"v\", \"n\":3, \"b\":真}}\n" +
		"S=AをINI変換\n" +
		"B=SをINI取得\n" +
		"BをJSONエンコードして表示"
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	// 値はすべて文字列に戻る（数値・真偽値の自動変換はしない）
	want := `{"top":"t","sec":{"k":"v","n":"3","b":"true"}}`
	if got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

func TestINIEncodeViaProgram(t *testing.T) {
	got, err := runCUICommands(t, "A={\"sec\":{\"k\":\"v\"}, \"top\":1}\nAをINI変換して表示")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := "top=1\n\n[sec]\nk=v"; got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

func TestINIEncodeErrorViaProgram(t *testing.T) {
	_, err := runCUICommands(t, "A={\"a\":{\"b\":{\"c\":1}}}\nAをINI変換して表示")
	if err == nil {
		t.Fatal("深い入れ子はエラーになるはず")
	}
}

// SJISで保存したINIファイルを SJISファイル読 で読み、INI取得できる
func TestINISJISFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conf.ini")
	content := "; 設定\n[データベース]\nホスト=localhost\n名前=けもの\n"
	if err := os.WriteFile(path, encodeForTest(t, japanese.ShiftJIS, content), 0o644); err != nil {
		t.Fatalf("err=%v", err)
	}
	code := "P=" + nakoString(path) + "\nS=PをSJISファイル読\nA=SをINI取得\nAをJSONエンコードして表示"
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	want := `{"データベース":{"ホスト":"localhost","名前":"けもの"}}`
	if got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}
