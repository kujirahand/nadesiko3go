package stdlib

import "testing"

// 『ヴ』『ｳﾞ』も濁音として相互に変換し、対応する全角文字の無い単独の
// 濁点・半濁点はそのまま残す(本家のplugin_system_string.mtsと同じ)。
func TestKatakanaWidthConversion(t *testing.T) {
	tests := []struct{ half, full string }{
		{"ｳﾞｧｲｵﾘﾝ", "ヴァイオリン"},
		{"ｶﾞｯｺｳ", "ガッコウ"},
		{"ﾎﾟﾝ", "ポン"},
	}
	for _, tt := range tests {
		if got := katakanaToFullWidth(tt.half); got != tt.full {
			t.Errorf("katakanaToFullWidth(%q) = %q, want %q", tt.half, got, tt.full)
		}
		if got := katakanaToHalfWidth(tt.full); got != tt.half {
			t.Errorf("katakanaToHalfWidth(%q) = %q, want %q", tt.full, got, tt.half)
		}
	}
	if got := katakanaToFullWidth("ﾞﾟ"); got != "ﾞﾟ" {
		t.Errorf("katakanaToFullWidth(単独の濁点) = %q, want そのまま", got)
	}
}
