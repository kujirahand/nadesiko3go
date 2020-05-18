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
	}
	return c
}
