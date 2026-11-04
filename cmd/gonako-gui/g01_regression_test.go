package main

// G01: .app の再ビルドが失敗しても、直前に使えていた正常な .app を失わない。
// 再現入力はIssueの手順どおり、フォルダ外を指すsymlinkを含むプロジェクトを
// 同じ.appへ梱包して失敗させる。

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		t.Skipf("シンボリックリンクを作れません: %v", err)
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
	if res.Warning != "" || len(res.Leftover) != 0 {
		t.Fatalf("公開前の失敗でwarningを返した: %q %v", res.Warning, res.Leftover)
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
	if res.Warning != "" || len(res.Leftover) != 0 {
		t.Fatalf("正常な置換でwarningを返した: %q %v", res.Warning, res.Leftover)
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
	first := buildAppFromFolder(folder, entry, out, "実機")
	if !first.OK {
		t.Fatalf("1回目の変換に失敗した: %s", first.Error)
	}
	if want := independentAppSize(t, out); first.Size != want || first.Size == 0 {
		t.Fatalf("初回のSizeが実native appと違う: got=%d want=%d", first.Size, want)
	}
	if got := runBuiltApp(t, exe, result); got != "旧版" {
		t.Fatalf("1回目の.appの実行結果が違う: %q", got)
	}

	// フォルダ外リンクで梱包を失敗させる。旧.appの全manifestが残ることも見る。
	beforeFailure := appTreeSnapshot(t, out)
	outside := filepath.Join(dir, "outside.txt")
	writeFile(t, outside, "フォルダの外")
	if err := os.Symlink(outside, filepath.Join(folder, "外へのリンク")); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}
	if res := buildAppFromFolder(folder, entry, out, "実機"); res.OK {
		t.Fatal("フォルダ外リンクがあるのに梱包が成功した")
	}
	assertAppUnchanged(t, out, beforeFailure)
	if got := runBuiltApp(t, exe, result); got != "旧版" {
		t.Fatalf("失敗後の旧.appが起動しない、または内容が違う: %q", got)
	}

	// 直して作り直すと、新しい.appに入れ替わる
	if err := os.Remove(filepath.Join(folder, "外へのリンク")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, entry, "「新版」を「"+result+"」へ保存\n")
	rebuilt := buildAppFromFolder(folder, entry, out, "実機")
	if !rebuilt.OK {
		t.Fatalf("3回目の変換に失敗した: %s", rebuilt.Error)
	}
	if want := independentAppSize(t, out); rebuilt.Size != want || rebuilt.Size == 0 {
		t.Fatalf("再ビルドのSizeが新しい実native appと違う: got=%d want=%d", rebuilt.Size, want)
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

// restoreTreePermissions makes every directory below root writable again, so a
// test that locked a folder on purpose can still be cleaned up.
func restoreTreePermissions(t *testing.T, root string) {
	t.Helper()
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err == nil && fi.IsDir() {
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
}

// entriesWithPrefix lists the hidden staging or backup entries next to an app.
func entriesWithPrefix(t *testing.T, dir, prefix string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			found = append(found, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(found)
	return found
}

// 新.appの公開後に旧backupの削除だけが失敗したら、変換は成功のままwarningと
// 残ったbackup pathを返す。新.appは公開済みなのでrollbackしない。
func TestG01CleanupFailureWarnsAndKeepsNewApp(t *testing.T) {
	skipIfCannotLockDir(t)
	dir := t.TempDir()
	useRuntime(t, dir)
	folder := filepath.Join(dir, "src")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>旧版</h1>")
	out := filepath.Join(dir, "アプリ.app")
	if res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "アプリ"); !res.OK {
		t.Fatalf("1回目の変換に失敗した: %s", res.Error)
	}
	locked := filepath.Join(out, "Contents", "Resources", "locked")
	writeFile(t, filepath.Join(locked, "kept.txt"), "消せない")
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restoreTreePermissions(t, dir) })

	writeFile(t, filepath.Join(folder, "index.html"), "<h1>新版</h1>")
	res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "アプリ")
	if !res.OK {
		t.Fatalf("公開後の片付け失敗を変換失敗にしている: %s", res.Error)
	}
	if res.Path != out {
		t.Fatalf("新.appのパスを返していない: %s", res.Path)
	}
	if !strings.Contains(res.Warning, "片付けられません") || !strings.Contains(res.Warning, "完全でない") {
		t.Fatalf("warningが部分損壊を明示していない: %q", res.Warning)
	}
	if len(res.Leftover) != 1 || res.Leftover[0] == "" {
		t.Fatalf("残ったbackup pathを返していない: %v", res.Leftover)
	}
	if _, err := os.Stat(res.Leftover[0]); err != nil {
		t.Fatalf("warningのbackup pathが実在しない: %v", err)
	}
	if _, err := os.Stat(filepath.Join(res.Leftover[0], "Contents", "Resources", "locked", "kept.txt")); err != nil {
		t.Fatalf("残ったbackupの内容を確認できない: %v", err)
	}

	exe := filepath.Join(out, "Contents", "MacOS", "アプリ")
	packed, err := bundle.Open(exe)
	if err != nil {
		t.Fatalf("新.appの実行ファイルを開けない: %v", err)
	}
	defer packed.Close()
	if data, ok := packed.ReadResource("index.html"); !ok || !strings.Contains(string(data), "新版") {
		t.Fatalf("新.appが新しい内容になっていない: %q %v", data, ok)
	}
}

// 旧.appを退避した後、新.appのrenameと復旧renameの両方が失敗したら、完全な
// 旧.appが残るbackup pathをerrorで明示する。この時点では公開していない。
func TestG01RollbackFailureNamesRescueBackup(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)
	folder, out := buildExistingApp(t, dir)
	before := appTreeSnapshot(t, out)

	renameBuiltApp = func(oldpath, newpath string) error {
		if strings.Contains(oldpath, ".staging-") || strings.Contains(oldpath, ".backup-") {
			return errors.New("rename失敗")
		}
		return os.Rename(oldpath, newpath)
	}
	t.Cleanup(func() { renameBuiltApp = os.Rename })

	res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "アプリ")
	if res.OK {
		t.Fatal("新.appを置けず復旧もできないのに変換が成功した")
	}
	if !strings.Contains(res.Error, "古いアプリは『") || !strings.Contains(res.Error, "元の場所へ戻せません") {
		t.Fatalf("救出backup pathをerrorで明示していない: %s", res.Error)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("公開していないのに最終パスが残っている: %v", err)
	}
	backups := entriesWithPrefix(t, dir, ".アプリ.app.backup-")
	if len(backups) != 1 {
		t.Fatalf("救出backupが1つでない: %v", backups)
	}
	if !strings.Contains(res.Error, backups[0]) {
		t.Fatalf("errorのpathが実際のbackupと違う: %s / %s", res.Error, backups[0])
	}
	after := appTreeSnapshot(t, backups[0])
	if diff := diffSnapshots(before, after); len(diff) != 0 {
		t.Fatalf("救出backupが旧.appの全manifestを保持していない: %v", diff)
	}
	_ = os.RemoveAll(backups[0])
}

// 変換成功の分岐がwarningと残pathを既存のダイアログで表示する。
func TestG01BuildWarningIsShownInUI(t *testing.T) {
	app, err := uiFS.ReadFile("ui/app.js")
	if err != nil {
		t.Fatalf("ui/app.js を読めない: %v", err)
	}
	source := string(app)
	start := strings.Index(source, "if (data.ok) {")
	if start < 0 {
		t.Fatal("変換結果の成功分岐がない")
	}
	end := strings.Index(source[start:], "} else {")
	if end < 0 {
		t.Fatal("変換結果の成功分岐の終わりがない")
	}
	success := source[start : start+end]
	for _, want := range []string{
		"data.warning",
		"data.leftover",
		"警告: ",
		"変換しました（警告あり）",
	} {
		if !strings.Contains(success, want) {
			t.Fatalf("成功分岐がwarningを表示しない: %q が無い", want)
		}
	}
}

// 実ランタイムで、公開後の片付け失敗が成功のままwarningになり、新.appが
// 起動することを確かめる。旧backupは部分損壊で、完全な救出物ではない。
func TestG01NativeCleanupFailureWarns(t *testing.T) {
	if os.Getenv("GONAKO_TEST_RUNTIME") == "" {
		t.Skip("GONAKO_TEST_RUNTIME 未設定。実.appの起動確認はnative枠で行う")
	}
	skipIfCannotLockDir(t)
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
	locked := filepath.Join(out, "Contents", "Resources", "locked")
	writeFile(t, filepath.Join(locked, "kept.txt"), "消せない")
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restoreTreePermissions(t, dir) })

	writeFile(t, entry, "「新版」を「"+result+"」へ保存\n")
	res := buildAppFromFolder(folder, entry, out, "実機")
	if !res.OK {
		t.Fatalf("公開後の片付け失敗を変換失敗にしている: %s", res.Error)
	}
	if !strings.Contains(res.Warning, "完全でない") {
		t.Fatalf("warningが部分損壊を明示していない: %q", res.Warning)
	}
	if len(res.Leftover) != 1 {
		t.Fatalf("残ったbackup pathを返していない: %v", res.Leftover)
	}
	if want := independentAppSize(t, out); res.Size != want {
		t.Fatalf("cleanup warning時のSizeが公開済みの新.appと違う: got=%d want=%d", res.Size, want)
	}
	if got := runBuiltApp(t, exe, result); got != "新版" {
		t.Fatalf("公開後の新.appが起動しない、または内容が違う: %q", got)
	}
	if _, err := os.Stat(filepath.Join(res.Leftover[0], "Contents", "Info.plist")); !os.IsNotExist(err) {
		t.Fatalf("部分削除されたbackupにInfo.plistが残っている: %v", err)
	}
	if _, err := os.Stat(filepath.Join(res.Leftover[0], "Contents", "Resources", "locked", "kept.txt")); err != nil {
		t.Fatalf("残ったbackupのlocked内容がない: %v", err)
	}
}

// 実ランタイムで、公開前の復旧失敗が完全な旧.appをbackupへ残し、その実行
// ファイルを起動できることを確かめる。
func TestG01NativeRollbackFailureRescue(t *testing.T) {
	if os.Getenv("GONAKO_TEST_RUNTIME") == "" {
		t.Skip("GONAKO_TEST_RUNTIME 未設定。実.appの起動確認はnative枠で行う")
	}
	dir := t.TempDir()
	useRuntime(t, dir)

	folder := filepath.Join(dir, "src")
	entry := filepath.Join(folder, "main.nako3")
	result := filepath.Join(dir, "result.txt")
	writeFile(t, entry, "「救出」を「"+result+"」へ保存\n")

	out := filepath.Join(dir, "実機.app")
	if res := buildAppFromFolder(folder, entry, out, "実機"); !res.OK {
		t.Fatalf("1回目の変換に失敗した: %s", res.Error)
	}

	renameBuiltApp = func(oldpath, newpath string) error {
		if strings.Contains(oldpath, ".staging-") || strings.Contains(oldpath, ".backup-") {
			return errors.New("rename失敗")
		}
		return os.Rename(oldpath, newpath)
	}
	t.Cleanup(func() { renameBuiltApp = os.Rename })

	res := buildAppFromFolder(folder, entry, out, "実機")
	if res.OK {
		t.Fatal("新.appを置けず復旧もできないのに変換が成功した")
	}
	backups := entriesWithPrefix(t, dir, ".実機.app.backup-")
	if len(backups) != 1 {
		t.Fatalf("救出backupが1つでない: %v", backups)
	}
	if !strings.Contains(res.Error, backups[0]) {
		t.Fatalf("errorのpathが実際のbackupと違う: %s / %s", res.Error, backups[0])
	}
	exe := filepath.Join(backups[0], "Contents", "MacOS", "実機")
	if got := runBuiltApp(t, exe, result); got != "救出" {
		t.Fatalf("救出backupの実nativeが起動しない、または内容が違う: %q", got)
	}
	_ = os.RemoveAll(backups[0])
}

// independentAppSize re-walks the published artifact in the test, so the Size
// the production path reports is compared with the real file system.
func independentAppSize(t *testing.T, path string) int64 {
	t.Helper()
	var total int64
	err := filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("公開物のサイズを測れない: %v", err)
	}
	return total
}

// Sizeは公開後の最終成果物から測る。初回0や旧.appのサイズを返さない。
func TestG01BuildSizeMatchesPublishedApp(t *testing.T) {
	t.Run("初回", func(t *testing.T) {
		dir := t.TempDir()
		useRuntime(t, dir)
		folder := filepath.Join(dir, "src")
		writeFile(t, filepath.Join(folder, "index.html"), "<h1>初回</h1>")
		out := filepath.Join(dir, "App.app")

		res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "App")
		if !res.OK {
			t.Fatalf("変換に失敗した: %s", res.Error)
		}
		want := independentAppSize(t, out)
		if res.Size != want {
			t.Fatalf("初回のSizeが公開物と違う: got=%d want=%d", res.Size, want)
		}
		if res.Size == 0 {
			t.Fatal("初回のSizeが0")
		}
	})

	t.Run("既存appの通常置換", func(t *testing.T) {
		dir := t.TempDir()
		useRuntime(t, dir)
		folder := filepath.Join(dir, "src")
		writeFile(t, filepath.Join(folder, "index.html"), "<h1>旧版</h1>")
		out := filepath.Join(dir, "App.app")
		if res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "App"); !res.OK {
			t.Fatalf("1回目の変換に失敗した: %s", res.Error)
		}
		oldSize := independentAppSize(t, out)

		// 新しいpayloadを確実に変えるため、圧縮されにくい乱数を足す
		blob := make([]byte, 64*1024)
		if _, err := cryptorand.Read(blob); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "data.bin"), blob, 0o644); err != nil {
			t.Fatal(err)
		}
		res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "App")
		if !res.OK {
			t.Fatalf("2回目の変換に失敗した: %s", res.Error)
		}
		want := independentAppSize(t, out)
		if res.Size != want {
			t.Fatalf("置換後のSizeが公開物と違う: got=%d want=%d", res.Size, want)
		}
		if res.Size == oldSize {
			t.Fatalf("置換後も旧.appのサイズを返している: %d", res.Size)
		}
	})

	t.Run("cleanup warning", func(t *testing.T) {
		skipIfCannotLockDir(t)
		dir := t.TempDir()
		useRuntime(t, dir)
		folder := filepath.Join(dir, "src")
		writeFile(t, filepath.Join(folder, "index.html"), "<h1>旧版</h1>")
		out := filepath.Join(dir, "App.app")
		if res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "App"); !res.OK {
			t.Fatalf("1回目の変換に失敗した: %s", res.Error)
		}
		locked := filepath.Join(out, "Contents", "Resources", "locked")
		writeFile(t, filepath.Join(locked, "kept.txt"), "消せない")
		if err := os.Chmod(locked, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { restoreTreePermissions(t, dir) })

		writeFile(t, filepath.Join(folder, "index.html"), "<h1>新版</h1>")
		res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "App")
		if !res.OK {
			t.Fatalf("公開後の片付け失敗を変換失敗にしている: %s", res.Error)
		}
		if res.Warning == "" || len(res.Leftover) != 1 {
			t.Fatalf("warning契約が変わっている: %q %v", res.Warning, res.Leftover)
		}
		want := independentAppSize(t, out)
		if res.Size != want {
			t.Fatalf("warning時のSizeが公開済みの新.appと違う: got=%d want=%d", res.Size, want)
		}
	})
}

// skipIfCannotLockDir skips tests that need a 0500 directory to block
// os.RemoveAll. Root and Windows ignore those permission bits.
func skipIfCannotLockDir(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("ディレクトリ権限で削除を止められない環境")
	}
}

// 出力先の親フォルダが無くても、以前のテンプレート書き出しと同じように作る。
func TestG01CreatesMissingOutputParent(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)
	folder := filepath.Join(dir, "src")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>やあ</h1>")
	out := filepath.Join(dir, "新しい", "階層", "App.app")

	res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), out, "App")
	if !res.OK {
		t.Fatalf("親フォルダが無い出力先で変換に失敗した: %s", res.Error)
	}
	if want := independentAppSize(t, out); res.Size != want || res.Size == 0 {
		t.Fatalf("作った.appのサイズが違う: got=%d want=%d", res.Size, want)
	}
	if _, err := os.Stat(filepath.Join(out, "Contents", "Info.plist")); err != nil {
		t.Fatalf("出力先の親フォルダが作られていない: %v", err)
	}
}

// 出力先が梱包対象フォルダそのものだと、既存の.appを退避したうえで
// プロジェクトフォルダごと入れ替えてしまい、ソース一式を失う。
func TestG01RefusesOutputEqualToSourceFolder(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)
	// フォルダ自体が正常な.appなら checkMacAppOutput を通ってしまう。
	folder := filepath.Join(dir, "Src.app")
	writeFile(t, filepath.Join(folder, "Contents", "Info.plist"), "<plist/>")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>やあ</h1>")
	before := appTreeSnapshot(t, folder)

	res := buildAppFromFolder(folder, filepath.Join(folder, "index.html"), folder, "App")
	if res.OK {
		t.Fatal("梱包対象フォルダ自身を出力先にして変換してしまった")
	}
	if !strings.Contains(res.Error, "同じ") {
		t.Fatalf("同一パスの説明が変わっている: %s", res.Error)
	}
	assertAppUnchanged(t, folder, before)
	if leftovers := leftoverEntries(t, dir); len(leftovers) != 0 {
		t.Fatalf("失敗後に一時成果物が残った: %v", leftovers)
	}
}

// 出力先の親をsymlink経由で指定しても、実体側の既存.appを梱包しない。
func TestG01SymlinkedOutputParentDoesNotPackExistingApp(t *testing.T) {
	dir := t.TempDir()
	useRuntime(t, dir)
	folder := filepath.Join(dir, "src")
	writeFile(t, filepath.Join(folder, "index.html"), "<h1>やあ</h1>")
	link := filepath.Join(dir, "link")
	if err := os.Symlink(folder, link); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}
	out := filepath.Join(link, "App.app")
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
		t.Fatal("symlink経由の出力先にある前回の.appが新しい.appに梱包された")
	}
}
