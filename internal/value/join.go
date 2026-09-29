package value

import "strings"

// JoinString joins values with sep, the way JavaScript's
// Array.prototype.join does: undefined and null become empty strings and
// every other value goes through ToString. It exists because the join rule
// differs from String() — String(null) is "null" — and several commands
// (『連結』『配列結合』『連続表示』) are specified as join in the
// TypeScript implementation (#221).
func JoinString(items []Value, sep string) string {
	parts := make([]string, len(items))
	for i, v := range items {
		switch v.Kind() {
		case KindUndefined, KindNull:
			// join は null/undefined を空文字にする
		default:
			parts[i] = ToString(v)
		}
	}
	return strings.Join(parts, sep)
}
