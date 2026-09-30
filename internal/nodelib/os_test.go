package nodelib

import "testing"

// TestWindowsStartLine は #218 の回帰テスト。
// cmd.exe の start は最初の引用符付き引数をウィンドウタイトルと解釈するため、
// 空タイトルを明示し、対象は必ず引用符で囲まなければならない。
// 引用を省くと URL 中の & がコマンド区切りになる。
func TestWindowsStartLine(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{"URL", "https://example.com", `start "" "https://example.com"`},
		{"&を含むURL", "https://example.com/?a=1&b=2", `start "" "https://example.com/?a=1&b=2"`},
		{"空白を含むパス", `C:\Program Files\app\file.txt`, `start "" "C:\Program Files\app\file.txt"`},
		{"引用符を含む対象", `bad"target`, `start "" "badtarget"`},
		{"%を含むURL", "https://example.com/%E3%83%86%E3%82%B9%E3%83%88", `start "" "https://example.com/%%E3%%83%%86%%E3%%82%%B9%%E3%%83%%88"`},
		{"%を含むパス", `C:\Users\test\50%\file.txt`, `start "" "C:\Users\test\50%%\file.txt"`},
		{"末尾に単独の%", `C:\Users\test\50%`, `start "" "C:\Users\test\50%%"`},
	}
	for _, tt := range tests {
		if got := windowsStartLine(tt.target); got != tt.want {
			t.Errorf("%s: windowsStartLine(%q) = %q, want %q", tt.name, tt.target, got, tt.want)
		}
	}
}
