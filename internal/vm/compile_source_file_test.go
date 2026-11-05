package vm_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// #351: 表示名が basename でも、相対的な『取り込む』は元ソースファイルの
// 場所を基準に解決し、多段の取り込みでも正しいファイルを使うこと。
// cwd 側には同じ相対構成で別の内容の decoy を置くので、cwd 基準で解決
// すると Sources と実行結果の両方が decoy 側になる。
func TestCompileProgramFileResolvesNestedRequiresFromSourcePath(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	outside := filepath.Join(base, "outside")
	write := func(root, name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(project, "main.nako3", "!「./helper.nako3」を取り込む\n補助実行\n")
	write(project, "helper.nako3", "!「./sub/leaf.nako3」を取り込む\n●補助実行とは\n    葉実行\nここまで\n")
	write(project, "sub/leaf.nako3", "●葉実行とは\n    「正しい葉」を表示\nここまで\n")
	write(outside, "helper.nako3", "!「./sub/leaf.nako3」を取り込む\n●補助実行とは\n    葉実行\nここまで\n")
	write(outside, "sub/leaf.nako3", "●葉実行とは\n    「誤った葉」を表示\nここまで\n")

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}

	mainPath := filepath.Join(project, "main.nako3")
	code := "!「./helper.nako3」を取り込む\n補助実行\n"
	prog, err := vm.CompileProgramFile(code, "main.nako3", mainPath)
	if err != nil {
		t.Fatalf("CompileProgramFile: %v", err)
	}

	// 表示名は basename のまま、依存は解決済みの実パスが入る。
	wantSources := []string{
		"main.nako3",
		filepath.Join(project, "helper.nako3"),
		filepath.Join(project, "sub", "leaf.nako3"),
	}
	if len(prog.Sources) != len(wantSources) {
		t.Fatalf("Sources = %+v", prog.Sources)
	}
	for i, want := range wantSources {
		if prog.Sources[i].Name != want {
			t.Errorf("Sources[%d] = %q, want %q", i, prog.Sources[i].Name, want)
		}
	}

	// 実行してもプロジェクト側の葉が使われる。
	var out bytes.Buffer
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	if err := vm.RunCompiled(prog, host); err != nil {
		t.Fatalf("RunCompiled: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "正しい葉" {
		t.Fatalf("出力 = %q, want 正しい葉", got)
	}

	if got, err := os.Getwd(); err != nil || got != outside {
		t.Fatalf("作業フォルダが変わった: %q, %v", got, err)
	}
}

// #351: 依存ファイルがメインを取り込む循環でも、新 API の挙動が従来
// API（実パスをそのまま渡す CUI 経路）と一致すること。メインは guard に
// 登録されないため、どちらの経路でもメインは一度だけ再展開され、実行
// 結果は同じになる。再展開されたメインのトークンは解決済み実パスを
// 持つため、新 API では Sources に実パスでもう1件登録される。これは
// basename 運用だった従来の GUI でも、cwd がプロジェクト内の場合に
// 起きていた登録と同じである。
func TestCompileProgramFileCircularIncludeBackToMain(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "main.nako3")
	helper := filepath.Join(dir, "helper.nako3")
	code := "!「./helper.nako3」を取り込む\n補助実行\n"
	if err := os.WriteFile(main, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("!「./main.nako3」を取り込む\n●補助実行とは\n    「補助」を表示\nここまで\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	progOld, err := vm.CompileProgram(code, main)
	if err != nil {
		t.Fatalf("CompileProgram: %v", err)
	}
	progNew, err := vm.CompileProgramFile(code, "main.nako3", main)
	if err != nil {
		t.Fatalf("CompileProgramFile: %v", err)
	}

	var outOld, outNew bytes.Buffer
	if err := vm.RunCompiled(progOld, vm.NewCUIHost(&outOld, strings.NewReader(""), nil)); err != nil {
		t.Fatalf("従来の実行: %v", err)
	}
	if err := vm.RunCompiled(progNew, vm.NewCUIHost(&outNew, strings.NewReader(""), nil)); err != nil {
		t.Fatalf("新規の実行: %v", err)
	}
	if outOld.String() != outNew.String() {
		t.Fatalf("出力が違う: 従来 %q / 新規 %q", outOld.String(), outNew.String())
	}

	// 従来はメインの再展開分も同じ実パスの Source に寄る。新 API は
	// 表示名のメインと、再展開分の実パスが別 Source になる。
	if len(progOld.Sources) != 2 || progOld.Sources[0].Name != main || progOld.Sources[1].Name != helper {
		t.Errorf("従来 Sources = %+v", progOld.Sources)
	}
	if len(progNew.Sources) != 3 || progNew.Sources[0].Name != "main.nako3" ||
		progNew.Sources[1].Name != helper || progNew.Sources[2].Name != main {
		t.Errorf("新規 Sources = %+v", progNew.Sources)
	}
}

// #351: 解決基準に絶対パスを使っても、メイン自身のエラー帰属と表示は
// basename のままであること。依存ファイル側の帰属は #340 のテストが
// 実パスのまま固定している。
func TestCompileProgramFileKeepsDisplayNameForMainErrors(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(project, "main.nako3")

	// 取り込み先が無い場合の字句解析エラー。
	_, err := vm.CompileProgramFile("!「./missing.nako3」を取り込む\n", "main.nako3", mainPath)
	if err == nil {
		t.Fatal("存在しない取り込み先でエラーにならなかった")
	}
	want := "[字句解析エラー]main.nako3(1行目): ファイル『./missing.nako3』が見つかりません。"
	if err.Error() != want {
		t.Fatalf("エラー = %q, want %q", err.Error(), want)
	}

	// メイン自身の実行時エラーも basename に帰属する。
	prog, err := vm.CompileProgramFile("「本体の失敗」でエラー発生\n", "main.nako3", mainPath)
	if err != nil {
		t.Fatalf("CompileProgramFile: %v", err)
	}
	var out bytes.Buffer
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	err = vm.RunCompiled(prog, host)
	var ne *errs.NakoError
	if !errors.As(err, &ne) {
		t.Fatalf("*errs.NakoError を期待したが %T: %v", err, err)
	}
	if ne.File != "main.nako3" {
		t.Fatalf("File = %q, want main.nako3", ne.File)
	}
	if want := "main.nako3(1行目)"; !strings.Contains(ne.Error(), want) {
		t.Fatalf("Error() = %q に %q がない", ne.Error(), want)
	}
}
