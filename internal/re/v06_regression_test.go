package re_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/re"
)

// TestV06SplitParts pins the data shape a split returns: the segments and the
// capture groups interleaved, with a missing capture marked undefined instead
// of being flattened to an empty string.
func TestV06SplitParts(t *testing.T) {
	tests := []struct {
		pattern, input string
		want           []string
	}{
		{"/(,)/", "a,b", []string{"a", ",", "b"}},
		{`/(\d)|(x)/`, "a1b2c", []string{"a", "1", "undefined", "b", "2", "undefined", "c"}},
		{"/(?:)/", "abc", []string{"a", "b", "c"}},
		{"/b*/", "abc", []string{"a", "c"}},
		{"/^/", "abc", []string{"abc"}},
		{"/(?:)/", "", []string{}},
		{"/x/", "", []string{""}},
		// sticky と global は区切りの結果を変えない
		{"/(,)/y", "a,b", []string{"a", ",", "b"}},
		{"/(,)/g", "a,b", []string{"a", ",", "b"}},
		{"/(a)(b)/", "abab", []string{"", "a", "b", "", "a", "b", ""}},
		{"/(い)/", "あいう", []string{"あ", "い", "う"}},
	}
	for _, tt := range tests {
		r, err := re.Compile(tt.pattern)
		if err != nil {
			t.Fatalf("Compile(%q): %v", tt.pattern, err)
		}
		if got := partTexts(r.Split(tt.input)); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Split(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
		}
	}
}

// TestV06CaptureParticipation pins that Find tells a group that did not take
// part apart from a group that matched the empty string.
func TestV06CaptureParticipation(t *testing.T) {
	r, err := re.Compile(`/(a)|(b)/`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	match, ok := r.Find("b")
	if !ok {
		t.Fatal("Find(b) = no match, want a match")
	}
	if got := len(match.Groups); got != 2 {
		t.Fatalf("組の数 = %d, want 2", got)
	}
	if match.Groups[0].Ok || match.Groups[0].Text != "" {
		t.Errorf("未参加の組 = %#v, want Ok=false", match.Groups[0])
	}
	if !match.Groups[1].Ok || match.Groups[1].Text != "b" {
		t.Errorf("参加した組 = %#v, want text \"b\"", match.Groups[1])
	}
}

// TestV06StickyFind pins the 『y』 flag: a match has to start where the
// previous one ended, and the first one starts at the beginning.
func TestV06StickyFind(t *testing.T) {
	if _, ok := mustCompile(t, "/b/y").Find("ab"); ok {
		t.Error(`/b/y は "ab" の先頭で一致しないので、一致なしになるべき`)
	}
	if match, ok := mustCompile(t, "/b/y").Find("ba"); !ok || match.Text != "b" {
		t.Errorf(`/b/y の "ba" = %#v (ok=%v), want "b"`, match, ok)
	}
	if got, want := mustCompile(t, "/ab/gy").FindAll("abab"), []string{"ab", "ab"}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindAll = %v, want %v", got, want)
	}
	// 途切れたらそこで止まる。空の一致のあとは1文字ぶん進む。
	if got, want := mustCompile(t, "/ab/gy").FindAll("ab ab"), []string{"ab"}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindAll = %v, want %v", got, want)
	}
	if got, want := mustCompile(t, "/(?:)/gy").FindAll("abc"), []string{"", "", "", ""}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindAll = %q, want %q", got, want)
	}
	if got, want := mustCompile(t, "/ab/g").FindAll("ab ab"), []string{"ab", "ab"}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindAll = %v, want %v", got, want)
	}
}

// TestV06StickyReplace pins that a sticky replacement only touches the
// contiguous run of matches from the beginning.
func TestV06StickyReplace(t *testing.T) {
	tests := []struct{ pattern, input, want string }{
		{"/a/y", "ba", "ba"},
		{"/a/y", "aa", "Xa"},
		{"/a/gy", "aba", "Xba"},
	}
	for _, tt := range tests {
		if got := mustCompile(t, tt.pattern).Replace(tt.input, "X"); got != tt.want {
			t.Errorf("Replace(%q, %q) = %q, want %q", tt.pattern, tt.input, got, tt.want)
		}
	}
}

// TestV06FlagErrors pins that a repeated, unknown or conflicting flag is
// reported with the offending letter, which is what the language error names.
func TestV06FlagErrors(t *testing.T) {
	_, err := re.Compile("/a/gg")
	var flagErr *re.FlagError
	if !errors.As(err, &flagErr) || !flagErr.Duplicate || flagErr.Flag != 'g' {
		t.Errorf(`Compile("/a/gg") = %v, want duplicate g`, err)
	}
	_, err = re.Compile("/a/zz")
	if !errors.As(err, &flagErr) || flagErr.Duplicate || flagErr.Flag != 'z' {
		t.Errorf(`Compile("/a/zz") = %v, want unknown z`, err)
	}
	// 『u』と『v』はどちらもUnicodeモードを選ぶので、同時には書けない
	_, err = re.Compile("/a/uv")
	if !errors.As(err, &flagErr) || flagErr.Duplicate || flagErr.Flag != 'u' || flagErr.Conflict != 'v' {
		t.Errorf(`Compile("/a/uv") = %v, want u and v in conflict`, err)
	}
	_, err = re.Compile("/a/vu")
	if !errors.As(err, &flagErr) || flagErr.Duplicate || flagErr.Flag != 'v' || flagErr.Conflict != 'u' {
		t.Errorf(`Compile("/a/vu") = %v, want v and u in conflict`, err)
	}
	// JavaScriptが受け付けるフラグはそのまま通す
	for _, pattern := range []string{"/a/dgimsy", "/a/i", "/a/gy", "/a/u", "/a/v"} {
		if _, err := re.Compile(pattern); err != nil {
			t.Errorf("Compile(%q) = %v, want no error", pattern, err)
		}
	}
}
