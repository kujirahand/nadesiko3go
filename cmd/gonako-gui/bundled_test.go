package main

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
)

// fakeRuntime stands in for the gonako-gui binary. Building only appends
// bytes to it, so the tests that just inspect the payload do not need a real
// executable.
func fakeRuntime(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "runtime")
	if err := os.WriteFile(path, []byte("RUNTIME-BYTES"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// useRuntime points the builder at a runtime prepared by the test.
//
// GONAKO_TEST_RUNTIME names a real gonako-gui binary. With one, the tests also
// run the application they build; without one they check the payload only.
func useRuntime(t *testing.T, dir string) (path string, real bool) {
	t.Helper()
	path = os.Getenv("GONAKO_TEST_RUNTIME")
	real = path != ""
	if !real {
		path = fakeRuntime(t, dir)
	}
	runtimePathForBuild = func() (string, error) { return path, nil }
	t.Cleanup(func() { runtimePathForBuild = os.Executable })
	return path, real
}

// writeFile writes one file, creating the folders above it.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// なでしこプログラムを梱包すると、フォルダ以下がそのままの相対パスで入る。
func TestBuildAppFromFolderProgram(t *testing.T) {
	dir := t.TempDir()
	_, real := useRuntime(t, dir)

	folder := filepath.Join(dir, "myapp")
	result := filepath.Join(dir, "result.txt")
	writeFile(t, filepath.Join(folder, "data", "greeting.txt"), "こんにちは同梱データ")
	writeFile(t, filepath.Join(folder, "main.nako3"),
		"S=「data/greeting.txt」を開く\nSを「"+result+"」へ保存\n")
	// 隠しフォルダは梱包しない
	writeFile(t, filepath.Join(folder, ".git", "config"), "x")

	out := filepath.Join(dir, "MyAppExe")
	res := buildAppFromFolder(folder, filepath.Join(folder, "main.nako3"), out, "テストアプリ")
	if !res.OK {
		t.Fatalf("変換に失敗した: %s", res.Error)
	}
	if res.Kind != bundle.KindProgram {
		t.Fatalf("種類が違う: %s", res.Kind)
	}

	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatalf("作った実行ファイルを開けない: %v", err)
	}
	defer packed.Close()

	if packed.Program == nil {
		t.Fatal("プログラムが入っていない")
	}
	if packed.Title != "テストアプリ" {
		t.Fatalf("タイトルが違う: %s", packed.Title)
	}
	// フォルダ名を挟まず、開発時と同じ相対パスで読める
	if data, ok := packed.ReadResource("data/greeting.txt"); !ok || string(data) != "こんにちは同梱データ" {
		t.Fatalf("同梱リソースが読めない: %q %v", data, ok)
	}
	if _, ok := packed.ReadResource(".git/config"); ok {
		t.Fatal("隠しフォルダが梱包されている")
	}

	if !real {
		return
	}
	// 本物のランタイムなら、できた実行ファイルを動かして確かめる
	if err := exec.Command(out).Run(); err != nil {
		t.Fatalf("作った実行ファイルが動かない: %v", err)
	}
	got, err := os.ReadFile(result)
	if err != nil {
		t.Fatalf("実行結果が書き出されていない: %v", err)
	}
	if strings.TrimSpace(string(got)) != "こんにちは同梱データ" {
		t.Fatalf("同梱リソースを読めていない: %q", got)
	}
}

// HTMLを梱包すると、開始ページ付きのHTMLアプリになる。
func TestBuildAppFromFolderHTML(t *testing.T) {
	dir := t.TempDir()
	_, real := useRuntime(t, dir)

	folder := filepath.Join(dir, "site")
	writeFile(t, filepath.Join(folder, "index.html"),
		`<!DOCTYPE html><html lang="ja"><head><meta charset="UTF-8">`+
			`<link rel="stylesheet" href="css/style.css"></head><body><h1>やあ</h1></body></html>`)
	writeFile(t, filepath.Join(folder, "css", "style.css"), "body{color:red}")

	out := filepath.Join(dir, "SiteApp")
	res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "サイト")
	if !res.OK {
		t.Fatalf("変換に失敗した: %s", res.Error)
	}
	if res.Kind != bundle.KindHTML {
		t.Fatalf("種類が違う: %s", res.Kind)
	}

	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatalf("作った実行ファイルを開けない: %v", err)
	}
	defer packed.Close()

	if packed.Entry != "index.html" {
		t.Fatalf("開始ページが違う: %s", packed.Entry)
	}
	if packed.Program != nil {
		t.Fatal("HTMLアプリにプログラムが入っている")
	}
	// WebViewへ配信できる形になっている
	fsys, err := packed.ResourceFS()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.html", "css/style.css"} {
		f, err := fsys.Open(name)
		if err != nil {
			t.Fatalf("%s を配信できない: %v", name, err)
		}
		f.Close()
	}

	if !real {
		return
	}
	// ウィンドウが開くので、起動できることだけ確かめて終わらせる
	cmd := exec.Command(out)
	if err := cmd.Start(); err != nil {
		t.Fatalf("起動できない: %v", err)
	}
	time.Sleep(2 * time.Second)
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		t.Fatal("起動直後に終了した")
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

// macOSでは .app フォルダとして書き出し、ひな形とアイコンを同梱する。
func TestBuildMacAppBundle(t *testing.T) {
	dir := t.TempDir()
	_, real := useRuntime(t, dir)

	folder := filepath.Join(dir, "src")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>やあ</h1>")

	out := filepath.Join(dir, "マイアプリ.app")
	res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "マイアプリ")
	if !res.OK {
		t.Fatalf("変換に失敗した: %s", res.Error)
	}
	if res.Path != out {
		t.Fatalf(".app のパスを返していない: %s", res.Path)
	}

	// .app の中身が揃っている
	exe := filepath.Join(out, "Contents", "MacOS", "マイアプリ")
	for _, p := range []string{
		filepath.Join(out, "Contents", "Info.plist"),
		filepath.Join(out, "Contents", "PkgInfo"),
		filepath.Join(out, "Contents", "Resources", "AppIcon.icns"),
		exe,
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s がない: %v", p, err)
		}
	}
	if fi, err := os.Stat(exe); err != nil || fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("実行権限が付いていない: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(out, "Contents", "Resources", "AppIcon.icns")); err != nil || fi.Size() == 0 {
		t.Fatalf("アイコンが空: %v", err)
	}

	// Info.plist に名前が埋まっている
	plist, err := os.ReadFile(filepath.Join(out, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<string>マイアプリ</string>",                    // CFBundleName
		"<string>AppIcon</string>",                  // CFBundleIconFile
		"<string>com.nadesiko3.gonako.app</string>", // 日本語名は識別子に使えないので app に寄る
		"<string>" + appVersion + "</string>",       // CFBundleVersion
	} {
		if !strings.Contains(string(plist), want) {
			t.Fatalf("Info.plist に %s がない:\n%s", want, plist)
		}
	}
	// ひな形のコメントに残る {{...}} が置換されずに漏れていないこと
	if strings.Contains(string(plist), "{{") {
		t.Fatalf("Info.plist に未置換の項目が残っている:\n%s", plist)
	}

	// 実行ファイルの中にペイロードが入っている
	packed, err := bundle.Open(exe)
	if err != nil {
		t.Fatalf(".app の実行ファイルを開けない: %v", err)
	}
	defer packed.Close()
	if packed.Entry != "index.html" {
		t.Fatalf("開始ページが違う: %s", packed.Entry)
	}

	if !real {
		return
	}
	// 本物のランタイムなら、.app として起動できることを確かめる
	cmd := exec.Command(exe)
	if err := cmd.Start(); err != nil {
		t.Fatalf("起動できない: %v", err)
	}
	time.Sleep(2 * time.Second)
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		t.Fatal("起動直後に終了した")
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

// 持ち込みの icon.icns があれば、それをアプリのアイコンにする。
func TestBuildMacAppUsesFolderIcon(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)

	folder := filepath.Join(dir, "src")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>やあ</h1>")
	writeFile(t, filepath.Join(folder, "icon.icns"), "MY-OWN-ICON")

	out := filepath.Join(dir, "app.app")
	if res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "app"); !res.OK {
		t.Fatalf("変換に失敗した: %s", res.Error)
	}
	got, err := os.ReadFile(filepath.Join(out, "Contents", "Resources", "AppIcon.icns"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "MY-OWN-ICON" {
		t.Fatalf("持ち込みアイコンが使われていない: %q", got)
	}
}

// .app を作り直せる。ただし .app でないフォルダは消さない。
func TestBuildMacAppRebuildAndGuard(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)

	folder := filepath.Join(dir, "src")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>やあ</h1>")
	entry := filepath.Join(folder, "index.html")

	// 2回目も成功し、前回の中身は残らない
	out := filepath.Join(dir, "app.app")
	if res := buildAppFromFolder(folder, entry, out, "app"); !res.OK {
		t.Fatalf("1回目に失敗した: %s", res.Error)
	}
	writeFile(t, filepath.Join(out, "Contents", "Resources", "ゴミ.txt"), "前回の残り")
	if res := buildAppFromFolder(folder, entry, out, "app"); !res.OK {
		t.Fatalf("2回目に失敗した: %s", res.Error)
	}
	if _, err := os.Stat(filepath.Join(out, "Contents", "Resources", "ゴミ.txt")); err == nil {
		t.Fatal("前回の中身が残っている")
	}

	// .app に見えても中身が違うフォルダは消さない
	notAnApp := filepath.Join(dir, "大事なフォルダ.app")
	writeFile(t, filepath.Join(notAnApp, "大事なファイル.txt"), "消えては困る")
	res := buildAppFromFolder(folder, entry, notAnApp, "app")
	if res.OK {
		t.Fatal("macOSアプリでないフォルダを上書きしてしまった")
	}
	if _, err := os.Stat(filepath.Join(notAnApp, "大事なファイル.txt")); err != nil {
		t.Fatalf("関係ないフォルダを消してしまった: %v", err)
	}
}

// 出力先がフォルダの中にあっても、自分自身を巻き込まない。
func TestBuildAppSkipsItsOwnOutput(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)

	folder := filepath.Join(dir, "app")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>やあ</h1>")

	out := filepath.Join(folder, "App")
	if res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "App"); !res.OK {
		t.Fatalf("変換に失敗した: %s", res.Error)
	}
	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()
	if _, ok := packed.ReadResource("App"); ok {
		t.Fatal("出力した実行ファイル自身が梱包されている")
	}
}

// fakePE builds just enough of a Windows実行ファイル to exercise the header
// walk: MS-DOSヘッダ、PE署名、COFFヘッダ、オプショナルヘッダ。
func fakePE(magic uint16, subsystem uint16) []byte {
	const peOff = 0x80
	data := make([]byte, peOff+24+72)
	data[0], data[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(data[0x3C:], peOff)
	copy(data[peOff:], "PE\x00\x00")
	opt := peOff + 24
	binary.LittleEndian.PutUint16(data[opt:], magic)
	binary.LittleEndian.PutUint16(data[opt+68:], subsystem)
	return data
}

func subsystemOf(t *testing.T, path string) uint16 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	peOff := int64(binary.LittleEndian.Uint32(data[0x3C:]))
	return binary.LittleEndian.Uint16(data[peOff+24+68:])
}

// Windowsの実行ファイルは、コンソール窓の出ないGUIアプリに書き換える。
func TestMakeWindowsGUI(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name  string
		data  []byte
		want  uint16
		magic uint16
	}{
		{"64bit(PE32+)のコンソールアプリ", fakePE(0x20b, 3), 2, 0x20b},
		{"32bit(PE32)のコンソールアプリ", fakePE(0x10b, 3), 2, 0x10b},
		{"すでにGUIなら触らない", fakePE(0x20b, 2), 2, 0x20b},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(dir, c.name+".exe")
			if err := os.WriteFile(path, c.data, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := makeWindowsGUI(path); err != nil {
				t.Fatal(err)
			}
			if got := subsystemOf(t, path); got != c.want {
				t.Fatalf("サブシステムが %d、期待は %d", got, c.want)
			}
		})
	}
}

// Windows以外の実行ファイルには手を出さない。
func TestMakeWindowsGUILeavesOthersAlone(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"macho":  "\xcf\xfa\xed\xfe ここはMach-O",
		"elf":    "\x7fELF ここはELF",
		"text":   "ただのテキスト",
		"tiny":   "MZ",
		"mzonly": "MZ" + strings.Repeat("\x00", 200), // PE署名が無い
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := makeWindowsGUI(path); err != nil {
			t.Fatalf("%s でエラー: %v", name, err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != content {
			t.Fatalf("%s が書き換えられた", name)
		}
	}
}

// buildSampleWindowsExe cross-compiles a tiny Windows executable, so that the
// header walk is checked against a file a compiler actually produced rather
// than only against the hand-made one above. cgoは要らないので、Windows以外の
// 環境でも作れる。
func buildSampleWindowsExe(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "sample")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(src, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(src, "go.mod"), "module sample\n\ngo 1.21\n")

	out := filepath.Join(dir, "sample.exe")
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = src
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("Windows向けにビルドできない環境: %v: %s", err, output)
	}
	return out
}

// 変換の出口でも書き換わることを、本物のWindows実行ファイルで確かめる。
func TestBuildAppMakesWindowsExeGUI(t *testing.T) {
	if testing.Short() {
		t.Skip("クロスビルドするので -short では飛ばす")
	}
	dir := t.TempDir()
	realPE := buildSampleWindowsExe(t, dir)
	if got := subsystemOf(t, realPE); got != 3 {
		t.Fatalf("見本がコンソールアプリでない: サブシステム=%d", got)
	}

	runtimePathForBuild = func() (string, error) { return realPE, nil }
	t.Cleanup(func() { runtimePathForBuild = os.Executable })

	folder := filepath.Join(dir, "src")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>やあ</h1>")

	out := filepath.Join(dir, "app.exe")
	if res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "app"); !res.OK {
		t.Fatalf("変換に失敗した: %s", res.Error)
	}
	if got := subsystemOf(t, out); got != 2 {
		t.Fatalf("GUIアプリになっていない: サブシステム=%d", got)
	}
	// 書き換えてもペイロードは読める
	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatalf("書き換え後にバンドルを読めない: %v", err)
	}
	defer packed.Close()
	if packed.Entry != "index.html" {
		t.Fatalf("開始ページが違う: %s", packed.Entry)
	}
}

// 変換できない指定は、理由の分かるエラーにする。
func TestBuildAppRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)

	folder := filepath.Join(dir, "app")
	writeFile(t, filepath.Join(folder, "memo.txt"), "ただのメモ")
	writeFile(t, filepath.Join(dir, "outside.nako3"), "「やあ」と表示")
	writeFile(t, filepath.Join(folder, "broken.nako3"), "もし\n")

	cases := []struct {
		name  string
		entry string
		want  string
	}{
		{"対応しない拡張子", filepath.Join(folder, "memo.txt"), "なでしこプログラム"},
		{"フォルダの外", filepath.Join(dir, "outside.nako3"), "フォルダの中"},
		{"文法エラー", filepath.Join(folder, "broken.nako3"), "エラー"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := buildAppFromFolder(folder, c.entry, filepath.Join(dir, "Out"), "Out")
			if res.OK {
				t.Fatal("エラーになっていない")
			}
			if !strings.Contains(res.Error, c.want) {
				t.Fatalf("説明が足りない: %s", res.Error)
			}
		})
	}
}
