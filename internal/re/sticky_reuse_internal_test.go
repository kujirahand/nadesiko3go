package re

import "testing"

// stickyFromTexts が全件連続のときに走査結果をそのまま使う（詰め直さない）
// ことを、先頭要素の置き場所の同一性で確かめる。sliceの容量成長やGC、race
// 計測の確保回数に依存しない。
func TestV06StickyFromTextsNoCopy(t *testing.T) {
	matches := []string{"a", "b", "c"}
	got := stickyFromTexts("abc", matches)
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("全件連続の結果 = %q, want [a b c]", got)
	}
	if &got[0] != &matches[0] {
		t.Error("全件連続で先頭要素の置き場所が違う。走査結果を詰め直している")
	}
}

// stickyFromTexts が途中で途切れたときに、先頭の連続分だけを詰め直すことを
// 確かめる。元の一致列のbackingを引きずらないので容量は1件分になる。
func TestV06StickyFromTextsCopiesAfterGap(t *testing.T) {
	matches := []string{"a", "b"}
	got := stickyFromTexts("aX", matches)
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("途中で途切れた結果 = %q, want [a]", got)
	}
	if cap(got) != 1 {
		t.Errorf("途中で途切れた結果の容量 = %d, want 1", cap(got))
	}
	if &got[0] == &matches[0] {
		t.Error("途中で途切れたのに元の一致列と同じ置き場所を使っている。詰め直していない")
	}
}

// stickyFromTexts が先頭で途切れたときにnilを返すことを確かめる。
func TestV06StickyFromTextsNilAfterFirstGap(t *testing.T) {
	if got := stickyFromTexts("Xabc", []string{"a"}); got != nil {
		t.Errorf("先頭で途切れた結果 = %v, want nil", got)
	}
}

// TestV06StickyModeRouting は、stickyの照合経路が3状態のどれになるかを固定
// する。位置指定の照合が要らないならscan、要るなら、本当に前後の文字を見る
// アサーションの有無でsuffixとcontextに分かれる。文字列の走査で拾っただけの
// トークンはsuffixへ倒す。
func TestV06StickyModeRouting(t *testing.T) {
	tests := []struct {
		pattern  string
		want     stickyMode
		textScan bool
	}{
		// 空一致し得ず、\bも綴らない: Goの走査が完全なのでscan
		{"/a+/gy", stickyScan, true},
		{`/\$a/gy`, stickyScan, true},
		{"/[a]b/gy", stickyScan, true},
		// 空一致し得るので位置指定。実アサーションが無いのでsuffix
		{"/a*|b/gy", stickySuffix, false},
		{"/(?:)/gy", stickySuffix, false},
		{`/(?:\Q\\E)?/gy`, stickySuffix, false},
		{`/(?:\^)?a?/gy`, stickySuffix, false},
		{`/(?:\$)?a?/gy`, stickySuffix, false},
		{`/(?:[$])?a?/gy`, stickySuffix, false},
		{`/(?:\\b)?a?/gy`, stickySuffix, false},
		{`/(?:\Q^\E)?a?/gy`, stickySuffix, false},
		// 本当に前後の文字を見るアサーションを含むのでcontext
		{"/^/gy", stickyContext, false},
		{"/a?$/gy", stickyContext, false},
		{`/\b./gy`, stickyContext, false},
		{`/\A/gy`, stickyContext, false},
		{`/\z/gy`, stickyContext, false},
		{`/\B/gy`, stickyContext, false},
		// 位置指定の照合が要らないパターンでも、実アサーションがあれば
		// 一致文字列だけの連続判定には使わない
		{`/a\B/gy`, stickyScan, false},
	}
	for _, tt := range tests {
		r, err := Compile(tt.pattern)
		if err != nil {
			t.Fatalf("Compile(%q): %v", tt.pattern, err)
		}
		if r.mode != tt.want {
			t.Errorf("Compile(%q).mode = %v, want %v", tt.pattern, r.mode, tt.want)
		}
		if r.textScan != tt.textScan {
			t.Errorf("Compile(%q).textScan = %v, want %v", tt.pattern, r.textScan, tt.textScan)
		}
		if (r.mode == stickyContext) != (r.tail != nil) {
			t.Errorf("Compile(%q): mode=%v tail=%v, 派生パターンはcontextのときだけ非nil", tt.pattern, r.mode, r.tail != nil)
		}
	}
}
