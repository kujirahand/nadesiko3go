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

// IsHexDigit : Is rune Digit?
func IsHexDigit(c rune) bool {
	return InRange(c, int('0'), int('9')) ||
		InRange(c, int('a'), int('f')) ||
		InRange(c, int('A'), int('F'))
}

// IsHankaku : Is rune Hankaku?
func IsHankaku(c rune) bool {
	return InRange(c, int('0'), int('9')) ||
		InRange(c, int('a'), int('z')) ||
		InRange(c, int('A'), int('Z'))
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

// Length : Count String Length
func Length(s string) int {
	r := []rune(s)
	return len(r)
}

// Equal : check a equal b
func Equal(a []rune, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// ToKatakana : Hira to Kana
func ToKatakana(s string) string {
	sr := []rune(s)
	for i, v := range sr {
		// カタカナ？
		if IsKatakana(v) {
			sr[i] = sr[i] - 0x60
		}
	}
	return string(sr)
}

// ToHiragana : Kana to Hira
func ToHiragana(s string) string {
	sr := []rune(s)
	for i, v := range sr {
		// ひらがな？
		if IsHiragana(v) {
			sr[i] = sr[i] + 0x60
		}
	}
	return string(sr)
}

// ToZenkaku : hankaku to zenkaku
func ToZenkaku(s string) string {
	sr := []rune(s)
	for i, v := range sr {
		// hankaku?
		if IsHankaku(v) {
			sr[i] = sr[i] + 0xFEE0
		}
	}
	return string(sr)
}

// ToHankaku : zen to han
func ToHankaku(s string) string {
	sr := []rune(s)
	for i, v := range sr {
		// hankaku?
		if InRange(v, int('Ａ'), int('Ｚ')) || InRange(v, int('ａ'), int('ｚ')) || InRange(v, '０', '９') {
			sr[i] = sr[i] - 0xFEE0
		}
	}
	return string(sr)
}

// ToZenkakuAndKigou : hankaku to zenkaku
func ToZenkakuAndKigou(s string) string {
	sr := []rune(s)
	for i, v := range sr {
		// hankaku?
		if InRange(v, 0x20, 0x7f) {
			sr[i] = sr[i] + 0xFEE0
		}
	}
	return string(sr)
}

// ToHankakuAndKigou : zen to han
func ToHankakuAndKigou(s string) string {
	sr := []rune(s)
	for i, v := range sr {
		// hankaku?
		if InRange(v, 0xFF00, 0xFF5F) {
			sr[i] = sr[i] - 0xFEE0
		}
	}
	return string(sr)
}

var hanKana = []string{
	"ｶﾞ", "ｷﾞ", "ｸﾞ", "ｹﾞ", "ｺﾞ",
	"ｻﾞ", "ｼﾞ", "ｽﾞ", "ｾﾞ", "ｿﾞ",
	"ﾀﾞ", "ﾁﾞ", "ﾂﾞ", "ﾃﾞ", "ﾄﾞ",
	"ﾊﾞ", "ﾊﾟ", "ﾋﾞ", "ﾋﾟ", "ﾌﾞ", "ﾌﾟ", "ﾍﾞ", "ﾍﾟ", "ﾎﾞ", "ﾎﾟ",
	"ﾜﾞ", "ｦﾞ", "ｳﾞ",
	"｡", "｢", "｣", "､", "･", "ｰ", "ﾞ", "ﾟ",
	"ｱ", "ｲ", "ｳ", "ｴ", "ｵ",
	"ｶ", "ｷ", "ｸ", "ｹ", "ｺ",
	"ｻ", "ｼ", "ｽ", "ｾ", "ｿ", "ﾀ", "ﾁ", "ﾂ", "ﾃ", "ﾄ",
	"ﾅ", "ﾆ", "ﾇ", "ﾈ", "ﾉ",
	"ﾊ", "ﾋ", "ﾌ", "ﾍ", "ﾎ",
	"ﾏ", "ﾐ", "ﾑ", "ﾒ", "ﾓ",
	"ﾔ", "ﾕ", "ﾖ",
	"ﾗ", "ﾘ", "ﾙ", "ﾚ", "ﾛ",
	"ﾜ", "ｦ", "ﾝ",
	"ｧ", "ｨ", "ｩ", "ｪ", "ｫ",
	"ｬ", "ｭ", "ｮ", "ｯ",
}
var zenKana = []string{
	"ガ", "ギ", "グ", "ゲ", "ゴ",
	"ザ", "ジ", "ズ", "ゼ", "ゾ",
	"ダ", "ヂ", "ヅ", "デ", "ド",
	"バ", "パ", "ビ", "ピ", "ブ", "プ", "ベ", "ペ", "ボ", "ポ",
	"ヷ", "ヺ", "ヴ",
	"。", "「", "」", "、", "・", "ー", "゛", "゜",
	"ア", "イ", "ウ", "エ", "オ",
	"カ", "キ", "ク", "ケ", "コ",
	"サ", "シ", "ス", "セ", "ソ",
	"タ", "チ", "ツ", "テ", "ト",
	"ナ", "ニ", "ヌ", "ネ", "ノ",
	"ハ", "ヒ", "フ", "ヘ", "ホ",
	"マ", "ミ", "ム", "メ", "モ",
	"ヤ", "ユ", "ヨ",
	"ラ", "リ", "ル", "レ", "ロ",
	"ワ", "ヲ", "ン",
	"ァ", "ィ", "ゥ", "ェ", "ォ",
	"ャ", "ュ", "ョ", "ッ",
}

// ToZenkakuKatakana : hankaku katakana to zenkaku katakana
func ToZenkakuKatakana(s string) string {
	// 辞書を初期化
	dic := map[string]int{}
	for i, v := range hanKana {
		dic[v] = i
	}

	res := ""
	sr := []rune(s)
	i := 0
	for i < len(sr) {
		// 濁点のチェック
		if i < len(sr)-1 {
			ch2 := string(sr[i]) + string(sr[i+1])
			val, ok := dic[ch2]
			if ok {
				res += zenKana[val]
				i += 2
				continue
			}
		}
		// 普通のカナをチェック
		ch1 := string(sr[i])
		val, ok := dic[ch1]
		if ok {
			res += zenKana[val]
			i++
			continue
		}
		res += ch1
		i++
	}
	return res
}

// ToHankakuKatakana : zen katakana to han katakana
func ToHankakuKatakana(s string) string {
	// 辞書を初期化
	dic := map[string]int{}
	for i, v := range zenKana {
		dic[v] = i
	}

	res := ""
	sr := []rune(s)
	i := 0
	for i < len(sr) {
		// 普通のカナをチェック
		ch1 := string(sr[i])
		val, ok := dic[ch1]
		if ok {
			res += hanKana[val]
			i++
			continue
		}
		res += ch1
		i++
	}
	return res
}
