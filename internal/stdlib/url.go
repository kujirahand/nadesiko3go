package stdlib

import (
	"encoding/base64"
	"net/url"
	"path"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

func urlImpls(m map[string]Impl) {
	m["URLエンコード"] = func(_ Context, a []value.Value) (value.Value, error) {
		s := url.QueryEscape(str(a, 0))
		return value.String(strings.ReplaceAll(s, "+", "%20")), nil
	}
	m["URLデコード"] = func(_ Context, a []value.Value) (value.Value, error) {
		s, err := url.PathUnescape(str(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		return value.String(s), nil
	}
	m["URLパラメータ解析"] = func(_ Context, a []value.Value) (value.Value, error) {
		d := value.NewDict()
		s := str(a, 0)
		// # 以降はフラグメントとして扱い、クエリ解析から除外する
		if i := strings.Index(s, "#"); i >= 0 {
			s = s[:i]
		}
		_, query, found := strings.Cut(s, "?")
		if !found {
			return value.DictValue(d), nil
		}
		for _, field := range strings.Split(query, "&") {
			if field == "" {
				continue
			}
			key, val, _ := strings.Cut(field, "=")
			decodedKey := decodeQueryComponent(key)
			decodedVal := decodeQueryComponent(val)
			d.Set(decodedKey, value.String(decodedVal))
		}
		return value.DictValue(d), nil
	}
	m["BASE64エンコード"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.String(base64.StdEncoding.EncodeToString([]byte(str(a, 0)))), nil
	}
	m["BASE64デコード"] = func(_ Context, a []value.Value) (value.Value, error) {
		decoded, err := base64.StdEncoding.DecodeString(str(a, 0))
		if err != nil {
			return value.Undefined(), err
		}
		return value.String(string(decoded)), nil
	}
	m["終端パス追加"] = func(_ Context, a []value.Value) (value.Value, error) {
		s := str(a, 0)
		if s == "" || strings.HasSuffix(s, "/") || strings.HasSuffix(s, `\`) {
			return value.String(s), nil
		}
		sep := "/"
		if strings.Contains(s, `\`) {
			sep = `\`
		}
		return value.String(s + sep), nil
	}
	m["終端パス除去"] = func(_ Context, a []value.Value) (value.Value, error) {
		s := str(a, 0)
		if strings.HasSuffix(s, "/") || strings.HasSuffix(s, `\`) {
			return value.String(s[:len(s)-1]), nil
		}
		return value.String(s), nil
	}
	m["終端パス削除"] = m["終端パス除去"]
	m["パス抽出"] = func(_ Context, a []value.Value) (value.Value, error) {
		s := str(a, 0)
		if i := lastPathSep(s); i >= 0 {
			return value.String(s[:i]), nil
		}
		return value.String(""), nil
	}
	m["ファイル名抽出"] = func(_ Context, a []value.Value) (value.Value, error) {
		s := str(a, 0)
		if i := lastPathSep(s); i >= 0 {
			return value.String(s[i+1:]), nil
		}
		return value.String(s), nil
	}
	m["拡張子変更"] = func(_ Context, a []value.Value) (value.Value, error) {
		name, ext := str(a, 0), strings.TrimSpace(str(a, 1))
		if ext != "" && !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		old := path.Ext(strings.ReplaceAll(name, `\`, "/"))
		return value.String(strings.TrimSuffix(name, old) + ext), nil
	}
	m["拡張子抽出"] = func(_ Context, a []value.Value) (value.Value, error) {
		s := str(a, 0)
		if i := lastPathSep(s); i >= 0 {
			s = s[i+1:]
		}
		if i := strings.LastIndex(s, "."); i >= 0 {
			return value.String(s[i:]), nil
		}
		return value.String(""), nil
	}
}

func lastPathSep(s string) int {
	i1 := strings.LastIndex(s, "/")
	i2 := strings.LastIndex(s, `\`)
	if i1 > i2 {
		return i1
	}
	return i2
}

// decodeQueryComponent は JavaScript の URLSearchParams と同じルールで
// クエリのキー・値をデコードする。'+' は空白に変換し、%XX は有効な
// 16進数のときだけデコードする。不正な % エスケープはそのまま残し、
// デコード後の無効な UTF-8 シーケンスは � に置換する。
func decodeQueryComponent(s string) string {
	s = strings.ReplaceAll(s, "+", " ")
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+2 >= len(s) || !isHexByte(s[i+1]) || !isHexByte(s[i+2]) {
			b.WriteByte(s[i])
			continue
		}
		b.WriteByte(hexValue(s[i+1])<<4 | hexValue(s[i+2]))
		i += 2
	}
	return strings.ToValidUTF8(b.String(), "�")
}

func isHexByte(c byte) bool {
	return ('0' <= c && c <= '9') || ('A' <= c && c <= 'F') || ('a' <= c && c <= 'f')
}

func hexValue(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}
