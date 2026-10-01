package gogen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/parser"
)

func TestGeneratedExternalEventUpdatesGlobal(t *testing.T) {
	registry, err := BuildRegistry([]string{"nodelib"})
	if err != nil {
		t.Fatal(err)
	}
	code := `完了=0
回数=0
●中断処理
  完了=1
  偽で戻る
ここまで
「中断処理」を強制終了時
(完了<1)の間
  回数=回数+1
ここまで
完了を表示`
	tree, err := parser.ParseSource(code, "external.nako3", registry.FuncList())
	if err != nil {
		t.Fatal(err)
	}
	prog, err := compiler.Compile(tree, "external.nako3", registry)
	if err != nil {
		t.Fatal(err)
	}
	src, err := Generate(prog, Options{Plugins: []string{"nodelib"}})
	if err != nil {
		t.Fatal(err)
	}
	// 受信タイミングを固定し、Goの変数へ昇格した古い値をループが読み続けないことを確認する。
	modified := strings.Replace(string(src), "\tregisterNatives(m)\n", `
	registerNatives(m)
	m.PostExternalEvent(func() bool {
		return rt.ToNumber(m.FindValue("回数")) >= 3
	}, func() error {
		_, err := m.CallFunc(m.FindFunc("中断処理"), nil)
		return err
	}, nil)
`, 1)
	file := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(file, []byte(modified), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	binary := filepath.Join(filepath.Dir(file), "external-test")
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, file)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("生成コードのビルド: %v\n%s", err, out)
	}
	out, err = exec.CommandContext(ctx, binary).CombinedOutput()
	if err != nil {
		t.Fatalf("生成コードの実行: %v\n%s", err, out)
	}
	if string(out) != "1\n" {
		t.Fatalf("生成コードの結果: %q", out)
	}
}
