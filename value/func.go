package value

import (
	"strconv"
	"strings"
)

// ToRunes : []runeに変換して返す
func (v *Value) ToRunes() []rune {
	s := v.ToString()
	return []rune(s)
}

// IntToStr : 整数を文字列に
func IntToStr(i int64) string {
	return strconv.FormatInt(i, 10)
}

// FloatToStr : 実数を文字列に
func FloatToStr(f float64) string {
	return strconv.FormatFloat(f, 'G', -1, 64)
}

// EncodeStrToJSON : Encode string for JSON
func EncodeStrToJSON(s string) string {
	r := s
	r = strings.Replace(r, "\\", "\\\\", -1)
	r = strings.Replace(r, "\r", "\\r", -1)
	r = strings.Replace(r, "\n", "\\n", -1)
	r = strings.Replace(r, "\t", "\\t", -1)
	r = strings.Replace(r, "\"", "\\\"", -1)
	return r
}
