//go:build unix

package nodelib_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestFileCopyAndMoveRejectFIFO は #406 の回帰テスト。
// 単体指定された FIFO は、内容を読みに行くと書き手が現れるまでブロックする。
// コピー/移動とも読み取り前に明示エラーにし、コピー先を作らず、
// 移動の場合は FIFO 原本を残すこと。
// （修正前のコードではこのテストはハングするため、実行時は
// go test -timeout で外側から守る前提）
func TestFileCopyAndMoveRejectFIFO(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{name: "ファイルコピー", code: "「src.fifo」を「dest」にファイルコピー"},
		{name: "ファイル上書コピー", code: "「src.fifo」を「dest」にファイル上書コピー"},
		{name: "ファイル移動", code: "「src.fifo」を「dest」にファイル移動"},
		{name: "ファイル上書移動", code: "「src.fifo」を「dest」にファイル上書移動"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "src.fifo")
			dest := filepath.Join(dir, "dest")

			if err := syscall.Mkfifo(src, 0o644); err != nil {
				t.Fatalf("FIFOを作成できません: %v", err)
			}

			err := runExpectError(t, dir, tt.code)
			if err == nil {
				t.Fatal("FIFOの処理がエラーになりませんでした")
			}
			if !strings.Contains(err.Error(), "移動できない形式のファイルです") {
				t.Fatalf("想定外のエラー: %v", err)
			}

			if _, err := os.Lstat(dest); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("コピー先が作成されています: %v", err)
			}

			// コピーでも移動でも、FIFO原本はそのまま残ること
			info, err := os.Lstat(src)
			if err != nil {
				t.Fatalf("FIFO原本が残っていません: %v", err)
			}
			if info.Mode()&os.ModeNamedPipe == 0 {
				t.Fatalf("原本がFIFOではありません: %v", info.Mode())
			}
		})
	}
}

// TestFileCopyRejectsSymlinkToFIFO は、FIFOへのシンボリックリンクを
// 単体指定した場合も、リンク先の種別で判定して明示エラーにすることを確認する。
// os.Stat がリンクを辿るため、リンク自体は通常ファイル扱いにしない。
func TestFileCopyRejectsSymlinkToFIFO(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "src.fifo")
	dest := filepath.Join(dir, "dest")

	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatalf("FIFOを作成できません: %v", err)
	}
	if err := os.Symlink("src.fifo", filepath.Join(dir, "src-link")); err != nil {
		t.Fatalf("FIFOへのリンクを作成できません: %v", err)
	}

	err := runExpectError(t, dir, "「src-link」を「dest」にファイルコピー")
	if err == nil {
		t.Fatal("FIFOへのリンクのコピーがエラーになりませんでした")
	}
	if !strings.Contains(err.Error(), "移動できない形式のファイルです") {
		t.Fatalf("想定外のエラー: %v", err)
	}
	if _, err := os.Lstat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("コピー先が作成されています: %v", err)
	}
}

// TestFileCopyRejectsDeviceFile は、FIFO以外の非通常ファイル（デバイス）も
// 同じく読み取り前に明示エラーにすることを確認する（#406）。
// /dev/null は読み取り自体は即座に終わるが、種別としては非通常ファイルであり、
// /dev/zero のようなデバイスは読み続けると終わらないため一律に弾く。
func TestFileCopyRejectsDeviceFile(t *testing.T) {
	if _, err := os.Lstat("/dev/null"); err != nil {
		t.Skipf("/dev/null がありません: %v", err)
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")

	err := runExpectError(t, dir, "「/dev/null」を「dest」にファイルコピー")
	if err == nil {
		t.Fatal("デバイスファイルのコピーがエラーになりませんでした")
	}
	if !strings.Contains(err.Error(), "移動できない形式のファイルです") {
		t.Fatalf("想定外のエラー: %v", err)
	}
	if _, err := os.Lstat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("コピー先が作成されています: %v", err)
	}
}

// TestFileCopySymlinkToRegularFileStillWorks は、通常ファイルへの
// シンボリックリンクは従来どおり内容がコピーできることを確認する（#406）。
func TestFileCopySymlinkToRegularFileStillWorks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "real.txt"), []byte("内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.txt", filepath.Join(dir, "link.txt")); err != nil {
		t.Fatalf("リンクを作成できません: %v", err)
	}

	runIn(t, dir, "「link.txt」を「dest.txt」にファイルコピー")

	data, err := os.ReadFile(filepath.Join(dir, "dest.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "内容" {
		t.Fatalf("コピー結果が不正です: %q", data)
	}
}
