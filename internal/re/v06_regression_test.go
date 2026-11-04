package re_test

import (
	"errors"
	"reflect"
	"regexp/syntax"
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
	if got, want := mustFindAll(t, mustCompile(t, "/ab/gy"), "abab"), []string{"ab", "ab"}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindAll = %v, want %v", got, want)
	}
	// 途切れたらそこで止まる。空の一致のあとは1文字ぶん進む。
	if got, want := mustFindAll(t, mustCompile(t, "/ab/gy"), "ab ab"), []string{"ab"}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindAll = %v, want %v", got, want)
	}
	if got, want := mustFindAll(t, mustCompile(t, "/(?:)/gy"), "abc"), []string{"", "", "", ""}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindAll = %q, want %q", got, want)
	}
	if got, want := mustFindAll(t, mustCompile(t, "/ab/g"), "ab ab"), []string{"ab", "ab"}; !reflect.DeepEqual(got, want) {
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
		if got := mustReplace(t, mustCompile(t, tt.pattern), tt.input, "X"); got != tt.want {
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
		if got := mustFindAll(t, mustCompile(t, tt.pattern), tt.input); !reflect.DeepEqual(got, tt.want) {
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
		if got := mustFindAll(t, mustCompile(t, tt.pattern), tt.input); !reflect.DeepEqual(got, tt.want) {
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
		if got := mustReplace(t, mustCompile(t, tt.pattern), tt.input, tt.template); got != tt.want {
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
		if got := mustFindAll(t, mustCompile(t, tt.pattern), tt.input); !reflect.DeepEqual(got, tt.want) {
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
		if got := mustFindAll(t, mustCompile(t, tt.pattern), tt.input); !reflect.DeepEqual(got, tt.want) {
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
		if got := mustFindAll(t, mustCompile(t, tt.pattern), tt.input); !reflect.DeepEqual(got, tt.want) {
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
		if got := mustFindAll(t, mustCompile(t, tt.pattern), tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FindAll(%q, %q) = %q, want %q", tt.pattern, tt.input, got, tt.want)
		}
	}
}

// TestV06StickyConcurrentReuse は、1つのコンパイル済みパターンを複数のgoroutine
// から同時に使えることを固定する。照合経路で使う派生パターンは、初めて必要に
// なったときに一度だけ作り、それ以降の照合は読み取りだけになる。
func TestV06StickyConcurrentReuse(t *testing.T) {
	positional := mustCompile(t, "/a*|b/gy")
	fast := mustCompile(t, "/a+/gy")
	context := mustCompile(t, `/\b./gy`)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				got, err := positional.FindAll("ab")
				if err != nil {
					t.Errorf("位置指定検索のFindAll: %v", err)
					return
				}
				if want := []string{"a", "", ""}; !reflect.DeepEqual(got, want) {
					t.Errorf("位置指定検索のFindAll = %q, want %q", got, want)
					return
				}
				got, err = fast.FindAll("aaab")
				if err != nil {
					t.Errorf("走査経路のFindAll: %v", err)
					return
				}
				if want := []string{"aaa"}; !reflect.DeepEqual(got, want) {
					t.Errorf("走査経路のFindAll = %q, want %q", got, want)
					return
				}
				got, err = context.FindAll("a b")
				if err != nil {
					t.Errorf("文脈付き検索のFindAll: %v", err)
					return
				}
				if want := []string{"a", " ", "b"}; !reflect.DeepEqual(got, want) {
					t.Errorf("文脈付き検索のFindAll = %q, want %q", got, want)
					return
				}
				if _, err := positional.Replace("ab", "X"); err != nil {
					t.Errorf("位置指定検索のReplace: %v", err)
					return
				}
				if _, err := fast.FindAllMatches("aaab"); err != nil {
					t.Errorf("走査経路のFindAllMatches: %v", err)
					return
				}
				if _, err := context.Replace("a b", "X"); err != nil {
					t.Errorf("文脈付き検索のReplace: %v", err)
					return
				}
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
		if got := mustFindAll(t, mustCompile(t, tt.pattern), tt.input); !reflect.DeepEqual(got, tt.want) {
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
	if got := mustFindAll(t, r, middle); !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("途中で途切れるFindAll = %q, want [a]", got)
	}
	if got := mustFindAll(t, r, middle); cap(got) > 1 {
		t.Errorf("途中で途切れるFindAllの容量 = %d, want 1以下", cap(got))
	}

	// 先頭で途切れる: nil
	if got := mustFindAll(t, r, first); got != nil {
		t.Errorf("先頭で途切れるFindAll = %v, want nil", got)
	}

	// 最後まで連続する: 全件返す
	if got := mustFindAll(t, r, strings.Repeat("a", 100)); len(got) != 100 {
		t.Errorf("全部連続するFindAllの件数 = %d, want 100", len(got))
	}
}

// TestV06StickyQuotedPatternDoesNotPanic は、Goの 『\Q』 のように後ろを
// リテラル扱いにする構文を含むstickyパターンでも、Compileも照合もpanicしない
// ことを固定する。閉じた引用は文脈を見ないので派生パターンを作らない。開いた
// 引用でも、文脈を見るならCompileで包みの中で閉じる。
func TestV06StickyQuotedPatternDoesNotPanic(t *testing.T) {
	r, err := re.Compile("/\\Q/gy")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// Goの正規表現では 『\Q』 だけで空に一致するパターンになる
	if got, want := mustFindAll(t, r, "abc"), []string{"", "", "", ""}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindAll = %q, want %q", got, want)
	}
	if got := mustFindAllMatches(t, r, "abc"); len(got) != 4 {
		t.Errorf("FindAllMatchesの件数 = %d, want 4", len(got))
	}
	// 末尾の一致も位置指定の照合を通る
	if got, want := mustFindAll(t, r, "a"), []string{"", ""}; !reflect.DeepEqual(got, want) {
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
		if got := mustFindAll(t, mustCompile(t, tt.pattern), tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FindAll(%q, %q) = %q, want %q", tt.pattern, tt.input, got, tt.want)
		}
	}
}

// TestV06SuffixContextFree は、文脈を見ない位置指定照合が、元のパターンを
// 対象の途中から当てて先頭一致だけを採ることを固定する。派生パターンを作ら
// ないので深さの上限に当たらず、全体と各組の位置だけ元の座標へ戻す。期待値は
// Nodeの実行結果（uフラグ相当のrune基準）に合わせている。
func TestV06SuffixContextFree(t *testing.T) {
	r := mustCompile(t, "/(あ)?(?<n>い?)/gy")
	if got, want := mustFindAll(t, r, "あいう"), []string{"あい", "", ""}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindAll = %q, want %q", got, want)
	}
	wantMatches := []struct {
		text   string
		groups []re.Group
	}{
		{"あい", []re.Group{{Text: "あ", Ok: true}, {Text: "い", Ok: true}}},
		{"", []re.Group{{Ok: false}, {Text: "", Ok: true}}},
		{"", []re.Group{{Ok: false}, {Text: "", Ok: true}}},
	}
	got := mustFindAllMatches(t, r, "あいう")
	if len(got) != len(wantMatches) {
		t.Fatalf("FindAllMatchesの件数 = %d, want %d", len(got), len(wantMatches))
	}
	for i, want := range wantMatches {
		if got[i].Text != want.text || !reflect.DeepEqual(got[i].Groups, want.groups) {
			t.Errorf("%d件目 = %#v, want text %q groups %#v", i, got[i], want.text, want.groups)
		}
	}
	// 置換は一致全体と組の位置をともに元の座標で見る
	if got, want := mustReplace(t, r, "あいう", "<$1:$<n>>"), "<あ:い><:>う<:>"; got != want {
		t.Errorf("Replace(部分一致) = %q, want %q", got, want)
	}
	if got, want := mustReplace(t, r, "あいう", "[$`|$'|$&]"), "[|う|あい][あい|う|]う[あいう||]"; got != want {
		t.Errorf("Replace($`と$') = %q, want %q", got, want)
	}
	// 区切りはg/yに依存しない
	parts := mustCompile(t, "/(あ)?(?<n>い?)/").Split("あいう")
	wantParts := []re.SplitPart{{Text: ""}, {Text: "あ"}, {Text: "い"}, {Text: "う"}}
	if !reflect.DeepEqual(parts, wantParts) {
		t.Errorf("Split = %#v, want %#v", parts, wantParts)
	}
}

// TestV06SuffixDepthContextFree は、元がvalidな深さのパターンをcontext-freeの
// 位置指定照合で成功させることを固定する。920は派生パターンを作って照合時に
// panicし、e526は正規化してから派生パターンを作ってerrorにしていた。深さ
// 996/997/998はGoの元のコンパイルが成功する入力である。
func TestV06SuffixDepthContextFree(t *testing.T) {
	for _, depth := range []int{996, 997, 998} {
		body := strings.Repeat("(", depth) + "a?" + strings.Repeat(")", depth)
		r, err := re.Compile("/" + body + "/gy")
		if err != nil {
			t.Fatalf("depth=%d のCompile: %v", depth, err)
		}
		if got, want := mustFindAll(t, r, "a"), []string{"a", ""}; !reflect.DeepEqual(got, want) {
			t.Errorf("depth=%d FindAll = %q, want %q", depth, got, want)
		}
		match, ok := r.Find("a")
		if !ok || match.Text != "a" || len(match.Groups) != depth {
			t.Fatalf("depth=%d Find = %#v (ok=%v), want \"a\" と %d 組", depth, match, ok, depth)
		}
		for i, group := range match.Groups {
			if !group.Ok || group.Text != "a" {
				t.Fatalf("depth=%d Findの%d組目 = %#v, want \"a\"", depth, i, group)
			}
		}
		matches := mustFindAllMatches(t, r, "a")
		if len(matches) != 2 {
			t.Fatalf("depth=%d FindAllMatchesの件数 = %d, want 2", depth, len(matches))
		}
		last := matches[1]
		if last.Text != "" || len(last.Groups) != depth {
			t.Fatalf("depth=%d 末尾の一致 = %#v", depth, last)
		}
		for i, group := range last.Groups {
			if !group.Ok || group.Text != "" || group.Ok == false {
				t.Fatalf("depth=%d 末尾の%d組目 = %#v, want 参加した空文字", depth, i, group)
			}
		}
		if got, want := mustReplace(t, r, "a", "<$1>"), "<a><>"; got != want {
			t.Errorf("depth=%d Replace = %q, want %q", depth, got, want)
		}
		parts := r.Split("a")
		if len(parts) != depth+2 {
			t.Fatalf("depth=%d Splitの件数 = %d, want %d", depth, len(parts), depth+2)
		}
		if parts[0].Text != "" || parts[len(parts)-1].Text != "" {
			t.Errorf("depth=%d Splitの端 = %q, %q, want 空文字", depth, parts[0].Text, parts[len(parts)-1].Text)
		}
		for i := 1; i < len(parts)-1; i++ {
			if parts[i].Text != "a" {
				t.Fatalf("depth=%d Splitの%d件目 = %q, want \"a\"", depth, i, parts[i].Text)
			}
		}
		// 同じパターンはgだけでも成功する。GoのFindAllは直前の一致に
		// 隣接する空一致を落とすので、stickyの一致列とはここで分かれる。
		global, err := re.Compile("/" + body + "/g")
		if err != nil {
			t.Fatalf("depth=%d のgだけのCompile: %v", depth, err)
		}
		if got, want := mustFindAll(t, global, "a"), []string{"a"}; !reflect.DeepEqual(got, want) {
			t.Errorf("depth=%d gだけのFindAll = %q, want %q", depth, got, want)
		}
	}
}

// TestV06DeepLiteralTokensStayContextFree は、深いパターンの中の 『^』『$』
// 『\b』が、文脈を見るアサーションではなく、エスケープしたリテラル・文字
// クラス・『\Q...\E』 の中の文字であるときに成功することを固定する。文字列の
// 走査だけで判断すると、これらをアサーションと誤認して、元がvalidでも深さの
// 都合で拒んでしまう。ここに並ぶ形は空一致し得るので位置指定の照合に入り、
// 派生パターンを作ると深さの上限に当たる。
func TestV06DeepLiteralTokensStayContextFree(t *testing.T) {
	const depth = 997
	tests := []struct {
		name, inner, input string
		want               []string
	}{
		{"エスケープしたリテラルの^", `(?:\^)?a?`, "^a", []string{"^a", ""}},
		{"エスケープしたリテラルの$", `(?:\$)?a?`, "$a", []string{"$a", ""}},
		{"文字クラスの中の$", `(?:[$])?a?`, "$a", []string{"$a", ""}},
		{"エスケープした円記号の後ろのb", `(?:\\b)?a?`, `\ba`, []string{`\ba`, ""}},
		{"引用の中の^", `(?:\Q^\E)?a?`, "^a", []string{"^a", ""}},
	}
	for _, tt := range tests {
		pattern := "/" + strings.Repeat("(", depth) + tt.inner + strings.Repeat(")", depth) + "/gy"
		r, err := re.Compile(pattern)
		if err != nil {
			t.Fatalf("%s: Compile: %v", tt.name, err)
		}
		if got := mustFindAll(t, r, tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: FindAll = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestV06DeepContextTailDeferred は、本当に前後の文字を見るアサーションを
// 含む深いパターンの扱いを固定する。派生パターンは元より深くなるので、元の
// コンパイルが成功しても作れないことがある。ただし派生パターンは使う操作で
// 初めて作るので、Compile自体は成功し、区切や先頭だけの照合・置換のように
// 使わない経路は成功する。使う経路では、入力によらず照合を始める前に同じ
// StickyVariantError を返す。これは残る制約であり、黙って一致なしに潰さない。
func TestV06DeepContextTailDeferred(t *testing.T) {
	const depth = 997
	checkVariantErr := func(inner, name string, err error) {
		t.Helper()
		var variantErr *re.StickyVariantError
		if !errors.As(err, &variantErr) {
			t.Fatalf("inner=%q の%s = %v, want StickyVariantError", inner, name, err)
		}
		if !variantErr.Depth {
			t.Errorf("inner=%q の%sのStickyVariantError.Depth = false, want true", inner, name)
		}
		var syntaxErr *syntax.Error
		if !errors.As(err, &syntaxErr) || syntaxErr.Code != syntax.ErrNestingDepth {
			t.Errorf("inner=%q の%sの原因 = %v, want ErrNestingDepth", inner, name, variantErr.Cause)
		}
	}
	for _, inner := range []string{"a?$", `a?\b`} {
		body := strings.Repeat("(", depth) + inner + strings.Repeat(")", depth)
		r, err := re.Compile("/" + body + "/y")
		if err != nil {
			t.Fatalf("inner=%q のy付きCompile: %v", inner, err)
		}
		// 派生パターンを使わない経路は成功する
		if match, ok := r.Find("a"); !ok || match.Text != "a" {
			t.Errorf("inner=%q のFind = %#v (ok=%v), want \"a\"", inner, match, ok)
		}
		if parts := r.Split("a"); len(parts) != depth+2 {
			t.Errorf("inner=%q のSplitの件数 = %d, want %d", inner, len(parts), depth+2)
		}
		if got, err := r.Replace("a", "<$1>"); err != nil || got != "<a>" {
			t.Errorf("inner=%q の非global置換 = %q, %v, want \"<a>\"", inner, got, err)
		}
		// 派生パターンが要る経路は、照合を始める前にエラーを返す。先頭で
		// 一致しない入力でも、結果が入力次第で揺れないよう同じエラーにする
		_, err = r.FindAll("a")
		checkVariantErr(inner, "FindAll", err)
		_, err = r.FindAll("b")
		checkVariantErr(inner, "FindAll(一致なし)", err)
		_, err = r.FindAllMatches("a")
		checkVariantErr(inner, "FindAllMatches", err)
		gy, err := re.Compile("/" + body + "/gy")
		if err != nil {
			t.Fatalf("inner=%q のgy付きCompile: %v", inner, err)
		}
		_, err = gy.Replace("a", "X")
		checkVariantErr(inner, "global置換", err)
		// 同じパターンでも、stickyでなければ派生パターンを作らないので成功する
		global, err := re.Compile("/" + body + "/g")
		if err != nil {
			t.Fatalf("inner=%q のg付きCompile: %v", inner, err)
		}
		if got, want := mustFindAll(t, global, "a"), []string{"a"}; !reflect.DeepEqual(got, want) {
			t.Errorf("inner=%q のg付きFindAll = %q, want %q", inner, got, want)
		}
	}
}

// TestV06ContextAbsoluteAssertions は、絶対位置を見る 『\A』『\z』と、開いた
// 引用を含む文脈付きパターンが、位置指定でも意味を保つことを固定する。
func TestV06ContextAbsoluteAssertions(t *testing.T) {
	tests := []struct {
		pattern, input string
		want           []string
	}{
		{`/\Aa/gy`, "aa", []string{"a"}},
		{`/a\z/gy`, "aa", nil},
		{`/a\z/gy`, "a", []string{"a"}},
		{`/\b\Q/gy`, "a", []string{"", ""}},
	}
	for _, tt := range tests {
		if got := mustFindAll(t, mustCompile(t, tt.pattern), tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FindAll(%q, %q) = %q, want %q", tt.pattern, tt.input, got, tt.want)
		}
	}
}

// TestV06NonASCIIFlagRejected は、最後の 『/』 より後を常にフラグ列として
// 検証し、ASCII英字以外の文字をパターンの一部として黙って受理しないことを
// 固定する。JavaScriptの読み取りは 『[a-zA-Z]*』 で終わる形だけをフラグ付きと
// みなすので、修正前は 『/a/é』 がパターンそのものとして通っていた。
func TestV06NonASCIIFlagRejected(t *testing.T) {
	tests := []struct {
		pattern string
		flag    rune
	}{
		{"/a/é", 'é'},
		{"/a/gé", 'é'},
		{"/あ/い", 'い'},
		{"/a/1", '1'},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			_, err := re.Compile(tt.pattern)
			var flagErr *re.FlagError
			if !errors.As(err, &flagErr) {
				t.Fatalf("Compile(%q) = %v, want *FlagError", tt.pattern, err)
			}
			if rune(flagErr.Flag) != tt.flag {
				t.Errorf("Flag = %q, want %q", flagErr.Flag, tt.flag)
			}
		})
	}
	// 末尾が 『/』 ならフラグ列は空で、本体の中の 『/』 はフラグではない
	if _, err := re.Compile("/a/b/"); err != nil {
		t.Errorf("/a/b/ は通るべき: %v", err)
	}
	// 先頭が 『/』 でない裸のパターンと、本体が空になる形はJavaScriptと同じく
	// パターンとして扱う
	for _, pattern := range []string{"a/b", "//"} {
		if _, err := re.Compile(pattern); err != nil {
			t.Errorf("%s は通るべき: %v", pattern, err)
		}
	}
}

// TestV06StickyScanRetention は、走査経路（stickyFromScan）で途中に途切れが
// あるとき、走査結果の大きなbackingを返り値が保持しないことを固定する。
// 文脈を見るアサーションを含むので textScan には入らず、位置指定も要らない
// 走査経路を通るパターンで確かめる。その経路選択自体は、内部テストの
// TestV06StickyModeRouting が同じ `/a\B/gy` で stickyScan・textScan=false と
// 固定している。
func TestV06StickyScanRetention(t *testing.T) {
	r := mustCompile(t, `/a\B/gy`)
	got := mustFindAll(t, r, "aaX"+strings.Repeat("a", 65536))
	if !reflect.DeepEqual(got, []string{"a", "a"}) {
		t.Fatalf("途中で途切れるFindAll = %q, want [a a]", got)
	}
	if cap(got) > 2 {
		t.Errorf("途中で途切れるFindAllの容量 = %d, want 2以下", cap(got))
	}
}
