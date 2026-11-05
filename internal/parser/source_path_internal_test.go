package parser

import (
	"path/filepath"
	"testing"
)

// #351: requireSourcePath の正規化。空なら表示名をそのまま解決基準にし
// （従来互換）、URL と埋め込みはファイルシステムのパスではないので
// 触らず、ローカルの相対パスだけを絶対化する。
func TestRequireSourcePath(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "project", "main.nako3")
	tests := []struct {
		name       string
		filename   string
		sourcePath string
		want       string
	}{
		{"空は filename をそのまま使う", "main.nako3", "", "main.nako3"},
		{"空は filename の絶対パスもそのまま使う", abs, "", abs},
		{"URL はそのまま", "main.nako3", "https://example.test/dir/main.nako3", "https://example.test/dir/main.nako3"},
		{"埋め込みはそのまま", "main.nako3", "embed:gonako-package/demo/main.nako3", "embed:gonako-package/demo/main.nako3"},
		{"絶対パスはそのまま", "main.nako3", abs, abs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := requireSourcePath(tt.filename, tt.sourcePath)
			if err != nil {
				t.Fatalf("requireSourcePath: %v", err)
			}
			if got != tt.want {
				t.Errorf("requireSourcePath(%q, %q) = %q, want %q", tt.filename, tt.sourcePath, got, tt.want)
			}
		})
	}

	// 相対パスは呼び出し時点の作業フォルダで絶対化される。
	rel := filepath.Join("project", "main.nako3")
	got, err := requireSourcePath("main.nako3", rel)
	if err != nil {
		t.Fatalf("requireSourcePath: %v", err)
	}
	want, err := filepath.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("requireSourcePath(相対) = %q, want %q", got, want)
	}
}
