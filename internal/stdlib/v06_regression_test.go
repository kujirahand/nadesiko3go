package stdlib_test

import (
	"reflect"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// v06Call runs one command through the registry, the way the VM does.
func v06Call(t *testing.T, name string, args ...value.Value) (value.Value, error) {
	t.Helper()
	r := stdlib.NewRegistry()
	entry, ok := r.Lookup(name)
	if !ok {
		t.Fatalf("命令『%s』が登録されていない", name)
	}
	return entry.Fn(newContext(), args)
}

// v06Items renders an array as texts, spelling an undefined item as
// 『undefined』 so a missing capture is visible in the failure message.
func v06Items(t *testing.T, v value.Value) []string {
	t.Helper()
	arr, ok := v.Array()
	if !ok {
		t.Fatalf("配列ではない: kind=%v value=%q", v.Kind(), value.ToString(v))
	}
	items := make([]string, arr.Len())
	for i := 0; i < arr.Len(); i++ {
		item := arr.Get(i)
		if item.Kind() == value.KindUndefined {
			items[i] = "undefined"
			continue
		}
		s, ok := item.String()
		if !ok {
			t.Fatalf("%d番目の要素が文字列ではない: kind=%v", i, item.Kind())
		}
		items[i] = s
	}
	return items
}

// TestV06SplitKeepsCaptures pins JavaScript's String.prototype.split: capture
// groups sit between the segments, and a group that did not take part is
// undefined rather than an empty string.
func TestV06SplitKeepsCaptures(t *testing.T) {
	tests := []struct {
		name, input, pattern string
		want                 []string
	}{
		{"区切りを捕獲する", "a,b", "/(,)/", []string{"a", ",", "b"}},
		{"未参加の組はundefined", "a1b2c", `/(\d)|(x)/`, []string{"a", "1", "undefined", "b", "2", "undefined", "c"}},
		{"空マッチは区切りにしない", "abc", "/(?:)/", []string{"a", "b", "c"}},
		{"空マッチのあと前進する", "abc", "/b*/", []string{"a", "c"}},
		{"行頭の空マッチは区切らない", "abc", "/^/", []string{"abc"}},
		{"空文字と空パターン", "", "/(?:)/", []string{}},
		{"空文字と一致しないパターン", "", "/x/", []string{""}},
		{"日本語のrune基準", "あいう", "/(い)/", []string{"あ", "い", "う"}},
		{"捕獲のない区切りは従来どおり", "a1b22c", "/[0-9]+/", []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v06Call(t, "正規表現区切", value.String(tt.input), value.String(tt.pattern))
			if err != nil {
				t.Fatalf("正規表現区切(%q, %q): %v", tt.input, tt.pattern, err)
			}
			if items := v06Items(t, got); !reflect.DeepEqual(items, tt.want) {
				t.Errorf("正規表現区切(%q, %q) = %v, want %v", tt.input, tt.pattern, items, tt.want)
			}
		})
	}
}

// TestV06StickyMatch pins the 『y』 flag: every match has to start where the
// previous one ended, so a fresh match starts at the beginning of the subject.
func TestV06StickyMatch(t *testing.T) {
	tests := []struct{ name, input, pattern, want string }{
		{"先頭でなければ一致しない", "ab", "/b/y", "null"},
		{"先頭なら一致する", "ba", "/b/y", "b"},
		{"g+yは連続する範囲だけ", "abab", "/ab/gy", "ab,ab"},
		{"g+yは途切れたら止まる", "ab ab", "/ab/gy", "ab"},
		{"gだけなら離れていても一致する", "ab ab", "/ab/g", "ab,ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v06Call(t, "正規表現マッチ", value.String(tt.input), value.String(tt.pattern))
			if err != nil {
				t.Fatalf("正規表現マッチ(%q, %q): %v", tt.input, tt.pattern, err)
			}
			if text := value.ToString(got); text != tt.want {
				t.Errorf("正規表現マッチ(%q, %q) = %q, want %q", tt.input, tt.pattern, text, tt.want)
			}
		})
	}
}

// TestV06StickyReplace pins that a sticky replacement only touches a match
// that starts where the previous one ended, so 『/a/y』 on 『ba』 changes nothing.
func TestV06StickyReplace(t *testing.T) {
	tests := []struct{ name, input, pattern, template, want string }{
		{"先頭でなければ置換しない", "ba", "/a/y", "X", "ba"},
		{"先頭なら置換する", "aa", "/a/y", "X", "Xa"},
		{"g+yは連続する範囲だけ", "aba", "/a/gy", "X", "Xba"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []value.Value{value.String(tt.input), value.String(tt.pattern), value.String(tt.template)}
			got, err := v06Call(t, "正規表現置換", args...)
			if err != nil {
				t.Fatalf("正規表現置換(%q, %q, %q): %v", tt.input, tt.pattern, tt.template, err)
			}
			if text := value.ToString(got); text != tt.want {
				t.Errorf("正規表現置換(%q, %q, %q) = %q, want %q", tt.input, tt.pattern, tt.template, text, tt.want)
			}
		})
	}
}

// TestV06StickyNullableCommands pins the command results for a nullable sticky
// pattern. Go's FindAll drops the empty match right after a match, so a sticky
// search that trusts it stops too early; the values here are Node's.
func TestV06StickyNullableCommands(t *testing.T) {
	tests := []struct {
		name, input, pattern string
		want                 []string
	}{
		{"区切りの直後の空一致", "ab", "/a*|b/gy", []string{"a", "", ""}},
		{"空一致の連続", "abc", "/b*/gy", []string{"", "b", "", ""}},
		{"空一致だけのパターン", "abc", "/(?:)/gy", []string{"", "", "", ""}},
		{"末尾の空一致", "aaa", "/a*/gy", []string{"aaa", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v06Call(t, "正規表現マッチ", value.String(tt.input), value.String(tt.pattern))
			if err != nil {
				t.Fatalf("正規表現マッチ(%q, %q): %v", tt.input, tt.pattern, err)
			}
			if items := v06Items(t, got); !reflect.DeepEqual(items, tt.want) {
				t.Errorf("正規表現マッチ(%q, %q) = %v, want %v", tt.input, tt.pattern, items, tt.want)
			}
		})
	}
}

// TestV06StickyReplaceKeepsRest pins that a sticky replacement touches only
// the contiguous matches from the beginning and leaves the rest of the subject
// alone, including the capture groups of a match that starts after the first.
func TestV06StickyReplaceKeepsRest(t *testing.T) {
	tests := []struct{ name, input, pattern, template, want string }{
		{"一致しない残りはそのまま", "ab ab", "/ab/gy", "X", "X ab"},
		{"連続した一致", "aab", "/a/gy", "X", "XXb"},
		{"空一致を含む置換", "ab", "/a*|b/gy", "X", "XXbX"},
		{"2件目以降の部分一致", "1a2b", `/(\d)([a-z])/gy`, "X$1", "X1X2"},
		{"日本語の部分一致", "いあい", "/(い)/gy", "<$1>", "<い>あい"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []value.Value{value.String(tt.input), value.String(tt.pattern), value.String(tt.template)}
			got, err := v06Call(t, "正規表現置換", args...)
			if err != nil {
				t.Fatalf("正規表現置換(%q, %q, %q): %v", tt.input, tt.pattern, tt.template, err)
			}
			if text := value.ToString(got); text != tt.want {
				t.Errorf("正規表現置換(%q, %q, %q) = %q, want %q", tt.input, tt.pattern, tt.template, text, tt.want)
			}
		})
	}
}

// TestV06StickyExtract pins the capture list of 正規表現抽出 for sticky matches.
func TestV06StickyExtract(t *testing.T) {
	got, err := v06Call(t, "正規表現抽出", value.String("1a2b"), value.String(`/(\d)([a-z])/gy`))
	if err != nil {
		t.Fatalf("正規表現抽出: %v", err)
	}
	if items := v06Items(t, got); !reflect.DeepEqual(items, []string{"1", "a", "2", "b"}) {
		t.Errorf("正規表現抽出 = %v, want [1 a 2 b]", items)
	}
}

// TestV06DuplicateFlags pins that a repeated flag is a language error. A fresh
// RegExp in JavaScript throws a SyntaxError; accepting 『/a/gg』 silently
// changes the meaning of the pattern.
func TestV06DuplicateFlags(t *testing.T) {
	if _, err := v06Call(t, "正規表現マッチ", value.String("a"), value.String("/a/gg")); err == nil {
		t.Fatal("『/a/gg』はエラーになるべき")
	} else if want := "正規表現『/a/gg』のフラグ『g』が重複しています。"; err.Error() != want {
		t.Errorf("Error()\n got %q\nwant %q", err.Error(), want)
	}
}

// TestV06InvalidFlags pins that a flag JavaScript does not know is an error
// instead of being ignored.
func TestV06InvalidFlags(t *testing.T) {
	if _, err := v06Call(t, "正規表現マッチ", value.String("a"), value.String("/a/zz")); err == nil {
		t.Fatal("『/a/zz』はエラーになるべき")
	} else if want := "正規表現『/a/zz』のフラグ『z』は使えません。"; err.Error() != want {
		t.Errorf("Error()\n got %q\nwant %q", err.Error(), want)
	}
}

// TestV06ConflictingFlags pins that 『u』 and 『v』 cannot be written together,
// as in JavaScript, and that each one alone is still accepted. This is input
// validation, so it does not depend on the engine supporting Unicode sets.
func TestV06ConflictingFlags(t *testing.T) {
	tests := []struct{ pattern, want string }{
		{"/a/uv", "正規表現『/a/uv』のフラグ『u』と『v』は同時に使えません。"},
		{"/a/vu", "正規表現『/a/vu』のフラグ『v』と『u』は同時に使えません。"},
		{"/a/guv", "正規表現『/a/guv』のフラグ『u』と『v』は同時に使えません。"},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if _, err := v06Call(t, "正規表現マッチ", value.String("a"), value.String(tt.pattern)); err == nil {
				t.Fatalf("『%s』はエラーになるべき", tt.pattern)
			} else if err.Error() != tt.want {
				t.Errorf("Error()\n got %q\nwant %q", err.Error(), tt.want)
			}
		})
	}
	for _, pattern := range []string{"/a/u", "/a/v"} {
		if _, err := v06Call(t, "正規表現マッチ", value.String("a"), value.String(pattern)); err != nil {
			t.Errorf("『%s』は通るべき: %v", pattern, err)
		}
	}
}
