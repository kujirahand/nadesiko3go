package nodelib_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// runIn runs a program with the working directory set to a fresh temporary
// folder, and reports what it printed.
func runIn(t *testing.T, dir, code string) string {
	t.Helper()
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)

	if err := vm.RunProgram(code, "main.nako3", host); err != nil {
		t.Fatalf("%s\n=> %v", code, err)
	}
	return strings.TrimRight(out.String(), "\n")
}

func TestFileCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `「あいうえお」を"a.txt"に保存
「保存: {"a.txt"が存在}」と表示
「中身: {"a.txt"を開く}」と表示
「サイズ: {"a.txt"のファイルサイズ取得}」と表示
「かきくけこ」を"a.txt"に追記
「追記後: {"a.txt"を開く}」と表示
"a.txt"を"b.txt"にファイルコピー
「コピー: {"b.txt"を開く}」と表示
"b.txt"のファイル削除
「削除後: {"b.txt"が存在}」と表示`)

	want := strings.Join([]string{
		"保存: true",
		"中身: あいうえお",
		"サイズ: 15",
		"追記後: あいうえおかきくけこ",
		"コピー: あいうえおかきくけこ",
		"削除後: false",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFolderCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `"sub"のフォルダ作成
「フォルダ: {"sub"がフォルダ存在}」と表示
「ファイル扱い: {"sub"が存在}」と表示
「x」を"sub/1.txt"に保存
「y」を"sub/2.md"に保存
A="sub"のファイル列挙
「列挙: {A}」と表示
B="sub/*.txt"のファイル列挙
「絞り込み: {B}」と表示`)

	want := strings.Join([]string{
		"フォルダ: true",
		"ファイル扱い: false",
		"列挙: 1.txt,2.md",
		"絞り込み: 1.txt",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPathCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `「名前: {"/a/b/c.txt"のファイル名抽出}」と表示
「パス: {"/a/b/c.txt"のパス抽出}」と表示
「拡張子: {"/a/b/c.txt"の拡張子抽出}」と表示
A="a"と"b"をパス結合
「結合: {A}」と表示`)

	want := strings.Join([]string{
		"名前: c.txt",
		"パス: /a/b",
		"拡張子: .txt",
		"結合: a/b",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestMissingFileReportsPath pins that a missing file is reported by name,
// rather than as a bare OS error.
func TestMissingFileReportsPath(t *testing.T) {
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	err := vm.RunProgram(`"存在しない.txt"を開く`, "main.nako3", host)
	if err == nil {
		t.Fatal("存在しないファイルがエラーにならなかった")
	}
	if !strings.Contains(err.Error(), "ファイル『存在しない.txt』が見つかりません。") {
		t.Errorf("メッセージ = %q", err.Error())
	}
}

func TestOSCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `「OSがある: {(OS取得の文字数)>0}」と表示
「アーキがある: {(OSアーキテクチャ取得の文字数)>0}」と表示
「カレント: {(カレントディレクトリ取得の文字数)>0}」と表示`)
	want := "OSがある: true\nアーキがある: true\nカレント: true"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestArgsAndInput(t *testing.T) {
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader("太郎\n42\n"), []string{"一", "二"})
	code := `A=コマンドライン
「引数: {A}」と表示
B=「名前? 」と尋ねる
「名前: {B}」と表示
C=「数? 」と尋ねる
「倍: {C*2}」と表示`
	if err := vm.RunProgram(code, "main.nako3", host); err != nil {
		t.Fatal(err)
	}
	// プロンプトは改行せずに書くので、入力と同じ行に並ぶ
	want := "引数: 一,二\n名前? 名前: 太郎\n数? 倍: 84\n"
	if out.String() != want {
		t.Errorf("got %q\nwant %q", out.String(), want)
	}
}

// TestExitStopsProgram pins that 『終了』 ends the program without reporting an
// error, and that the host learns the status.
func TestExitStopsProgram(t *testing.T) {
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	err := vm.RunProgram("「A」と表示\n終了\n「B」と表示", "main.nako3", host)
	if err != nil {
		t.Fatalf("『終了』がエラーになった: %v", err)
	}
	if got := strings.TrimRight(out.String(), "\n"); got != "A" {
		t.Errorf("出力 = %q, want \"A\"", got)
	}
	if !host.Exited || host.ExitCode != 0 {
		t.Errorf("Exited=%v ExitCode=%d, want true/0", host.Exited, host.ExitCode)
	}
}

func TestExitCode(t *testing.T) {
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	if err := vm.RunProgram("3で強制終了", "main.nako3", host); err != nil {
		t.Fatal(err)
	}
	if host.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", host.ExitCode)
	}
}

// TestNodelibDoesNotReachCompat pins that the compat runner stays inside
// plugin_system: a nodelib command is not even a known name there.
func TestNodelibDoesNotReachCompat(t *testing.T) {
	if _, err := vm.RunSource(`"a.txt"が存在`, "main.nako3", nil); err == nil {
		t.Error("compat経路でnodelibの命令が使えてしまった")
	}
}

func TestRunFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.nako3")
	if err := os.WriteFile(path, []byte("「やあ」と表示"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	if err := vm.RunFile(path, host); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimRight(out.String(), "\n"); got != "やあ" {
		t.Errorf("出力 = %q, want \"やあ\"", got)
	}
}
