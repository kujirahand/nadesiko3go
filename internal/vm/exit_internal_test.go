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
	if !m.exited {
		t.Error("VM に終了が記録されていない")
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
