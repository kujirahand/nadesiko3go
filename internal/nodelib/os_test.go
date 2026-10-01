package nodelib

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

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

// TestPromptValueMatchesConsoleNumberConversion は #220 の回帰テスト。
// ダイアログと標準入力の尋が同じ入力変換を使うことを確認する。
func TestPromptValueMatchesConsoleNumberConversion(t *testing.T) {
	tests := []struct {
		input  string
		kind   value.Kind
		number float64
		text   string
	}{
		{input: "1e3", kind: value.KindNumber, number: 1000},
		{input: "0x10", kind: value.KindNumber, number: 16},
		{input: "１２", kind: value.KindString, text: "１２"},
		{input: "  7", kind: value.KindNumber, number: 7},
		{input: "12.5", kind: value.KindNumber, number: 12.5},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := promptValue(tt.input)
			if got.Kind() != tt.kind {
				t.Fatalf("promptValue(%q) kind = %v, want %v", tt.input, got.Kind(), tt.kind)
			}
			if tt.kind == value.KindNumber {
				if n, _ := got.Number(); n != tt.number {
					t.Errorf("promptValue(%q) = %v, want %v", tt.input, n, tt.number)
				}
			} else if s, _ := got.String(); s != tt.text {
				t.Errorf("promptValue(%q) = %q, want %q", tt.input, s, tt.text)
			}
		})
	}
}
