package bundle_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// TestA02BuildRoundTripSizes は 8/32/64MiB のダミーランタイムと
// 非圧縮リソースを梱包し、読み戻しが一致することと、出力が入力を
// 大きく上回らないことを固定する (A02 の受け入れ条件)。
func TestA02BuildRoundTripSizes(t *testing.T) {
	if testing.Short() {
		t.Skip("大容量の梱包は -short では省略する")
	}
	for _, mib := range []int{8, 32, 64} {
		mib := mib
		t.Run(fmt.Sprintf("%dMiB", mib), func(t *testing.T) {
			dir := t.TempDir()
			size := int64(mib) << 20

			runtimePath := filepath.Join(dir, "runtime")
			a02WriteZeros(t, runtimePath, size)

			resources := filepath.Join(dir, "res")
			if err := os.Mkdir(resources, 0o755); err != nil {
				t.Fatal(err)
			}
			resourcePath := filepath.Join(resources, "payload.bin")
			wantHash := a02WriteRandom(t, resourcePath, size)

			prog, err := vm.CompileProgram("1を表示", "main.nako3")
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(dir, "app")
			if err := bundle.BuildSpec(out, runtimePath, bundle.Spec{
				Kind:        bundle.KindProgram,
				Program:     prog,
				Name:        "main.nako3",
				ResourceDir: resources,
				Flat:        true,
			}); err != nil {
				t.Fatal(err)
			}

			// 入力の合計にフッタとZIPの目録が乗るだけ。二重に積んでいたら
			// ここで大きく超える。
			info, err := os.Stat(out)
			if err != nil {
				t.Fatal(err)
			}
			if got, limit := info.Size(), 2*size+(1<<20); got > limit {
				t.Errorf("出力サイズ = %d, want <= %d (入力の合計 %d を大きく上回っています)", got, limit, 2*size)
			}

			packed, err := bundle.Open(out)
			if err != nil {
				t.Fatal(err)
			}
			defer packed.Close()
			data, ok := packed.ReadResource("payload.bin")
			if !ok {
				t.Fatal("同梱したリソースを読めません")
			}
			if sha256.Sum256(data) != wantHash {
				t.Error("読み戻したリソースが梱包前と一致しません")
			}
			if packed.Program == nil || packed.Name != "main.nako3" {
				t.Errorf("読み戻したプログラムが不正です: name=%q program=%v", packed.Name, packed.Program != nil)
			}
			if got := packed.Resources(); len(got) != 1 || got[0] != "payload.bin" {
				t.Errorf("同梱リソース = %v, want [payload.bin]", got)
			}
		})
	}
}

// TestA02MidStreamFailurePreservesExistingOutput は、ペイロードの書き出しが
// 途中まで進んだ後に失敗しても、既存の出力と一時ファイルの後始末が
// 保たれることを固定する。
func TestA02MidStreamFailurePreservesExistingOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windowsではシンボリックリンクの作成に権限が要る")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "app")
	old := []byte("PRE-EXISTING-BUILD-OUTPUT")
	if err := os.WriteFile(out, old, 0o755); err != nil {
		t.Fatal(err)
	}
	resources := filepath.Join(dir, "res")
	if err := os.Mkdir(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	// 辞書順で先のファイルは書き終わり、後ろの壊れたリンクで失敗する。
	if err := os.WriteFile(filepath.Join(resources, "a.bin"), []byte("先に書かれる中身"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing"), filepath.Join(resources, "z-broken")); err != nil {
		t.Fatal(err)
	}

	err := bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{
		Kind: bundle.KindHTML, Entry: "index.html", ResourceDir: resources,
	})
	if err == nil {
		t.Fatal("壊れたリンクを梱包してしまいました")
	}
	data, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(data, old) {
		t.Fatalf("途中失敗で既存の出力が変わりました: %q", data)
	}
	assertNoBuildTemps(t, dir)
}

// TestA02BuildTempIsNotPacked は、出力先がリソースフォルダ内にあっても
// 作成中の一時ファイルが梱包されないことを固定する。逐次書き出しでは
// ペイロードを作る時点で一時ファイルが存在するため。
func TestA02BuildTempIsNotPacked(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "app")
	rt := fakeRuntime(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("中身"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := bundle.BuildSpec(out, rt, bundle.Spec{
		Kind:        bundle.KindHTML,
		Entry:       "index.html",
		ResourceDir: dir,
		Flat:        true,
		Skip:        map[string]bool{out: true, rt: true},
	}); err != nil {
		t.Fatal(err)
	}
	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()
	got := packed.Resources()
	if len(got) != 1 || got[0] != "index.html" {
		t.Fatalf("同梱リソース = %v, want [index.html]", got)
	}
	for _, name := range got {
		if strings.HasPrefix(name, ".gonako-build-") {
			t.Errorf("一時ファイルが梱包されています: %s", name)
		}
	}
}

// a02WriteZeros は指定サイズのダミーランタイムを書く。
func a02WriteZeros(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	chunk := make([]byte, 1<<20)
	for written := int64(0); written < size; written += int64(len(chunk)) {
		n := int64(len(chunk))
		if remaining := size - written; remaining < n {
			n = remaining
		}
		if _, err := f.Write(chunk[:n]); err != nil {
			t.Fatal(err)
		}
	}
}

// a02WriteRandom は圧縮の効かないリソースを書き、その SHA-256 を返す。
func a02WriteRandom(t *testing.T, path string, size int64) [32]byte {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	hash := sha256.New()
	source := rand.New(rand.NewSource(1))
	chunk := make([]byte, 1<<20)
	for written := int64(0); written < size; written += int64(len(chunk)) {
		n := int64(len(chunk))
		if remaining := size - written; remaining < n {
			n = remaining
		}
		if _, err := source.Read(chunk[:n]); err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(chunk[:n]); err != nil {
			t.Fatal(err)
		}
		hash.Write(chunk[:n])
	}
	var sum [32]byte
	copy(sum[:], hash.Sum(nil))
	return sum
}
