package re_test

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/re"
)

// TestV06SplitParts は区切りが返すデータの形を固定する。区切りの断片と捕獲が
// 交互に並び、参加しなかった捕獲は空文字へ潰さずundefinedとして印を付ける。
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

// TestV06CaptureParticipation は、Find が参加しなかった組と空文字に一致した組を
// 区別できることを固定する。
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

// TestV06StickyFind は 『y』 フラグを固定する。一致は前の一致が終わった位置から
// 始まる必要があり、最初の一致は先頭から探す。
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

// TestV06StickyReplace は、stickyの置換が先頭から連続する一致だけを対象にする
// ことを固定する。
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

// TestV06StickyNullable は、一致の直後にある空一致も1件として数えることを
// 固定する。GoのFindAllはその空一致を落とすので、その一致列から始めるsticky
// 検索は途中で止まってしまう。ここに並ぶ期待値はNodeの結果。
func TestV06StickyNullable(t *testing.T) {
	tests := []struct {
		pattern, input string
		want           []string
	}{
		{"/a*|b/gy", "ab", []string{"a", "", ""}},
		{"/b*/gy", "abc", []string{"", "b", "", ""}},
		{"/(?:)/gy", "abc", []string{"", "", "", ""}},
		{"/a*/gy", "aaa", []string{"aaa", ""}},
		{"/a?/gy", "abc", []string{"a", "", "", ""}},
		{"/a|/gy", "abc", []string{"a", "", "", ""}},
		{"/x?/gy", "abc", []string{"", "", "", ""}},
		{`/\d*/gy`, "a1b", []string{"", "1", "", ""}},
	}
	for _, tt := range tests {
		if got := mustCompile(t, tt.pattern).FindAll(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FindAll(%q, %q) = %q, want %q", tt.pattern, tt.input, got, tt.want)
		}
	}
}

// TestV06StickyContext は、先頭以外から始まる一致でも ^ $ \b \B が前後の文字を
// 見ていることを固定する。一致位置で対象を切り出すとその文脈が失われる。
func TestV06StickyContext(t *testing.T) {
	tests := []struct {
		pattern, input string
		want           []string
	}{
		{"/^a/gy", "a a", []string{"a"}},                 // 2件目の^は行頭ではない
		{"/a$/gy", "a a", nil},                           // 先頭のaは$に届かない
		{"/a$/gy", "a", []string{"a"}},                   // 末尾なら一致する
		{"/a$/mgy", "a\nb", []string{"a"}},               // mの$は改行の手前
		{`/\b./gy`, "a b", []string{"a", " ", "b"}},      // 位置1,2の\bは前の文字で決まる
		{`/a\B/gy`, "aab", []string{"a", "a"}},           // 2件目の\Bは後ろの文字で決まる
		{`/\Ba/gy`, "aab", nil},                          // 先頭の\Bは境界なので一致しない
		{`/^\w\n/mgy`, "a\nb\n", []string{"a\n", "b\n"}}, // 改行直後の^を保つ
		{"/a|b$/gy", "ab", []string{"a", "b"}},           // 2件目の$は文字列の終端
	}
	for _, tt := range tests {
		if got := mustCompile(t, tt.pattern).FindAll(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FindAll(%q, %q) = %q, want %q", tt.pattern, tt.input, got, tt.want)
		}
	}
}

// TestV06StickyCaptureAndReplace は、先頭以外から始まるstickyの一致で捕獲の
// 位置がずれないことと、どの一致も消費しなかった文字がそのまま残ることを
// 固定する。
func TestV06StickyCaptureAndReplace(t *testing.T) {
	tests := []struct {
		pattern, input, template, want string
	}{
		{`/(\d)([a-z])/gy`, "1a2b", "X$1", "X1X2"},
		{"/(a)(b)/gy", "abab", "$2$1", "baba"},
		{"/(い)/gy", "いあい", "<$1>", "<い>あい"},
		{"/a/gy", "aab", "X", "XXb"},
		{"/ab/gy", "ab ab", "X", "X ab"},
		{"/a*|b/gy", "ab", "X", "XXbX"},
	}
	for _, tt := range tests {
		if got := mustCompile(t, tt.pattern).Replace(tt.input, tt.template); got != tt.want {
			t.Errorf("Replace(%q, %q, %q) = %q, want %q", tt.pattern, tt.input, tt.template, got, tt.want)
		}
	}
}

// TestV06StickyRuneBased はJavaScriptとの意図的なrune基準の差を固定する。対象は
// UTF-16コード単位ではなくruneでたどる。Nodeに 『u』 フラグを付けるとこの値に
// 一致し、付けない場合はBMP外の文字が2つに分かれる（AGENTS.md §5）。
func TestV06StickyRuneBased(t *testing.T) {
	tests := []struct {
		pattern, input string
		want           []string
	}{
		{"/./gy", "𩸽あ", []string{"𩸽", "あ"}},
		{"/(?:)/gy", "𩸽あ", []string{"", "", ""}},
		{"/a*/gy", "𩸽あ", []string{"", "", ""}},
	}
	for _, tt := range tests {
		if got := mustCompile(t, tt.pattern).FindAll(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FindAll(%q, %q) = %q, want %q", tt.pattern, tt.input, got, tt.want)
		}
	}
}

// TestV06FlagErrors は、重複・未知・同時指定不可のフラグを、問題の文字を添えて
// 報告することを固定する。言語のエラー文面もその文字を示す。
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

// TestV06StickyNonEmptyFastPath は、空一致し得ないstickyの高速経路を固定する。
// そのときGoのFindAll走査は完全なので、先頭からの連続だけを確かめればよい。
func TestV06StickyNonEmptyFastPath(t *testing.T) {
	tests := []struct {
		pattern, input string
		want           []string
	}{
		{"/a+/gy", "aaab", []string{"aaa"}},
		{`/\w+/gy`, "ab cd", []string{"ab"}},
		{`/([a-z])(\d)/gy`, "a1b2c3", []string{"a1", "b2", "c3"}},
		{"/a+\\b/gy", "aa bb", []string{"aa"}}, // \bを含むので位置指定側
	}
	for _, tt := range tests {
		if got := mustCompile(t, tt.pattern).FindAll(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FindAll(%q, %q) = %q, want %q", tt.pattern, tt.input, got, tt.want)
		}
	}
}

// TestV06StickyEmptySubject は空の対象での境界を固定する。そこでは 『\b』 だけが
// 偽になるゼロ幅アサーションで、そのため位置指定の照合を残している。
func TestV06StickyEmptySubject(t *testing.T) {
	tests := []struct {
		pattern, input string
		want           []string
	}{
		{"/(?:)/gy", "", []string{""}},
		{"/a*/gy", "", []string{""}},
		{"/a/gy", "", nil},
		{"/^/gy", "", []string{""}},
		{"/$/gy", "", []string{""}},
		{`/\B/gy`, "", []string{""}},
		{`/\b/gy`, "", nil},
	}
	for _, tt := range tests {
		if got := mustCompile(t, tt.pattern).FindAll(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FindAll(%q, %q) = %q, want %q", tt.pattern, tt.input, got, tt.want)
		}
	}
}

// TestV06StickyWordBoundaryIsPositional は証明の反例を固定する。『a+|\b』 は空の
// 対象では一致しないが、文脈の中では空一致する。『\B』 は空の対象でも一致する。
// どちらも位置指定の照合を使うべきで、ここに並ぶ期待値はNodeの結果。
func TestV06StickyWordBoundaryIsPositional(t *testing.T) {
	tests := []struct {
		pattern, input string
		want           []string
	}{
		{`/a+|\b/gy`, "a b", []string{"a", "", "", ""}},
		{`/a|\B/gy`, "aab", []string{"a", "a", ""}},
		{`/\b/gy`, "a b", []string{"", "", "", ""}},
		{`/a+\b/gy`, "aa bb", []string{"aa"}},
	}
	for _, tt := range tests {
		if got := mustCompile(t, tt.pattern).FindAll(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FindAll(%q, %q) = %q, want %q", tt.pattern, tt.input, got, tt.want)
		}
	}
}

// TestV06StickyConcurrentReuse は、1つのコンパイル済みパターンを複数のgoroutine
// から同時に使えることを固定する。派生パターンは sync.Once の下で1度だけ作る。
func TestV06StickyConcurrentReuse(t *testing.T) {
	positional := mustCompile(t, "/a*|b/gy")
	fast := mustCompile(t, "/a+/gy")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if got, want := positional.FindAll("ab"), []string{"a", "", ""}; !reflect.DeepEqual(got, want) {
					t.Errorf("位置指定検索のFindAll = %q, want %q", got, want)
					return
				}
				if got, want := fast.FindAll("aaab"), []string{"aaa"}; !reflect.DeepEqual(got, want) {
					t.Errorf("走査経路のFindAll = %q, want %q", got, want)
					return
				}
				positional.Replace("ab", "X")
				fast.FindAllMatches("aaab")
			}
		}()
	}
	wg.Wait()
}

// TestV06StickyTextScanBoundary は、一致文字列だけで連続を確かめる経路の境界を
// 固定する。文脈を見るアサーションを含むパターンはこの経路を使わず、従来の
// 位置列に戻る。
func TestV06StickyTextScanBoundary(t *testing.T) {
	tests := []struct {
		pattern, input string
		want           []string
	}{
		// 文字列の前置きだけを見ると誤る反例（$ があるので位置列の経路）
		{"/abc$/gy", "abcabc", nil},
		{"/^a/gy", "aa", []string{"a"}},
		{`/\ba/gy`, "a a", []string{"a"}},
		// アサーションを含まないパターンは一致文字列の連続で判定できる
		{`/\w+/gy`, "ab cd", []string{"ab"}},
		{"/[0-9]+/gy", "20240102", []string{"20240102"}},
		{"/[0-9]/gy", "20240102", []string{"2", "0", "2", "4", "0", "1", "0", "2"}},
		{"/[a-z]+/gy", "abc123def", []string{"abc"}},
	}
	for _, tt := range tests {
		if got := mustCompile(t, tt.pattern).FindAll(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FindAll(%q, %q) = %q, want %q", tt.pattern, tt.input, got, tt.want)
		}
	}
}

// TestV06StickyTextScanRetention は、一致文字列の連続判定で途中に途切れがある
// とき、走査結果の大きなbackingを返り値が保持しないことを固定する。結果の
// literalと保持容量は別に確かめる。全件連続で詰め直さないことは、同じpackageの
// TestV06StickyFromTextsNoCopy が置き場所の同一性で確かめる。
func TestV06StickyTextScanRetention(t *testing.T) {
	middle := "aX" + strings.Repeat("a", 65536)
	first := "X" + strings.Repeat("a", 65536)
	r := mustCompile(t, "/a/gy")

	// 途中で途切れる: 結果は先頭の1件、容量はその1件分まで
	if got := r.FindAll(middle); !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("途中で途切れるFindAll = %q, want [a]", got)
	}
	if got := r.FindAll(middle); cap(got) > 1 {
		t.Errorf("途中で途切れるFindAllの容量 = %d, want 1以下", cap(got))
	}

	// 先頭で途切れる: nil
	if got := r.FindAll(first); got != nil {
		t.Errorf("先頭で途切れるFindAll = %v, want nil", got)
	}

	// 最後まで連続する: 全件返す
	if got := r.FindAll(strings.Repeat("a", 100)); len(got) != 100 {
		t.Errorf("全部連続するFindAllの件数 = %d, want 100", len(got))
	}
}

// TestV06StickyQuotedPatternDoesNotPanic は、Goの 『\Q』 のように後ろを
// リテラル扱いにする構文を含むstickyパターンでも、位置指定照合の変種を作る
// ところでpanicしないことを固定する。Compile時に作るので、照合中に
// MustCompileへ到達しない。
func TestV06StickyQuotedPatternDoesNotPanic(t *testing.T) {
	r, err := re.Compile("/\\Q/gy")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// Goの正規表現では 『\Q』 だけで空に一致するパターンになる
	if got, want := r.FindAll("abc"), []string{"", "", "", ""}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindAll = %q, want %q", got, want)
	}
	if got := r.FindAllMatches("abc"); len(got) != 4 {
		t.Errorf("FindAllMatchesの件数 = %d, want 4", len(got))
	}
	// 末尾の一致も位置指定の照合を通る
	if got, want := r.FindAll("a"), []string{"", ""}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindAll(a) = %q, want %q", got, want)
	}
	// 引用の閉じ方（閉じた引用、エスケープされた円記号、開いた引用の連続）で
	// 結果が変わらないこと。期待値は修正前の e526ff8 と一致する。
	variants := []struct {
		pattern, input string
		want           []string
	}{
		{`/\Qa\E\Qb/gy`, "ab", []string{"ab"}},
		{`/\\Q/gy`, "\\Q", []string{"\\Q"}},
		{`/\Qab/gy`, "xab", nil},
		{`/\Qabc\E/gy`, "abc", []string{"abc"}},
		{`/\Q)\Q/gy`, ")x", nil},
	}
	for _, tt := range variants {
		if got := mustCompile(t, tt.pattern).FindAll(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FindAll(%q, %q) = %q, want %q", tt.pattern, tt.input, got, tt.want)
		}
	}
}
