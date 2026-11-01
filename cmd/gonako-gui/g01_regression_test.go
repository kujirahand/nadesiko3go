package main

// G01: .app の再ビルドが失敗しても、直前に使えていた正常な .app を失わない。
// 再現入力はIssueの手順どおり、フォルダ外を指すsymlinkを含むプロジェクトを
// 同じ.appへ梱包して失敗させる。

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
)

// appTreeSnapshot records every entry below dir with a content hash and mode,
// so a rebuild that changed the bundle shows up as a diff instead of an
// eyeball comparison.
func appTreeSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if fi.IsDir() {
			snapshot[rel+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		snapshot[rel] = hex.EncodeToString(sum[:]) + " " + fi.Mode().String()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

// diffSnapshots reports the entries that differ between two snapshots.
func diffSnapshots(before, after map[string]string) []string {
	var diff []string
	for name, want := range before {
		got, ok := after[name]
		switch {
		case !ok:
			diff = append(diff, name+" が消えた")
		case got != want:
			diff = append(diff, name+" が変わった")
		}
	}
	for name := range after {
		if _, ok := before[name]; !ok {
			diff = append(diff, name+" が増えた")
		}
	}
	sort.Strings(diff)
	return diff
}

// leftoverEntries lists the private staging and backup entries a build may
// leave next to the finished app. They are hidden, so the listing filters on
// the leading dot instead of enumerating every name a build could choose.
func leftoverEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var leftovers []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			leftovers = append(leftovers, entry.Name())
		}
	}
	return leftovers
}

// 再ビルドの梱包が失敗しても、既存の.appは1バイトも変わらない。
func TestG01FailedRebuildKeepsExistingApp(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)

	folder := filepath.Join(dir, "src")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>旧版</h1>")
	out := filepath.Join(dir, "アプリ.app")
	if res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "アプリ"); !res.OK {
		t.Fatalf("1回目の変換に失敗した: %s", res.Error)
	}
	writeFile(t, filepath.Join(out, "Contents", "Resources", "旧版のしるし.txt"), "これは残る")
	before := appTreeSnapshot(t, out)

	// フォルダ外を指すsymlinkがあると、既定の梱包は失敗する
	outside := filepath.Join(dir, "outside.txt")
	writeFile(t, outside, "フォルダの外")
	if err := os.Symlink(outside, filepath.Join(folder, "外へのリンク")); err != nil {
		t.Fatal(err)
	}

	res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "アプリ")
	if res.OK {
		t.Fatal("フォルダ外リンクがあるのに梱包が成功した")
	}
	if !strings.Contains(res.Error, "フォルダ外") {
		t.Fatalf("梱包エラーの説明が変わっている: %s", res.Error)
	}

	after := appTreeSnapshot(t, out)
	if diff := diffSnapshots(before, after); len(diff) != 0 {
		t.Fatalf("失敗した再ビルドが既存の.appを変えた: %v", diff)
	}
	if data, err := os.ReadFile(filepath.Join(out, "Contents", "Resources", "旧版のしるし.txt")); err != nil || string(data) != "これは残る" {
		t.Fatalf("旧版のファイルが失われた: %q %v", data, err)
	}
	if leftovers := leftoverEntries(t, dir); len(leftovers) != 0 {
		t.Fatalf("失敗後に一時成果物が残った: %v", leftovers)
	}
}

// buildExistingApp leaves one working .app behind, so the failure tests all
// start from the same known-good artifact.
func buildExistingApp(t *testing.T, dir string) (folder, out string) {
	t.Helper()
	folder = filepath.Join(dir, "src")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>旧版</h1>")
	out = filepath.Join(dir, "アプリ.app")
	if res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "アプリ"); !res.OK {
		t.Fatalf("1回目の変換に失敗した: %s", res.Error)
	}
	writeFile(t, filepath.Join(out, "Contents", "Resources", "旧版のしるし.txt"), "これは残る")
	return folder, out
}

// assertAppUnchanged compares the bundle with the snapshot taken before the
// failed rebuild.
func assertAppUnchanged(t *testing.T, out string, before map[string]string) {
	t.Helper()
	after := appTreeSnapshot(t, out)
	if diff := diffSnapshots(before, after); len(diff) != 0 {
		t.Fatalf("失敗した再ビルドが既存の.appを変えた: %v", diff)
	}
}

// 権限設定に失敗しても、既存の.appは変わらない。
func TestG01ChmodFailureKeepsExistingApp(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)
	folder, out := buildExistingApp(t, dir)
	before := appTreeSnapshot(t, out)

	chmodBuiltApp = func(string, os.FileMode) error { return os.ErrPermission }
	t.Cleanup(func() { chmodBuiltApp = os.Chmod })

	res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "アプリ")
	if res.OK {
		t.Fatal("実行権限を付けられないのに変換が成功した")
	}
	if !strings.Contains(res.Error, "実行権限") {
		t.Fatalf("権限エラーの説明が変わっている: %s", res.Error)
	}
	assertAppUnchanged(t, out, before)
	if leftovers := leftoverEntries(t, dir); len(leftovers) != 0 {
		t.Fatalf("失敗後に一時成果物が残った: %v", leftovers)
	}
}

// 最後の置換に失敗したら、退避した旧.appを元の場所へ戻す。
func TestG01InstallFailureRestoresExistingApp(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)
	folder, out := buildExistingApp(t, dir)
	before := appTreeSnapshot(t, out)

	renameBuiltApp = func(oldpath, newpath string) error {
		if strings.Contains(oldpath, ".staging-") {
			return errors.New("置換に失敗した")
		}
		return os.Rename(oldpath, newpath)
	}
	t.Cleanup(func() { renameBuiltApp = os.Rename })

	res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "アプリ")
	if res.OK {
		t.Fatal("置換に失敗したのに変換が成功した")
	}
	if !strings.Contains(res.Error, "新しいアプリを置けません") {
		t.Fatalf("置換エラーの説明が変わっている: %s", res.Error)
	}
	assertAppUnchanged(t, out, before)
	if leftovers := leftoverEntries(t, dir); len(leftovers) != 0 {
		t.Fatalf("失敗後に一時成果物が残った: %v", leftovers)
	}
}

// 成功したときだけ新しい.appに入れ替わり、一時フォルダも退避先も残さない。
// 実行ファイル名とBundle Identifierは最終basenameから作る。
func TestG01SuccessfulRebuildSwapsInNewApp(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)
	folder := filepath.Join(dir, "src")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>旧版</h1>")
	out := filepath.Join(dir, "MyTool.app")
	if res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "MyTool"); !res.OK {
		t.Fatalf("1回目の変換に失敗した: %s", res.Error)
	}
	writeFile(t, filepath.Join(out, "Contents", "Resources", "旧版のしるし.txt"), "これは消える")

	writeFile(t, filepath.Join(folder, "index.html"), "<h1>新版</h1>")
	res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "表示タイトル")
	if !res.OK {
		t.Fatalf("2回目の変換に失敗した: %s", res.Error)
	}
	if res.Path != out {
		t.Fatalf(".app のパスを返していない: %s", res.Path)
	}
	if _, err := os.Stat(filepath.Join(out, "Contents", "Resources", "旧版のしるし.txt")); err == nil {
		t.Fatal("前回の.appの中身が残っている")
	}

	exe := filepath.Join(out, "Contents", "MacOS", "MyTool")
	fi, err := os.Stat(exe)
	if err != nil {
		t.Fatalf("最終basenameの実行ファイルがない: %v", err)
	}
	if fi.Mode().Perm()&0o111 == 0 {
		t.Fatal("実行権限が付いていない")
	}
	plist, err := os.ReadFile(filepath.Join(out, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<string>表示タイトル</string>",
		"<string>com.nadesiko3.gonako.mytool</string>",
	} {
		if !strings.Contains(string(plist), want) {
			t.Fatalf("Info.plist に %s がない:\n%s", want, plist)
		}
	}

	packed, err := bundle.Open(exe)
	if err != nil {
		t.Fatalf("新しい.appの実行ファイルを開けない: %v", err)
	}
	defer packed.Close()
	if packed.Entry != "index.html" {
		t.Fatalf("開始ページが違う: %s", packed.Entry)
	}

	if leftovers := leftoverEntries(t, dir); len(leftovers) != 0 {
		t.Fatalf("成功後に一時成果物が残った: %v", leftovers)
	}
}

// .app の形をしていないフォルダは置き換えない。
func TestG01RefusesNonAppFolder(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)
	folder := filepath.Join(dir, "src")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>やあ</h1>")
	out := filepath.Join(dir, "大事なフォルダ.app")
	writeFile(t, filepath.Join(out, "大事なファイル.txt"), "消えては困る")
	before := appTreeSnapshot(t, out)

	res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "app")
	if res.OK {
		t.Fatal("macOSアプリでないフォルダを上書きしてしまった")
	}
	if !strings.Contains(res.Error, "macOSアプリではない") {
		t.Fatalf("保護の説明が変わっている: %s", res.Error)
	}
	assertAppUnchanged(t, out, before)
	if leftovers := leftoverEntries(t, dir); len(leftovers) != 0 {
		t.Fatalf("失敗後に一時成果物が残った: %v", leftovers)
	}
}

// 出力先がフォルダの中でも、前回の.appを新しい.appへ梱包しない。
func TestG01RebuildInsideSourceFolderDoesNotPackPreviousApp(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)
	folder := filepath.Join(dir, "src")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>やあ</h1>")
	out := filepath.Join(folder, "App.app")
	if res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "App"); !res.OK {
		t.Fatalf("1回目の変換に失敗した: %s", res.Error)
	}
	writeFile(t, filepath.Join(out, "Contents", "Resources", "旧版のしるし.txt"), "前回の残り")
	if res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "App"); !res.OK {
		t.Fatalf("2回目の変換に失敗した: %s", res.Error)
	}

	packed, err := bundle.Open(filepath.Join(out, "Contents", "MacOS", "App"))
	if err != nil {
		t.Fatalf("新しい.appの実行ファイルを開けない: %v", err)
	}
	defer packed.Close()
	if _, ok := packed.ReadResource("App.app/Contents/Resources/旧版のしるし.txt"); ok {
		t.Fatal("前回の.appが新しい.appに梱包された")
	}
}

// 実ランタイムが用意されているときだけ、失敗後の旧.appと成功後の新.appを
// 実際に起動して確かめる。GONAKO_TEST_RUNTIMEは実gonako-guiを指す。
func TestG01NativeRebuildKeepsRunnableApp(t *testing.T) {
	if os.Getenv("GONAKO_TEST_RUNTIME") == "" {
		t.Skip("GONAKO_TEST_RUNTIME 未設定。実.appの起動確認はnative枠で行う")
	}
	dir := t.TempDir()
	useRuntime(t, dir)

	folder := filepath.Join(dir, "src")
	entry := filepath.Join(folder, "main.nako3")
	result := filepath.Join(dir, "result.txt")
	writeFile(t, entry, "「旧版」を「"+result+"」へ保存\n")

	out := filepath.Join(dir, "実機.app")
	exe := filepath.Join(out, "Contents", "MacOS", "実機")
	if res := buildAppFromFolder(folder, entry, out, "実機"); !res.OK {
		t.Fatalf("1回目の変換に失敗した: %s", res.Error)
	}
	if got := runBuiltApp(t, exe, result); got != "旧版" {
		t.Fatalf("1回目の.appの実行結果が違う: %q", got)
	}

	// フォルダ外リンクで梱包を失敗させる
	outside := filepath.Join(dir, "outside.txt")
	writeFile(t, outside, "フォルダの外")
	if err := os.Symlink(outside, filepath.Join(folder, "外へのリンク")); err != nil {
		t.Fatal(err)
	}
	if res := buildAppFromFolder(folder, entry, out, "実機"); res.OK {
		t.Fatal("フォルダ外リンクがあるのに梱包が成功した")
	}
	if got := runBuiltApp(t, exe, result); got != "旧版" {
		t.Fatalf("失敗後の旧.appが起動しない、または内容が違う: %q", got)
	}

	// 直して作り直すと、新しい.appに入れ替わる
	if err := os.Remove(filepath.Join(folder, "外へのリンク")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, entry, "「新版」を「"+result+"」へ保存\n")
	if res := buildAppFromFolder(folder, entry, out, "実機"); !res.OK {
		t.Fatalf("3回目の変換に失敗した: %s", res.Error)
	}
	if got := runBuiltApp(t, exe, result); got != "新版" {
		t.Fatalf("成功後の.appの実行結果が違う: %q", got)
	}
}

// runBuiltApp starts a bundled application, waits for it to write its result,
// then stops it. A program app keeps its window open after the program ends,
// so waiting for the process to exit would hang the test.
func runBuiltApp(t *testing.T, exe, result string) string {
	t.Helper()
	_ = os.Remove(result)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe)
	// 子がWebKitの補助プロセスを残しても、パイプ待ちで停止しないようにする
	cmd.WaitDelay = 5 * time.Second
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatalf("作った.appを起動できない: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(result); err == nil {
			return strings.TrimSpace(string(data))
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("作った.appが期限内に結果を書かない: %s", output.String())
	return ""
}
