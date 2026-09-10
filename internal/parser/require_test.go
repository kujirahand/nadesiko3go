package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/parser"
)

// TestParseSourceRequireRejectsUnsupportedTargets は、取込が未対応の対象
// （URL・貯蔵庫・拡張プラグイン・非対応拡張子）に対して、誤ってパースが
// 通ったりpanicしたりせず、なでしこ形式の明確なエラーになることを確認する
// (#58)。
func TestParseSourceRequireRejectsUnsupportedTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.nako3")

	tests := []struct {
		name string
		code string
		want string
	}{
		{"url", "!「https://example.com/lib.nako3」を取込。", "URL"},
		{"storage", "!「貯蔵庫:foo.nako3」を取込。", "未対応"},
		{"js-plugin", "!「foo.js」を取込。", "拡張子"},
		{"bad-extension", "!「foo.txt」を取込。", "拡張子"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tt.code), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := parser.ParseSource(tt.code, path, lexer.FuncList{})
			if err == nil {
				t.Fatalf("エラーになりませんでした: %s", tt.code)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("エラー = %v, %qを含んでいません", err, tt.want)
			}
		})
	}
}
