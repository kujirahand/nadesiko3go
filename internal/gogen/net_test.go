package gogen

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/parser"
)

// TestGeneratedAJAXFailureCallback は『AJAX失敗時』で登録したコールバックが
// gogen の生成コードでも呼ばれることを確かめる。以前は AJAX:ONERROR を
// SetSysVar で保存していたため、大域変数を持たないプログラムでは
// コールバックが黙って捨てられていた。
func TestGeneratedAJAXFailureCallback(t *testing.T) {
	if testing.Short() {
		t.Skip("go build/run; skipped under -short")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	code := fmt.Sprintf(`
●(テキストで)成功CB
　「成功: {テキスト}」と表示
ここまで
●(メッセージで)失敗CB
　「失敗: {メッセージ}」と表示
ここまで
「失敗CB」のAJAX失敗時
URL = "http://%s"
URLへ「成功CB」でAJAX送信時
`, addr)

	registry, err := BuildRegistry([]string{"nodelib"})
	if err != nil {
		t.Fatal(err)
	}
	tree, err := parser.ParseSource(code, "main.nako3", registry.FuncList())
	if err != nil {
		t.Fatal(err)
	}
	prog, err := compiler.Compile(tree, "main.nako3", registry)
	if err != nil {
		t.Fatal(err)
	}
	src, err := Generate(prog, Options{Plugins: []string{"nodelib"}})
	if err != nil {
		t.Fatal(err)
	}

	dir := sharedModule(t)
	got, runErr := runInModule(t, dir, src)
	if runErr != nil {
		t.Fatalf("生成コードの実行: %v\n%s", runErr, got)
	}
	if !strings.HasPrefix(got, "失敗: ") {
		t.Fatalf("エラーコールバックが呼ばれていない: %q", got)
	}
}
