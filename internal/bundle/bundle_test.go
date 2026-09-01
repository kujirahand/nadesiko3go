package bundle_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// fakeRuntime writes a stand-in for the gonako executable. The bundle format
// does not care what the runtime bytes are, so a placeholder is enough.
func fakeRuntime(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "runtime")
	if err := os.WriteFile(path, []byte("これはランタイムの中身のつもり"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBuildAndOpen(t *testing.T) {
	dir := t.TempDir()
	runtime := fakeRuntime(t, dir)

	prog, err := vm.CompileProgram("「やあ」と表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packed")
	if err := bundle.Build(out, runtime, prog, "main.nako3", ""); err != nil {
		t.Fatal(err)
	}

	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatalf("Open = %v", err)
	}
	defer packed.Close()
	if packed.Name != "main.nako3" {
		t.Errorf("Name = %q, want main.nako3", packed.Name)
	}
	if packed.Program == nil || len(packed.Program.Funcs) == 0 {
		t.Fatal("プログラムが読めていない")
	}

	// 土台のランタイムはそのまま先頭に残っている
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "これはランタイムの中身のつもり") {
		t.Error("ランタイムの中身が壊れている")
	}
}

// TestOpenPlainRuntime pins that an executable with nothing appended reports
// ErrNoBundle, which is how the plain command tells it is not packed.
func TestOpenPlainRuntime(t *testing.T) {
	dir := t.TempDir()
	_, err := bundle.Open(fakeRuntime(t, dir))
	if !errors.Is(err, bundle.ErrNoBundle) {
		t.Errorf("Open = %v, want ErrNoBundle", err)
	}
}

func TestResources(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "images")
	if err := os.MkdirAll(filepath.Join(res, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"a.txt":     "あ",
		"sub/b.txt": "い",
	} {
		if err := os.WriteFile(filepath.Join(res, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packed")
	if err := bundle.Build(out, fakeRuntime(t, dir), prog, "main.nako3", res); err != nil {
		t.Fatal(err)
	}

	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()

	// パスは指定したフォルダ名ごと残る。開発中と同じ書き方で読めるようにするため。
	base := filepath.Base(res)
	for name, want := range map[string]string{
		base + "/a.txt":     "あ",
		base + "/sub/b.txt": "い",
	} {
		got, ok := packed.ReadResource(name)
		if !ok {
			t.Errorf("リソース %s が見つからない (入っているのは %v)", name, packed.Resources())
			continue
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if _, ok := packed.ReadResource("ない.txt"); ok {
		t.Error("入っていないリソースが読めてしまった")
	}
}

// TestRebuildDoesNotStack pins that packing an already-packed executable
// replaces the payload instead of adding a second one.
func TestRebuildDoesNotStack(t *testing.T) {
	dir := t.TempDir()
	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}

	first := filepath.Join(dir, "first")
	if err := bundle.Build(first, fakeRuntime(t, dir), prog, "main.nako3", ""); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(dir, "second")
	if err := bundle.Build(second, first, prog, "main.nako3", ""); err != nil {
		t.Fatal(err)
	}

	a, _ := os.Stat(first)
	b, _ := os.Stat(second)
	if a.Size() != b.Size() {
		t.Errorf("二度目のサイズ = %d, want %d (積み上がっている)", b.Size(), a.Size())
	}
}

// TestBundledProgramRuns pins the whole path: compile, pack, read back, run,
// and read a resource with the ordinary file command.
func TestBundledProgramRuns(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "data")
	if err := os.MkdirAll(res, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res, "msg.txt"), []byte("同梱の中身"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := "A=「data/msg.txt」を開く\n「読めた: {A}」と表示\nB=「data/msg.txt」が存在\n「存在: {B}」と表示"
	prog, err := vm.CompileProgram(code, "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packed")
	if err := bundle.Build(out, fakeRuntime(t, dir), prog, "main.nako3", res); err != nil {
		t.Fatal(err)
	}

	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()

	// リソースの実体がない場所で動かす
	var buf strings.Builder
	host := vm.NewCUIHost(&buf, strings.NewReader(""), nil)
	host.Bundle = packed
	elsewhere := t.TempDir()
	previous, _ := os.Getwd()
	if err := os.Chdir(elsewhere); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)

	if err := vm.RunCompiled(packed.Program, host); err != nil {
		t.Fatal(err)
	}
	want := "読めた: 同梱の中身\n存在: true\n"
	if buf.String() != want {
		t.Errorf("出力 = %q, want %q", buf.String(), want)
	}
}

// TestTruncatedBundleIsRejected pins that a damaged tail is reported rather
// than read as if it were valid.
func TestTruncatedBundleIsRejected(t *testing.T) {
	dir := t.TempDir()
	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packed")
	if err := bundle.Build(out, fakeRuntime(t, dir), prog, "main.nako3", ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	// ペイロードの真ん中を削って、フッタが示す長さと食い違わせる
	broken := filepath.Join(dir, "broken")
	if err := os.WriteFile(broken, append(data[:len(data)-60], data[len(data)-21:]...), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.Open(broken); err == nil {
		t.Error("壊れたバンドルが読めてしまった")
	}
}

func TestBundledAdvancedResources(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "assets")
	if err := os.MkdirAll(filepath.Join(res, "audio"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res, "audio/sound.bin"), []byte{0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res, "config.txt"), []byte("設定データ"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := `
Txt = 「assets/config.txt」を開く
「設定: {Txt}」と表示
Bin = 「assets/audio/sound.bin」をバイナリ読
「バイナリ長: {Binの要素数}」と表示
「バイナリ先頭: {Bin[0]}」と表示
「実ファイル書き込み」を"output.txt"に保存
「書き込み存在: {"output.txt"が存在}」と表示
`
	prog, err := vm.CompileProgram(code, "app.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "app_bundle")
	if err := bundle.Build(out, fakeRuntime(t, dir), prog, "app.nako3", res); err != nil {
		t.Fatal(err)
	}

	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()

	// 別のディレクトリで実行
	runDir := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(runDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)

	var buf strings.Builder
	host := vm.NewCUIHost(&buf, strings.NewReader(""), nil)
	host.Bundle = packed

	if err := vm.RunCompiled(packed.Program, host); err != nil {
		t.Fatalf("RunCompiled: %v", err)
	}

	want := strings.Join([]string{
		"設定: 設定データ",
		"バイナリ長: 3",
		"バイナリ先頭: 1",
		"書き込み存在: true",
	}, "\n") + "\n"

	if buf.String() != want {
		t.Errorf("出力:\n%s\nwant:\n%s", buf.String(), want)
	}
}
