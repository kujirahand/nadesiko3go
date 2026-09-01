package stdlib

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// dictImpls returns the dictionary and JSON commands.
func dictImpls(m map[string]Impl) {
	m["辞書キー列挙"] = func(_ Context, a []value.Value) (value.Value, error) {
		d, ok := arg(a, 0).Dict()
		if !ok {
			return value.ArrayValue(value.NewArray()), nil
		}
		keys := d.Keys()
		items := make([]value.Value, len(keys))
		for i, k := range keys {
			items[i] = value.String(k)
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	}

	m["辞書キー存在"] = func(_ Context, a []value.Value) (value.Value, error) {
		d, ok := arg(a, 0).Dict()
		if !ok {
			return value.Bool(false), nil
		}
		_, found := d.Get(str(a, 1))
		return value.Bool(found), nil
	}

	m["辞書キー削除"] = func(_ Context, a []value.Value) (value.Value, error) {
		d, ok := arg(a, 0).Dict()
		if !ok {
			return value.Undefined(), errors.New("『辞書キー削除』で辞書以外が指定されました。")
		}
		d.Delete(str(a, 1))
		return arg(a, 0), nil
	}

	m["JSONエンコード"] = func(_ Context, a []value.Value) (value.Value, error) {
		out, err := encodeJSON(arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		return value.String(out), nil
	}

	m["JSONデコード"] = func(_ Context, a []value.Value) (value.Value, error) {
		return decodeJSON(str(a, 0))
	}
}

// encodeJSON renders a value the way JSON.stringify does: keys in insertion
// order, no HTML escaping, and undefined dropped.
func encodeJSON(v value.Value) (string, error) {
	var b strings.Builder
	if err := writeJSON(&b, v); err != nil {
		return "", err
	}
	return b.String(), nil
}

func writeJSON(b *strings.Builder, v value.Value) error {
	switch v.Kind() {
	case value.KindUndefined, value.KindNull, value.KindFunc:
		b.WriteString("null")
		return nil
	case value.KindBool:
		b.WriteString(value.ToString(v))
		return nil
	case value.KindNumber:
		n, _ := v.Number()
		// JSONは NaN と Infinity を表せないので null になる
		if math.IsNaN(n) || math.IsInf(n, 0) {
			b.WriteString("null")
			return nil
		}
		b.WriteString(value.NumberToString(n))
		return nil
	case value.KindString:
		s, _ := v.String()
		return writeJSONString(b, s)
	case value.KindArray:
		arr, _ := v.Array()
		b.WriteByte('[')
		for i := 0; i < arr.Len(); i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeJSON(b, arr.Get(i)); err != nil {
				return err
			}
		}
		b.WriteByte(']')
		return nil
	case value.KindDict:
		d, _ := v.Dict()
		b.WriteByte('{')
		first := true
		for _, k := range d.Keys() {
			item, _ := d.Get(k)
			if item.Kind() == value.KindUndefined {
				continue // undefined のキーは出力しない
			}
			if !first {
				b.WriteByte(',')
			}
			first = false
			if err := writeJSONString(b, k); err != nil {
				return err
			}
			b.WriteByte(':')
			if err := writeJSON(b, item); err != nil {
				return err
			}
		}
		b.WriteByte('}')
		return nil
	}
	b.WriteString("null")
	return nil
}

// writeJSONString quotes a string without escaping non-ASCII, so that Japanese
// stays readable as JSON.stringify leaves it.
func writeJSONString(b *strings.Builder, s string) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	b.Write(bytes.TrimRight(buf.Bytes(), "\n"))
	return nil
}

// decodeJSON parses JSON text into nadesiko values, keeping the key order the
// text had.
func decodeJSON(s string) (value.Value, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return value.Undefined(), errors.New("JSONデコードに失敗しました。")
	}
	v, err := decodeJSONValue(dec, tok)
	if err != nil {
		return value.Undefined(), errors.New("JSONデコードに失敗しました。")
	}
	return v, nil
}

func decodeJSONValue(dec *json.Decoder, tok json.Token) (value.Value, error) {
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '[':
			var items []value.Value
			for dec.More() {
				next, err := dec.Token()
				if err != nil {
					return value.Undefined(), err
				}
				item, err := decodeJSONValue(dec, next)
				if err != nil {
					return value.Undefined(), err
				}
				items = append(items, item)
			}
			if _, err := dec.Token(); err != nil { // ']'
				return value.Undefined(), err
			}
			return value.ArrayValue(value.NewArray(items...)), nil
		case '{':
			d := value.NewDict()
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return value.Undefined(), err
				}
				key, _ := keyTok.(string)
				valTok, err := dec.Token()
				if err != nil {
					return value.Undefined(), err
				}
				item, err := decodeJSONValue(dec, valTok)
				if err != nil {
					return value.Undefined(), err
				}
				d.Set(key, item)
			}
			if _, err := dec.Token(); err != nil { // '}'
				return value.Undefined(), err
			}
			return value.DictValue(d), nil
		}
		return value.Undefined(), errors.New("JSONの括弧が対応していません。")
	case nil:
		return value.Null(), nil
	case bool:
		return value.Bool(t), nil
	case string:
		return value.String(t), nil
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return value.Undefined(), err
		}
		return value.Number(f), nil
	}
	return value.Undefined(), errors.New("JSONに未知の値があります。")
}
