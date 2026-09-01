package nodelib_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestCryptoCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `
「ハッシュ一覧: {(ハッシュ関数一覧取得の要素数)>0}」と表示
A=「hello」の「sha256」でハッシュ値計算
「sha256: {A}」と表示
B=「hello」の「md5」でハッシュ値計算
「md5: {B}」と表示
U=ランダムUUID生成
「UUID長: {Uの文字数}」と表示
P=5のランダム配列生成
「配列長: {Pの要素数}」と表示
`)
	want := strings.Join([]string{
		"ハッシュ一覧: true",
		"sha256: 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		"md5: 5d41402abc4b2a76b9719d911017c592",
		"UUID長: 36",
		"配列長: 5",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestZipCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `
"src"のフォルダ作成
「test content」を"src/test.txt"に保存
"src"を"archive.zip"へ圧縮
「zip存在: {"archive.zip"が存在}」と表示
"archive.zip"を"out_dir"へ解凍
「解凍後存在: {"out_dir/src/test.txt"が存在}」と表示
「解凍後内容: {"out_dir/src/test.txt"を開く}」と表示
`)
	want := strings.Join([]string{
		"zip存在: true",
		"解凍後存在: true",
		"解凍後内容: test content",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestExtendedFileCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `
"dirA/sub"のフォルダ作成
「a」を"dirA/1.txt"に保存
「b」を"dirA/sub/2.txt"に保存
「c」を"dirA/sub/3.png"に保存
List="dirA/*.txt"の全ファイル列挙
「全列挙件数: {Listの要素数}」と表示
Info="dirA/1.txt"のファイル情報取得
「情報サイズ: {Info["サイズ"]}」と表示
「情報ディレクトリ: {Info["ディレクトリ"]}」と表示
Tmp=一時フォルダ作成
「一時フォルダ存在: {Tmpがフォルダ存在}」と表示
Rel="dirA"で"sub/2.txt"を相対パス展開
「相対展開: {Relの文字数 > 0}」と表示
「デスクトップ: {(デスクトップの文字数)>0}」と表示
「マイドキュメント: {(マイドキュメントの文字数)>0}」と表示
「母艦パス取得: {(母艦パス取得の文字数)>0}」と表示
`)
	want := strings.Join([]string{
		"全列挙件数: 2",
		"情報サイズ: 1",
		"情報ディレクトリ: false",
		"一時フォルダ存在: true",
		"相対展開: true",
		"デスクトップ: true",
		"マイドキュメント: true",
		"母艦パス取得: true",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestExtendedOSCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `
1と1がASSERT等
「ASSERT成功」と表示
Out=「echo hello」を起動待機
「起動待機: {Outのトリム}」と表示
`)
	want := strings.Join([]string{
		"ASSERT成功",
		"起動待機: hello",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestNetCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `
A={}
A["name"] = "太郎"
A["age"] = "20"
PostData = AのPOSTデータ生成
「POSTデータ: {PostData}」と表示
「IP取得: {(自分IPアドレス取得の文字数)>0}」と表示
「IPV6取得: {(自分IPV6アドレス取得の文字数)>0}」と表示
`)
	// query params ordering in Go url.Values is sorted: age=20&name=...
	if !strings.Contains(got, "POSTデータ: age=20&name=%E5%A4%AA%E9%83%8E") ||
		!strings.Contains(got, "IP取得: true") ||
		!strings.Contains(got, "IPV6取得: true") {
		t.Errorf("NetCommands unexpected result: %s", got)
	}
}

func TestEncodingCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `
「sjisサポート: {"sjis"の文字コード変換サポート判定}」と表示
「eucサポート: {"euc-jp"の文字コード変換サポート判定}」と表示
「unknownサポート: {"unknown_xxx"の文字コード変換サポート判定}」と表示

「こんにちは」を"sjis.txt"にSJISファイル保存
「sjis読: {"sjis.txt"をSJISファイル読}」と表示

「さようなら」を"euc.txt"にEUCファイル保存
「euc読: {"euc.txt"をEUCファイル読}」と表示

Bin = 「テスト」をSJIS変換
「sjis復元: {BinからSJIS取得}」と表示

Bin2 = 「日本語」を"euc-jp"へエンコーディング変換
「euc復元: {Bin2を"euc-jp"からエンコーディング取得}」と表示
`)
	want := strings.Join([]string{
		"sjisサポート: true",
		"eucサポート: true",
		"unknownサポート: false",
		"sjis読: こんにちは",
		"euc読: さようなら",
		"sjis復元: テスト",
		"euc復元: 日本語",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFileEventCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `
●コピーCB
　「コピーコールバック」と表示
ここまで
●移動CB
　「移動コールバック」と表示
ここまで
●削除CB
　「削除コールバック」と表示
ここまで

「data」を"orig.txt"に保存
ファイル処理強制停止
「停止設定完了」と表示

"orig.txt"を"copied.txt"へ「コピーCB」でファイルコピー時
「コピー存在: {"copied.txt"が存在}」と表示

"copied.txt"を"moved.txt"へ「移動CB」でファイル移動時
「移動存在: {"moved.txt"が存在}」と表示

"moved.txt"を「削除CB」でファイル削除時
「削除存在: {"moved.txt"が存在}」と表示
`)
	want := strings.Join([]string{
		"停止設定完了",
		"コピーコールバック",
		"コピー存在: true",
		"移動コールバック",
		"移動存在: true",
		"削除コールバック",
		"削除存在: false",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestProcessAndStdinEventCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `
●(Sで)起動CB
　「起動コールバック: {Sのトリム}」と表示
ここまで
「echo 起動テスト」を「起動CB」で起動時
`)
	want := "起動コールバック: 起動テスト"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader("入力行テスト\n"), nil)
	code := `●取得CB
　「取得: {対象}」と表示
ここまで
「取得CB」を標準入力取得時`
	if err := vm.RunProgram(code, "main.nako3", host); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimRight(out.String(), "\n"); got != "取得: 入力行テスト" {
		t.Errorf("got %q, want %q", got, "取得: 入力行テスト")
	}
}

func TestZipEventCommands(t *testing.T) {
	dir := t.TempDir()
	got := runIn(t, dir, `
●圧縮CB
　「圧縮完了: {対象}」と表示
ここまで
●解凍CB
　「解凍完了: {対象}」と表示
ここまで

"zip_src"のフォルダ作成
「file text」を"zip_src/a.txt"に保存
"zip_src"を"out.zip"へ「圧縮CB」で圧縮時
「zip存在: {"out.zip"が存在}」と表示

"out.zip"を"zip_dest"へ「解凍CB」で解凍時
「解凍存在: {"zip_dest/zip_src/a.txt"が存在}」と表示

"7z"に圧縮解凍ツールパス変更
「パス変更OK」と表示
`)
	want := strings.Join([]string{
		"圧縮完了: true",
		"zip存在: true",
		"解凍完了: true",
		"解凍存在: true",
		"パス変更OK",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestNetExtendedCommands(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, "GET応答")
			return
		}
		if r.Method == http.MethodPost {
			_ = r.ParseMultipartForm(10 << 20)
			if r.MultipartForm != nil && len(r.MultipartForm.Value["name"]) > 0 {
				val := r.FormValue("name")
				fmt.Fprintf(w, "FORM応答:%s", val)
				return
			}
			_ = r.ParseForm()
			val := r.FormValue("name")
			fmt.Fprintf(w, "POST応答:%s", val)
			return
		}
	}))
	defer server.Close()

	code := fmt.Sprintf(`
●AJAX_CB
　「AJAX受信: {対象}」と表示
ここまで
●GET_CB
　「GET受信: {対象}」と表示
ここまで
●POST_CB
　「POST受信: {対象}」と表示
ここまで
●FORM_CB
　「FORM受信: {対象}」と表示
ここまで
●ERR_CB
　「エラーハンドラ」と表示
ここまで

URL = "%s"
URLへ「AJAX_CB」でAJAX送信時
URLへ「GET_CB」でGET送信時
Ans1 = URLのAJAX保障送信
「保障GET: {Ans1}」と表示

Params = {}
Params["name"] = "test"
「POST_CB」でURLへParamsをPOST送信時
Ans2 = URLへParamsをPOST保障送信
「保障POST: {Ans2}」と表示

「FORM_CB」でURLへParamsをPOSTフォーム送信時
Ans3 = URLへParamsをPOSTフォーム保障送信
「保障FORM: {Ans3}」と表示

"dummy"にAJAXオプション設定
「ERR_CB」のAJAX失敗時
「オプション設定完了」と表示
`, server.URL)

	got := runIn(t, dir, code)
	want := strings.Join([]string{
		"AJAX受信: GET応答",
		"GET受信: GET応答",
		"保障GET: GET応答",
		"POST受信: POST応答:test",
		"保障POST: POST応答:test",
		"FORM受信: FORM応答:test",
		"保障FORM: FORM応答:test",
		"オプション設定完了",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

