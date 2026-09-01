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
		_, query, found := strings.Cut(str(a, 0), "?")
		if !found {
			return value.DictValue(d), nil
		}
		for _, field := range strings.Split(query, "&") {
			if field == "" {
				continue
			}
			key, val, _ := strings.Cut(field, "=")
			decodedKey, err := url.PathUnescape(key)
			if err != nil {
				return value.Undefined(), err
			}
			decodedVal, err := url.PathUnescape(val)
			if err != nil {
				return value.Undefined(), err
			}
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
		if s == "" || strings.HasSuffix(s, "/") {
			return value.String(s), nil
		}
		return value.String(s + "/"), nil
	}
	m["終端パス除去"] = func(_ Context, a []value.Value) (value.Value, error) {
		return value.String(strings.TrimSuffix(str(a, 0), "/")), nil
	}
	m["終端パス削除"] = m["終端パス除去"]
	m["パス抽出"] = func(_ Context, a []value.Value) (value.Value, error) {
		s := str(a, 0)
		if i := strings.LastIndex(s, "/"); i >= 0 {
			return value.String(s[:i]), nil
		}
		return value.String(""), nil
	}
	m["ファイル名抽出"] = func(_ Context, a []value.Value) (value.Value, error) {
		s := str(a, 0)
		if i := strings.LastIndex(s, "/"); i >= 0 {
			return value.String(s[i+1:]), nil
		}
		return value.String(s), nil
	}
	m["拡張子変更"] = func(_ Context, a []value.Value) (value.Value, error) {
		name, ext := str(a, 0), strings.TrimSpace(str(a, 1))
		if ext != "" && !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		old := path.Ext(name)
		return value.String(strings.TrimSuffix(name, old) + ext), nil
	}
}
