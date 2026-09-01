package nodelib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

func netCommands(m map[string]command) {
	client := &http.Client{Timeout: 30 * time.Second}

	m["自分IPアドレス取得"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return value.String("127.0.0.1"), nil
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return value.String(ipnet.IP.String()), nil
				}
			}
		}
		return value.String("127.0.0.1"), nil
	}}

	m["自分IPV6アドレス取得"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return value.String("::1"), nil
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() == nil && ipnet.IP.To16() != nil {
					return value.String(ipnet.IP.String()), nil
				}
			}
		}
		return value.String("::1"), nil
	}}

	m["POSTデータ生成"] = command{
		josi: [][]string{{"の", "を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			v := argAt(a, 0)
			if d, ok := v.Dict(); ok {
				vals := url.Values{}
				for _, k := range d.Keys() {
					if item, ok := d.Get(k); ok {
						vals.Set(k, value.ToString(item))
					}
				}
				return value.String(vals.Encode()), nil
			}
			return value.String(value.ToString(v)), nil
		},
	}

	m["AJAXテキスト取得"] = command{
		josi: [][]string{{"から", "の", "を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			reqURL := str(a, 0)
			resp, err := client.Get(reqURL)
			if err != nil {
				return value.Undefined(), err
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return value.Undefined(), err
			}
			return value.String(string(data)), nil
		},
	}
	m["AJAX内容取得"] = m["AJAXテキスト取得"]
	m["AJAX受信"] = m["AJAXテキスト取得"]

	m["AJAX_JSON取得"] = command{
		josi: [][]string{{"から", "の", "を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			reqURL := str(a, 0)
			resp, err := client.Get(reqURL)
			if err != nil {
				return value.Undefined(), err
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return value.Undefined(), err
			}
			return parseJSONBytes(data)
		},
	}

	m["AJAXバイナリ取得"] = command{
		josi: [][]string{{"から", "の", "を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			reqURL := str(a, 0)
			resp, err := client.Get(reqURL)
			if err != nil {
				return value.Undefined(), err
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return value.Undefined(), err
			}
			items := make([]value.Value, len(data))
			for i, b := range data {
				items[i] = value.Number(float64(b))
			}
			return value.ArrayValue(value.NewArray(items...)), nil
		},
	}

	m["POST送信"] = command{
		josi: [][]string{{"まで", "へ", "に"}, {"を", "の"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			reqURL := str(a, 0)
			bodyStr := str(a, 1)
			resp, err := client.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(bodyStr))
			if err != nil {
				return value.Undefined(), err
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return value.Undefined(), err
			}
			return value.String(string(data)), nil
		},
	}

	m["POSTフォーム送信"] = command{
		josi: [][]string{{"まで", "へ", "に"}, {"を", "の"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			reqURL := str(a, 0)
			var bodyStr string
			v := argAt(a, 1)
			if d, ok := v.Dict(); ok {
				vals := url.Values{}
				for _, k := range d.Keys() {
					if item, ok := d.Get(k); ok {
						vals.Set(k, value.ToString(item))
					}
				}
				bodyStr = vals.Encode()
			} else {
				bodyStr = value.ToString(v)
			}
			resp, err := client.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(bodyStr))
			if err != nil {
				return value.Undefined(), err
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return value.Undefined(), err
			}
			return value.String(string(data)), nil
		},
	}

	m["DISCORD送信"] = command{
		josi: [][]string{{"へ", "に"}, {"を", "の"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			webhookURL := str(a, 0)
			msg := str(a, 1)
			payload := map[string]string{"content": msg}
			data, err := json.Marshal(payload)
			if err != nil {
				return value.Undefined(), err
			}
			resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(data))
			if err != nil {
				return value.Undefined(), err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				return value.Undefined(), fmt.Errorf("Discordへの送信に失敗しました (HTTP %d)", resp.StatusCode)
			}
			return value.Bool(true), nil
		},
	}
}

func parseJSONBytes(data []byte) (value.Value, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return value.Undefined(), err
	}
	return parseJSONValue(dec, tok)
}

func parseJSONValue(dec *json.Decoder, tok json.Token) (value.Value, error) {
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
				item, err := parseJSONValue(dec, next)
				if err != nil {
					return value.Undefined(), err
				}
				items = append(items, item)
			}
			if _, err := dec.Token(); err != nil {
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
				key, ok := keyTok.(string)
				if !ok {
					return value.Undefined(), fmt.Errorf("invalid json key: %v", keyTok)
				}
				valTok, err := dec.Token()
				if err != nil {
					return value.Undefined(), err
				}
				val, err := parseJSONValue(dec, valTok)
				if err != nil {
					return value.Undefined(), err
				}
				d.Set(key, val)
			}
			if _, err := dec.Token(); err != nil {
				return value.Undefined(), err
			}
			return value.DictValue(d), nil
		}
	case string:
		return value.String(t), nil
	case json.Number:
		f, err := strconv.ParseFloat(string(t), 64)
		if err != nil {
			return value.Undefined(), err
		}
		return value.Number(f), nil
	case bool:
		return value.Bool(t), nil
	case nil:
		return value.Null(), nil
	}
	return value.Undefined(), nil
}
