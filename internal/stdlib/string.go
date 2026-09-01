package stdlib

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// stringImpls returns the plugin_system_string commands.
//
// Every command that counts or slices characters works in runes, matching the
// Array.from-based implementation on the TypeScript side (AGENTS.md §5).
func stringImpls(m map[string]Impl) {
	m["文字数"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.Number(float64(runeLen(str(a, 0)))), nil
	}
	m["文字列分解"] = func(_ Context, a []value.Value) (value.Value, error) {
		runes := []rune(str(a, 0))
		items := make([]value.Value, len(runes))
		for i, r := range runes {
			items[i] = value.String(string(r))
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	}
	m["文字列連結"] = func(_ Context, a []value.Value) (value.Value, error) {
		var b strings.Builder
		for _, v := range a {
			b.WriteString(value.ToString(v))
		}
		return value.String(b.String()), nil
	}
	m["連結"] = m["文字列連結"]
	m["追加"] = func(_ Context, a []value.Value) (value.Value, error) {
		if arr, ok := arg(a, 0).Array(); ok {
			arr.Set(arr.Len(), arg(a, 1))
			return arg(a, 0), nil
		}
		return value.String(str(a, 0) + str(a, 1)), nil
	}
	m["一行追加"] = func(ctx Context, a []value.Value) (value.Value, error) {
		if _, ok := arg(a, 0).Array(); ok {
			return m["追加"](ctx, a)
		}
		return value.String(str(a, 0) + str(a, 1) + "\n"), nil
	}

	// --- 位置を数える命令。位置は1から数える。 ---

	m["何文字目"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.Number(float64(indexOfRunes(str(a, 0), str(a, 1), 0))), nil
	}
	m["文字検索"] = func(_ Context, a []value.Value) (value.Value, error) {
		from := int(value.ToNumber(arg(a, 1)))
		if from <= 0 {
			from = 1
		}
		return value.Number(float64(indexOfRunes(str(a, 0), str(a, 2), from-1))), nil
	}

	// --- 部分文字列 ---

	m["文字抜出"] = func(_ Context, a []value.Value) (value.Value, error) {
		runes := []rune(str(a, 0))
		start := int(value.ToNumber(arg(a, 1)))
		count := int(value.ToNumber(arg(a, 2)))
		if count <= 0 {
			return value.String(""), nil
		}
		// 負の開始位置は末尾から数える
		if start < 0 {
			start = len(runes) + start + 1
			if start < 0 {
				start = 1
			}
		}
		return value.String(string(sliceRunes(runes, start-1, start+count-1))), nil
	}
	m["文字抜き出"] = m["文字抜出"]
	m["MID"] = m["文字抜出"]
	m["文字左部分"] = func(_ Context, a []value.Value) (value.Value, error) {
		runes := []rune(str(a, 0))
		return value.String(string(sliceRunes(runes, 0, int(value.ToNumber(arg(a, 1)))))), nil
	}
	m["LEFT"] = m["文字左部分"]
	m["文字右部分"] = func(_ Context, a []value.Value) (value.Value, error) {
		runes := []rune(str(a, 0))
		start := len(runes) - int(value.ToNumber(arg(a, 1)))
		if start < 0 {
			start = 0
		}
		return value.String(string(sliceRunes(runes, start, len(runes)))), nil
	}
	m["RIGHT"] = m["文字右部分"]
	m["文字挿入"] = func(_ Context, a []value.Value) (value.Value, error) {
		runes := []rune(str(a, 0))
		at := int(value.ToNumber(arg(a, 1)))
		if at <= 0 {
			at = 1
		}
		if at > len(runes)+1 {
			at = len(runes) + 1
		}
		return value.String(string(runes[:at-1]) + str(a, 2) + string(runes[at-1:])), nil
	}
	m["文字削除"] = func(_ Context, a []value.Value) (value.Value, error) {
		runes := []rune(str(a, 0))
		start := int(value.ToNumber(arg(a, 1))) - 1
		count := int(value.ToNumber(arg(a, 2)))
		return value.String(string(spliceRunes(runes, start, count))), nil
	}

	// --- 検索と置換 ---

	m["文字始"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.Bool(strings.HasPrefix(str(a, 0), str(a, 1))), nil
	}
	m["文字終"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.Bool(strings.HasSuffix(str(a, 0), str(a, 1))), nil
	}
	m["出現回数"] = func(_ Context, a []value.Value) (value.Value, error) {
		// split(a).length - 1 と同じ。区切り文字が空なら文字数になる。
		return value.Number(float64(len(splitString(str(a, 0), str(a, 1))) - 1)), nil
	}
	m["置換"] = func(_ Context, a []value.Value) (value.Value, error) {
		// split して join するので、区切り文字が空なら間に挟み込む形になる
		return value.String(strings.Join(splitString(str(a, 0), str(a, 1)), str(a, 2))), nil
	}
	m["単置換"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.String(strings.Replace(str(a, 0), str(a, 1), str(a, 2), 1)), nil
	}
	m["区切"] = func(_ Context, a []value.Value) (value.Value, error) {
		parts := splitString(str(a, 0), str(a, 1))
		items := make([]value.Value, len(parts))
		for i, p := range parts {
			items[i] = value.String(p)
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	}
	m["文字列分割"] = func(_ Context, a []value.Value) (value.Value, error) {
		s, sep := str(a, 0), str(a, 1)
		before, after, found := strings.Cut(s, sep)
		items := []value.Value{value.String(s)}
		if found {
			items = []value.Value{value.String(before), value.String(after)}
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	}
	m["切取"] = func(ctx Context, a []value.Value) (value.Value, error) {
		s, marker := str(a, 0), str(a, 1)
		before, after, found := strings.Cut(s, marker)
		if !found {
			ctx.SetSysVar("対象", value.String(""))
			return value.String(s), nil
		}
		ctx.SetSysVar("対象", value.String(after))
		return value.String(before), nil
	}
	m["範囲切取"] = func(ctx Context, a []value.Value) (value.Value, error) {
		s, start, end := str(a, 0), str(a, 1), str(a, 2)
		before, rest, found := strings.Cut(s, start)
		if !found {
			ctx.SetSysVar("対象", value.String(s))
			return value.String(""), nil
		}
		inside, after, found := strings.Cut(rest, end)
		if !found {
			ctx.SetSysVar("対象", value.String(before))
			return value.String(rest), nil
		}
		ctx.SetSysVar("対象", value.String(before+after))
		return value.String(inside), nil
	}
	m["出現"] = func(_ Context, a []value.Value) (value.Value, error) {
		container, wanted := arg(a, 0), arg(a, 1)
		if arr, ok := container.Array(); ok {
			for i := 0; i < arr.Len(); i++ {
				if value.StrictEquals(arr.Get(i), wanted) {
					return value.Bool(true), nil
				}
			}
			return value.Bool(false), nil
		}
		return value.Bool(strings.Contains(value.ToString(container), value.ToString(wanted))), nil
	}
	m["リフレイン"] = func(_ Context, a []value.Value) (value.Value, error) {
		n := int(value.ToNumber(arg(a, 1)))
		if n < 0 {
			n = 0
		}
		return value.String(strings.Repeat(value.ToString(arg(a, 0)), n)), nil
	}

	// --- 空白の削除 ---

	m["トリム"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.String(strings.TrimFunc(str(a, 0), isSpace)), nil
	}
	m["空白除去"] = m["トリム"]
	m["左トリム"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.String(strings.TrimLeftFunc(str(a, 0), isSpace)), nil
	}
	m["右トリム"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.String(strings.TrimRightFunc(str(a, 0), isSpace)), nil
	}
	m["末尾空白除去"] = m["右トリム"]
	m["かなか判定"] = firstRuneBetween(0x3041, 0x309f)
	m["カタカナ判定"] = firstRuneBetween(0x30a1, 0x30fa)
	m["数字判定"] = func(_ Context, a []value.Value) (value.Value, error) {
		r, _ := firstRune(str(a, 0))
		return value.Bool((r >= '0' && r <= '9') || (r >= '０' && r <= '９')), nil
	}
	m["通貨形式"] = func(_ Context, a []value.Value) (value.Value, error) {
		s := str(a, 0)
		parts := strings.SplitN(s, ".", 2)
		start := 0
		if strings.HasPrefix(parts[0], "-") || strings.HasPrefix(parts[0], "+") {
			start = 1
		}
		for i := len(parts[0]) - 3; i > start; i -= 3 {
			parts[0] = parts[0][:i] + "," + parts[0][i:]
		}
		return value.String(strings.Join(parts, ".")), nil
	}

	// --- 文字種の変換 ---

	m["大文字変換"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.String(strings.ToUpper(str(a, 0))), nil
	}
	m["小文字変換"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.String(strings.ToLower(str(a, 0))), nil
	}
	m["平仮名変換"] = shiftRunes(0x30A1, 0x30F6, -0x60)  // カタカナ → ひらがな
	m["カタカナ変換"] = shiftRunes(0x3041, 0x3096, +0x60) // ひらがな → カタカナ
	m["英数全角変換"] = mapRunes(func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r + 0xFEE0
		}
		return r
	})
	m["英数半角変換"] = mapRunes(func(r rune) rune {
		if (r >= 'Ａ' && r <= 'Ｚ') || (r >= 'ａ' && r <= 'ｚ') || (r >= '０' && r <= '９') {
			return r - 0xFEE0
		}
		return r
	})
	m["英数記号全角変換"] = mapRunes(func(r rune) rune {
		if r == ' ' {
			return '　'
		}
		if r >= 0x21 && r <= 0x7e {
			return r + 0xfee0
		}
		return r
	})
	m["英数記号半角変換"] = mapRunes(func(r rune) rune {
		if r == '　' {
			return ' '
		}
		if r >= 0xff01 && r <= 0xff5f {
			return r - 0xfee0
		}
		return r
	})
	m["カタカナ全角変換"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.String(katakanaToFullWidth(str(a, 0))), nil
	}
	m["カタカナ半角変換"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.String(katakanaToHalfWidth(str(a, 0))), nil
	}
	m["全角変換"] = func(ctx Context, a []value.Value) (value.Value, error) {
		kana, _ := m["カタカナ全角変換"](ctx, a)
		return m["英数記号全角変換"](ctx, []value.Value{kana})
	}
	m["半角変換"] = func(ctx Context, a []value.Value) (value.Value, error) {
		kana, _ := m["カタカナ半角変換"](ctx, a)
		return m["英数記号半角変換"](ctx, []value.Value{kana})
	}

	// --- 桁揃え ---

	m["ゼロ埋"] = padLeft('0')
	m["空白埋"] = padLeft(' ')

	// --- 文字コード ---

	m["ASC"] = func(_ Context, a []value.Value) (value.Value, error) {
		s := str(a, 0)
		if s == "" {
			return value.Number(0), nil
		}
		r, _ := utf8.DecodeRuneInString(s)
		return value.Number(float64(r)), nil
	}
	m["CHR"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.String(string(rune(int32(value.ToNumber(arg(a, 0)))))), nil
	}
}

const (
	fullWidthKana       = "アイウエオカキクケコサシスセソタチツテトナニヌネノハヒフヘホマミムメモヤユヨラリルレロワヲンァィゥェォャュョッ、。ー「」"
	halfWidthKana       = "ｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄﾅﾆﾇﾈﾉﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜｦﾝｧｨｩｪｫｬｭｮｯ､｡ｰ｢｣ﾞﾟ"
	fullWidthVoicedKana = "ガギグゲゴザジズゼゾダヂヅデドバビブベボパピプペポ"
	halfWidthVoicedKana = "ｶﾞｷﾞｸﾞｹﾞｺﾞｻﾞｼﾞｽﾞｾﾞｿﾞﾀﾞﾁﾞﾂﾞﾃﾞﾄﾞﾊﾞﾋﾞﾌﾞﾍﾞﾎﾞﾊﾟﾋﾟﾌﾟﾍﾟﾎﾟ"
)

func katakanaToFullWidth(s string) string {
	full, half := []rune(fullWidthKana), []rune(halfWidthKana)
	voicedFull, voicedHalf := []rune(fullWidthVoicedKana), []rune(halfWidthVoicedKana)
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if i+1 < len(runes) {
			for j := 0; j+1 < len(voicedHalf); j += 2 {
				if runes[i] == voicedHalf[j] && runes[i+1] == voicedHalf[j+1] {
					b.WriteRune(voicedFull[j/2])
					i++
					goto converted
				}
			}
		}
		for j, r := range half {
			if runes[i] == r {
				b.WriteRune(full[j])
				goto converted
			}
		}
		b.WriteRune(runes[i])
	converted:
	}
	return b.String()
}

func katakanaToHalfWidth(s string) string {
	full, half := []rune(fullWidthKana), []rune(halfWidthKana)
	voicedFull, voicedHalf := []rune(fullWidthVoicedKana), []rune(halfWidthVoicedKana)
	var b strings.Builder
	for _, r := range s {
		if i := runeIndex(voicedFull, r); i >= 0 {
			b.WriteRune(voicedHalf[i*2])
			b.WriteRune(voicedHalf[i*2+1])
			continue
		}
		if i := runeIndex(full, r); i >= 0 {
			b.WriteRune(half[i])
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func runeIndex(items []rune, wanted rune) int {
	for i, r := range items {
		if r == wanted {
			return i
		}
	}
	return -1
}

func firstRune(s string) (rune, bool) {
	for _, r := range s {
		return r, true
	}
	return 0, false
}

func firstRuneBetween(low, high rune) Impl {
	return func(_ Context, a []value.Value) (value.Value, error) {
		r, ok := firstRune(str(a, 0))
		return value.Bool(ok && r >= low && r <= high), nil
	}
}

// padLeft builds 『ゼロ埋』 and 『空白埋』, which pad on the left to a width
// counted in runes and never truncate.
func padLeft(fill rune) Impl {
	return func(_ Context, a []value.Value) (value.Value, error) {
		s := value.ToString(arg(a, 0))
		width := int(value.ToNumber(arg(a, 1)))
		if n := runeLen(s); width < n {
			width = n
		}
		return value.String(strings.Repeat(string(fill), width-runeLen(s)) + s), nil
	}
}

// shiftRunes builds a command that moves the runes in a range by delta, which
// is how kana conversion works.
func shiftRunes(from, to, delta rune) Impl {
	return mapRunes(func(r rune) rune {
		if r >= from && r <= to {
			return r + delta
		}
		return r
	})
}

func mapRunes(f func(rune) rune) Impl {
	return func(_ Context, a []value.Value) (value.Value, error) {
		return value.String(strings.Map(f, value.ToString(arg(a, 0)))), nil
	}
}

// indexOfRunes reports the 1-based rune position of sub in s, searching from
// the rune offset from. It reports 0 when sub does not occur.
func indexOfRunes(s, sub string, from int) int {
	runes := []rune(s)
	if from < 0 {
		from = 0
	}
	if from > len(runes) {
		return 0
	}
	at := strings.Index(string(runes[from:]), sub)
	if at < 0 {
		return 0
	}
	return from + runeLen(string(runes[from:])[:at]) + 1
}

// sliceRunes takes runes[i:j], clamping both ends.
func sliceRunes(runes []rune, i, j int) []rune {
	if i < 0 {
		i = 0
	}
	if j > len(runes) {
		j = len(runes)
	}
	if i >= j {
		return nil
	}
	return runes[i:j]
}

// spliceRunes removes count runes starting at start, as Array.prototype.splice
// does: a negative start counts from the end, and the range is clamped.
func spliceRunes(runes []rune, start, count int) []rune {
	if start < 0 {
		start = len(runes) + start
		if start < 0 {
			start = 0
		}
	}
	if start > len(runes) {
		start = len(runes)
	}
	if count < 0 {
		count = 0
	}
	end := start + count
	if end > len(runes) {
		end = len(runes)
	}
	out := make([]rune, 0, len(runes)-(end-start))
	out = append(out, runes[:start]...)
	return append(out, runes[end:]...)
}

// splitString splits s by sep. An empty separator splits into single
// characters, as JavaScript's String.prototype.split does — by code point
// here, rather than by UTF-16 code unit.
func splitString(s, sep string) []string {
	if sep == "" {
		runes := []rune(s)
		out := make([]string, len(runes))
		for i, r := range runes {
			out[i] = string(r)
		}
		return out
	}
	return strings.Split(s, sep)
}

// isSpace matches what JavaScript's \s does, which is what トリム removes.
func isSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v', '\f', 0x00A0, 0xFEFF, 0x2028, 0x2029:
		return true
	}
	return unicode.Is(unicode.Zs, r)
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

// str reads an argument as a string.
func str(args []value.Value, i int) string { return value.ToString(arg(args, i)) }
