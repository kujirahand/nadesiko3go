// Package inilib はINI形式の文字列を扱う命令（INI取得・INI変換）を実装する。
// 本家の外部プラグイン相当ではなく、gonako独自の新規機能である。
// plugin_system の互換保証対象外なので、Goらしい実装でよい。
package inilib

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) FuncList() lexer.FuncList {
	return lexer.FuncList{
		"INI取得": {Name: "INI取得", Type: "func", Josi: [][]string{{"を", "の", "から"}}, Pure: true},
		"INI変換": {Name: "INI変換", Type: "func", Josi: [][]string{{"を", "から", "の"}}, Pure: true},
	}
}

func (p *Plugin) Impls() map[string]stdlib.Impl {
	return map[string]stdlib.Impl{
		"INI取得": func(_ stdlib.Context, args []value.Value) (value.Value, error) {
			s := ""
			if len(args) > 0 {
				s = value.ToString(args[0])
			}
			return parseINI(s)
		},
		"INI変換": func(_ stdlib.Context, args []value.Value) (value.Value, error) {
			var v value.Value = value.Undefined()
			if len(args) > 0 {
				v = args[0]
			}
			s, err := encodeINI(v)
			if err != nil {
				return value.Undefined(), err
			}
			return value.String(s), nil
		},
	}
}

// parseINI はINI文字列を辞書にする。
//   - [sec] の下のキーは {sec: {key: val}} の2段にし、セクション前のキーは最上位に置く
//   - 値はすべて文字列。行頭が ; か # の行はコメント（行途中の ; # は値の一部）
//   - 値全体が " で囲まれていれば引用符を外し、中身をそのまま値にする
//   - 同じキーは後の値が勝ち、同じセクションが再度現れたらマージする
func parseINI(s string) (value.Value, error) {
	s = strings.TrimPrefix(s, "\ufeff")
	root := value.NewDict()
	cur := root // キーの書き込み先（最初は最上位、[sec] の後はそのセクション）
	for i, raw := range strings.Split(s, "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || line[0] == ';' || line[0] == '#' {
			continue
		}
		if line[0] == '[' {
			if !strings.HasSuffix(line, "]") {
				return value.Undefined(), fmt.Errorf("INI取得に失敗しました。%d行目: セクションの括弧が閉じていません。", lineNo)
			}
			name := strings.TrimSpace(line[1 : len(line)-1])
			if name == "" {
				return value.Undefined(), fmt.Errorf("INI取得に失敗しました。%d行目: セクション名が空です。", lineNo)
			}
			if existing, ok := root.Get(name); ok {
				d, isDict := existing.Dict()
				if !isDict || d == nil {
					return value.Undefined(), fmt.Errorf("INI取得に失敗しました。%d行目: セクション「%s」が同名のキーと重複しています。", lineNo, name)
				}
				cur = d
			} else {
				cur = value.NewDict()
				root.Set(name, value.DictValue(cur))
			}
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			return value.Undefined(), fmt.Errorf("INI取得に失敗しました。%d行目: 「=」がありません。", lineNo)
		}
		key := strings.TrimSpace(line[:eq])
		if key == "" {
			return value.Undefined(), fmt.Errorf("INI取得に失敗しました。%d行目: キーが空です。", lineNo)
		}
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		if cur == root {
			// セクション前のキーが、既存のセクション名と衝突する場合
			if existing, ok := root.Get(key); ok {
				if d, isDict := existing.Dict(); isDict && d != nil {
					return value.Undefined(), fmt.Errorf("INI取得に失敗しました。%d行目: キー「%s」が同名のセクションと重複しています。", lineNo, key)
				}
			}
		}
		cur.Set(key, value.String(val))
	}
	return value.DictValue(root), nil
}

// encodeINI は辞書をINI文字列にする。
// INI形式ではセクションより前に書いたキーが最上位のキーになるため、
// 辞書のキー順そのままではなく「最上位スカラーを先に、セクションを後に」出力する
// （それぞれの中では辞書のキー順を保つ）。
func encodeINI(v value.Value) (string, error) {
	root, ok := v.Dict()
	if !ok || root == nil {
		return "", errors.New("INI変換できるのは辞書のみです。")
	}
	var scalars, sections strings.Builder
	for _, k := range root.Keys() {
		item, _ := root.Get(k)
		if d, isDict := item.Dict(); isDict && d != nil {
			if err := checkSectionName(k); err != nil {
				return "", err
			}
			sections.WriteString("[" + k + "]\n")
			for _, sk := range d.Keys() {
				sv, _ := d.Get(sk)
				line, err := encodeEntry(k+"."+sk, sk, sv)
				if err != nil {
					return "", err
				}
				sections.WriteString(line)
			}
			sections.WriteString("\n")
			continue
		}
		line, err := encodeEntry(k, k, item)
		if err != nil {
			return "", err
		}
		scalars.WriteString(line)
	}
	out := scalars.String()
	if out != "" && sections.Len() > 0 {
		out += "\n"
	}
	out += sections.String()
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return "", nil
	}
	return out + "\n", nil
}

func checkSectionName(name string) error {
	if strings.TrimSpace(name) == "" || name != strings.TrimSpace(name) ||
		strings.ContainsAny(name, "\r\n]") {
		return fmt.Errorf("INI変換に失敗しました。セクション名「%s」はINIで表せません。", name)
	}
	return nil
}

// encodeEntry は key=value の1行を作る。pathはエラー文面用の位置表記。
func encodeEntry(path, key string, v value.Value) (string, error) {
	if key == "" || key != strings.TrimSpace(key) || strings.ContainsAny(key, "=\r\n") ||
		key[0] == '[' || key[0] == ';' || key[0] == '#' {
		return "", fmt.Errorf("INI変換に失敗しました。キー「%s」はINIで表せません。", key)
	}
	var s string
	switch v.Kind() {
	case value.KindString, value.KindNumber:
		s = value.ToString(v)
	case value.KindBool:
		if value.ToBool(v) {
			s = "true"
		} else {
			s = "false"
		}
	case value.KindDict:
		// セクションの中の辞書（最上位の辞書は呼び出し側でセクションにする）
		return "", fmt.Errorf("INI変換に失敗しました。「%s」は2段より深い入れ子です。", path)
	case value.KindArray:
		return "", fmt.Errorf("INI変換に失敗しました。「%s」は配列でINIでは表せません。", path)
	case value.KindNull, value.KindUndefined:
		return "", fmt.Errorf("INI変換に失敗しました。「%s」の値が空(null/undefined)です。", path)
	default:
		return "", fmt.Errorf("INI変換に失敗しました。「%s」の値の種類はINIで表せません。", path)
	}
	if strings.ContainsAny(s, "\r\n") {
		return "", fmt.Errorf("INI変換に失敗しました。「%s」の値に改行が含まれています。", path)
	}
	// 前後に空白がある値や、全体が " で囲まれた値は、取得時に壊れないよう引用符で囲む
	if s != strings.TrimSpace(s) || (len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"') {
		s = `"` + s + `"`
	}
	return key + "=" + s + "\n", nil
}
