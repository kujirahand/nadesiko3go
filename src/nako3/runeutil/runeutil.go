package runeutil

// InRange : check min <= c <= max
func InRange(c rune, min, max int) bool {
	return rune(min) <= c && c <= rune(max)
}

// IsLower : Is rune lower case?
func IsLower(c rune) bool {
	return rune('a') <= c && c <= rune('z')
}

// IsUpper : Is rune upper case?
func IsUpper(c rune) bool {
	return rune('A') <= c && c <= rune('Z')
}

// IsLetter : Is rune alphabet
func IsLetter(c rune) bool {
	return IsLower(c) || IsUpper(c)
}

// IsDigit : Is rune Digit?
func IsDigit(c rune) bool {
	return rune('0') <= c && c <= rune('9')
}

// IsFlag : Is rune Flag?
func IsFlag(c rune) bool {
	return InRange(c, 0x21, 0x2F) ||
		InRange(c, 0x3A, 0x40) ||
		InRange(c, 0x5B, 0x60) ||
		InRange(c, 0x7B, 0x7E)
}

// IsWordRune : Is rune WORD token ?
func IsWordRune(c rune) bool {
	return c == '_' ||
		IsLetter(c) ||
		IsHiragana(c) ||
		IsKatakana(c) ||
		IsKanji(c) ||
		IsEmoji(c)
}

// IsGreek : Is rune Greek ?
func IsGreek(c rune) bool {
	return InRange(c, 0x0370, 0x03FF) || // Greek
		InRange(c, 0x1F00, 0x1FFF) || // Greek Extended
		InRange(c, 0x10140, 0x1018F) // Ancient Greek Numbers
}

// IsLatin : Is rune Latine ?
func IsLatin(c rune) bool {
	return InRange(c, 0x80, 0xFF) || // Latin-1
		InRange(c, 0x0100, 0x024F) // Latin Extend-A/B
}

// IsKanji : Is rune Kanji ?
func IsKanji(c rune) bool {
	return InRange(c, 0x2E80, 0x2FDF) || // CJK部首補助
		rune(c) == '々' || // 3005
		rune(c) == '〇' || // 3007
		rune(c) == '〻' || // 303B
		InRange(c, 0x3400, 0x4DBF) || // CJK漢字拡張A
		InRange(c, 0x4E00, 0x9FFC) || // CJK統合漢字
		InRange(c, 0xF900, 0xFAFF) || // CJK互換漢字
		InRange(c, 0x20000, 0x2FFFF) // CJK統合漢字拡張B-Fなど
}

// IsEmoji : Is rune Emoji ?
func IsEmoji(c rune) bool {
	return InRange(c, 0x2700, 0x27BF) || // 装飾記号
		InRange(c, 0x1F650, 0x1F67F) || // 装飾用絵記号
		InRange(c, 0x1F600, 0x1F64F) || // 顔文字
		InRange(c, 0x2600, 0x26FF) || // その他の記号
		InRange(c, 0x1F300, 0x1F5FF) || // その他の記号と絵文字
		InRange(c, 0x1F900, 0x1F9FF) || // 記号と絵文字補助
		InRange(c, 0x1FA70, 0x1FAFF) || // 絵文字機拡張A
		InRange(c, 0x1F680, 0x1F6FF) // 交通と地図の記号
}

// IsMultibytes : Is rune multibytes?
func IsMultibytes(c rune) bool {
	return (c > 0xFF)
}

// IsHiragana : Is rune Hiragana?
func IsHiragana(c rune) bool {
	// 3040～309F
	// ('ぁ' <= c && c <= 'ん') // 0x3041 - 3093
	return rune(0x3040) <= c && c <= rune(0x309F)
}

// IsKatakana : Is rune Ktakana?
func IsKatakana(c rune) bool {
	// 30A0～30FF | 31F0～31FF
	return (rune(0x30A0) <= c && c <= rune(0x30FF)) ||
		(rune(0x31F0) <= c && c <= rune(0x31FF))
}

// HasRune : Has rune in runes?
func HasRune(runes []rune, c rune) bool {
	for _, v := range runes {
		if v == c {
			return true
		}
	}
	return false
}
