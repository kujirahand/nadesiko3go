//go:build ignore

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
