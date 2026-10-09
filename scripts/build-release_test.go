//go:build ignore

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// 生成したバッチを別フォルダから実行し、引数と失敗時の終了コードを確認する。
func TestUploadBatchExecution(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windowsのcmd.exeで検証する")
	}
	for _, tc := range []struct {
		name       string
		authCode   string
		uploadCode string
		wantCode   int
	}{
		{"成功", "0", "0", 0},
		{"認証失敗", "1", "0", 1},
		{"アップロード失敗", "0", "7", 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "release files")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			name := "gonako-gui-3.8.9-windows-amd64.zip"
			if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := writeUploadScripts(config{version: "3.8.9", outDir: dir}); err != nil {
				t.Fatal(err)
			}
			// 実際のGitHubへ送信せず、CLIに渡された引数を記録する。
			mockPath := filepath.Join(dir, "mock gh.cmd")
			logPath := filepath.Join(dir, "args.txt")
			mock := "@echo off\r\nif \"%~1\"==\"auth\" exit /b " + tc.authCode + "\r\n" +
				">\"%UPLOAD_TEST_LOG%\" echo %*\r\nexit /b " + tc.uploadCode + "\r\n"
			if err := os.WriteFile(mockPath, []byte(mock), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GH_EXE", mockPath)
			t.Setenv("UPLOAD_TEST_LOG", logPath)
			cmd := exec.Command("cmd.exe", "/d", "/c", "call", filepath.Join(dir, "upload-3.8.9.bat"))
			cmd.Dir = t.TempDir()
			output, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					code = exitErr.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if code != tc.wantCode {
				t.Fatalf("終了コード = %d, want %d: %s", code, tc.wantCode, output)
			}
			args, err := os.ReadFile(logPath)
			if tc.authCode != "0" {
				if !os.IsNotExist(err) {
					t.Fatalf("認証失敗後にアップロードが実行された: %s, %v", args, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"release upload \"3.8.9\"", filepath.Join(dir, name), "--repo kujirahand/nadesiko3go --clobber"} {
				if !strings.Contains(string(args), want) {
					t.Errorf("引数に %q がない: %s", want, args)
				}
			}
		})
	}
}

func TestFilterZipsByVersion(t *testing.T) {
	tests := []struct {
		name    string
		paths   []string
		version string
		want    []string
	}{
		{
			name:    "同一バージョンだけ抽出",
			paths:   []string{"/tmp/gonako-3.8.7-darwin-arm64.zip", "/tmp/gonako-3.8.6-darwin-arm64.zip"},
			version: "3.8.7",
			want:    []string{"/tmp/gonako-3.8.7-darwin-arm64.zip"},
		},
		{
			name: "旧版CLI・GUIが混在しても新版だけ抽出",
			paths: []string{
				"/tmp/gonako-3.8.6-darwin-arm64.zip",
				"/tmp/gonako-gui-3.8.6-darwin-arm64.app.zip",
				"/tmp/gonako-3.8.7-darwin-arm64.zip",
				"/tmp/gonako-gui-3.8.7-darwin-arm64.app.zip",
			},
			version: "3.8.7",
			want: []string{
				"/tmp/gonako-3.8.7-darwin-arm64.zip",
				"/tmp/gonako-gui-3.8.7-darwin-arm64.app.zip",
			},
		},
		{
			name:    "該当なしは空",
			paths:   []string{"/tmp/gonako-3.8.6-darwin-arm64.zip"},
			version: "3.8.7",
			want:    nil,
		},
		{
			name:    "空入力",
			paths:   nil,
			version: "3.8.7",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterZipsByVersion(tt.paths, tt.version)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d: got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestWriteUploadScripts_OldVersionExcluded は、旧版ZIPが残っていても
// アップロードスクリプトに旧版が記載されないことを確認する回帰テスト。
func TestWriteUploadScripts_OldVersionExcluded(t *testing.T) {
	dir := t.TempDir()

	// 旧版ZIP（3.8.6）
	oldCLI := filepath.Join(dir, "gonako-3.8.6-darwin-arm64.zip")
	oldGUI := filepath.Join(dir, "gonako-gui-3.8.6-darwin-arm64.app.zip")
	// 新版ZIP（3.8.7）
	newCLI := filepath.Join(dir, "gonako-3.8.7-darwin-arm64.zip")
	newGUI := filepath.Join(dir, "gonako-gui-3.8.7-darwin-arm64.app.zip")

	for _, p := range []string{oldCLI, oldGUI, newCLI, newGUI} {
		if err := os.WriteFile(p, []byte("dummy"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config{
		version: "3.8.7",
		outDir:  dir,
	}
	if err := writeUploadScripts(cfg); err != nil {
		t.Fatalf("writeUploadScripts: %v", err)
	}

	// シェルスクリプトの内容を確認
	shContent, err := os.ReadFile(filepath.Join(dir, "upload-3.8.7.sh"))
	if err != nil {
		t.Fatal(err)
	}
	sh := string(shContent)

	// 旧版が含まれていないことを確認
	if strings.Contains(sh, "gonako-3.8.6") {
		t.Errorf("シェルスクリプトに旧版 (3.8.6) が含まれている:\n%s", sh)
	}
	// 新版が含まれていることを確認
	if !strings.Contains(sh, "gonako-3.8.7-darwin-arm64.zip") {
		t.Errorf("シェルスクリプトに新版CLIが含まれていない:\n%s", sh)
	}
	if !strings.Contains(sh, "gonako-gui-3.8.7-darwin-arm64.app.zip") {
		t.Errorf("シェルスクリプトに新版GUIが含まれていない:\n%s", sh)
	}

	// バッチファイルの内容も確認
	batContent, err := os.ReadFile(filepath.Join(dir, "upload-3.8.7.bat"))
	if err != nil {
		t.Fatal(err)
	}
	bat := string(batContent)

	if strings.Contains(bat, "gonako-3.8.6") {
		t.Errorf("バッチファイルに旧版 (3.8.6) が含まれている:\n%s", bat)
	}
	if !strings.Contains(bat, "gonako-3.8.7-darwin-arm64.zip") {
		t.Errorf("バッチファイルに新版CLIが含まれていない:\n%s", bat)
	}
	if !strings.Contains(bat, "gonako-gui-3.8.7-darwin-arm64.app.zip") {
		t.Errorf("バッチファイルに新版GUIが含まれていない:\n%s", bat)
	}
}
