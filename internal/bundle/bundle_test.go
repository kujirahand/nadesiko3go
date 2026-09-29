package bundle_test

import (
	"errors"
	"fmt"
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

// TestRejectsSymlinkOutsideResources pins that a link reaching outside the
// resource folder fails the build instead of packing the outside file
// (Issue #187)。
func TestRejectsSymlinkOutsideResources(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "res")
	if err := os.MkdirAll(res, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res, "ok.txt"), []byte("中身"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(outside, []byte("TOP-SECRET-OUTSIDE-CONTENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(res, "link.txt")); err != nil {
		t.Skipf("シンボリックリンクを作れない環境なので飛ばす: %v", err)
	}

	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	err = bundle.Build(filepath.Join(dir, "packed"), fakeRuntime(t, dir), prog, "main.nako3", res)
	if err == nil {
		t.Fatal("フォルダ外を指すリンクを梱包してしまった")
	}
	if !strings.Contains(err.Error(), "link.txt") {
		t.Errorf("どのリンクが原因か分かるエラーにする: %v", err)
	}
}

// TestPacksSymlinkInsideResources pins that a link whose target stays inside
// the resource folder is still packed under the link's own name.
func TestPacksSymlinkInsideResources(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "res")
	if err := os.MkdirAll(res, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res, "real.txt"), []byte("実体の中身"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.txt", filepath.Join(res, "link.txt")); err != nil {
		t.Skipf("シンボリックリンクを作れない環境なので飛ばす: %v", err)
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

	got, ok := packed.ReadResource("res/link.txt")
	if !ok {
		t.Fatalf("リンク名のリソースが見つからない (入っているのは %v)", packed.Resources())
	}
	if string(got) != "実体の中身" {
		t.Errorf("link.txt = %q, want 実体の中身", got)
	}
}

// TestPacksSymlinkedResourceDir pins that a resource argument that is itself
// a link to a folder packs the real folder's contents under the link's name.
// リンク名がリソースのプレフィックスになるのは、開発中に書いたパス
// (「linkres/a.txt」など) がそのまま通るようにするため。
func TestPacksSymlinkedResourceDir(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "realres")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "a.txt"), []byte("中身"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linkres")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("シンボリックリンクを作れない環境なので飛ばす: %v", err)
	}

	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packed")
	if err := bundle.Build(out, fakeRuntime(t, dir), prog, "main.nako3", link); err != nil {
		t.Fatal(err)
	}
	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()

	got, ok := packed.ReadResource("linkres/a.txt")
	if !ok {
		t.Fatalf("リンク名のリソースが見つからない (入っているのは %v)", packed.Resources())
	}
	if string(got) != "中身" {
		t.Errorf("linkres/a.txt = %q, want 中身", got)
	}
}

// TestSkipsSymlinkToInsideDir pins that a link pointing at a folder inside
// the resource tree packs under the real folder's name only, so nothing is
// duplicated under the link's name.
func TestSkipsSymlinkToInsideDir(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "res")
	if err := os.MkdirAll(filepath.Join(res, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res, "sub", "a.txt"), []byte("中身"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sub", filepath.Join(res, "alias")); err != nil {
		t.Skipf("シンボリックリンクを作れない環境なので飛ばす: %v", err)
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

	if _, ok := packed.ReadResource("res/sub/a.txt"); !ok {
		t.Errorf("実体側のリソースが見つからない (入っているのは %v)", packed.Resources())
	}
	for _, name := range packed.Resources() {
		if strings.HasPrefix(name, "res/alias") {
			t.Errorf("リンク名で重複梱包されている: %s", name)
		}
	}
}

// TestPacksSymlinkOutsideWithIncludeOption pins that with Spec.IncludeSymlink
// set, a link reaching outside the resource folder is packed by following its
// target instead of failing the build (Issue #263)。
func TestPacksSymlinkOutsideWithIncludeOption(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "res")
	if err := os.MkdirAll(res, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res, "ok.txt"), []byte("中身"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(outside, []byte("外部の実体"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(res, "link.txt")); err != nil {
		t.Skipf("シンボリックリンクを作れない環境なので飛ばす: %v", err)
	}

	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packed")
	var warns []string
	capture := func(format string, args ...any) {
		warns = append(warns, fmt.Sprintf(format, args...))
	}
	if err := bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{
		Program:        prog,
		Name:           "main.nako3",
		ResourceDir:    res,
		IncludeSymlink: true,
		Warn:           capture,
	}); err != nil {
		t.Fatal(err)
	}
	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()

	got, ok := packed.ReadResource("res/link.txt")
	if !ok {
		t.Fatalf("リンク名のリソースが見つからない (入っているのは %v)", packed.Resources())
	}
	if string(got) != "外部の実体" {
		t.Errorf("link.txt = %q, want 外部の実体", got)
	}
	// 範囲外を指すリンクは警告を出す (Issue #263 のレビュー指定)
	if !hasWarning(warns, "フォルダ外") {
		t.Errorf("範囲外リンクの警告が出ていない: %v", warns)
	}
}

// hasWarning は警告のどれかが part を含むかを返す。
func hasWarning(warns []string, part string) bool {
	for _, w := range warns {
		if strings.Contains(w, part) {
			return true
		}
	}
	return false
}

// TestPacksSymlinkedDirOutsideWithIncludeOption pins that with the option set,
// a link to a folder outside the resources packs the whole tree under the
// link's name.
func TestPacksSymlinkedDirOutsideWithIncludeOption(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "res")
	if err := os.MkdirAll(res, 0o755); err != nil {
		t.Fatal(err)
	}
	ext := filepath.Join(dir, "ext")
	if err := os.MkdirAll(filepath.Join(ext, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ext, "a.txt"), []byte("外のa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ext, "sub", "b.txt"), []byte("外のb"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ext, "sub", ".hidden"), []byte("伏せ字"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(ext, filepath.Join(res, "alias")); err != nil {
		t.Skipf("シンボリックリンクを作れない環境なので飛ばす: %v", err)
	}

	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packed")
	if err := bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{
		Program:        prog,
		Name:           "main.nako3",
		ResourceDir:    res,
		IncludeSymlink: true,
	}); err != nil {
		t.Fatal(err)
	}
	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()

	for name, want := range map[string]string{
		"res/alias/a.txt":     "外のa",
		"res/alias/sub/b.txt": "外のb",
	} {
		got, ok := packed.ReadResource(name)
		if !ok {
			t.Fatalf("%s が見つからない (入っているのは %v)", name, packed.Resources())
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	for _, name := range packed.Resources() {
		if strings.Contains(name, ".hidden") {
			t.Errorf("隠しファイルが梱包されている: %s", name)
		}
	}
}

// TestIncludeSymlinkCycleIsError pins that links forming a loop report an
// error instead of recursing forever (Issue #263 のレビュー指定では循環は
// エラー)。
func TestIncludeSymlinkCycleIsError(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "res")
	if err := os.MkdirAll(res, 0o755); err != nil {
		t.Fatal(err)
	}
	ext := filepath.Join(dir, "ext")
	if err := os.MkdirAll(ext, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ext, "a.txt"), []byte("外のa"), 0o644); err != nil {
		t.Fatal(err)
	}
	// ext の中に ext 自身へ戻るリンク (ループ)
	if err := os.Symlink(ext, filepath.Join(ext, "loop")); err != nil {
		t.Skipf("シンボリックリンクを作れない環境なので飛ばす: %v", err)
	}
	// res の中の ext へ向かうリンク
	if err := os.Symlink(ext, filepath.Join(res, "alias")); err != nil {
		t.Skipf("シンボリックリンクを作れない環境なので飛ばす: %v", err)
	}

	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packed")
	err = bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{
		Program:        prog,
		Name:           "main.nako3",
		ResourceDir:    res,
		IncludeSymlink: true,
	})
	if err == nil {
		t.Fatal("循環リンクはエラーになるはず")
	}
	if !strings.Contains(err.Error(), "循環") {
		t.Errorf("循環が分かるエラーにする: %v", err)
	}
}

// TestIncludeSymlinkBrokenLinkWarnsAndSkips pins that a link with no target
// is skipped with a warning instead of failing the build (Issue #263 の
// レビュー指定)。
func TestIncludeSymlinkBrokenLinkWarnsAndSkips(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "res")
	if err := os.MkdirAll(res, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res, "ok.txt"), []byte("中身"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "gone.txt"), filepath.Join(res, "broken.txt")); err != nil {
		t.Skipf("シンボリックリンクを作れない環境なので飛ばす: %v", err)
	}

	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packed")
	var warns []string
	if err := bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{
		Program:        prog,
		Name:           "main.nako3",
		ResourceDir:    res,
		IncludeSymlink: true,
		Warn: func(format string, args ...any) {
			warns = append(warns, fmt.Sprintf(format, args...))
		},
	}); err != nil {
		t.Fatal(err)
	}
	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()

	if _, ok := packed.ReadResource("res/broken.txt"); ok {
		t.Error("壊れたリンクが梱包されている")
	}
	if _, ok := packed.ReadResource("res/ok.txt"); !ok {
		t.Errorf("他のリソースが失われている: %v", packed.Resources())
	}
	if !hasWarning(warns, "broken.txt") {
		t.Errorf("壊れたリンクの警告が出ていない: %v", warns)
	}
}

// TestIncludeSymlinkHiddenTargetWarns pins that a link reaching a hidden
// file warns (Issue #263 のレビュー指定)。
func TestIncludeSymlinkHiddenTargetWarns(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "res")
	if err := os.MkdirAll(res, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res, ".secret.txt"), []byte("隠し"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".secret.txt", filepath.Join(res, "shown.txt")); err != nil {
		t.Skipf("シンボリックリンクを作れない環境なので飛ばす: %v", err)
	}

	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "packed")
	var warns []string
	if err := bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{
		Program:        prog,
		Name:           "main.nako3",
		ResourceDir:    res,
		IncludeSymlink: true,
		Warn: func(format string, args ...any) {
			warns = append(warns, fmt.Sprintf(format, args...))
		},
	}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		packed, err := bundle.Open(out)
		if err == nil {
			packed.Close()
		}
	}()

	if !hasWarning(warns, "隠し") {
		t.Errorf("隠しファイルリンクの警告が出ていない: %v", warns)
	}
}

// TestSkipMatchesLinkTarget pins that a link whose target is the output file
// being written is still skipped, even though only the link's name is walked
// (Spec.Skip はリンク未解決のパスで渡される)。
func TestSkipMatchesLinkTarget(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "res")
	if err := os.MkdirAll(res, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res, "a.txt"), []byte("中身"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(res, "packed")
	if err := os.Symlink("packed", filepath.Join(res, "alias")); err != nil {
		t.Skipf("シンボリックリンクを作れない環境なので飛ばす: %v", err)
	}

	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	err = bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{
		Program:     prog,
		Name:        "main.nako3",
		ResourceDir: res,
		Skip:        map[string]bool{out: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()

	for _, name := range packed.Resources() {
		if name == "res/packed" || name == "res/alias" {
			t.Errorf("出力ファイルがリソースに巻き込まれている: %s", name)
		}
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
