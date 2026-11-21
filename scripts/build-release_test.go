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

func webkitTagScript(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{"webkit-gtk-tag.sh", filepath.Join("scripts", "webkit-gtk-tag.sh")} {
		if _, err := os.Stat(candidate); err == nil {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				t.Fatal(err)
			}
			return abs
		}
	}
	t.Fatal("webkit-gtk-tag.sh が見つかりません")
	return ""
}

func writeFakePkgConfig(t *testing.T, found string) string {
	t.Helper()
	dir := t.TempDir()
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--exists\" ] && [ \"$2\" = \"" + found + "\" ]; then\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "pkg-config"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runWebkitTagScript(t *testing.T, goos, pkgDir string) (string, int) {
	t.Helper()
	cmd := exec.Command(webkitTagScript(t))
	cmd.Env = []string{"GOOS=" + goos, "PATH=" + pkgDir + ":/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = exitErr.ExitCode()
	}
	return string(out), code
}

func TestDarwinClangArch(t *testing.T) {
	arch, err := darwinClangArch("amd64")
	if err != nil || arch != "x86_64" {
		t.Fatalf("amd64 -> %q, %v", arch, err)
	}
	arch, err = darwinClangArch("arm64")
	if err != nil || arch != "arm64" {
		t.Fatalf("arm64 -> %q, %v", arch, err)
	}
	if _, err := darwinClangArch("386"); err == nil {
		t.Fatal("386 を受け付けてしまった")
	}
}

func TestMergeEnvOverridesEarlierKeys(t *testing.T) {
	got := mergeEnv(
		[]string{"CGO_LDFLAGS=-O2 -g", "PATH=/usr/bin", "KEEP=1"},
		[]string{"CGO_LDFLAGS=-arch x86_64", "CC=clang"},
	)
	want := []string{"PATH=/usr/bin", "KEEP=1", "CGO_LDFLAGS=-arch x86_64", "CC=clang"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWebkitGTKTagScript(t *testing.T) {
	t.Run("4.0があればタグは空", func(t *testing.T) {
		out, code := runWebkitTagScript(t, "linux", writeFakePkgConfig(t, "webkit2gtk-4.0"))
		if code != 0 {
			t.Fatalf("終了コード = %d, 出力 = %s", code, out)
		}
		if strings.TrimSpace(out) != "" {
			t.Fatalf("タグ = %q, want empty", out)
		}
	})
	t.Run("4.1だけならタグを出す", func(t *testing.T) {
		out, code := runWebkitTagScript(t, "linux", writeFakePkgConfig(t, "webkit2gtk-4.1"))
		if code != 0 {
			t.Fatalf("終了コード = %d, 出力 = %s", code, out)
		}
		if strings.TrimSpace(out) != "webkit2gtk_4_1" {
			t.Fatalf("タグ = %q", out)
		}
	})
	t.Run("どちらも無ければ失敗する", func(t *testing.T) {
		out, code := runWebkitTagScript(t, "linux", writeFakePkgConfig(t, ""))
		if code == 0 {
			t.Fatalf("成功してしまった: %s", out)
		}
		if !strings.Contains(out, "libwebkit2gtk-4.1-dev") {
			t.Fatalf("案内がありません: %s", out)
		}
	})
	t.Run("Linux以外はpkg-configが無くても空", func(t *testing.T) {
		out, code := runWebkitTagScript(t, "darwin", t.TempDir())
		if code != 0 {
			t.Fatalf("終了コード = %d, 出力 = %s", code, out)
		}
		if strings.TrimSpace(out) != "" {
			t.Fatalf("タグ = %q, want empty", out)
		}
	})
}
