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

// ajaxOptions は『AJAXオプション設定』で設定された辞書を検証した結果。
// 対応範囲は Issue #331 で次のとおり固定した（テストは ajax_options_test.go、
// 文書は docs/compat.md 第8節）。
//
//   - オプションを読む命令: 『AJAXテキスト取得』（『AJAX内容取得』『AJAX受信』は
//     同一実装の別名）、『AJAX_JSON取得』、『AJAXバイナリ取得』、
//     『AJAX送信時』（『AJAX受信時』『GET送信時』は同一実装）、
//     『AJAX保障送信』（『HTTP保障取得』『GET保障送信』は同一実装）。
//     本家 TypeScript 版が fetch(url, options) に AJAXオプションを渡す命令群と一致する。
//   - オプションを読まない命令: 『POST送信』『POSTフォーム送信』『POST送信時』
//     『POSTフォーム送信時』『POST保障送信』『POSTフォーム保障送信』と
//     DISCORD系。これらは本家でも自前の method・Content-Type・body を組み立てる
//     専用命令であり、AJAXオプションを波及させない。
//   - 対応項目は method（文字列）・headers（辞書）・body（文字列）だけ。
//     それ以外の項目は無言で捨てずエラーにする。
type ajaxOptions struct {
	method  string
	headers http.Header
	host    string
	hasHost bool
	body    string
	hasBody bool
}

// isHTTPTokenChar は RFC 9110 の token（メソッド名・ヘッダー名に使える文字）か。
func isHTTPTokenChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}
	return false
}

func isHTTPToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isHTTPTokenChar(s[i]) {
			return false
		}
	}
	return true
}

// validHeaderValue はヘッダー値に使える文字だけか（タブと表示可能な文字、
// および obs-text）。改行を含むとヘッダー注入になるため、事前に弾く。
func validHeaderValue(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\t' || (c >= 0x20 && c != 0x7f) {
			continue
		}
		return false
	}
	return true
}

// parseAjaxOptions は『AJAXオプション』の値を検証して ajaxOptions にする。
// 未設定・空文字列・空辞書は従来どおり GET・追加ヘッダーなし・body なしになる。
func parseAjaxOptions(v value.Value) (ajaxOptions, error) {
	opts := ajaxOptions{method: http.MethodGet, headers: make(http.Header)}
	switch v.Kind() {
	case value.KindUndefined:
		return opts, nil
	case value.KindString:
		if s, _ := v.String(); strings.TrimSpace(s) == "" {
			return opts, nil
		}
		return opts, fmt.Errorf("AJAXオプションには辞書を指定してください")
	}
	d, ok := v.Dict()
	if !ok {
		return opts, fmt.Errorf("AJAXオプションには辞書を指定してください")
	}
	seen := make(map[string]bool)
	for _, key := range d.Keys() {
		item, _ := d.Get(key)
		name := strings.ToLower(key)
		switch name {
		case "method", "headers", "body":
			if seen[name] {
				return opts, fmt.Errorf("AJAXオプションの項目『%s』が重複しています", key)
			}
			seen[name] = true
		default:
			return opts, fmt.Errorf("AJAXオプションの項目『%s』には対応していません。対応している項目はmethod・headers・bodyです", key)
		}
		switch name {
		case "method":
			if item.Kind() != value.KindString {
				return opts, fmt.Errorf("AJAXオプションのmethodには文字列を指定してください")
			}
			s, _ := item.String()
			method := strings.ToUpper(strings.TrimSpace(s))
			if !isHTTPToken(method) {
				return opts, fmt.Errorf("AJAXオプションのmethod『%s』が不正です", s)
			}
			opts.method = method
		case "body":
			if item.Kind() != value.KindString {
				return opts, fmt.Errorf("AJAXオプションのbodyには文字列を指定してください")
			}
			opts.body, _ = item.String()
			opts.hasBody = true
		case "headers":
			hd, ok := item.Dict()
			if !ok {
				return opts, fmt.Errorf("AJAXオプションのheadersには辞書を指定してください")
			}
			seenHeaders := make(map[string]bool)
			for _, hk := range hd.Keys() {
				hv, _ := hd.Get(hk)
				if seenHeaders[strings.ToLower(hk)] {
					return opts, fmt.Errorf("AJAXオプションのヘッダー『%s』が重複しています", hk)
				}
				seenHeaders[strings.ToLower(hk)] = true
				if !isHTTPToken(hk) {
					return opts, fmt.Errorf("AJAXオプションのヘッダー名『%s』が不正です", hk)
				}
				switch hv.Kind() {
				case value.KindString, value.KindNumber, value.KindBool:
				default:
					return opts, fmt.Errorf("AJAXオプションのヘッダー『%s』の値には文字列を指定してください", hk)
				}
				val := value.ToString(hv)
				if !validHeaderValue(val) {
					return opts, fmt.Errorf("AJAXオプションのヘッダー『%s』の値が不正です", hk)
				}
				// Go の http.Client が req.Header では無視するヘッダーの扱い。
				// 無言で捨てないため、Host は req.Host へ反映し、
				// Transport が管理する Content-Length / Transfer-Encoding はエラーにする。
				switch strings.ToLower(hk) {
				case "host":
					opts.host = val
					opts.hasHost = true
				case "content-length", "transfer-encoding":
					return opts, fmt.Errorf("AJAXオプションのヘッダー『%s』は指定できません", hk)
				default:
					opts.headers.Set(hk, val)
				}
			}
		}
	}
	// fetch と同じく、GET・HEAD のリクエストに body は付けられない。
	// 付けても Go の Transport が黙って捨てる場合があるため、ここで明示する。
	if opts.hasBody && (opts.method == http.MethodGet || opts.method == http.MethodHead) {
		return opts, fmt.Errorf("AJAXオプションのbodyはmethodがGET・HEADのとき指定できません")
	}
	return opts, nil
}

// newAjaxRequest は『AJAXオプション』を反映したリクエストを作る共通処理。
// AJAXオプション対応命令は client.Get ではなく、必ずこちらを使う。
func newAjaxRequest(optVal value.Value, reqURL string) (*http.Request, error) {
	opts, err := parseAjaxOptions(optVal)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if opts.hasBody {
		body = strings.NewReader(opts.body)
	}
	req, err := http.NewRequest(opts.method, reqURL, body)
	if err != nil {
		return nil, err
	}
	for k, vs := range opts.headers {
		req.Header[k] = append([]string(nil), vs...)
	}
	if opts.hasHost {
		req.Host = opts.host
	}
	return req, nil
}

// doAjax は AJAXオプション対応命令の共通送信処理。
func doAjax(client *http.Client, optVal value.Value, reqURL string) (*http.Response, error) {
	req, err := newAjaxRequest(optVal, reqURL)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func netCommands(m map[string]command) {
	client := &http.Client{Timeout: 30 * time.Second}

	// ajaxOptionValue は現在の『AJAXオプション』の実効値を返す。
	//
	// VMはプログラム中に名前が現れない変数に記憶領域を割り当てないため、
	// 『AJAXオプション設定』の SetSysVar が黙って捨てられることがある
	// （guilib の『DOM親要素』と同じ事情）。その場合でも設定が届くよう、
	// 設定命令は実行単位の CommandState にも控えを保存する。状態を
	// netCommands のクロージャ変数に持つと、レジストリを使い回すホストで
	// 実行をまたいで残ってしまうため、控えも判定結果も CommandState に置く。
	// 読み出し側は、システム変数が使える場合（書いて読み直せる場合）に
	// だけそちらを正とし、プログラム中の直接代入を優先する。使えない場合は
	// 控えを使う（その場合は直接代入自体がプログラムに現れない）。
	ajaxOptionValue := func(ctx stdlib.Context) value.Value {
		usable := ctx.CommandState("__ajaxOptionUsable")
		if usable.Kind() == value.KindUndefined {
			saved := ctx.SysVar("AJAXオプション")
			const probe = "__AJAXオプション確認用__"
			ctx.SetSysVar("AJAXオプション", value.String(probe))
			ok := false
			if s, isStr := ctx.SysVar("AJAXオプション").String(); isStr && s == probe {
				ok = true
			}
			ctx.SetSysVar("AJAXオプション", saved)
			ctx.SetCommandState("__ajaxOptionUsable", value.Bool(ok))
			usable = value.Bool(ok)
		}
		if b, _ := usable.Bool(); b {
			return ctx.SysVar("AJAXオプション")
		}
		if savedCopy := ctx.CommandState("__ajaxOptionCopy"); savedCopy.Kind() != value.KindUndefined {
			return savedCopy
		}
		return ctx.SysVar("AJAXオプション")
	}

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
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			reqURL := str(a, 0)
			resp, err := doAjax(client, ajaxOptionValue(ctx), reqURL)
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
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			reqURL := str(a, 0)
			resp, err := doAjax(client, ajaxOptionValue(ctx), reqURL)
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
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			reqURL := str(a, 0)
			resp, err := doAjax(client, ajaxOptionValue(ctx), reqURL)
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
			v := argAt(a, 0)
			ctx.SetSysVar("AJAXオプション", v)
			// システム変数に記憶領域が無い場合に備えた控え（ajaxOptionValue のコメント参照）。
			ctx.SetCommandState("__ajaxOptionCopy", v)
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
		resp, err := doAjax(client, ajaxOptionValue(ctx), reqURL)
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
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			reqURL := str(a, 0)
			resp, err := doAjax(client, ajaxOptionValue(ctx), reqURL)
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
