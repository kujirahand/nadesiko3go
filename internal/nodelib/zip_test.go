package nodelib

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeTestZip はテスト用に任意のエントリ名を持つZIPを作る。
func writeTestZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// Issue #194: 宛先を省略（既定値"."）しても全エントリを展開する。
func TestExtractZipDestDot(t *testing.T) {
	dir := t.TempDir()
	writeTestZip(t, filepath.Join(dir, "f.zip"), map[string]string{
		"a.txt":     "hello",
		"sub/b.txt": "world",
	})
	t.Chdir(dir)

	if err := extractZip("f.zip", "."); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", filepath.Join("sub", "b.txt")} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s が展開されていません: %v", name, err)
		}
	}
}

// Zip Slip対策が維持されていること（".."を含むエントリは拒否し、
// 安全なエントリだけを展開した上でエラーを返す）。
func TestExtractZipZipSlip(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	writeTestZip(t, filepath.Join(dir, "f.zip"), map[string]string{
		"../evil.txt":         "evil",
		"sub/../../evil2.txt": "evil2",
		"ok/good.txt":         "good",
	})

	if err := extractZip(filepath.Join(dir, "f.zip"), dest); err == nil {
		t.Error("範囲外エントリを含むのにエラーになりませんでした")
	} else {
		// リンク経由ではないので、理由の分類が「外へ出る」側になっていること
		if !strings.Contains(err.Error(), "展開先の外へ出る") {
			t.Errorf("エラー文言に「展開先の外へ出る」が含まれません: %v", err)
		}
		if strings.Contains(err.Error(), "シンボリックリンク経由") {
			t.Errorf("リンク経由でないのにエラー文言に分類が含まれます: %v", err)
		}
	}
	for _, name := range []string{"evil.txt", "evil2.txt"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("展開先の外に書き込まれました: %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "ok", "good.txt")); err != nil {
		t.Errorf("通常エントリが展開されていません: %v", err)
	}
}

// Issue #185: 展開先の既存シンボリックリンク経由で外へ書き込ませない。
func TestExtractZipSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("シンボリックリンクの作成に権限が必要なため")
	}
	dir := t.TempDir()
	outside := filepath.Join(dir, "outside")
	dest := filepath.Join(dir, "dest")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 展開先の外を指す既存リンクと、既存ファイルを指すリンクを置く
	if err := os.Symlink(outside, filepath.Join(dest, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "keep.txt"), filepath.Join(dest, "victim.txt")); err != nil {
		t.Fatal(err)
	}
	// 存在しないファイルを指す壊れたリンク（dangling）も置く。
	// StatではなくLstatで検査しているので、リンク先が無くても拒否できる。
	if err := os.Symlink(filepath.Join(outside, "nonexistent.txt"), filepath.Join(dest, "broken.txt")); err != nil {
		t.Fatal(err)
	}
	writeTestZip(t, filepath.Join(dir, "f.zip"), map[string]string{
		"link/pwn.txt": "pwn",
		"link/sub/":    "", // ディレクトリエントリが既存リンクに当たるケース
		"victim.txt":   "overwritten",
		"broken.txt":   "dangling",
		"ok.txt":       "ok",
	})

	if err := extractZip(filepath.Join(dir, "f.zip"), dest); err == nil {
		t.Error("リンク経由エントリを含むのにエラーになりませんでした")
	} else {
		// 「外へ出る」ではなく「リンク経由」として分類されていること
		if !strings.Contains(err.Error(), "シンボリックリンク経由") {
			t.Errorf("エラー文言に「シンボリックリンク経由」が含まれません: %v", err)
		}
	}
	if _, err := os.Lstat(filepath.Join(outside, "pwn.txt")); !os.IsNotExist(err) {
		t.Error("シンボリックリンク経由で展開先の外に書き込まれました")
	}
	if _, err := os.Lstat(filepath.Join(outside, "sub")); !os.IsNotExist(err) {
		t.Error("シンボリックリンク経由で展開先の外にフォルダが作られました")
	}
	if _, err := os.Lstat(filepath.Join(outside, "nonexistent.txt")); !os.IsNotExist(err) {
		t.Error("壊れたリンクの先に書き込まれました")
	}
	if body, err := os.ReadFile(filepath.Join(outside, "keep.txt")); err != nil || string(body) != "keep" {
		t.Errorf("リンク先の既存ファイルが上書きされました: %q err=%v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(dest, "ok.txt")); err != nil || string(body) != "ok" {
		t.Errorf("通常エントリが展開されていません: %q err=%v", body, err)
	}
}

// 展開先自身がシンボリックリンクでも、リンク先の内側へ正しく展開する。
func TestExtractZipDestSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("シンボリックリンクの作成に権限が必要なため")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	link := filepath.Join(dir, "destlink")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	writeTestZip(t, filepath.Join(dir, "f.zip"), map[string]string{"a.txt": "hello"})

	if err := extractZip(filepath.Join(dir, "f.zip"), link); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(real, "a.txt")); err != nil || string(body) != "hello" {
		t.Errorf("リンク先へ展開されていません: %q err=%v", body, err)
	}
}

// 展開先の**内側**を指す無害なシンボリックリンクは os.Root により
// 透過的に辿れる（Issue #261 の os.Root 移行による改善）。
func TestExtractZipInsideSymlinkFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("シンボリックリンクの作成に権限が必要なため")
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	if err := os.MkdirAll(filepath.Join(dest, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	// dest/inlink -> inside （内側を指す無害な相対リンク）
	// os.Root は絶対パスを指すシンボリックリンクを拒否するため、相対パスで張る
	if err := os.Symlink("inside", filepath.Join(dest, "inlink")); err != nil {
		t.Fatal(err)
	}
	writeTestZip(t, filepath.Join(dir, "f.zip"), map[string]string{
		"inlink/a.txt": "hello",
		"ok.txt":       "ok",
	})

	err := extractZip(filepath.Join(dir, "f.zip"), dest)
	if err != nil {
		t.Fatalf("内側を指すリンク経由のエントリが拒否された: %v", err)
	}
	// inlink/a.txt は dest/inside/a.txt に展開されるはず
	if body, err := os.ReadFile(filepath.Join(dest, "inside", "a.txt")); err != nil || string(body) != "hello" {
		t.Errorf("リンク経由で展開されませんでした: %q err=%v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(dest, "ok.txt")); err != nil || string(body) != "ok" {
		t.Errorf("通常エントリが展開されていません: %q err=%v", body, err)
	}
}

// 大量の不正エントリを持つZIPでもエラー文言が巨大化しないこと
// （表示は先頭5件し、残りは「他N件」にまとめる）。
func TestExtractZipManyRejectedLimited(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	entries := make(map[string]string)
	for i := 0; i < 8; i++ {
		entries[fmt.Sprintf("../evil%02d.txt", i)] = "evil"
	}
	writeTestZip(t, filepath.Join(dir, "f.zip"), entries)

	err := extractZip(filepath.Join(dir, "f.zip"), dest)
	if err == nil {
		t.Fatal("範囲外エントリを含むのにエラーになりませんでした")
	}
	msg := err.Error()
	if !strings.Contains(msg, "8件") {
		t.Errorf("拒否件数が8件として報告されません: %v", err)
	}
	if !strings.Contains(msg, "他3件") {
		t.Errorf("先頭5件を超えた分が「他N件」に集約されません: %v", err)
	}
	// 表示は先頭5件のはず（mapの列挙順に依存しない検証として
	// 名前中出现数を数える）
	if n := strings.Count(msg, "evil"); n != 5 {
		t.Errorf("表示されるエントリ名が5件ではありません(出現%d件): %v", n, err)
	}
}

// Issue #189: 圧縮の入出力が同じ実体なら拒否し、元データを残す。
func TestCreateZipSameFile(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("zip payload")
	src := filepath.Join(dir, "f.zip")
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := createZip(src, src); err == nil {
		t.Error("入出力が同じファイルでもエラーになりませんでした")
	}
	if got, err := os.ReadFile(src); err != nil || string(got) != string(payload) {
		t.Errorf("元データが失われました: %q err=%v", got, err)
	}

	// 表記違いの同一実体を拒否する検査。filepath.Joinは内部でCleanするため、
	// 「dir/sub/../f.zip」は「dir/f.zip」に正規化され意味がない。
	// カレントを移動し、Cleanされない生の文字列「./f.zip」で渡す。
	t.Chdir(dir)
	if err := createZip(src, "./f.zip"); err == nil {
		t.Error("表記違い（./f.zip）の同一ファイルでもエラーになりませんでした")
	}
	if got, err := os.ReadFile(src); err != nil || string(got) != string(payload) {
		t.Errorf("元データが失われました: %q err=%v", got, err)
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link.zip")
		if err := os.Symlink(src, link); err != nil {
			t.Fatal(err)
		}
		if err := createZip(src, link); err == nil {
			t.Error("シンボリックリンク経由の同一実体でもエラーになりませんでした")
		}
	}
}

// 出力が入力フォルダの内側にあると、作成中のZIP自身を梱包するため拒否する。
func TestCreateZipDestInsideSrc(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "zsrc")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := createZip(srcDir, filepath.Join(srcDir, "x.zip")); err == nil {
		t.Error("圧縮先が圧縮元フォルダの内側でもエラーになりませんでした")
	}
}

// 通常の圧縮→解凍の往復が維持されていること。
func TestZipRoundTrip(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "zsrc")
	dest := filepath.Join(dir, "dest")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(dir, "out.zip")
	if err := createZip(srcDir, zipPath); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		filepath.Join("zsrc", "a.txt"):        "hello",
		filepath.Join("zsrc", "sub", "b.txt"): "world",
	} {
		if got, err := os.ReadFile(filepath.Join(dest, name)); err != nil || string(got) != want {
			t.Errorf("%s = %q, want %q (err=%v)", name, got, want, err)
		}
	}
}
