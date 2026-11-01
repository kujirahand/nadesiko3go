package stdlib_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// v06Call はVMと同じように、レジストリ越しに命令を1つ実行する。
func v06Call(t *testing.T, name string, args ...value.Value) (value.Value, error) {
	t.Helper()
	r := stdlib.NewRegistry()
	entry, ok := r.Lookup(name)
	if !ok {
		t.Fatalf("命令『%s』が登録されていない", name)
	}
	return entry.Fn(newContext(), args)
}

// v06Items は配列を文字列の並びにする。undefinedの要素は 『undefined』 と書いて、
// 捕獲が欠けたことを失敗メッセージで見えるようにする。
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

// TestV06SplitKeepsCaptures はJavaScriptの String.prototype.split を固定する。
// 捕獲は区切りの断片の間に並び、参加しなかった組は空文字ではなくundefinedに
// なる。
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

// TestV06StickyMatch は 『y』 フラグを固定する。どの一致も前の一致が終わった位置
// から始まる必要があり、最初の一致は対象の先頭から探す。
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

// TestV06StickyReplace は、stickyの置換が前の一致の終端から始まる一致だけを
// 対象にすることを固定する。『ba』 に対する 『/a/y』 は何も変えない。
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

// TestV06StickyNullableCommands は、空一致し得るstickyのパターンで命令が返す値を
// 固定する。GoのFindAllは一致の直後の空一致を落とすので、それを信じるsticky
// 検索は途中で止まる。ここに並ぶ期待値はNodeの結果。
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

// TestV06StickyReplaceKeepsRest は、stickyの置換が先頭から連続する一致だけを
// 対象にし、残りの文字をそのまま残すことを固定する。2件目以降の一致の捕獲も
// 同じように扱う。
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

// TestV06StickyExtract は、stickyの一致に対する 正規表現抽出 の捕獲一覧を固定する。
func TestV06StickyExtract(t *testing.T) {
	got, err := v06Call(t, "正規表現抽出", value.String("1a2b"), value.String(`/(\d)([a-z])/gy`))
	if err != nil {
		t.Fatalf("正規表現抽出: %v", err)
	}
	if items := v06Items(t, got); !reflect.DeepEqual(items, []string{"1", "a", "2", "b"}) {
		t.Errorf("正規表現抽出 = %v, want [1 a 2 b]", items)
	}
}

// TestV06DuplicateFlags は、重複したフラグを言語のエラーにすることを固定する。
// JavaScriptでは新しいRegExpがSyntaxErrorを投げる。『/a/gg』 を無言で受理すると
// 書いた意図と違うパターンが動く。
func TestV06DuplicateFlags(t *testing.T) {
	if _, err := v06Call(t, "正規表現マッチ", value.String("a"), value.String("/a/gg")); err == nil {
		t.Fatal("『/a/gg』はエラーになるべき")
	} else if want := "正規表現『/a/gg』のフラグ『g』が重複しています。"; err.Error() != want {
		t.Errorf("Error()\n got %q\nwant %q", err.Error(), want)
	}
}

// TestV06InvalidFlags は、JavaScriptが知らないフラグを無視せずエラーにすることを
// 固定する。
func TestV06InvalidFlags(t *testing.T) {
	if _, err := v06Call(t, "正規表現マッチ", value.String("a"), value.String("/a/zz")); err == nil {
		t.Fatal("『/a/zz』はエラーになるべき")
	} else if want := "正規表現『/a/zz』のフラグ『z』は使えません。"; err.Error() != want {
		t.Errorf("Error()\n got %q\nwant %q", err.Error(), want)
	}
}

// TestV06ConflictingFlags は、JavaScriptと同じく 『u』 と 『v』 を同時に書けず、
// 片方だけなら受理されることを固定する。入力の検証なので、エンジンがUnicode
// 集合演算に対応しているかどうかとは切り離してある。
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

// TestV06ClosedQuoteCommands は、Goの 『\Q...\E』 を閉じたまま含むパターンを
// 命令の入口から固定する。期待値はimmutable archiveのe526/baseが返した成功
// literalで、920は同じ入口でpanicしていた。
func TestV06ClosedQuoteCommands(t *testing.T) {
	tests := []struct {
		name, input, pattern string
		want                 []string
	}{
		{"閉じた引用のoptional", "a", `/(?:\Q\\E)?/gy`, []string{"", ""}},
		{"閉じた引用と語境界", `\a`, `/\Q\\E\b/gy`, []string{`\`}},
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
	// 名前付き組でも、閉じた引用の位置指定照合はpanicしない
	got, err := v06Call(t, "正規表現抽出", value.String("a"), value.String(`/(?<n>(?:\Q\\E)?)/gy`))
	if err != nil {
		t.Fatalf("正規表現抽出: %v", err)
	}
	if items := v06Items(t, got); !reflect.DeepEqual(items, []string{"", ""}) {
		t.Errorf("正規表現抽出 = %v, want [空 空]", items)
	}
}

// TestV06SuffixCaptureCommands は、context-freeの位置指定照合が返す一致全体と
// 捕獲の位置を、命令の戻り値と置換テンプレートで固定する。期待値はNodeの
// 実行結果で、未参加の組は空文字の参加と区別できる。
func TestV06SuffixCaptureCommands(t *testing.T) {
	got, err := v06Call(t, "正規表現置換", value.String("あいう"), value.String("/(あ)?(?<n>い?)/gy"), value.String("<$1:$<n>>"))
	if err != nil {
		t.Fatalf("正規表現置換: %v", err)
	}
	if text := value.ToString(got); text != "<あ:い><:>う<:>" {
		t.Errorf("正規表現置換 = %q, want %q", text, "<あ:い><:>う<:>")
	}
}

// TestV06DeepContextLanguageError は、元のパターンは正しいが文脈付きの位置
// 指定照合にだけ使えない深さの入力を、言語のエラーとして返すことを固定する。
// 未受入の残余制約なので、一致なしへ潰さず、パターン不正とも言わない。
func TestV06DeepContextLanguageError(t *testing.T) {
	body := strings.Repeat("(", 997) + "a?$" + strings.Repeat(")", 997)
	pattern := "/" + body + "/gy"
	_, err := v06Call(t, "正規表現マッチ", value.String("a"), value.String(pattern))
	if err == nil {
		t.Fatal("深い文脈付きパターンはエラーになるべき")
	}
	if want := "正規表現『" + pattern + "』はyフラグの位置指定照合には深すぎます。"; err.Error() != want {
		t.Errorf("Error()\n got %q\nwant %q", err.Error(), want)
	}
	// 同じパターンでもgだけなら成功する
	got, err := v06Call(t, "正規表現マッチ", value.String("a"), value.String("/"+body+"/g"))
	if err != nil {
		t.Fatalf("gだけの正規表現マッチ: %v", err)
	}
	if items := v06Items(t, got); !reflect.DeepEqual(items, []string{"a"}) {
		t.Errorf("gだけの正規表現マッチ = %v, want [a]", items)
	}
}
