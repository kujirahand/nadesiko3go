package vm

import (
	"math"
	"unicode/utf8"
	"unsafe"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// 文字列の添字アクセスと、範囲オブジェクトによるスライス (本家 #2590, #2599)。
//
// 文字列はrune単位で数え、負の位置は末尾から数える。範囲 {先頭, 末尾} の
// 末尾は含めない（JavaScriptの slice と同じ）。

// runeCacheSize は添字で読んだ長い文字列のrune配列を保持する件数。
// 本家と同じく、2つの文字列を交互に読むループでも入れ替わらない件数にする。
const runeCacheSize = 4

// runeCacheMin より短い文字列はキャッシュせず、その都度先頭から数える。
const runeCacheMin = 64

type runeCacheEntry struct {
	s     string // 元の文字列。保持することで同じアドレスが別の文字列に再利用されない
	runes []rune // ASCIIだけの文字列では作らない（nil）
	ascii bool
}

// asRange は添字が範囲オブジェクト（先頭・末尾が数値の辞書）なら、その2つを返す。
func asRange(index value.Value) (float64, float64, bool) {
	d, ok := index.Dict()
	if !ok {
		return 0, 0, false
	}
	first, ok1 := d.Get("先頭")
	last, ok2 := d.Get("末尾")
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	a, ok1 := first.Number()
	b, ok2 := last.Number()
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	return a, b, true
}

// sliceBounds は JavaScript の slice(start, end) と同じ規則で、長さ n に対する
// 開始・終了位置を求める。負の位置は末尾から数え、範囲外は切り詰める。
func sliceBounds(start, end float64, n int) (int, int) {
	clamp := func(f float64) int {
		if math.IsNaN(f) {
			return 0
		}
		f = math.Trunc(f)
		if f < 0 {
			f += float64(n)
			if f < 0 {
				return 0
			}
		}
		if f > float64(n) {
			return n
		}
		return int(f)
	}
	s, e := clamp(start), clamp(end)
	if e < s {
		e = s
	}
	return s, e
}

// sliceArray は配列の範囲 [先頭, 末尾) を浅いコピーで返す。
func sliceArray(arr *value.Array, start, end float64) value.Value {
	s, e := sliceBounds(start, end, arr.Len())
	items := make([]value.Value, 0, e-s)
	for i := s; i < e; i++ {
		items = append(items, arr.Get(i))
	}
	return value.ArrayValue(value.NewArray(items...))
}

// stringIndex は文字列の添字アクセスを行う。整数（または整数を表す文字列）なら1文字、
// 範囲なら部分文字列を返す。範囲外の1文字は undefined、空の範囲は空文字列になる。
func (m *VM) stringIndex(s string, index value.Value) value.Value {
	if start, end, ok := asRange(index); ok {
		runes, ascii := m.stringChars(s)
		if ascii {
			a, b := sliceBounds(start, end, len(s))
			return value.String(s[a:b])
		}
		a, b := sliceBounds(start, end, len(runes))
		return value.String(string(runes[a:b]))
	}
	i, ok := stringIndexPos(index)
	if !ok {
		// length は文字数（rune単位）を返す。それ以外のプロパティは無い
		if str, isStr := index.String(); isStr && str == "length" {
			return value.Number(float64(utf8.RuneCountInString(s)))
		}
		return value.Undefined()
	}
	if i >= 0 && len(s) < runeCacheMin {
		// 短い文字列は先頭から数えるほうが速い
		for _, r := range s {
			if i == 0 {
				return value.String(string(r))
			}
			i--
		}
		return value.Undefined()
	}
	runes, ascii := m.stringChars(s)
	n := len(runes)
	if ascii {
		n = len(s)
	}
	if i < 0 {
		i += n
	}
	if i < 0 || i >= n {
		return value.Undefined()
	}
	if ascii {
		return value.String(s[i : i+1])
	}
	return value.String(string(runes[i]))
}

// stringIndexPos は文字列の添字に使える整数位置を返す。数値は整数のみ、
// 文字列は "-1" や "12" のような整数表記のみを受け付ける。
func stringIndexPos(index value.Value) (int, bool) {
	if n, ok := index.Number(); ok {
		if math.IsNaN(n) || math.IsInf(n, 0) || math.Trunc(n) != n {
			return 0, false
		}
		return int(n), true
	}
	str, ok := index.String()
	if !ok || str == "" {
		return 0, false
	}
	digits := str
	if digits[0] == '-' {
		digits = digits[1:]
	}
	if digits == "" || (len(digits) > 1 && digits[0] == '0') || len(digits) > 15 {
		return 0, false
	}
	n := 0
	for i := 0; i < len(digits); i++ {
		c := digits[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	if str[0] == '-' {
		n = -n
	}
	return n, true
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// stringChars は文字列をrune配列にする。ASCIIだけの文字列は配列を作らず ascii=true を返し、
// 呼び出し側はバイト位置をそのまま使う。長い文字列は文字種の判定結果とrune配列を
// 最近使った数件だけ保持し、『S[I]』や『S[I…J]』を繰り返すループで毎回全文を走査しないようにする。
// 同じ文字列かどうかは中身ではなくデータのアドレスと長さで判定する（O(1)）。
func (m *VM) stringChars(s string) ([]rune, bool) {
	if len(s) < runeCacheMin {
		if isASCII(s) {
			return nil, true
		}
		return []rune(s), false
	}
	p := unsafe.StringData(s)
	cache := &m.runeCache
	for i := range cache {
		e := cache[i]
		if e.s != "" && len(e.s) == len(s) && unsafe.StringData(e.s) == p {
			// ヒットしたものを先頭へ移し、長く使われていないものから破棄する
			copy(cache[1:i+1], cache[:i])
			cache[0] = e
			return e.runes, e.ascii
		}
	}
	e := runeCacheEntry{s: s, ascii: isASCII(s)}
	if !e.ascii {
		e.runes = []rune(s)
	}
	copy(cache[1:], cache[:runeCacheSize-1])
	cache[0] = e
	return e.runes, e.ascii
}
