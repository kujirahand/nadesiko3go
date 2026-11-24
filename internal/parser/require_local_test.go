package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

// 取り込みテスト用の環境を作る。パッケージ探索は埋め込みFSだけにして、
// 実行ファイルの隣や環境変数の影響を受けないようにする。
func setupLocalRequireTest(t *testing.T, embedded fstest.MapFS) {
	t.Helper()
	oldExe, oldFS := runtimeExecutable, packageFiles
	t.Cleanup(func() { runtimeExecutable, packageFiles = oldExe, oldFS })
	runtimeExecutable = func() (string, error) { return filepath.Join(t.TempDir(), "gonako"), nil }
	packageFiles = embedded
	t.Setenv("GONAKO_PACKAGE_PATH", "")
}

func writeLocalFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return full
}

// 接頭辞のない名前は、取り込み元のフォルダを先に探す（#542）。
func TestRequireBareNameResolvesFromSourceDir(t *testing.T) {
	setupLocalRequireTest(t, fstest.MapFS{})
	dir := t.TempDir()
	sub := writeLocalFile(t, dir, "sub.nako3", "「子」と表示")
	got, err := resolveRequirePath("sub.nako3", filepath.Join(dir, "main.nako3"), lexer.Token{})
	if err != nil || got != sub {
		t.Fatalf("解決結果=%q, %v; 期待=%q", got, err, sub)
	}
	// 拡張子を省略した名前も同じ。
	got, err = resolveRequirePath("sub", filepath.Join(dir, "main.nako3"), lexer.Token{})
	if err != nil || got != sub {
		t.Fatalf("拡張子省略: %q, %v", got, err)
	}
	// 実際に取り込んで構文解析まで通る。
	main := writeLocalFile(t, dir, "main.nako3", "!「sub.nako3」を取り込む。\n")
	if _, err := ParseSource("!「sub.nako3」を取り込む。\n", main, requireTestFuncs()); err != nil {
		t.Fatal(err)
	}
}

// 取り込み元のフォルダに無ければ、従来どおりパッケージを探す。
func TestRequireBareNameFallsBackToPackage(t *testing.T) {
	setupLocalRequireTest(t, fstest.MapFS{"pkg.nako3": &fstest.MapFile{Data: []byte("「埋込」と表示")}})
	dir := t.TempDir()
	got, err := resolveRequirePath("pkg.nako3", filepath.Join(dir, "main.nako3"), lexer.Token{})
	if err != nil || got != "embed:gonako-package/pkg.nako3" {
		t.Fatalf("解決結果=%q, %v", got, err)
	}
}

// 手元のファイルが同名のパッケージを隠すときは、手元を使って警告する。
func TestRequireLocalShadowsPackageWithWarning(t *testing.T) {
	setupLocalRequireTest(t, fstest.MapFS{"dup.nako3": &fstest.MapFile{Data: []byte("「埋込」と表示")}})
	dir := t.TempDir()
	local := writeLocalFile(t, dir, "dup.nako3", "「手元」と表示")
	var warns []string
	got, err := resolveRequirePathWarn("dup.nako3", filepath.Join(dir, "main.nako3"), lexer.Token{}, &warns)
	if err != nil || got != local {
		t.Fatalf("解決結果=%q, %v", got, err)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "dup.nako3") || !strings.Contains(warns[0], "パッケージ") {
		t.Fatalf("警告 = %q", warns)
	}
	// 同名のパッケージが無ければ警告しない。
	warns = nil
	writeLocalFile(t, dir, "only.nako3", "")
	if _, err := resolveRequirePathWarn("only.nako3", filepath.Join(dir, "main.nako3"), lexer.Token{}, &warns); err != nil || len(warns) != 0 {
		t.Fatalf("不要な警告: %q, %v", warns, err)
	}
}

// 見つからないときは、探した場所と、よくある間違いの案内を添える。
func TestRequireNotFoundMessageIsHelpful(t *testing.T) {
	setupLocalRequireTest(t, fstest.MapFS{"lib.nako3": &fstest.MapFile{Data: []byte("")}})
	dir := t.TempDir()
	from := filepath.Join(dir, "main.nako3")
	writeLocalFile(t, dir, "Case.nako3", "")
	writeLocalFile(t, dir, "ext.nako", "")

	cases := []struct {
		name  string
		wants []string
	}{
		{"none.nako3", []string{"見つかりません", "取り込み元ファイルと同じフォルダ", "埋め込みパッケージ"}},
		{"case.nako3", []string{"大文字・小文字が違う『Case.nako3』"}},
		{"ext.nako3", []string{"拡張子が違う『ext.nako』"}},
		{"./lib.nako3", []string{"見つかりません", "./ を付けずに"}},
	}
	for _, c := range cases {
		// 大文字・小文字を区別しないファイルシステム（macOS・Windows）では、
		// 大文字小文字違いの名前でも見つかるため、エラーのケースを飛ばす。
		if _, statErr := os.Stat(filepath.Join(dir, c.name)); statErr == nil {
			continue
		}
		_, err := resolveRequirePath(c.name, from, lexer.Token{})
		if err == nil {
			t.Fatalf("%s: エラーになりません", c.name)
		}
		for _, w := range c.wants {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: エラー %q に %q がありません", c.name, err.Error(), w)
			}
		}
		// プロジェクトの絶対パスは表示に出さない。
		if strings.Contains(err.Error(), dir) {
			t.Errorf("%s: エラーに絶対パスが含まれています: %q", c.name, err.Error())
		}
	}
}

// 保存前のプログラムは、フォルダを基準にする取り込みを保存の案内つきで断る。
func TestRequireUnsavedSource(t *testing.T) {
	setupLocalRequireTest(t, fstest.MapFS{"pkg.nako3": &fstest.MapFile{Data: []byte("")}})
	for _, name := range []string{"sub.nako3", "./sub.nako3", "../sub.nako3"} {
		_, err := resolveRequirePath(name, UnsavedSourcePath, lexer.Token{})
		if err == nil || !strings.Contains(err.Error(), "保存") {
			t.Errorf("%s: 保存を促すエラーになりません: %v", name, err)
		}
	}
	// 保存の有無で結果が変わらないものは通す。
	for _, name := range []string{"pkg.nako3", "貯蔵庫:demo.nako3", "https://example.com/a.nako3"} {
		if got, err := resolveRequirePath(name, UnsavedSourcePath, lexer.Token{}); err != nil {
			t.Errorf("%s: 保存前でも通るはずが %q, %v", name, got, err)
		}
	}
	// ParseSourceFile へ目印を渡した場合も同じ。
	_, err := ParseSourceFile("!「sub.nako3」を取り込む。\n", "gui.nako3", UnsavedSourcePath, lexer.FuncList{})
	if err == nil || !strings.Contains(err.Error(), "保存") {
		t.Fatalf("ParseSourceFile: %v", err)
	}
}

func TestCheckUnsavedRequires(t *testing.T) {
	setupLocalRequireTest(t, fstest.MapFS{"pkg.nako3": &fstest.MapFile{Data: []byte("")}})
	funcs := requireTestFuncs()
	if err := CheckUnsavedRequires("「あ」を表示\n", funcs); err != nil {
		t.Fatalf("取り込みなし: %v", err)
	}
	if err := CheckUnsavedRequires("!「pkg.nako3」を取り込む。\n", funcs); err != nil {
		t.Fatalf("パッケージ: %v", err)
	}
	if err := CheckUnsavedRequires("!「sub.nako3」を取り込む。\n", funcs); err == nil || !strings.Contains(err.Error(), "保存") {
		t.Fatalf("ローカル取り込み: %v", err)
	}
	if err := CheckUnsavedRequires("!「./sub.nako3」を取り込む。\n", funcs); err == nil {
		t.Fatal("相対パスの取り込みがエラーになりません")
	}
}
