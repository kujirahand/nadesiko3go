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

// TestV06QuoteOpenAtEnd は、位置指定照合用の包みを組む前に引用の開閉を
// 正しく判定できることを固定する。エスケープされた円記号の後ろのQは引用を
// 開かない。
func TestV06QuoteOpenAtEnd(t *testing.T) {
	tests := []struct {
		src  string
		want bool
	}{
		{"", false},
		{"abc", false},
		{`\Q`, true},
		{`\Qab`, true},
		{`\Qa\E`, false},
		{`\Qa\E\Qb`, true},
		{`\\Q`, false},
		{`\\\Q`, true},
		{`\Qabc\E`, false},
		{`(a)\Q`, true},
	}
	for _, tt := range tests {
		if got := quoteOpenAtEnd(tt.src); got != tt.want {
			t.Errorf("quoteOpenAtEnd(%q) = %v, want %v", tt.src, got, tt.want)
		}
	}
}
