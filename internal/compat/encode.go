package compat

import (
	"math"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// encodeValue renders a value in the tagged form SPEC.md defines, so that the
// type and the key order survive the trip through JSON.
//
// A container that contains itself is rendered as {"t":"circular"} rather than
// recursed into.
func encodeValue(v value.Value) map[string]any {
	return encodeWithSeen(v, map[any]bool{})
}

func encodeWithSeen(v value.Value, seen map[any]bool) map[string]any {
	switch v.Kind() {
	case value.KindUndefined:
		return map[string]any{"t": "undefined"}

	case value.KindNull:
		return map[string]any{"t": "null"}

	case value.KindBool:
		b, _ := v.Bool()
		return map[string]any{"t": "bool", "v": b}

	case value.KindNumber:
		return encodeNumber(v)

	case value.KindString:
		s, _ := v.String()
		// len はコードポイント数。UTF-8のバイト数ではない。
		return map[string]any{"t": "str", "v": s, "len": runeLen(s)}

	case value.KindArray:
		arr, _ := v.Array()
		if seen[arr] {
			return map[string]any{"t": "circular"}
		}
		seen[arr] = true
		defer delete(seen, arr)

		items := make([]map[string]any, arr.Len())
		for i := range items {
			items[i] = encodeWithSeen(arr.Get(i), seen)
		}
		return map[string]any{"t": "arr", "len": arr.Len(), "v": items}

	case value.KindDict:
		d, _ := v.Dict()
		if seen[d] {
			return map[string]any{"t": "circular"}
		}
		seen[d] = true
		defer delete(seen, d)

		keys := d.Keys()
		values := make(map[string]any, len(keys))
		for _, k := range keys {
			item, _ := d.Get(k)
			values[k] = encodeWithSeen(item, seen)
		}
		return map[string]any{"t": "obj", "keys": keys, "v": values}

	case value.KindFunc:
		return map[string]any{"t": "func"}
	}
	return map[string]any{"t": "undefined"}
}

// encodeNumber renders a number. The values JSON cannot hold become strings,
// and a whole number is marked so that 1 and 1.0 stay distinguishable.
func encodeNumber(v value.Value) map[string]any {
	n, _ := v.Number()
	switch {
	case math.IsNaN(n):
		return map[string]any{"t": "num", "v": "NaN"}
	case math.IsInf(n, 1):
		return map[string]any{"t": "num", "v": "Infinity"}
	case math.IsInf(n, -1):
		return map[string]any{"t": "num", "v": "-Infinity"}
	case n == 0 && math.Signbit(n):
		return map[string]any{"t": "num", "v": "-0"}
	}
	// int は常に出す。整数かどうかを、値そのものとは別に固定するため。
	return map[string]any{"t": "num", "v": n, "int": n == math.Trunc(n)}
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
