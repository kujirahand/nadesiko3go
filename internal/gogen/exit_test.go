package gogen

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// buildAndRunWithNodelib は nodelib を含めて生成・ビルドし、標準出力・標準エラー出力と終了コードを返す。
func buildAndRunWithNodelib(t *testing.T, code string) (stdout, stderr string, exitCode int) {
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
	goMod := "module gogentest\n\ngo 1.23\n\n" +
		"require github.com/kujirahand/nadesiko3go v0.0.0\n\n" +
		"replace github.com/kujirahand/nadesiko3go => " + repoRoot(t) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	tidy.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s\n--- source ---\n%s", err, out, src)
	}
	binary := filepath.Join(dir, "exit-test")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s\n--- source ---\n%s", err, out, src)
	}
	var out, errOut bytes.Buffer
	run := exec.Command(binary)
	run.Stdout = &out
	run.Stderr = &errOut
	err = run.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		exitCode = 0
	case errors.As(err, &exitErr):
		exitCode = exitErr.ExitCode()
	default:
		t.Fatalf("生成コードの実行: %v", err)
	}
	return out.String(), errOut.String(), exitCode
}

// 『終了』『強制終了』が、VM版と生成コード版で同じ出力・同じ終了コードになる (#337)。
func TestGeneratedExitMatchesVM(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{"1_エラー監視内の終了", "エラー監視\n  終了\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n"},
		{"2_ユーザー定義関数内の終了", "●止める処理とは\n  終了\nここまで\nエラー監視\n  止める処理を実行\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n"},
		{"3_入れ子のエラー監視内の終了", "エラー監視\n  エラー監視\n    終了\n  エラーならば\n    「内側」を表示\n  ここまで\nエラーならば\n  「外側」を表示\nここまで\n「後続」を表示\n"},
		{"4_エラー監視内の強制終了", "エラー監視\n  5で強制終了\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n"},
		{"8_対照_通常のエラーは捕捉される", "エラー監視\n  「E」でエラー発生\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n"},
		{"関数内にエラー監視を持つ", "●Fとは\n  エラー監視\n    「前」を表示\n    終了\n  エラーならば\n    「捕捉された」を表示\n  ここまで\n  「関数の後続」を表示\nここまで\nFを実行\n「後続」を表示\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("go build/run per case; skipped under -short")
			}
			var want strings.Builder
			host := vm.NewCUIHost(&want, strings.NewReader(""), nil)
			if err := vm.RunProgram(tc.code, "main.nako3", host); err != nil {
				t.Fatalf("vm: %v", err)
			}
			got, gotStderr, gotCode := buildAndRunWithNodelib(t, tc.code)
			if got != want.String() {
				t.Errorf("出力が一致しない\nVM  : %q\ngogen: %q", want.String(), got)
			}
			// VMがエラーを返していないケースでは、生成バイナリも標準エラーへ何も出さない。
			if gotStderr != "" {
				t.Errorf("標準エラー出力が空でない: %q", gotStderr)
			}
			if gotCode != host.ExitCode {
				t.Errorf("終了コードが一致しない VM=%d gogen=%d", host.ExitCode, gotCode)
			}
		})
	}
}

// TestGeneratedBinaryExitCodeContract は #334 の受け入れ条件を、生成バイナリの
// プロセス終了コード・標準出力・標準エラー出力の固定値で直接確かめる。
// 生成 main の終了コード処理は #337 で cmd/gonako の finish と揃えたが、
// #337 の TestGeneratedExitMatchesVM はエラー監視絡みのケースだけだったので、
// ここでは監視なしの素のケースと、通常CUIの規約で導いたVM側の期待値との
// 突き合わせを併せて固定する。固定値でも見るのは、VMと生成側が同じ誤った
// 規約に揃ってしまった場合でも「7で強制終了はプロセスとして7」という
// 外部仕様を守るため。
// なお生成 Run が RealSleep を有効にしない点は #333 の範囲であり、待機の
// 実時間性は終了コードに影響しないので、ここでは触らない。
func TestGeneratedBinaryExitCodeContract(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		wantStdout string
		wantStderr string
		wantCode   int
	}{
		{"強制終了_素の7", "7で強制終了\n", "", "", 7},
		{"通常終了", "「OK」を表示\n", "OK\n", "", 0},
		{"終了_素", "「前」を表示\n終了\n「後続」を表示\n", "前\n", "", 0},
		{"強制終了_0", "0で強制終了\n", "", "", 0},
		{"実行時エラー_捕捉されない", "「E」でエラー発生\n", "", "[実行時エラー]main.nako3(1行目): E\n", 1},
		{"強制終了_秒後コールバック内", "0.01秒後には\n  7で強制終了\nここまで\n「main終わり」を表示\n", "main終わり\n", "", 7},
		{"強制終了_エラーを捕捉した後", "エラー監視\n  「E」でエラー発生\nエラーならば\n  「捕捉された」を表示\nここまで\n7で強制終了\n", "捕捉された\n", "", 7},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("go build/run per case; skipped under -short")
			}
			// VM側の期待値は通常CUIの finish の規約で導く: エラーがあれば
			// 診断を標準エラーへ出して1、なければ Host の明示終了コード
			// （非ゼロのときだけ）をプロセスの終了コードにする。
			var want strings.Builder
			host := vm.NewCUIHost(&want, strings.NewReader(""), nil)
			runErr := vm.RunProgram(tc.code, "main.nako3", host)
			wantCode, wantStderr := 0, ""
			if runErr != nil {
				wantCode, wantStderr = 1, runErr.Error()+"\n"
			} else if host.Exited && host.ExitCode != 0 {
				wantCode = host.ExitCode
			}
			if want.String() != tc.wantStdout || wantStderr != tc.wantStderr || wantCode != tc.wantCode {
				t.Fatalf("VM規約由来の期待値と固定値（外部仕様）が食い違う VM規約: stdout=%q stderr=%q code=%d / 固定値: stdout=%q stderr=%q code=%d",
					want.String(), wantStderr, wantCode, tc.wantStdout, tc.wantStderr, tc.wantCode)
			}
			got, gotStderr, gotCode := buildAndRunWithNodelib(t, tc.code)
			if got != tc.wantStdout {
				t.Errorf("標準出力が一致しない got %q want %q", got, tc.wantStdout)
			}
			if gotStderr != tc.wantStderr {
				t.Errorf("標準エラー出力が一致しない got %q want %q", gotStderr, tc.wantStderr)
			}
			if gotCode != tc.wantCode {
				t.Errorf("終了コードが一致しない got %d want %d", gotCode, tc.wantCode)
			}
		})
	}
}
