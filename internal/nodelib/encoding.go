package nodelib

import (
	"errors"
	"io"
	"os"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

func encodingCommands(m map[string]command) {
	m["文字コード変換サポート判定"] = command{
		josi: [][]string{{"の", "を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			enc := findEncoding(str(a, 0))
			return value.Bool(enc != nil), nil
		},
	}

	m["SJIS変換"] = command{
		josi: [][]string{{"に", "へ", "を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			text := str(a, 0)
			encoded, err := encodeString(text, japanese.ShiftJIS)
			if err != nil {
				return value.Undefined(), err
			}
			return toByteArrayValue(encoded), nil
		},
	}

	m["SJIS取得"] = command{
		josi: [][]string{{"から", "を", "で"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			bytes := toBytes(argAt(a, 0))
			decoded, err := decodeBytes(bytes, japanese.ShiftJIS)
			if err != nil {
				return value.Undefined(), err
			}
			return value.String(decoded), nil
		},
	}

	m["SJISファイル読"] = command{
		josi: [][]string{{"を", "から"}},
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			name := str(a, 0)
			var data []byte
			var ok bool
			if data, ok = ctx.ReadResource(name); !ok {
				var err error
				data, err = os.ReadFile(name)
				if err != nil {
					return value.Undefined(), fileError("読み込め", name, err)
				}
			}
			decoded, err := decodeBytes(data, japanese.ShiftJIS)
			if err != nil {
				return value.Undefined(), fileError("デコードでき", name, err)
			}
			return value.String(decoded), nil
		},
	}

	m["SJISファイル保存"] = command{
		josi:       [][]string{{"を"}, {"へ", "に"}},
		returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			text := str(a, 0)
			dst := str(a, 1)
			data, err := encodeString(text, japanese.ShiftJIS)
			if err != nil {
				return value.Undefined(), fileError("エンコードでき", dst, err)
			}
			if err := os.WriteFile(dst, data, 0o644); err != nil {
				return value.Undefined(), fileError("保存でき", dst, err)
			}
			return value.Undefined(), nil
		},
	}

	m["EUCファイル読"] = command{
		josi: [][]string{{"を", "から"}},
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			name := str(a, 0)
			var data []byte
			var ok bool
			if data, ok = ctx.ReadResource(name); !ok {
				var err error
				data, err = os.ReadFile(name)
				if err != nil {
					return value.Undefined(), fileError("読み込め", name, err)
				}
			}
			decoded, err := decodeBytes(data, japanese.EUCJP)
			if err != nil {
				return value.Undefined(), fileError("デコードでき", name, err)
			}
			return value.String(decoded), nil
		},
	}

	m["EUCファイル保存"] = command{
		josi:       [][]string{{"を"}, {"へ", "に"}},
		returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			text := str(a, 0)
			dst := str(a, 1)
			data, err := encodeString(text, japanese.EUCJP)
			if err != nil {
				return value.Undefined(), fileError("エンコードでき", dst, err)
			}
			if err := os.WriteFile(dst, data, 0o644); err != nil {
				return value.Undefined(), fileError("保存でき", dst, err)
			}
			return value.Undefined(), nil
		},
	}

	m["エンコーディング変換"] = command{
		josi: [][]string{{"を"}, {"へ", "で"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			text := str(a, 0)
			codeName := str(a, 1)
			enc := findEncoding(codeName)
			if enc == nil {
				return value.Undefined(), errors.New("未対応のエンコーディング『" + codeName + "』です。")
			}
			data, err := encodeString(text, enc)
			if err != nil {
				return value.Undefined(), err
			}
			return toByteArrayValue(data), nil
		},
	}

	m["エンコーディング取得"] = command{
		josi: [][]string{{"を"}, {"から", "で"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			bytes := toBytes(argAt(a, 0))
			codeName := str(a, 1)
			enc := findEncoding(codeName)
			if enc == nil {
				return value.Undefined(), errors.New("未対応のエンコーディング『" + codeName + "』です。")
			}
			decoded, err := decodeBytes(bytes, enc)
			if err != nil {
				return value.Undefined(), err
			}
			return value.String(decoded), nil
		},
	}
}

func findEncoding(name string) encoding.Encoding {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch lower {
	case "sjis", "shift_jis", "shift-jis", "cp932", "ms932", "windows-31j":
		return japanese.ShiftJIS
	case "euc-jp", "euc_jp", "eucjp":
		return japanese.EUCJP
	case "iso-2022-jp", "iso2022jp", "jis":
		return japanese.ISO2022JP
	case "utf-8", "utf8":
		return unicode.UTF8
	}
	if enc, err := ianaindex.IANA.Encoding(lower); err == nil && enc != nil {
		return enc
	}
	if enc, err := ianaindex.MIME.Encoding(lower); err == nil && enc != nil {
		return enc
	}
	return nil
}

func encodeString(s string, enc encoding.Encoding) ([]byte, error) {
	encoder := enc.NewEncoder()
	return io.ReadAll(transform.NewReader(strings.NewReader(s), encoder))
}

func decodeBytes(b []byte, enc encoding.Encoding) (string, error) {
	decoder := enc.NewDecoder()
	decoded, err := io.ReadAll(transform.NewReader(strings.NewReader(string(b)), decoder))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func toBytes(v value.Value) []byte {
	if arr, ok := v.Array(); ok {
		b := make([]byte, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			b[i] = byte(value.ToNumber(arr.Get(i)))
		}
		return b
	}
	return []byte(value.ToString(v))
}

func toByteArrayValue(data []byte) value.Value {
	items := make([]value.Value, len(data))
	for i, b := range data {
		items[i] = value.Number(float64(b))
	}
	return value.ArrayValue(value.NewArray(items...))
}
