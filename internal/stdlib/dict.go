package stdlib

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"sort"
	"strconv"
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
	m["ハッシュキー列挙"] = m["辞書キー列挙"]
	m["ハッシュ内容列挙"] = func(_ Context, a []value.Value) (value.Value, error) {
		d, ok := arg(a, 0).Dict()
		if !ok {
			return value.Undefined(), errors.New("『ハッシュ内容列挙』でハッシュ以外が与えられました。")
		}
		items := make([]value.Value, 0, d.Len())
		for _, key := range d.Keys() {
			v, _ := d.Get(key)
			items = append(items, v)
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	}

	m["辞書キー存在"] = func(_ Context, a []value.Value) (value.Value, error) {
		if d, ok := arg(a, 0).Dict(); ok {
			_, found := d.Get(str(a, 1))
			return value.Bool(found), nil
		}
		if arr, ok := arg(a, 0).Array(); ok {
			return value.Bool(arr.HasProp(str(a, 1))), nil
		}
		return value.Bool(false), nil
	}
	m["ハッシュキー存在"] = m["辞書キー存在"]

	m["辞書キー削除"] = func(_ Context, a []value.Value) (value.Value, error) {
		if d, ok := arg(a, 0).Dict(); ok {
			d.Delete(str(a, 1))
			return arg(a, 0), nil
		}
		if arr, ok := arg(a, 0).Array(); ok {
			arr.DeleteProp(str(a, 1))
			return arg(a, 0), nil
		}
		return value.Undefined(), errors.New("『辞書キー削除』で辞書以外が指定されました。")
	}
	m["ハッシュキー削除"] = m["辞書キー削除"]

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
	m["JSON変換"] = m["JSONエンコード"]
	m["JSON取得"] = m["JSONデコード"]
	m["JSONエンコード整形"] = func(_ Context, a []value.Value) (value.Value, error) {
		plain, err := encodeJSON(arg(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		// 汎用のmapへ戻すとキー順がソートされ、<, >, & も再エスケープされる。
		// エンコード済みの文字列はそのままに、空白と改行だけを整形する。
		var b bytes.Buffer
		if err := json.Indent(&b, []byte(plain), "", "  "); err != nil {
			return value.Undefined(), err
		}
		return value.String(b.String()), nil
	}
}

// EncodeJSON はなでしこの値をJSON.stringifyと同じ規則でJSON文字列にする。
// gonako-gui のwnako3ブリッジなど、パッケージの外から値を受け渡すために公開する。
func EncodeJSON(v value.Value) (string, error) { return encodeJSON(v) }

// DecodeJSON はJSON文字列をキーの順序を保ったままなでしこの値にする。
func DecodeJSON(s string) (value.Value, error) { return decodeJSON(s) }

// encodeJSON renders a value the way JSON.stringify does: no HTML escaping,
// undefined and function values dropped from objects, and keys ordered the
// JavaScript way (array-index keys first in numeric order, then the rest in
// insertion order).
func encodeJSON(v value.Value) (string, error) {
	var b strings.Builder
	if err := writeJSON(&b, v, make(map[any]bool)); err != nil {
		return "", err
	}
	return b.String(), nil
}

// jsonDictKeys は辞書のキーを JSON.stringify の順序で返す (Issue #417)。
// JavaScript の OrdinaryOwnPropertyKeys と同じく、配列インデックス形式の
// キーを数値の昇順で先に並べ、それ以外のキーは挿入順のまま後ろに置く。
// 並び替えはこの出力のときだけであり、Dict 自身の保持順 (反復や
// 『辞書キー列挙』の結果) は変えない。インデックス形式のキーが無ければ
// 余分な振り分けをせず、そのまま返す。
func jsonDictKeys(d *value.Dict) []string {
	keys := d.Keys()
	type indexedKey struct {
		key string
		num uint32
	}
	var indexed []indexedKey
	for _, k := range keys {
		if n, ok := jsArrayIndex(k); ok {
			indexed = append(indexed, indexedKey{key: k, num: n})
		}
	}
	if len(indexed) == 0 {
		return keys
	}
	sort.SliceStable(indexed, func(i, j int) bool { return indexed[i].num < indexed[j].num })
	ordered := make([]string, 0, len(keys))
	for _, ik := range indexed {
		ordered = append(ordered, ik.key)
	}
	for _, k := range keys {
		if _, ok := jsArrayIndex(k); !ok {
			ordered = append(ordered, k)
		}
	}
	return ordered
}

// jsArrayIndex はキーが JavaScript の配列インデックスかどうかを返す。
// 定義 (OrdinaryOwnPropertyKeys / array index) は、文字列が
// ToString(ToUint32(P)) と一致し、かつ 2^32-1 でないこと。つまり
// "0" から "4294967294" (2^32-2) までの10進表記であり、先行ゼロ
// ("01"・"00")・負数 ("-1"・"-0")・小数 ("1.0")・符号 ("+1")・
// 空白 (" 1")・空文字列はインデックスではない。
func jsArrayIndex(key string) (uint32, bool) {
	if key == "" || len(key) > 10 {
		return 0, false
	}
	if len(key) > 1 && key[0] == '0' {
		return 0, false // 先行ゼロは正規の10進表記ではない
	}
	for i := 0; i < len(key); i++ {
		if key[i] < '0' || key[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(key, 10, 64)
	if err != nil || n > 4294967294 {
		return 0, false
	}
	return uint32(n), true
}

// writeJSON は値をJSONとして書き出す。seen は現在の再帰経路上にある
// 辞書・配列を記録し、循環参照を検出する（辞書・配列は参照型なので、
// 自身や祖先を要素として持ちうる）。循環参照は JavaScript の
// JSON.stringify が投げる TypeError と同じく、明示的なエラーにする。
// Go のスタックオーバーフローは回復不能なので、ここで止める。
func writeJSON(b *strings.Builder, v value.Value, seen map[any]bool) error {
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
		if seen[arr] {
			return errors.New("JSONエンコードできません。配列が循環参照しています。")
		}
		seen[arr] = true
		defer delete(seen, arr)
		b.WriteByte('[')
		for i := 0; i < arr.Len(); i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeJSON(b, arr.Get(i), seen); err != nil {
				return err
			}
		}
		b.WriteByte(']')
		return nil
	case value.KindDict:
		d, _ := v.Dict()
		if seen[d] {
			return errors.New("JSONエンコードできません。辞書が循環参照しています。")
		}
		seen[d] = true
		defer delete(seen, d)
		b.WriteByte('{')
		first := true
		for _, k := range jsonDictKeys(d) {
			item, _ := d.Get(k)
			if item.Kind() == value.KindUndefined || item.Kind() == value.KindFunc {
				// undefined と関数のキーは出力しない (JSON.stringify と同じ)。
				// 配列の中の関数は従来どおり null になる (writeJSON の先頭の分岐)。
				continue
			}
			if !first {
				b.WriteByte(',')
			}
			first = false
			if err := writeJSONString(b, k); err != nil {
				return err
			}
			b.WriteByte(':')
			if err := writeJSON(b, item, seen); err != nil {
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
	// 先頭の値の後に空白以外が残っていればエラーにする。
	// JSON.parse が末尾のゴミを許さないのと同じ規則。
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
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
