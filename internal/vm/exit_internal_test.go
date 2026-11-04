package vm

import (
	"io"
	"strings"
	"testing"
)

// InvokeCommand（GUIブリッジなど実行の外からの命令呼び出し）では、
// 『終了』がエラーにならず正常終了として扱われる (#337)。
func TestInvokeCommandExit(t *testing.T) {
	prog, err := CompileProgram("", "exit.nako3")
	if err != nil {
		t.Fatal(err)
	}
	host := NewCUIHost(io.Discard, strings.NewReader(""), nil)
	m := New(prog, runtimeRegistry(), host, DefaultOptions())
	if err := m.Run(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.InvokeCommand("終了", nil); err != nil {
		t.Fatalf("終了がエラーになった: %v", err)
	}
	if !host.Exited {
		t.Error("Host に終了が記録されていない")
	}
	if m.entry != 0 {
		t.Errorf("entry = %d, want 0", m.entry)
	}
}

// Run 完了後に外から CallFunc を直接呼ぶ（GUIの DispatchEvent 相当）場合も、
// 終了は panic せず nil として返る (#337)。
func TestCallFuncExitAfterRun(t *testing.T) {
	prog, err := CompileProgram("●おわり処理とは\n  終了\nここまで\n", "exit.nako3")
	if err != nil {
		t.Fatal(err)
	}
	host := NewCUIHost(io.Discard, strings.NewReader(""), nil)
	m := New(prog, runtimeRegistry(), host, DefaultOptions())
	if err := m.Run(); err != nil {
		t.Fatal(err)
	}
	fn := m.FindFunc("おわり処理")
	if fn == nil {
		t.Fatal("関数が見つからない")
	}
	if _, err := m.CallFunc(fn, nil); err != nil {
		t.Fatalf("CallFunc が終了でエラーを返した: %v", err)
	}
	if !host.Exited {
		t.Error("Host に終了が記録されていない")
	}
	if m.entry != 0 {
		t.Errorf("entry = %d, want 0", m.entry)
	}
}

// 命令実行上限のエラーは『終了』と違い nakoPanic のままなので、エラー監視に
// 捕捉される（protect がハンドラへ再開する）ことを確かめる (#337 レビュー対応)。
// 捕捉されれば、最終的に返るエラーはハンドラ内の命令（7行目）から出た上限
// エラーになる。捕捉されなければループ内（4行目）から出る。
// なお上限を使い切った後のハンドラは命令を実行できないため、最終結果が
// エラーになる点は従来どおり（master でも同一の結果になることを確認済み）。
func TestInstructionLimitStaysCatchable(t *testing.T) {
	code := "エラー監視\n  カウンタX=0\n  10000回\n    カウンタX=カウンタX+1\n  ここまで\nエラーならば\n  「捕捉した」を表示\nここまで\n「終わり」を表示\n"
	prog, err := CompileProgram(code, "limit.nako3")
	if err != nil {
		t.Fatal(err)
	}
	host := NewCUIHost(io.Discard, strings.NewReader(""), nil)
	opts := DefaultOptions()
	opts.MaxInstructions = 100
	m := New(prog, runtimeRegistry(), host, opts)
	err = m.Run()
	if err == nil {
		t.Fatal("上限エラーが返らなかった")
	}
	if !strings.Contains(err.Error(), "実行した命令が多すぎます") {
		t.Fatalf("上限エラーではない: %v", err)
	}
	if !strings.Contains(err.Error(), "(7行目)") {
		t.Fatalf("エラー監視に捕捉されていない（ハンドラへ再開していない）: %v", err)
	}
}
