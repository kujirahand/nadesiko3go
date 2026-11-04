package inilib

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// dictToMap はテスト比較用に、2段までの辞書を map に展開する。
// 文字列は string、辞書は map[string]any になる。
func dictToMap(t *testing.T, v value.Value) map[string]any {
	t.Helper()
	d, ok := v.Dict()
	if !ok || d == nil {
		t.Fatalf("辞書ではありません: %v", v.Kind())
	}
	m := map[string]any{}
	for _, k := range d.Keys() {
		item, _ := d.Get(k)
		switch item.Kind() {
		case value.KindString:
			s, _ := item.String()
			m[k] = s
		case value.KindDict:
			m[k] = dictToMap(t, item)
		default:
			t.Fatalf("キー %q の種類が想定外: %v", k, item.Kind())
		}
	}
	return m
}

func mustParse(t *testing.T, s string) map[string]any {
	t.Helper()
	v, err := parseINI(s)
	if err != nil {
		t.Fatalf("parseINI(%q): %v", s, err)
	}
	return dictToMap(t, v)
}

func sec(m map[string]any, name string) map[string]any {
	d, _ := m[name].(map[string]any)
	return d
}

func TestParseSections(t *testing.T) {
	m := mustParse(t, "top=1\n[a]\nx=2\ny=3\n[b]\nz=4\n")
	if m["top"] != "1" {
		t.Errorf("最上位キー: %v", m)
	}
	if sec(m, "a")["x"] != "2" || sec(m, "a")["y"] != "3" || sec(m, "b")["z"] != "4" {
		t.Errorf("セクション: %v", m)
	}
}

func TestParseComments(t *testing.T) {
	m := mustParse(t, "; c1\n# c2\n  ; c3\n\t# c4\nk=v ; not comment\nh=a#b\n")
	if m["k"] != "v ; not comment" || m["h"] != "a#b" {
		t.Errorf("インラインコメントは値の一部: %v", m)
	}
	if len(m) != 2 {
		t.Errorf("コメント行がキーになっている: %v", m)
	}
}

func TestParseQuotes(t *testing.T) {
	m := mustParse(t, "a=\"  x ; y # z = w \"\nb=  plain  \nc=\"\"\nd=\"x\n")
	if m["a"] != "  x ; y # z = w " {
		t.Errorf("引用符の中身はそのまま: %q", m["a"])
	}
	if m["b"] != "plain" {
		t.Errorf("前後空白の除去: %q", m["b"])
	}
	if m["c"] != "" {
		t.Errorf("空の引用符: %q", m["c"])
	}
	if m["d"] != "\"x" {
		t.Errorf("片側だけの引用符は値の一部: %q", m["d"])
	}
}

func TestParseDuplicatesAndMerge(t *testing.T) {
	m := mustParse(t, "k=1\nk=2\n[s]\na=1\n[t]\nb=1\n[s]\na=9\nc=3\n")
	if m["k"] != "2" {
		t.Errorf("後の値が勝つ: %v", m)
	}
	s := sec(m, "s")
	if s["a"] != "9" || s["c"] != "3" || len(s) != 2 {
		t.Errorf("セクションのマージ: %v", m)
	}
	if sec(m, "t")["b"] != "1" {
		t.Errorf("他セクションが壊れた: %v", m)
	}
}

func TestParseEmptyValue(t *testing.T) {
	m := mustParse(t, "k=\n[s]\nj =  \n")
	if v, ok := m["k"]; !ok || v != "" {
		t.Errorf("空値: %v", m)
	}
	if v, ok := sec(m, "s")["j"]; !ok || v != "" {
		t.Errorf("空値(セクション内): %v", m)
	}
}

func TestParseValueContainsEqual(t *testing.T) {
	m := mustParse(t, "url=a=b=c\n")
	if m["url"] != "a=b=c" {
		t.Errorf("最初の = で分割: %q", m["url"])
	}
}

func TestParseBOMAndCRLF(t *testing.T) {
	m := mustParse(t, "\ufeff[s]\r\nk=v\r\n; c\r\nj=w\r\n")
	s := sec(m, "s")
	if s["k"] != "v" || s["j"] != "w" {
		t.Errorf("BOM/CRLF: %v", m)
	}
	// BOM が先頭のキーに混ざらない
	m = mustParse(t, "\ufeffk=v\r\n")
	if m["k"] != "v" {
		t.Errorf("BOM付き先頭キー: %v", m)
	}
}

func TestParseCaseAndJapanese(t *testing.T) {
	m := mustParse(t, "[Sec]\nKey=A\nkey=b\n[sec]\nx=1\n[設定]\n名前=けもの\n")
	if sec(m, "Sec")["Key"] != "A" || sec(m, "Sec")["key"] != "b" || sec(m, "sec")["x"] != "1" {
		t.Errorf("大文字小文字の保持: %v", m)
	}
	if sec(m, "設定")["名前"] != "けもの" {
		t.Errorf("日本語: %v", m)
	}
}

func TestParseEmpty(t *testing.T) {
	m := mustParse(t, "")
	if len(m) != 0 {
		t.Errorf("空入力: %v", m)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"閉じていない括弧", "[sec\nk=v\n", "括弧が閉じていません"},
		{"空のセクション名", "[]\nk=v\n", "セクション名が空"},
		{"空白だけのセクション名", "[  ]\n", "セクション名が空"},
		{"=の無い行", "k=v\nbroken\n", "「=」がありません"},
		{"キーが空", "=v\n", "キーが空"},
		{"キーとセクションの重複", "s=1\n[s]\n", "重複"},
		{"セクションとキーの重複", "[s]\nx=1\n[t]\n[s]\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseINI(tt.in)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("エラーにならないはず: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("エラーになるはず")
			}
			if !strings.HasPrefix(err.Error(), "INI取得に失敗しました。") || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("エラー文面: %v", err)
			}
		})
	}
}

func buildDict(pairs ...any) value.Value {
	d := value.NewDict()
	for i := 0; i+1 < len(pairs); i += 2 {
		d.Set(pairs[i].(string), pairs[i+1].(value.Value))
	}
	return value.DictValue(d)
}

func TestEncodeBasic(t *testing.T) {
	v := buildDict(
		"sec", buildDict("a", value.String("x"), "n", value.Number(3), "f", value.Number(1.5), "b", value.Bool(true), "c", value.Bool(false)),
		"top", value.String("t"),
		"s2", buildDict("k", value.String("日本語")),
	)
	got, err := encodeINI(v)
	if err != nil {
		t.Fatal(err)
	}
	// 最上位スカラーが先、セクションは辞書の順
	want := "top=t\n\n[sec]\na=x\nn=3\nf=1.5\nb=true\nc=false\n\n[s2]\nk=日本語\n"
	if got != want {
		t.Errorf("got=%q\nwant=%q", got, want)
	}
}

func TestEncodeOnlySections(t *testing.T) {
	got, err := encodeINI(buildDict("s", buildDict("k", value.String("v"))))
	if err != nil {
		t.Fatal(err)
	}
	if got != "[s]\nk=v\n" {
		t.Errorf("got=%q", got)
	}
}

func TestEncodeEmpty(t *testing.T) {
	got, err := encodeINI(buildDict())
	if err != nil || got != "" {
		t.Errorf("got=%q err=%v", got, err)
	}
}

func TestEncodeErrors(t *testing.T) {
	arr := value.ArrayValue(value.NewArray(value.String("a")))
	tests := []struct {
		name string
		v    value.Value
		want string
	}{
		{"辞書以外", value.String("x"), "INI変換できるのは辞書のみです。"},
		{"配列(最上位)", arr, "INI変換できるのは辞書のみです。"},
		{"深い入れ子", buildDict("s", buildDict("t", buildDict("k", value.String("v")))), "2段より深い"},
		{"配列の値(セクション内)", buildDict("s", buildDict("k", arr)), "配列"},
		{"配列の値(最上位)", buildDict("k", arr), "配列"},
		{"nullの値", buildDict("s", buildDict("k", value.Null())), "null"},
		{"nullの値(最上位)", buildDict("k", value.Null()), "null"},
		{"undefinedの値", buildDict("k", value.Undefined()), "undefined"},
		{"改行を含む値", buildDict("k", value.String("a\nb")), "改行"},
		{"=を含むキー", buildDict("a=b", value.String("v")), "キー"},
		{"空のキー", buildDict("", value.String("v")), "キー"},
		{"]を含むセクション名", buildDict("a]b", buildDict("k", value.String("v"))), "セクション名"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := encodeINI(tt.v)
			if err == nil {
				t.Fatal("エラーになるはず")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("エラー文面: %v", err)
			}
			if tt.want != "INI変換できるのは辞書のみです。" && !strings.HasPrefix(err.Error(), "INI変換に失敗しました。") {
				t.Errorf("エラーの接頭辞: %v", err)
			}
		})
	}
}

// 辞書 → INI → 辞書 の往復で値が変わらない（値はすべて文字列になる）
func TestRoundTripDictToINI(t *testing.T) {
	v := buildDict(
		"top", value.String("t"),
		"空白", value.String("  padded  "),
		"quoted", value.String(`"q"`),
		"empty", value.String(""),
		"s", buildDict("a", value.String("x ; y # z = w"), "名前", value.String("けもの"), "e", value.String("")),
	)
	text, err := encodeINI(v)
	if err != nil {
		t.Fatal(err)
	}
	got := mustParse(t, text)
	want := dictToMap(t, v)
	if len(got) != len(want) {
		t.Fatalf("got=%v want=%v (text=%q)", got, want, text)
	}
	for k, w := range want {
		if ws, ok := w.(string); ok {
			if got[k] != ws {
				t.Errorf("key %q: got=%q want=%q", k, got[k], ws)
			}
			continue
		}
		wm := w.(map[string]any)
		gm := sec(got, k)
		for kk, wv := range wm {
			if gm[kk] != wv {
				t.Errorf("section %q key %q: got=%q want=%q", k, kk, gm[kk], wv)
			}
		}
	}
}

// INI → 辞書 → INI → 辞書 の往復で内容が変わらない
func TestRoundTripINIToDict(t *testing.T) {
	src := "top=1\n[a]\nx=hello world\ny=\"  sp  \"\n[b]\nz=\n"
	v, err := parseINI(src)
	if err != nil {
		t.Fatal(err)
	}
	text, err := encodeINI(v)
	if err != nil {
		t.Fatal(err)
	}
	m1 := mustParse(t, src)
	m2 := mustParse(t, text)
	if m1["top"] != m2["top"] {
		t.Errorf("top: %v vs %v", m1, m2)
	}
	for _, s := range []string{"a", "b"} {
		for k, w := range sec(m1, s) {
			if sec(m2, s)[k] != w {
				t.Errorf("[%s] %s: got=%q want=%q", s, k, sec(m2, s)[k], w)
			}
		}
	}
}
