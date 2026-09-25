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

// 本家 #2457・#2478 の回帰ケース。末尾の未濁音の誤変換、濁点の消失、
// 濁音ペアの境界誤一致が起きないことを確かめる。
func TestKatakanaToFullWidthBoundary(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ｲﾛﾊﾆﾎﾍﾄ", "イロハニホヘト"}, // 末尾の ﾄ が ド にならない
		{"ﾄ", "ト"},
		{"ｳﾞ", "ヴ"},
		{"ﾞｷ", "ﾞキ"}, // 濁点を次の文字に結合しない
		{"ﾟﾋ", "ﾟヒ"},
		{"ﾞ", "ﾞ"}, // 単独の濁点は消さない
		{"ﾟ", "ﾟ"},
		{"ｶﾞ", "ガ"},
		{"ｲﾛﾊﾆﾎﾍﾄﾞ", "イロハニホヘド"},
		{"ABCｱ1", "ABCア1"}, // 変換対象外文字との境界
	}
	for _, tt := range tests {
		if got := katakanaToFullWidth(tt.in); got != tt.want {
			t.Errorf("katakanaToFullWidth(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
