package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
)

// #351 の再現構成を作る。プロジェクト側は main → helper → sub/leaf の
// 多段取り込みで、cwd 側には同じ相対構成の decoy（別の出力）を置く。
// cwd 基準で解決すると、ビルドは通っても decoy の内容が使われる。
func writeIncludeProject(t *testing.T, project, outside string) {
	t.Helper()
	files := map[string]struct{ body string }{
		"main":      {"!「./helper.nako3」を取り込む\n補助実行\n"},
		"helper":    {"!「./sub/leaf.nako3」を取り込む\n●補助実行とは\n    葉実行\nここまで\n"},
		"leaf":      {"●葉実行とは\n    「正しい葉」を表示\nここまで\n"},
		"decoyHelp": {"!「./sub/leaf.nako3」を取り込む\n●補助実行とは\n    葉実行\nここまで\n"},
		"decoyLeaf": {"●葉実行とは\n    「誤った葉」を表示\nここまで\n"},
	}
	paths := map[string]string{
		"main":      filepath.Join(project, "main.nako3"),
		"helper":    filepath.Join(project, "helper.nako3"),
		"leaf":      filepath.Join(project, "sub", "leaf.nako3"),
		"decoyHelp": filepath.Join(outside, "helper.nako3"),
		"decoyLeaf": filepath.Join(outside, "sub", "leaf.nako3"),
	}
	for key, f := range files {
		writeFile(t, paths[key], f.body)
	}
}

// chdirOutside はテスト中だけ作業フォルダをプロジェクト外へ移す。
// os.Chdir はプロセス全体に効くため、このパッケージのテストでは
// t.Parallel を併用しないこと。
func chdirOutside(t *testing.T, outside string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func assertCwd(t *testing.T, want string) {
	t.Helper()
	got, err := os.Getwd()
	if err != nil || got != want {
		t.Fatalf("作業フォルダが変わった: %q, %v (want %q)", got, err, want)
	}
}

// #351: 「Go言語でビルド」は、プロジェクト外が作業フォルダでも元
// ファイルの隣から相対 include を解決すること。
func TestBuildWithGoResolvesRelativeIncludeFromSourcePath(t *testing.T) {
	if testing.Short() {
		t.Skip("go mod tidy/go build; skipped under -short")
	}
	root := repoRoot(t)
	restore := swapSourceDir(t, func() (string, error) { return root, nil })
	defer restore()

	called := false
	origDownload := downloadSourceFunc
	downloadSourceFunc = func(string) error { called = true; return nil }
	defer func() { downloadSourceFunc = origDownload }()

	base := t.TempDir()
	project := filepath.Join(base, "project")
	outside := filepath.Join(base, "outside")
	writeIncludeProject(t, project, outside)
	chdirOutside(t, outside)

	out := filepath.Join(base, "built-app")
	res := buildWithGo(filepath.Join(project, "main.nako3"), out)
	if called {
		t.Fatal("既存のチェックアウトがあるのにダウンロード関数が呼ばれた")
	}
	if !res.OK {
		t.Fatalf("ビルドに失敗した: %s", res.Error)
	}
	assertCwd(t, outside)

	outBytes, err := exec.Command(res.Path).CombinedOutput()
	if err != nil {
		t.Fatalf("ビルドした実行ファイルが動かない: %v\n%s", err, outBytes)
	}
	if got := strings.TrimSpace(string(outBytes)); got != "正しい葉" {
		t.Fatalf("出力 = %q, want 正しい葉", got)
	}
}

// #351: 「フォルダ変換」も、プロジェクト外が作業フォルダでも元
// ファイルの隣から相対 include を解決すること。梱包された IR の
// Sources で、依存がプロジェクト側の実パスであることを確かめる。
func TestBuildAppFromFolderResolvesRelativeIncludeFromEntryPath(t *testing.T) {
	base := t.TempDir()
	_, _ = useRuntime(t, base)

	project := filepath.Join(base, "project")
	outside := filepath.Join(base, "outside")
	writeIncludeProject(t, project, outside)
	chdirOutside(t, outside)

	out := filepath.Join(base, "ConvertedApp")
	res := buildAppFromFolder(project, filepath.Join(project, "main.nako3"), out, "変換アプリ")
	if !res.OK {
		t.Fatalf("変換に失敗した: %s", res.Error)
	}
	assertCwd(t, outside)

	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatalf("作った実行ファイルを開けない: %v", err)
	}
	defer packed.Close()
	if packed.Name != "main.nako3" {
		t.Fatalf("梱包名 = %q, want main.nako3", packed.Name)
	}
	if packed.Program == nil {
		t.Fatal("プログラムが入っていない")
	}
	wantSources := []string{
		"main.nako3",
		filepath.Join(project, "helper.nako3"),
		filepath.Join(project, "sub", "leaf.nako3"),
	}
	if len(packed.Program.Sources) != len(wantSources) {
		t.Fatalf("Sources = %+v", packed.Program.Sources)
	}
	for i, want := range wantSources {
		if packed.Program.Sources[i].Name != want {
			t.Errorf("Sources[%d] = %q, want %q", i, packed.Program.Sources[i].Name, want)
		}
	}
}

// #351: 取り込み先が無い場合、両経路ともエラー表示は従来どおり
// basename のままで、プロジェクトの絶対パスを露出しないこと。
func TestBuildPathsMissingIncludeKeepsDisplayName(t *testing.T) {
	base := t.TempDir()
	_, _ = useRuntime(t, base)

	project := filepath.Join(base, "project")
	outside := filepath.Join(base, "outside")
	writeFile(t, filepath.Join(project, "main.nako3"), "!「./missing.nako3」を取り込む\n")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	chdirOutside(t, outside)

	want := "[字句解析エラー]main.nako3(1行目): ファイル『./missing.nako3』が見つかりません。"
	mainPath := filepath.Join(project, "main.nako3")

	goRes := buildWithGo(mainPath, filepath.Join(base, "built-app"))
	if goRes.OK {
		t.Fatal("Goビルドが成功してしまった")
	}
	if goRes.Error != want {
		t.Errorf("Goビルドのエラー = %q, want %q", goRes.Error, want)
	}

	folderRes := buildAppFromFolder(project, mainPath, filepath.Join(base, "ConvertedApp"), "変換アプリ")
	if folderRes.OK {
		t.Fatal("フォルダ変換が成功してしまった")
	}
	if folderRes.Error != want {
		t.Errorf("フォルダ変換のエラー = %q, want %q", folderRes.Error, want)
	}
	assertCwd(t, outside)
}
