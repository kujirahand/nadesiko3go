package stdlib_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// TestURLParameterParsing は本家 TypeScript 版 #2493 の修正と照合した
// URLパラメータ解析の回帰テストである。値に ? や = を含む場合や
// 空値、URLエンコード、フラグメント、重複キーなどを確認する。
func TestURLParameterParsing(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()

	e, ok := r.Lookup("URLパラメータ解析")
	if !ok || e.Fn == nil {
		t.Fatal("URLパラメータ解析 が登録されていない")
	}

	lookup := func(url string, key string) string {
		got, err := e.Fn(ctx, []value.Value{value.String(url)})
		if err != nil {
			t.Fatalf("URLパラメータ解析(%q): %v", url, err)
		}
		d, ok := got.Dict()
		if !ok {
			t.Fatalf("URLパラメータ解析(%q) = %v, want dict", url, got.Kind())
		}
		v, exists := d.Get(key)
		if !exists {
			return ""
		}
		return value.ToString(v)
	}

	tests := []struct {
		url  string
		key  string
		want string
	}{
		{"http://hoge.com/", "a", ""},
		{"https://nadesi.com/?a=3&b=5", "a", "3"},
		{"https://nadesi.com/?a=3&b=5", "b", "5"},
		{"https://nadesi.com/?a=3=3&b=5", "a", "3=3"},
		{"https://nadesi.com/?a=3=3&b=5", "b", "5"},
		{"https://nadesi.com/?a&b=5", "a", ""},
		{"https://nadesi.com/?a&b=5", "b", "5"},
		{"https://nadesi.com/?next=a?b", "next", "a?b"},
		{"https://nadesi.com/?token=a=b", "token", "a=b"},
		{"https://nadesi.com/?a=1#frag", "a", "1"},
		{"https://nadesi.com/?a=1&a=2", "a", "2"},
		{"https://nadesi.com/?a=hello+world", "a", "hello world"},
		{"https://nadesi.com/?a=%ZZ", "a", "%ZZ"},
		{"https://nadesi.com/?__proto__=x", "__proto__", "x"},
		{"https://nadesi.com/?a=", "a", ""},
		{"https://nadesi.com/?a", "a", ""},
		{"https://nadesi.com/?a=%2B", "a", "+"},
		{"https://nadesi.com/p#frag?a=1", "a", ""},
		{"https://nadesi.com/?=x", "", "x"},
		{"?x=+", "x", " "},
		{"https://nadesi.com/?a=%FF", "a", "�"},
		{"https://nadesi.com/?a=%C0%AF", "a", "�"},
	}

	for _, tt := range tests {
		if got := lookup(tt.url, tt.key); got != tt.want {
			t.Errorf("URLパラメータ解析(%q)[%q] = %q, want %q", tt.url, tt.key, got, tt.want)
		}
	}
}

// TestURLParameterParsingEmptyQuery はクエリが無い・空の場合に
// 空の辞書を返すことを確認する。
func TestURLParameterParsingEmptyQuery(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()
	e, _ := r.Lookup("URLパラメータ解析")

	for _, url := range []string{
		"http://hoge.com/",
		"https://nadesi.com/?",
		"https://nadesi.com/?&",
		"https://nadesi.com/p#frag?a=1",
	} {
		got, err := e.Fn(ctx, []value.Value{value.String(url)})
		if err != nil {
			t.Fatalf("URLパラメータ解析(%q): %v", url, err)
		}
		d, ok := got.Dict()
		if !ok {
			t.Fatalf("URLパラメータ解析(%q) = %v, want dict", url, got.Kind())
		}
		if d.Len() != 0 {
			t.Errorf("URLパラメータ解析(%q) の要素数 = %d, want 0", url, d.Len())
		}
	}
}
