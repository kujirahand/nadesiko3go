package re_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/re"
)

func mustCompile(t *testing.T, pattern string) *re.Regexp {
	t.Helper()
	r, err := re.Compile(pattern)
	if err != nil {
		t.Fatalf("Compile(%q): %v", pattern, err)
	}
	return r
}

func TestCompileFlags(t *testing.T) {
	tests := []struct {
		pattern string
		global  bool
	}{
		{"/a/g", true},
		{"/a/", false},
		{"/a/gi", true},
		// 『/pat/flags』の形でなければ、パターンそのものとして g 付きで扱う
		{"a", true},
		{"", true},
	}
	for _, tt := range tests {
		if got := mustCompile(t, tt.pattern); got.Global != tt.global {
			t.Errorf("Compile(%q).Global = %v, want %v", tt.pattern, got.Global, tt.global)
		}
	}
}

func TestFindAndReplace(t *testing.T) {
	if got := mustCompile(t, "/[0-9]/g").FindAll("a1b2"); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Errorf("FindAll = %v, want [1 2]", got)
	}
	// 空のパターンは各位置で一致する
	if got := mustCompile(t, "").FindAll("abc"); len(got) != 4 {
		t.Errorf("空パターンの一致数 = %d, want 4", len(got))
	}
	if got := mustCompile(t, "/[0-9]+/").Find("abc123"); !reflect.DeepEqual(got, []string{"123"}) {
		t.Errorf("Find = %v, want [123]", got)
	}
	if got := mustCompile(t, "/[0-9]+/").Find("abc"); got != nil {
		t.Errorf("Find = %v, want nil", got)
	}

	tests := []struct {
		pattern, in, template, want string
	}{
		{"/[0-9]/g", "a1b2c3", "-", "a-b-c-"},
		// g がなければ最初の一致だけ
		{"/[0-9]/", "a1b2c3", "-", "a-b2c3"},
		// $1 のような後方参照。Goの $1年 は「1年」という名前の組と読まれるので
		// 波括弧で包み直している。
		{`/(\d+)-(\d+)-(\d+)/`, "2024-01-02", "$1年$2月$3日", "2024年01月02日"},
		{"/b/g", "abc", "[$&]", "a[b]c"},
		// 参照でないドル記号はそのまま残る
		{"/b/g", "abc", "$", "a$c"},
		{"/[いう]/g", "あいうえお", "*", "あ**えお"},
	}
	for _, tt := range tests {
		if got := mustCompile(t, tt.pattern).Replace(tt.in, tt.template); got != tt.want {
			t.Errorf("Replace(%q, %q, %q) = %q, want %q", tt.pattern, tt.in, tt.template, got, tt.want)
		}
	}

	if got := mustCompile(t, "/[0-9]+/").Split("a1b22c"); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("Split = %v, want [a b c]", got)
	}
}

// TestUnsupported pins which constructs RE2 leaves out, so that adding a second
// engine later has a clear target (AGENTS.md §15).
func TestUnsupported(t *testing.T) {
	for _, pattern := range []string{
		`/(ab)\1/`,  // 後方参照
		`/a(?=b)/`,  // 先読み
		`/a(?!b)/`,  // 否定先読み
		`/(?<=a)b/`, // 後読み
		`/(?<!a)b/`, // 否定後読み
	} {
		_, err := re.Compile(pattern)
		if !errors.Is(err, re.ErrUnsupported) {
			t.Errorf("Compile(%q) = %v, want ErrUnsupported", pattern, err)
		}
	}
	// エスケープした円記号のあとの数字は後方参照ではない
	if _, err := re.Compile(`/a\\1/`); errors.Is(err, re.ErrUnsupported) {
		t.Error(`Compile("/a\\\\1/") を後方参照と誤判定した`)
	}
}

// TestRuneBased pins the deliberate difference from JavaScript: the dot counts
// a character outside the BMP as one, not two (AGENTS.md §5).
func TestRuneBased(t *testing.T) {
	if got := mustCompile(t, "/./g").Replace("𩸽", "X"); got != "X" {
		t.Errorf(`Replace("𩸽") = %q, want "X" (JS版は "XX")`, got)
	}
}

func TestJavaScriptUnicodeEscapes(t *testing.T) {
	if got := mustCompile(t, `^[\u0020-\u007E\uFF61-\uFF9F]`).Find("abc"); got == nil {
		t.Fatal("JavaScript形式のUnicodeエスケープがASCIIに一致しません")
	}
	pattern := `^[\u3005\u3007\u4E00-\u9FFF]|[\uD840-\uD87F][\uDC00-\uDFFF]`
	if got := mustCompile(t, pattern).Find("色"); got == nil {
		t.Fatal("Unicodeエスケープを含む漢字範囲に一致しません")
	}
}
