package gogen

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/parser"
)

// buildSleepBinary は code を main パッケージで生成し、ビルド済みバイナリの
// パスを返す。実行時間の計測に go run を使うとコンパイル時間が混ざるので、
// 必ず先にビルドしてから計測する。レジストリは『強制終了』(nodelib) も
// 使えるよう buildAndRunWithNodelib と同じ構成にする。
func buildSleepBinary(t *testing.T, code string) string {
	t.Helper()
	registry, err := BuildRegistry([]string{"nodelib"})
	if err != nil {
		t.Fatal(err)
	}
	tree, err := parser.ParseSource(code, "main.nako3", registry.FuncList())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	prog, err := compiler.Compile(tree, "main.nako3", registry)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	src, err := Generate(prog, Options{Plugins: []string{"nodelib"}})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	writeGoMod(t, dir)
	tidyModule(t, dir, src)
	binary := filepath.Join(dir, "sleep-test")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s\n--- source ---\n%s", err, out, src)
	}
	return binary
}

// TestGenerateRealSleepScope は、RealSleep の有効化が生成 main だけに
// 閉じていることを生成ソースで固定する (#333)。ライブラリ生成には
// RealSleep を有効にする代入が現れず（Run の説明コメントで有効化の
// 方法に触れるのはよい）、既定の Run はどちらのパッケージでも
// DefaultOptions（仮想時間）のままであること。有効化の代入は行単位で
// 照合する: 生成ソースは Generate 自身が go/format で整形済みなので、
// インデント込みの行一致は安定した前提にできる。
func TestGenerateRealSleepScope(t *testing.T) {
	code := "0.2秒待\n「OK」を表示\n"

	mainSrc := string(generateForPackage(t, code, "main"))
	for _, want := range []string{
		"func Run(host rt.Host) error",
		"return RunWithOptions(host, rt.DefaultOptions())",
		"func RunWithOptions(host rt.Host, opts rt.Options) error",
		"\topts.RealSleep = true\n",
		"RunWithOptions(host, opts)",
	} {
		if !strings.Contains(mainSrc, want) {
			t.Errorf("main パッケージの生成コードに %q がない\n--- source ---\n%s", want, mainSrc)
		}
	}

	libSrc := string(generateForPackage(t, code, "nakolib"))
	if strings.Contains(libSrc, "RealSleep = true") {
		t.Errorf("ライブラリ生成に RealSleep の有効化が波及している\n--- source ---\n%s", libSrc)
	}
	if strings.Contains(libSrc, "func main()") {
		t.Errorf("ライブラリ生成に func main がある\n--- source ---\n%s", libSrc)
	}
	for _, want := range []string{
		"func Run(host rt.Host) error",
		"return RunWithOptions(host, rt.DefaultOptions())",
		"func RunWithOptions(host rt.Host, opts rt.Options) error",
	} {
		if !strings.Contains(libSrc, want) {
			t.Errorf("ライブラリの生成コードに %q がない\n--- source ---\n%s", want, libSrc)
		}
	}
}

// TestGeneratedBinaryRealSleepWaits は #333 の受け入れ条件を、生成バイナリの
// 実経過時間と出力で直接確かめる。閾値は下限のみ: 0.2秒の待機に対し
// 150ms とし、仮想時間だった頃の約10ms とは十分に離しつつ、CI の
// タイマー粒度や負荷で割らない余裕を持たせる。上限は置かない
// （負荷で遅くなることはあっても、速すぎることだけが不具合である）。
func TestGeneratedBinaryRealSleepWaits(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{"秒待の後に表示", "「前」を表示\n0.2秒待\n「OK」を表示\n", "前\nOK\n"},
		{"秒後のコールバック順序", "0.05秒後には\n「後」を表示\nここまで\n「先」を表示\n0.2秒待\n", "先\n後\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("go build/run per case; skipped under -short")
			}
			binary := buildSleepBinary(t, tc.code)
			started := time.Now()
			out, err := exec.Command(binary).Output()
			elapsed := time.Since(started)
			if err != nil {
				t.Fatalf("生成バイナリの実行: %v", err)
			}
			if string(out) != tc.want {
				t.Errorf("出力が違う got %q want %q", out, tc.want)
			}
			if elapsed < 150*time.Millisecond {
				t.Errorf("実経過時間 = %v。0.2秒待が実時間で待っていない（仮想時間のまま）", elapsed)
			}
		})
	}
}

// TestGeneratedBinaryRealSleepExitCode は、実時間待機の後でも生成 main の
// 終了コード処理 (#334) が保たれることを確かめる。Run から
// RunWithOptions への委譲で、待機と『強制終了』の組み合わせが壊れて
// いないことの固定である。
func TestGeneratedBinaryRealSleepExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("go build/run; skipped under -short")
	}
	binary := buildSleepBinary(t, "0.2秒待\n「OK」を表示\n7で強制終了\n")
	var out, errOut bytes.Buffer
	run := exec.Command(binary)
	run.Stdout = &out
	run.Stderr = &errOut
	started := time.Now()
	err := run.Run()
	elapsed := time.Since(started)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("生成バイナリの実行: err = %v, want 終了コード7の ExitError (stderr=%q)", err, errOut.String())
	}
	if exitErr.ExitCode() != 7 {
		t.Errorf("終了コード = %d, want 7 (stderr=%q)", exitErr.ExitCode(), errOut.String())
	}
	if out.String() != "OK\n" {
		t.Errorf("出力 = %q, want %q", out.String(), "OK\n")
	}
	if errOut.String() != "" {
		t.Errorf("標準エラー出力 = %q, want 空", errOut.String())
	}
	if elapsed < 150*time.Millisecond {
		t.Errorf("実経過時間 = %v。0.2秒待が実時間で待っていない", elapsed)
	}
}

// TestGeneratedLibrarySleepOptions は、ライブラリ生成の既定 Run が
// 仮想時間のまま高速に終わり、利用側が RunWithOptions で RealSleep を
// 明示的に選んだときだけ実時間で待つことを固定する (#333)。
// 計測は呼び出し側プロセスの中で Run の前後を測るので、プロセス起動や
// ビルドの時間は混ざらない。
func TestGeneratedLibrarySleepOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("go build/run; skipped under -short")
	}
	src := generateForPackage(t, "0.5秒待\n「OK」を表示\n", "nakolib")

	dir := t.TempDir()
	libDir := filepath.Join(dir, "nakolib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "nako.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	caller := `package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"gogentest/nakolib"

	rt "github.com/kujirahand/nadesiko3go/pkg/runtime"
)

func runOnce(realSleep bool) (string, time.Duration, error) {
	var buf bytes.Buffer
	host := rt.NewCUIHost(&buf, strings.NewReader(""), nil)
	started := time.Now()
	var err error
	if realSleep {
		opts := rt.DefaultOptions()
		opts.RealSleep = true
		err = nakolib.RunWithOptions(host, opts)
	} else {
		err = nakolib.Run(host)
	}
	return buf.String(), time.Since(started), err
}

func main() {
	out1, d1, err := runOnce(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out2, d2, err := runOnce(true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("virtual=%d out=%q\n", d1.Milliseconds(), out1)
	fmt.Printf("real=%d out=%q\n", d2.Milliseconds(), out2)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(caller), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGoMod(t, dir)
	tidyModule(t, dir, src)

	binary := filepath.Join(dir, "lib-test")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s\n--- source ---\n%s", err, out, src)
	}
	out, err := exec.Command(binary).Output()
	if err != nil {
		t.Fatalf("呼び出し側の実行: %v\n%s", err, out)
	}
	got := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		key, rest, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("想定外の出力行: %q", line)
		}
		msText, _, _ := strings.Cut(rest, " ")
		ms, err := strconv.Atoi(msText)
		if err != nil {
			t.Fatalf("経過時間を読めない行: %q", line)
		}
		got[key] = ms
		if !strings.Contains(line, `out="OK\n"`) {
			t.Errorf("出力が OK でない行: %q", line)
		}
	}
	// 0.5秒待に対し、既定（仮想時間）は半分未満で終わり、RealSleep は
	// 8割以上かかること。どちらも境界から十分に離してフレークを避ける。
	if ms, ok := got["virtual"]; !ok || ms >= 250 {
		t.Errorf("既定 Run の経過時間 = %dms。仮想時間で高速なはず", ms)
	}
	if ms, ok := got["real"]; !ok || ms < 400 {
		t.Errorf("RunWithOptions(RealSleep) の経過時間 = %dms。実時間で待つはず", ms)
	}
}
