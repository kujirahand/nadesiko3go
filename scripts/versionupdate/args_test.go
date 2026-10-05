package versionupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    ParsedArgs
		wantErr bool
	}{
		{
			name: "位置引数なし、フラグなし",
			args: []string{},
			want: ParsedArgs{},
		},
		{
			name: "位置引数のみ",
			args: []string{"3.8.2"},
			want: ParsedArgs{Positional: []string{"3.8.2"}},
		},
		{
			name: "前置フラグ --check",
			args: []string{"--check"},
			want: ParsedArgs{Check: true},
		},
		{
			name: "前置フラグ --nadesiko",
			args: []string{"--nadesiko", "3.9.0"},
			want: ParsedArgs{Nadesiko: "3.9.0"},
		},
		{
			name: "前置フラグ --stable",
			args: []string{"--stable", "3.8.2"},
			want: ParsedArgs{Stable: "3.8.2"},
		},
		{
			name: "位置引数の後に --nadesiko（Issue #211 のケース）",
			args: []string{"3.8.2", "--nadesiko", "3.9.0"},
			want: ParsedArgs{
				Positional: []string{"3.8.2"},
				Nadesiko:   "3.9.0",
			},
		},
		{
			name: "位置引数の後に --check",
			args: []string{"3.8.2", "--check"},
			want: ParsedArgs{
				Check:      true,
				Positional: []string{"3.8.2"},
			},
		},
		{
			name: "フラグが混在",
			args: []string{"--nadesiko", "3.9.0", "3.8.2", "--check"},
			want: ParsedArgs{
				Check:      true,
				Nadesiko:   "3.9.0",
				Positional: []string{"3.8.2"},
			},
		},
		{
			name: "終端マーカー --",
			args: []string{"--check", "--", "--nadesiko", "3.9.0"},
			want: ParsedArgs{
				Check:      true,
				Positional: []string{"--nadesiko", "3.9.0"},
			},
		},
		{
			name:    "--nadesiko の後に値がない",
			args:    []string{"--nadesiko"},
			wantErr: true,
		},
		{
			name:    "--stable の後に値がない",
			args:    []string{"--stable"},
			wantErr: true,
		},
		{
			name:    "未知のフラグ",
			args:    []string{"--unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Check != tt.want.Check {
				t.Errorf("Check = %v, want %v", got.Check, tt.want.Check)
			}
			if got.Nadesiko != tt.want.Nadesiko {
				t.Errorf("Nadesiko = %q, want %q", got.Nadesiko, tt.want.Nadesiko)
			}
			if got.Stable != tt.want.Stable {
				t.Errorf("Stable = %q, want %q", got.Stable, tt.want.Stable)
			}
			if !reflect.DeepEqual(got.Positional, tt.want.Positional) {
				t.Errorf("Positional = %v, want %v", got.Positional, tt.want.Positional)
			}
		})
	}
}

// TestInvalidNadesikoDoesNotDeleteReleaseArtifacts は #191 の回帰テスト。
// 不正な --nadesiko を指定しても、release/ の成果物が保持されることを確認する。
func TestInvalidNadesikoDoesNotDeleteReleaseArtifacts(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("テストファイルのパスを取得できません")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "../.."))
	buildDir := t.TempDir()
	binaryPath := filepath.Join(buildDir, "version-update")
	build := exec.Command("go", "build", "-o", binaryPath, "./scripts/version-update.go")
	build.Dir = repoRoot
	build.Env = envWithGOCACHE(os.Environ(), filepath.Join(buildDir, "gocache"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("version-update のビルドに失敗しました: %v\n%s", err, output)
	}

	workDir := t.TempDir()
	releaseDir := filepath.Join(workDir, "release")
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(releaseDir, "old-artifact.zip")
	want := []byte("keep this release artifact")
	if err := os.WriteFile(artifactPath, want, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binaryPath, "--nadesiko", "invalid", "3.8.7")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("不正な --nadesiko が成功しました: %s", output)
	}
	if strings.Contains(string(output), "[削除]") {
		t.Errorf("エラー終了前に成果物の削除が実行されました: %s", output)
	}
	got, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("成果物が残っていません: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("成果物の内容が変更されました: got %q, want %q", got, want)
	}
}

func envWithGOCACHE(env []string, cacheDir string) []string {
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, "GOCACHE=") {
			result = append(result, entry)
		}
	}
	return append(result, "GOCACHE="+cacheDir)
}
