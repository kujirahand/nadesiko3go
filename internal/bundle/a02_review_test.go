package bundle_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// TestBuildInPlaceRebuild は、出力先がランタイム自身である再梱包が成立する
// ことを固定する。Windows は開いたままのファイルを os.Rename で置換できない
// ため、ランタイムの読み込み後に閉じる順序の回帰テストにもなる (A02 の
// レビュー指摘)。
func TestBuildInPlaceRebuild(t *testing.T) {
	dir := t.TempDir()
	resources := filepath.Join(dir, "res")
	if err := os.Mkdir(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("再梱包でも読めるリソース")
	if err := os.WriteFile(filepath.Join(resources, "payload.bin"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}

	// app は最初はただのランタイム。同じパスを出力先に指定して再梱包する。
	app := fakeRuntime(t, dir)
	spec := bundle.Spec{
		Kind:        bundle.KindProgram,
		Program:     prog,
		Name:        "main.nako3",
		ResourceDir: resources,
		Flat:        true,
	}
	if err := bundle.BuildSpec(app, app, spec); err != nil {
		t.Fatalf("同じパスへの再梱包に失敗しました: %v", err)
	}

	packed, err := bundle.Open(app)
	if err != nil {
		t.Fatal(err)
	}
	data, ok := packed.ReadResource("payload.bin")
	if !ok {
		packed.Close()
		t.Fatal("同梱したリソースを読めません")
	}
	if sha256.Sum256(data) != sha256.Sum256(payload) {
		packed.Close()
		t.Error("読み戻したリソースが梱包前と一致しません")
	}
	if packed.Program == nil || packed.Name != "main.nako3" {
		packed.Close()
		t.Errorf("読み戻したプログラムが不正です: name=%q program=%v", packed.Name, packed.Program != nil)
	}
	if err := packed.Close(); err != nil {
		t.Fatal(err)
	}

	// 二度目も同じパスへ。既存ペイロードを切り詰めて積み上げない。
	first, err := os.Stat(app)
	if err != nil {
		t.Fatal(err)
	}
	if err := bundle.BuildSpec(app, app, spec); err != nil {
		t.Fatalf("二度目の再梱包に失敗しました: %v", err)
	}
	second, err := os.Stat(app)
	if err != nil {
		t.Fatal(err)
	}
	if first.Size() != second.Size() {
		t.Errorf("二度目のサイズ = %d, want %d (ペイロードが積み上がっています)", second.Size(), first.Size())
	}
	assertNoBuildTemps(t, dir)
}

// TestCorruptFooterPayloadLengthIsIgnored は、末尾のフッタが int64 の範囲を
// 超える長さを主張していても、負数や実ファイルを超える長さとして扱わず、
// ペイロードなしのランタイムとして梱包できることを固定する (A02 の
// レビュー指摘)。修正前は負の長さが巨大な正数になり、読み込みが EOF で
// 失敗していた。
func TestCorruptFooterPayloadLengthIsIgnored(t *testing.T) {
	for _, length := range []uint64{1 << 63, 1<<63 + 1, ^uint64(0)} {
		t.Run(fmt.Sprintf("%#x", length), func(t *testing.T) {
			dir := t.TempDir()
			prog, err := vm.CompileProgram("1を表示", "main.nako3")
			if err != nil {
				t.Fatal(err)
			}

			// 正常なバンドルを作り、末尾のペイロード長だけを壊して
			// 「壊れたランタイム」として使う。
			base := filepath.Join(dir, "base")
			if err := bundle.Build(base, fakeRuntime(t, dir), prog, "main.nako3", ""); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(base)
			if err != nil {
				t.Fatal(err)
			}
			binary.BigEndian.PutUint64(data[len(data)-8:], length)
			corrupt := filepath.Join(dir, "corrupt-runtime")
			if err := os.WriteFile(corrupt, data, 0o755); err != nil {
				t.Fatal(err)
			}

			out := filepath.Join(dir, "app")
			if err := bundle.Build(out, corrupt, prog, "main.nako3", ""); err != nil {
				t.Fatalf("壊れた長さのランタイムを扱えませんでした: %v", err)
			}

			// 壊れたフッタは無視し、ランタイム全体をそのまま書き出す。
			// 長さを負数として扱うと CopyN が 0 バイトになり、ランタイムが
			// 黙って欠落する (修正前の挙動)。
			got, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(got, data) {
				t.Errorf("壊れたランタイムのバイト列がそのまま残っていません")
			}
			// 出力は実ファイル + 小さなペイロード + フッタに収まる。
			if limit := int64(len(data)) + 1<<20; int64(len(got)) > limit {
				t.Errorf("出力サイズ = %d, want <= %d (壊れた長さをそのまま使っています)", len(got), limit)
			}

			packed, err := bundle.Open(out)
			if err != nil {
				t.Fatalf("梱包した出力を読み戻せません: %v", err)
			}
			defer packed.Close()
			if packed.Program == nil {
				t.Error("読み戻したプログラムがありません")
			}
		})
	}
}
