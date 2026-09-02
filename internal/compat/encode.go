package compat

import (
	"bytes"
	"encoding/json"
	"math"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// orderedObject is a JSON object that marshals its fields in the order they
// were set, not alphabetically.
//
// nadesiko3's check_compat.mjs compares results with a plain
// JSON.stringify(comparable(...)) — a string comparison, not a deep-equal —
// so a Go map[string]any (which encoding/json always sorts by key) can never
// match a TS object's insertion order once there is more than one field.
// SPEC.md's value table (§値表現) fixes each kind's field order for exactly
// this reason; this type is what lets encodeValue reproduce it.
type orderedObject struct {
	keys   []string
	values map[string]any
}

func ordered() *orderedObject {
	return &orderedObject{values: map[string]any{}}
}

// set adds or overwrites a field, keeping its first-seen position.
func (o *orderedObject) set(key string, value any) *orderedObject {
	if _, exists := o.values[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
	return o
}

func (o *orderedObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(o.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// encodeValue renders a value in the tagged form SPEC.md defines, so that the
// type and the key order survive the trip through JSON.
//
// A container that contains itself is rendered as {"t":"circular"} rather than
// recursed into.
func encodeValue(v value.Value) *orderedObject {
	return encodeWithSeen(v, map[any]bool{})
}

func encodeWithSeen(v value.Value, seen map[any]bool) *orderedObject {
	switch v.Kind() {
	case value.KindUndefined:
		return ordered().set("t", "undefined")

	case value.KindNull:
		return ordered().set("t", "null")

	case value.KindBool:
		b, _ := v.Bool()
		return ordered().set("t", "bool").set("v", b)

	case value.KindNumber:
		return encodeNumber(v)

	case value.KindString:
		s, _ := v.String()
		// len はコードポイント数。UTF-8のバイト数ではない。
		return ordered().set("t", "str").set("v", s).set("len", runeLen(s))

	case value.KindArray:
		arr, _ := v.Array()
		if seen[arr] {
			return ordered().set("t", "circular")
		}
		seen[arr] = true
		defer delete(seen, arr)

		items := make([]*orderedObject, arr.Len())
		for i := range items {
			items[i] = encodeWithSeen(arr.Get(i), seen)
		}
		return ordered().set("t", "arr").set("len", arr.Len()).set("v", items)

	case value.KindDict:
		d, _ := v.Dict()
		if seen[d] {
			return ordered().set("t", "circular")
		}
		seen[d] = true
		defer delete(seen, d)

		keys := d.Keys()
		values := ordered()
		for _, k := range keys {
			item, _ := d.Get(k)
			values.set(k, encodeWithSeen(item, seen))
		}
		return ordered().set("t", "obj").set("keys", keys).set("v", values)

	case value.KindFunc:
		return ordered().set("t", "func")
	}
	return ordered().set("t", "undefined")
}

// encodeNumber renders a number. The values JSON cannot hold become strings,
// and a whole number is marked so that 1 and 1.0 stay distinguishable.
func encodeNumber(v value.Value) *orderedObject {
	n, _ := v.Number()
	switch {
	case math.IsNaN(n):
		return ordered().set("t", "num").set("v", "NaN")
	case math.IsInf(n, 1):
		return ordered().set("t", "num").set("v", "Infinity")
	case math.IsInf(n, -1):
		return ordered().set("t", "num").set("v", "-Infinity")
	case n == 0 && math.Signbit(n):
		return ordered().set("t", "num").set("v", "-0")
	}
	// int は常に出す。整数かどうかを、値そのものとは別に固定するため。
	return ordered().set("t", "num").set("v", n).set("int", n == math.Trunc(n))
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
