package nodelib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// encodeDictAsFormData は辞書のキーを挿入順でURLエンコードして
// application/x-www-form-urlencoded 形式の文字列にする。
// url.Values.Encode() はキーをソートしてしまうため、挿入順を保つために手で組み立てる。
func encodeDictAsFormData(d *value.Dict) string {
	var pairs []string
	for _, k := range d.Keys() {
		if item, ok := d.Get(k); ok {
			pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(value.ToString(item)))
		}
	}
	return strings.Join(pairs, "&")
}

// buildMultipartForm は辞書を multipart/form-data のボディに組み立てる。
// 戻り値は (ボディ, Content-Type)。辞書以外はエラーにする。
// POSTデータ生成の戻り値（URLエンコード済み文字列）を渡すミスが起きやすいため、案内を添える。
func buildMultipartForm(cmdName string, v value.Value) (*bytes.Buffer, string, error) {
	d, ok := v.Dict()
	if !ok {
		hint := ""
		if v.Kind() == value.KindString {
			hint = "。『POSTデータ生成』で作った文字列は渡せません。辞書型のデータをそのまま渡してください（文字列のボディを送るには『POST送信』を使います）"
		}
		return nil, "", fmt.Errorf("『%s』のパラメータには辞書型のデータを指定してください%s", cmdName, hint)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, k := range d.Keys() {
		item, ok := d.Get(k)
		if !ok {
			continue
		}
		if err := mw.WriteField(k, value.ToString(item)); err != nil {
			return nil, "", err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return &body, mw.FormDataContentType(), nil
}

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
				return value.String(encodeDictAsFormData(d)), nil
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
			var bodyStr string
			v := argAt(a, 1)
			if d, ok := v.Dict(); ok {
				bodyStr = encodeDictAsFormData(d)
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

	m["POSTフォーム送信"] = command{
		josi: [][]string{{"まで", "へ", "に"}, {"を", "の"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			reqURL := str(a, 0)
			v := argAt(a, 1)
			body, contentType, err := buildMultipartForm("POSTフォーム送信", v)
			if err != nil {
				return value.Undefined(), err
			}
			resp, err := client.Post(reqURL, contentType, body)
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

	m["AJAXオプション設定"] = command{
		josi:       [][]string{{"に", "へ", "と"}},
		returnNone: true,
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			ctx.SetSysVar("AJAXオプション", argAt(a, 0))
			return value.Undefined(), nil
		},
	}

	m["AJAX失敗時"] = command{
		josi:       [][]string{{"の"}},
		returnNone: true,
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			ctx.SetSysVar("AJAX:ONERROR", argAt(a, 0))
			return value.Undefined(), nil
		},
	}

	ajaxSendFn := func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
		callbackVal := argAt(a, 0)
		reqURL := str(a, 1)
		resp, err := client.Get(reqURL)
		if err != nil {
			if onErr := ctx.SysVar("AJAX:ONERROR"); onErr.Kind() != value.KindUndefined {
				if fn, ok := toFunc(ctx, onErr); ok {
					_, _ = ctx.CallFunc(fn, []value.Value{value.String(err.Error())})
					return value.Undefined(), nil
				}
			}
			return value.Undefined(), err
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return value.Undefined(), err
		}
		textVal := value.String(string(data))
		ctx.SetSysVar("対象", textVal)
		if fn, ok := toFunc(ctx, callbackVal); ok {
			return ctx.CallFunc(fn, []value.Value{textVal})
		}
		return value.Undefined(), nil
	}
	m["AJAX送信時"] = command{josi: [][]string{{"の", "で"}, {"まで", "へ", "に"}}, returnNone: true, fn: ajaxSendFn}
	m["AJAX受信時"] = command{josi: [][]string{{"で", "の"}, {"から", "を", "まで", "へ", "に"}}, returnNone: true, fn: ajaxSendFn}
	m["GET送信時"] = command{josi: [][]string{{"の", "で"}, {"まで", "へ", "に", "から", "を"}}, returnNone: true, fn: ajaxSendFn}

	m["POST送信時"] = command{
		josi:       [][]string{{"の", "で"}, {"まで", "へ", "に"}, {"を", "の", "で"}},
		returnNone: true,
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			callbackVal := argAt(a, 0)
			reqURL := str(a, 1)
			var bodyStr string
			v := argAt(a, 2)
			if d, ok := v.Dict(); ok {
				bodyStr = encodeDictAsFormData(d)
			} else {
				bodyStr = value.ToString(v)
			}
			resp, err := client.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(bodyStr))
			if err != nil {
				if onErr := ctx.SysVar("AJAX:ONERROR"); onErr.Kind() != value.KindUndefined {
					if fn, ok := toFunc(ctx, onErr); ok {
						_, _ = ctx.CallFunc(fn, []value.Value{value.String(err.Error())})
						return value.Undefined(), nil
					}
				}
				return value.Undefined(), err
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return value.Undefined(), err
			}
			textVal := value.String(string(data))
			ctx.SetSysVar("対象", textVal)
			if fn, ok := toFunc(ctx, callbackVal); ok {
				return ctx.CallFunc(fn, []value.Value{textVal})
			}
			return value.Undefined(), nil
		},
	}

	m["POSTフォーム送信時"] = command{
		josi:       [][]string{{"の", "で"}, {"まで", "へ", "に"}, {"を", "の", "で"}},
		returnNone: true,
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			callbackVal := argAt(a, 0)
			reqURL := str(a, 1)
			v := argAt(a, 2)
			body, contentType, err := buildMultipartForm("POSTフォーム送信時", v)
			if err != nil {
				return value.Undefined(), err
			}
			resp, err := client.Post(reqURL, contentType, body)
			if err != nil {
				if onErr := ctx.SysVar("AJAX:ONERROR"); onErr.Kind() != value.KindUndefined {
					if fn, ok := toFunc(ctx, onErr); ok {
						_, _ = ctx.CallFunc(fn, []value.Value{value.String(err.Error())})
						return value.Undefined(), nil
					}
				}
				return value.Undefined(), err
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return value.Undefined(), err
			}
			textVal := value.String(string(data))
			ctx.SetSysVar("対象", textVal)
			if fn, ok := toFunc(ctx, callbackVal); ok {
				return ctx.CallFunc(fn, []value.Value{textVal})
			}
			return value.Undefined(), nil
		},
	}

	m["AJAX保障送信"] = command{
		josi: [][]string{{"まで", "へ", "に", "の", "から", "を"}},
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
	m["HTTP保障取得"] = command{
		josi: [][]string{{"の", "から", "を", "まで", "へ", "に"}},
		fn:   m["AJAX保障送信"].fn,
	}
	m["GET保障送信"] = m["AJAX保障送信"]

	m["POST保障送信"] = command{
		josi: [][]string{{"まで", "へ", "に", "の", "から", "を"}, {"を", "の", "で"}},
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			return m["POST送信"].fn(ctx, a)
		},
	}

	m["POSTフォーム保障送信"] = command{
		josi: [][]string{{"まで", "へ", "に", "の", "から", "を"}, {"を", "の", "で"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			reqURL := str(a, 0)
			v := argAt(a, 1)
			body, contentType, err := buildMultipartForm("POSTフォーム保障送信", v)
			if err != nil {
				return value.Undefined(), err
			}
			resp, err := client.Post(reqURL, contentType, body)
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

	m["DISCORDファイル送信"] = command{
		josi:       [][]string{{"へ", "に"}, {"と"}, {"を"}},
		returnNone: true,
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			webhookURL := str(a, 0)
			filePath := str(a, 1)
			msg := str(a, 2)

			var fileData []byte
			var ok bool
			if fileData, ok = ctx.ReadResource(filePath); !ok {
				var err error
				fileData, err = os.ReadFile(filePath)
				if err != nil {
					return value.Undefined(), fileError("読み込め", filePath, err)
				}
			}

			var body bytes.Buffer
			mw := multipart.NewWriter(&body)
			if err := mw.WriteField("content", msg); err != nil {
				return value.Undefined(), err
			}
			part, err := mw.CreateFormFile("file", filepath.Base(filePath))
			if err != nil {
				return value.Undefined(), err
			}
			if _, err := part.Write(fileData); err != nil {
				return value.Undefined(), err
			}
			if err := mw.Close(); err != nil {
				return value.Undefined(), err
			}

			resp, err := client.Post(webhookURL, mw.FormDataContentType(), &body)
			if err != nil {
				return value.Undefined(), err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				return value.Undefined(), fmt.Errorf("『DISCORDファイル送信』に失敗しました (HTTP %d)", resp.StatusCode)
			}
			return value.Undefined(), nil
		},
	}
}

// parseJSONBytes はJSONバイト列をパースしてなでしこの値に変換する。
// 『JSONデコード』（stdlib.decodeJSON）と同じ規則・文面で、
// 解析失敗はすべて「JSONデコードに失敗しました。」を返す。
func parseJSONBytes(data []byte) (value.Value, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return value.Undefined(), errors.New("JSONデコードに失敗しました。")
	}
	v, err := parseJSONValue(dec, tok)
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
