package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/parser"
)

// writeSourceFiles は root 以下へ name→内容 のファイルを作る。
func writeSourceFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// chdirForTest はテスト中だけ作業フォルダを移し、終了時に戻す。
// os.Chdir はプロセス全体に効くため、このパッケージのテストでは
// t.Parallel を併用しないこと。
func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// #351: 表示名が basename でも、相対的な『取り込む』は表示名ではなく
// 元ソースファイルの場所を基準に解決すること。cwd 側には同名の壊れた
// decoy を置くので、cwd 基準で解決すると構文エラーで失敗する。
func TestParseSourceFileResolvesRequiresFromSourcePath(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	outside := filepath.Join(base, "outside")
	writeSourceFiles(t, project, map[string]string{
		"main.nako3":     "!「./helper.nako3」を取り込む\n補助実行\n",
		"helper.nako3":   "!「./sub/leaf.nako3」を取り込む\n●補助実行とは\n    葉実行\nここまで\n",
		"sub/leaf.nako3": "●葉実行とは\n    「正しい葉」と表示\nここまで\n",
	})
	writeSourceFiles(t, outside, map[string]string{
		"helper.nako3":   "もし\n",
		"sub/leaf.nako3": "もし\n",
	})
	chdirForTest(t, outside)

	code := "!「./helper.nako3」を取り込む\n補助実行\n"
	mainPath := filepath.Join(project, "main.nako3")
	if _, err := parser.ParseSourceFile(code, "main.nako3", mainPath, sourceFileTestFuncs()); err != nil {
		t.Fatalf("ParseSourceFile: %v", err)
	}
	if got, err := os.Getwd(); err != nil || got != outside {
		t.Fatalf("作業フォルダが変わった: %q, %v", got, err)
	}

	// sourcePath が相対パスでも、呼び出し時点の場所から絶対化して使う。
	if _, err := parser.ParseSourceFile(code, "main.nako3", filepath.Join("..", "project", "main.nako3"), sourceFileTestFuncs()); err != nil {
		t.Fatalf("ParseSourceFile(相対 sourcePath): %v", err)
	}

	// 互換 API の ParseSource は、filename 自身が解決基準である従来の
	// 意味のまま。basename だけでは cwd 基準になり見つからない。
	if _, err := parser.ParseSource(code, "main.nako3", sourceFileTestFuncs()); err == nil {
		t.Fatal("ParseSource(basename) が成功してしまった")
	}
}

// #351: sourcePath が空の ParseSourceFile は、ParseSource と完全に
// 同じ挙動（互換）であること。
func TestParseSourceFileEmptySourcePathMatchesParseSource(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	writeSourceFiles(t, project, map[string]string{
		"main.nako3":   "!「./helper.nako3」を取り込む\n補助実行\n",
		"helper.nako3": "●補助実行とは\n    「補助」と表示\nここまで\n",
	})
	mainPath := filepath.Join(project, "main.nako3")

	okCode := "!「./helper.nako3」を取り込む\n補助実行\n"
	if _, err := parser.ParseSource(okCode, mainPath, sourceFileTestFuncs()); err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	if _, err := parser.ParseSourceFile(okCode, mainPath, "", sourceFileTestFuncs()); err != nil {
		t.Fatalf("ParseSourceFile(sourcePath=\"\"): %v", err)
	}

	missingCode := "!「./missing.nako3」を取り込む\n"
	_, errA := parser.ParseSource(missingCode, mainPath, sourceFileTestFuncs())
	_, errB := parser.ParseSourceFile(missingCode, mainPath, "", sourceFileTestFuncs())
	if errA == nil || errB == nil {
		t.Fatalf("存在しない取り込み先でエラーにならなかった: %v / %v", errA, errB)
	}
	if errA.Error() != errB.Error() {
		t.Fatalf("エラーが違う: %q vs %q", errA.Error(), errB.Error())
	}
}

// #351: 取り込み先が無い場合のエラーは、解決基準に絶対パスを使っても
// 表示名（basename）のままであること。絶対パスが露出すると、GUI の
// エラー表示が従来と変わってしまう。
func TestParseSourceFileMissingRequireKeepsDisplayName(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	outside := filepath.Join(base, "outside")
	writeSourceFiles(t, project, map[string]string{
		"main.nako3": "!「./missing.nako3」を取り込む\n",
	})
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	chdirForTest(t, outside)

	_, err := parser.ParseSourceFile("!「./missing.nako3」を取り込む\n", "main.nako3", filepath.Join(project, "main.nako3"), lexer.FuncList{})
	if err == nil {
		t.Fatal("存在しない取り込み先でエラーにならなかった")
	}
	want := "[字句解析エラー]main.nako3(1行目): ファイル『./missing.nako3』が見つかりません。"
	if err.Error() != want {
		t.Fatalf("エラー = %q, want %q", err.Error(), want)
	}
	if strings.Contains(err.Error(), project) {
		t.Fatalf("エラーにプロジェクトの絶対パスが露出している: %q", err.Error())
	}
}

// sourceFileTestFuncs は、取り込み展開が差し込む名前空間命令と表示を
// 解決できる最小の命令一覧。
func sourceFileTestFuncs() lexer.FuncList {
	return lexer.FuncList{
		"表示":       {Name: "表示", Type: "func", Josi: [][]string{{"と"}}},
		"名前空間設定":   {Name: "名前空間設定", Type: "func", Josi: [][]string{{"に", "へ"}}},
		"プラグイン名設定": {Name: "プラグイン名設定", Type: "func", Josi: [][]string{{"に", "へ"}}},
		"名前空間ポップ":  {Name: "名前空間ポップ", Type: "func"},
	}
}
