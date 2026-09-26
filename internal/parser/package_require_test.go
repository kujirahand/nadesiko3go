package parser

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

func TestRequirePackageSearchOrder(t *testing.T) {
	root := t.TempDir()
	envDir := filepath.Join(root, "env")
	runtimeDir := filepath.Join(root, "runtime")
	name := "demo.nako3"
	write := func(dir string) string {
		t.Helper()
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("「読込」と表示"), 0644); err != nil {
			t.Fatal(err)
		}
		return file
	}
	envFile := write(envDir)
	runtimeFile := write(filepath.Join(runtimeDir, "gonako-package"))
	parentFile := write(filepath.Join(root, "gonako-package"))
	oldExe, oldFS := runtimeExecutable, packageFiles
	t.Cleanup(func() { runtimeExecutable, packageFiles = oldExe, oldFS })
	runtimeExecutable = func() (string, error) { return filepath.Join(runtimeDir, "gonako"), nil }
	packageFiles = fstest.MapFS{"gonako-package/" + name: &fstest.MapFile{Data: []byte("「埋込」と表示")}}
	t.Setenv("GONAKO_PACKAGE_PATH", envDir)
	check := func(want string) {
		t.Helper()
		got, err := resolveRequirePath(name, filepath.Join(root, "main.nako3"), lexer.Token{})
		if err != nil || got != want {
			t.Fatalf("解決結果=%q, %v; 期待=%q", got, err, want)
		}
	}
	check(envFile)
	if got, err := resolveRequirePath("demo", "main.nako3", lexer.Token{}); err != nil || got != envFile {
		t.Fatalf("省略形: %q, %v", got, err)
	}
	if err := os.Remove(envFile); err != nil {
		t.Fatal(err)
	}
	check(runtimeFile)
	if err := os.Remove(runtimeFile); err != nil {
		t.Fatal(err)
	}
	check(parentFile)
	if err := os.Remove(parentFile); err != nil {
		t.Fatal(err)
	}
	check("embed:gonako-package/" + name)
	// 埋め込みの実読込と、兄弟ファイルの相対取込・重複ガードも確認する。
	packageFiles = fstest.MapFS{
		"gonako-package/demo.nako3":       &fstest.MapFile{Data: []byte("!「./demo/child.nako3」を取り込む。\n!「./demo/child.nako3」を取り込む。")},
		"gonako-package/demo/child.nako3": &fstest.MapFile{Data: []byte("「埋込」と表示")},
	}
	if _, err := ParseSource("!「demo.nako3」を取り込む。", "main.nako3", requireTestFuncs()); err != nil {
		t.Fatal(err)
	}
}

func TestRequireExplicitLocalPath(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "lib.nako3")
	if err := os.WriteFile(file, nil, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GONAKO_PACKAGE_PATH", "")
	for _, name := range []string{"./lib.nako3", "../lib.nako3", file} {
		from := filepath.Join(root, "main.nako3")
		if strings.HasPrefix(name, "../") {
			from = filepath.Join(root, "child", "main.nako3")
		}
		got, err := resolveRequirePath(name, from, lexer.Token{})
		if err != nil || got != file {
			t.Fatalf("%s: %q, %v", name, got, err)
		}
	}
	if _, err := resolveRequirePath("lib.nako3", filepath.Join(root, "main.nako3"), lexer.Token{}); err == nil {
		t.Fatal("裸のパスでローカルファイルを取り込みました")
	}
}

func TestRequireURLAndRelativeImport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.nako3":
			fmt.Fprint(w, "!「./child.nako3」を取り込む。")
		case "/child.nako3":
			fmt.Fprint(w, "「URL」と表示")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	if _, err := ParseSource("!「"+server.URL+"/index.nako3」を取り込む。", "main.nako3", requireTestFuncs()); err != nil {
		t.Fatal(err)
	}
	_, err := loadRequireFile(server.URL+"/missing.nako3", lexer.Token{})
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("404エラー=%v", err)
	}
	for _, name := range []string{server.URL + "/file.txt", "https:///lib.nako3"} {
		if _, err := resolveRequirePath(name, "", lexer.Token{}); err == nil {
			t.Fatalf("不正URL: %s", name)
		}
	}
	got, err := resolveRequirePath("貯蔵庫：demo.nako3", "", lexer.Token{})
	if err != nil || got != "https://n3s.nadesi.com/plain/demo.nako3" {
		t.Fatalf("貯蔵庫: %q %v", got, err)
	}
}

func requireTestFuncs() lexer.FuncList {
	return lexer.FuncList{
		"表示":       {Name: "表示", Type: "func", Josi: [][]string{{"と"}}},
		"名前空間設定":   {Name: "名前空間設定", Type: "func", Josi: [][]string{{"に", "へ"}}},
		"プラグイン名設定": {Name: "プラグイン名設定", Type: "func", Josi: [][]string{{"に", "へ"}}},
		"名前空間ポップ":  {Name: "名前空間ポップ", Type: "func"},
	}
}
