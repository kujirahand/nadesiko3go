package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/deskutil"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

func TestCreateNewFolder(t *testing.T) {
	base := t.TempDir()
	created, err := createNewFolder(base, "資料")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "資料")
	if created != want {
		t.Fatalf("created path = %q, want %q", created, want)
	}
	if info, err := os.Stat(created); err != nil || !info.IsDir() {
		t.Fatalf("folder was not created: info=%v err=%v", info, err)
	}
}

func TestCreateNewFolderInDocuments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windowsではレジストリの設定を使用する")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	documents := deskutil.DocumentsDir()
	if err := os.MkdirAll(documents, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := createNewFolder("$DOCUMENTS", "project")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(documents, "project") {
		t.Fatalf("作成先 = %q", got)
	}
}

func TestCreateNewFolderRejectsInvalidName(t *testing.T) {
	for _, name := range []string{"", ".", "..", "a/b", `a\b`} {
		if _, err := createNewFolder(t.TempDir(), name); err == nil {
			t.Errorf("createNewFolder accepted invalid name %q", name)
		}
	}
}

func TestCreateAIProject(t *testing.T) {
	projectPath := t.TempDir()
	gotPath, files, err := createAIProject(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != projectPath {
		t.Fatalf("project path = %q, want %q", gotPath, projectPath)
	}
	if got := aiProjectFileNames(files); len(got) != 1 || got[0] != "AGENTS.md" {
		t.Fatalf("created files = %#v", got)
	}

	if _, err := os.Stat(filepath.Join(projectPath, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("CLAUDE.mdを作成してしまった: %v", err)
	}

	for _, name := range []string{"AGENTS.md"} {
		data, err := os.ReadFile(filepath.Join(projectPath, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, required := range []string{
			"必ず日本語で応答", "日本語プログラミング言語", "https://nadesi.com/",
			"基本文法", "自由な自然言語ではありません", "命令、助詞、連文", "ここまで",
			"gonako doc キーワード --json", "gonako doc キーワード --web",
			"gonako run main.nako3", "命令名や助詞を推測",
			"gonako-gui run main.nako3", "GUI(ウィンドウ)アプリ",
			"キャンバス作成", "ブラウザ(wnako3)", "実行ファイルに変換",
		} {
			if !strings.Contains(text, required) {
				t.Errorf("%sに%qがありません", name, required)
			}
		}
	}
}

func TestCreateAIProjectKeepsExistingClaudeFile(t *testing.T) {
	projectPath := t.TempDir()
	path := filepath.Join(projectPath, "CLAUDE.md")
	if err := os.WriteFile(path, []byte("残す"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := createAIProject(projectPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "残す" {
		t.Fatalf("CLAUDE.mdが変更された: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(projectPath, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAIProjectDoesNotOverwriteExistingFiles(t *testing.T) {
	projectPath := t.TempDir()
	important := filepath.Join(projectPath, "AGENTS.md")
	if err := os.WriteFile(important, []byte("残す"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := createAIProject(projectPath); err == nil {
		t.Fatal("既存のAGENTS.mdを受け入れてしまった")
	}
	data, err := os.ReadFile(important)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "残す" {
		t.Fatalf("既存ファイルを上書きした: %q", data)
	}
	if _, err := os.Stat(filepath.Join(projectPath, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("片方だけ作成してしまった: %v", err)
	}
}

func TestCreateProjectMainFile(t *testing.T) {
	projectPath := t.TempDir()
	gotPath, err := createProjectMainFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(projectPath, "main.nako3")
	if gotPath != want {
		t.Fatalf("created path = %q, want %q", gotPath, want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != defaultMainProgram {
		t.Fatalf("main.nako3の内容 = %q, want %q", data, defaultMainProgram)
	}
}

func TestCreateProjectMainFileDoesNotOverwriteExisting(t *testing.T) {
	projectPath := t.TempDir()
	important := filepath.Join(projectPath, "main.nako3")
	if err := os.WriteFile(important, []byte("残す"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := createProjectMainFile(projectPath); err == nil {
		t.Fatal("既存のmain.nako3を受け入れてしまった")
	}
	data, err := os.ReadFile(important)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "残す" {
		t.Fatalf("既存ファイルを上書きした: %q", data)
	}
}

// Issue #482: テンプレートAGENTS.mdの ```nako3 例は、AIが手本として真似する。
// 命令名の改名や助詞の変更で手本が壊れないよう、GUI命令を含むレジストリで
// 全ブロックがコンパイルできることを確認する。
func TestAIProjectGuideExamplesCompile(t *testing.T) {
	blocks := extractNako3Blocks(aiDevelopmentGuide)
	if len(blocks) < 5 {
		t.Fatalf("nako3ブロックが %d 件しか抽出できませんでした", len(blocks))
	}
	registry := stdlib.NewRegistry(guiPlugins()...)
	const windowExample = `{"サイズ":[640,480],"位置":"中央"}で母艦のウィンドウ変更`
	foundWindow := false
	for i, code := range blocks {
		if _, err := vm.CompileWithRegistry(code, "AGENTS.md", registry); err != nil {
			t.Errorf("ブロック %d がコンパイルできません: %v\n%s", i+1, err, code)
		}
		if strings.Contains(code, windowExample) {
			foundWindow = true
		}
	}
	if !foundWindow {
		t.Fatal("母艦のウィンドウ変更の例を ```nako3 ブロックとしてコンパイル対象に含めてください")
	}
}

// extractNako3Blocks はMarkdownから ```nako3 フェンスの中身を取り出す。
func extractNako3Blocks(markdown string) []string {
	var blocks []string
	rest := markdown
	for {
		start := strings.Index(rest, "```nako3\n")
		if start < 0 {
			return blocks
		}
		rest = rest[start+len("```nako3\n"):]
		end := strings.Index(rest, "\n```")
		if end < 0 {
			return blocks
		}
		blocks = append(blocks, rest[:end])
		rest = rest[end+len("\n```"):]
	}
}
