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

// mustFindAll は、エラーが返るはずのない照合を失敗として扱うための補助。
func mustFindAll(t *testing.T, r *re.Regexp, s string) []string {
	t.Helper()
	got, err := r.FindAll(s)
	if err != nil {
		t.Fatalf("FindAll(%q): %v", s, err)
	}
	return got
}

// mustFindAllMatches は mustFindAll の捕獲付き版。
func mustFindAllMatches(t *testing.T, r *re.Regexp, s string) []re.Match {
	t.Helper()
	got, err := r.FindAllMatches(s)
	if err != nil {
		t.Fatalf("FindAllMatches(%q): %v", s, err)
	}
	return got
}

// mustReplace は mustFindAll の置換版。
func mustReplace(t *testing.T, r *re.Regexp, s, template string) string {
	t.Helper()
	got, err := r.Replace(s, template)
	if err != nil {
		t.Fatalf("Replace(%q, %q): %v", s, template, err)
	}
	return got
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
	if got := mustFindAll(t, mustCompile(t, "/[0-9]/g"), "a1b2"); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Errorf("FindAll = %v, want [1 2]", got)
	}
	// 空のパターンは各位置で一致する
	if got := mustFindAll(t, mustCompile(t, ""), "abc"); len(got) != 4 {
		t.Errorf("空パターンの一致数 = %d, want 4", len(got))
	}
	if got, ok := mustCompile(t, "/[0-9]+/").Find("abc123"); !ok || got.Text != "123" || len(got.Groups) != 0 {
		t.Errorf("Find = %#v (ok=%v), want text \"123\" without groups", got, ok)
	}
	if _, ok := mustCompile(t, "/[0-9]+/").Find("abc"); ok {
		t.Error("Find = ok, want no match")
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
		// 非gでも元文字列上の一致位置の文脈を保つ（Issue #208）
		{`/\Boo/g`, "foo", "X", "fX"},
		{`/\Boo/`, "foo", "X", "fX"},
		{`/\B(o+)/`, "foo", "[$1]", "f[oo]"},
		// JavaScriptの置換テンプレート（Issue #214）
		{"/b/", "abc", "$$x", "a$xc"},              // $$ は1個の $ に畳む
		{"/(b)/", "abc", "$$$1", "a$bc"},           // $$ の畳み込みと $1 の併用
		{"/b/", "aXbXc", "[$`|$']", "aX[aX|Xc]Xc"}, // 一致より前/後の部分
		{"/a/g", "aaa", "$`|$'", "|aaa|aaa|"},      // g付きでは各一致位置で展開
		{`/(?<x>b)/`, "abc", "<$<x>>", "a<b>c"},    // 名前付き組の参照
		{"/b/", "abc", "$<x>", "a$<x>c"},           // 名前付き組が無ければリテラル
		{`/(?<x>b)/`, "abc", "$<y>", "ac"},         // 存在しない名前は空文字
		{`/(?<x>b)/`, "abc", "$<x", "a$<xc"},       // > がなければリテラル
		{"/(b)/", "abc", "$2", "a$2c"},             // 存在しない組はリテラル
		{"/(b)/", "abc", "$12", "ab2c"},            // 2桁が範囲外なら1桁+リテラル
		{"/(b)/", "abc", "[$01]", "a[b]c"},         // $01 は組1
		{"/((a)x)?b/", "abc", "<$1|$2>", "a<|>c"},  // 未参加の組は空文字
		{"/(a)/", "abc", "${1}", "${1}bc"},         // ${} はJSではリテラル
	}
	for _, tt := range tests {
		if got := mustReplace(t, mustCompile(t, tt.pattern), tt.in, tt.template); got != tt.want {
			t.Errorf("Replace(%q, %q, %q) = %q, want %q", tt.pattern, tt.in, tt.template, got, tt.want)
		}
	}

	if got := mustCompile(t, "/[0-9]+/").Split("a1b22c"); !reflect.DeepEqual(partTexts(got), []string{"a", "b", "c"}) {
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

// TestUnsupportedVSetOperations は、『v』フラグの集合演算を含むパターンを
// 未対応として弾くことを固定する（#401）。受理したままGoの構文で解釈すると、
// JavaScriptと違う結果を黙って返すためである。集合演算でないもの、クラス外
// の記法、『v』なしのパターンは従来どおり通す。
func TestUnsupportedVSetOperations(t *testing.T) {
	for _, pattern := range []string{
		`/[\w&&\d]/v`, // 積集合（JSでは数字だけ、Goでは "a" にも一致する）
		`/[\w--a]/v`,  // 差集合（JSでは "a" と "-" に一致しない）
		`/[\q{ab}]/v`, // 文字列リテラル
		`/[--]/v`,     // JSのvでは構文エラーになる形も未対応に倒す
		`/[a--]/v`,
	} {
		_, err := re.Compile(pattern)
		if !errors.Is(err, re.ErrUnsupported) {
			t.Errorf("Compile(%q) = %v, want ErrUnsupported", pattern, err)
		}
		var vSetErr *re.UnsupportedVSetError
		if !errors.As(err, &vSetErr) {
			t.Errorf("Compile(%q) = %v, want UnsupportedVSetError", pattern, err)
		}
	}
	for _, pattern := range []string{
		`/a/v`,           // v単独は受理のまま
		`/a&&b/v`,        // クラス外はリテラル
		`/a--b/v`,        // クラス外はリテラル
		`/[\&&]/v`,       // エスケープした & は演算子ではない
		`/[\]]/v`,        // エスケープした ] はクラスを閉じない
		`/[[:alpha:]]/v`, // POSIXクラスの括弧は入れ子にしない
		`/[^]]/v`,        // 否定直後の ] はリテラル
		`/[\w&&\d]/`,     // vがなければ対象外（従来のGoの解釈のまま）
	} {
		if _, err := re.Compile(pattern); err != nil {
			t.Errorf("Compile(%q) = %v, want no error", pattern, err)
		}
	}
}

// TestRuneBased pins the deliberate difference from JavaScript: the dot counts
// a character outside the BMP as one, not two (AGENTS.md §5).
func TestRuneBased(t *testing.T) {
	if got := mustReplace(t, mustCompile(t, "/./g"), "𩸽", "X"); got != "X" {
		t.Errorf(`Replace("𩸽") = %q, want "X" (JS版は "XX")`, got)
	}
}

func TestJavaScriptUnicodeEscapes(t *testing.T) {
	if _, ok := mustCompile(t, `^[\u0020-\u007E\uFF61-\uFF9F]`).Find("abc"); !ok {
		t.Fatal("JavaScript形式のUnicodeエスケープがASCIIに一致しません")
	}
	pattern := `^[\u3005\u3007\u4E00-\u9FFF]|[\uD840-\uD87F][\uDC00-\uDFFF]`
	if _, ok := mustCompile(t, pattern).Find("色"); !ok {
		t.Fatal("Unicodeエスケープを含む漢字範囲に一致しません")
	}
}

// partTexts は区切りの断片を文字列の並びにする。undefinedの断片は 『undefined』
// と書いて、捕獲が欠けたことを失敗メッセージで見えるようにする。
func partTexts(parts []re.SplitPart) []string {
	texts := make([]string, len(parts))
	for i, part := range parts {
		if part.Undefined {
			texts[i] = "undefined"
			continue
		}
		texts[i] = part.Text
	}
	return texts
}
