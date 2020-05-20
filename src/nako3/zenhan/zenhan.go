package zenhan

// EncodeString : Zenkaku to Hankaku
func EncodeRunes(s []rune) []rune {
	for i, v := range s {
		s[i] = EncodeRune(v)
	}
	return s
}

// EncodeRune : Zenkaku to Hankaku
func EncodeRune(c rune) rune {
	switch {
	case c <= 0xFF:
		return c
	// BASIC convert (num alpha flag)
	case '！' <= c && c <= '〜':
		return rune(c + ('!' - '！'))
	// カンマ
	case c == '，' || c == '、':
		return ','
	// 。
	case c == '。':
		return ';'
	// 全角スペース
	case rune(0x2002) <= c && c <= rune(0x200B):
		return ' '
	case c == rune(0x3000): // 日本語全角スペース
		return ' '
	case c == rune(0xFEFF):
		return ' '
	}
	return c
}
