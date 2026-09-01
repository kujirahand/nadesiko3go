// Package text provides rune-based string operations. Language features must
// use these helpers instead of byte indexing or len(string).
package text

import "unicode/utf8"

func RuneLen(s string) int { return utf8.RuneCountInString(s) }

// RuneSlice uses zero-based, half-open rune indexes and clamps both indexes to
// the string bounds.
func RuneSlice(s string, i, j int) string {
	runes := []rune(s)
	if i < 0 {
		i = 0
	}
	if j > len(runes) {
		j = len(runes)
	}
	if j < 0 || i > len(runes) || i >= j {
		return ""
	}
	return string(runes[i:j])
}

func RuneAt(s string, i int) string {
	runes := []rune(s)
	if i < 0 || i >= len(runes) {
		return ""
	}
	return string(runes[i])
}
