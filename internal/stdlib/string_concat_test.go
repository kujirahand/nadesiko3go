package stdlib_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// 上流Issue #221 の回帰テスト。本家 TypeScript 版の『連結』は
// 引数を配列にして a.join('') する実装なので、JavaScript の join 規則に
// 従い null と undefined は空文字として連結される。Go版もそれに揃える。
func TestConcatNullUndefined(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()

	call := func(cmd string, args ...value.Value) string {
		e, ok := r.Lookup(cmd)
		if !ok || e.Fn == nil {
			t.Fatalf("%s が登録されていない", cmd)
		}
		got, err := e.Fn(ctx, args)
		if err != nil {
			t.Fatalf("%s%v: %v", cmd, args, err)
		}
		return value.ToString(got)
	}

	tests := []struct {
		name string
		args []value.Value
		want string
	}{
		{"NULLを含む", []value.Value{value.String("a"), value.Number(1), value.Null()}, "a1"},
		{"undefinedを含む", []value.Value{value.String("a"), value.Undefined(), value.String("b")}, "ab"},
		{"NULLとundefined混在", []value.Value{value.Null(), value.String("x"), value.Undefined(), value.String("y")}, "xy"},
		{"NULLだけ", []value.Value{value.Null()}, ""},
		{"undefinedだけ", []value.Value{value.Undefined()}, ""},
		// 配列の中の null/undefined は arrayToString が空文字にするので
		// 従来どおりカンマ区切りのまま残る（["a",[1,null]] → "a1,"）
		{"配列中のNULL", []value.Value{value.String("a"), value.ArrayValue(value.NewArray(value.Number(1), value.Null()))}, "a1,"},
		// 通常の引数は従来どおり
		{"通常", []value.Value{value.String("A"), value.String("B"), value.String("C")}, "ABC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, cmd := range []string{"連結", "文字列連結"} {
				if got := call(cmd, tt.args...); got != tt.want {
					t.Errorf("%s = %q, want %q", cmd, got, tt.want)
				}
			}
		})
	}
}

// 同じ join 規則を持つ『配列結合』『配列只結合』も本家の a.join(s) に揃える。
func TestArrayJoinNullUndefined(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()

	call := func(cmd string, args ...value.Value) string {
		e, ok := r.Lookup(cmd)
		if !ok || e.Fn == nil {
			t.Fatalf("%s が登録されていない", cmd)
		}
		got, err := e.Fn(ctx, args)
		if err != nil {
			t.Fatalf("%s%v: %v", cmd, args, err)
		}
		return value.ToString(got)
	}

	arr := func(items ...value.Value) value.Value {
		return value.ArrayValue(value.NewArray(items...))
	}
	tests := []struct {
		name string
		cmd  string
		args []value.Value
		want string
	}{
		{"配列結合/NULLを含む", "配列結合", []value.Value{arr(value.Number(1), value.Null()), value.String("-")}, "1-"},
		{"配列結合/undefinedを含む", "配列結合", []value.Value{arr(value.String("a"), value.Undefined(), value.String("b")), value.String(",")}, "a,,b"},
		{"配列結合/通常", "配列結合", []value.Value{arr(value.String("a"), value.String("b")), value.String("-")}, "a-b"},
		{"配列只結合/NULLを含む", "配列只結合", []value.Value{arr(value.Number(1), value.Null())}, "1"},
		// 配列でなければ文字列を改行で区切って繋ぎ直す（従来どおり）
		{"配列結合/非配列", "配列結合", []value.Value{value.String("a\nb"), value.String("-")}, "a-b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := call(tt.cmd, tt.args...); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

// 上流Issue #222 の回帰テスト。本家 TypeScript 版の『単置換』は
// String(s).replace(a, b) なので、置換文字列の $ パターンが展開される。
// 文字列検索の置換にはキャプチャがないため、$$・$&・$`・$' だけが
// 展開され、$1・$<name>・末尾の孤立した $ はそのまま残る。
func TestReplaceOnceDollarPattern(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()

	e, ok := r.Lookup("単置換")
	if !ok || e.Fn == nil {
		t.Fatal("単置換 が登録されていない")
	}
	call := func(s, from, to string) string {
		got, err := e.Fn(ctx, []value.Value{value.String(s), value.String(from), value.String(to)})
		if err != nil {
			t.Fatalf("単置換(%q, %q, %q): %v", s, from, to, err)
		}
		return value.ToString(got)
	}

	tests := []struct {
		name       string
		s, from, to string
		want       string
	}{
		{"$&は一致文字列", "あXあ", "X", "$&$&", "あXXあ"},
		{"$$は$1文字", "あXあ", "X", "$$$&", "あ$Xあ"},
		{"$$の後の&はそのまま", "あXあ", "X", "$$&$&", "あ$&Xあ"},
		{"$`は一致位置より前", "あXあ", "X", "$`", "あああ"},
		{"$'は一致位置より後", "あXあ", "X", "$'", "あああ"},
		{"キャプチャなしの$nはそのまま", "あXあ", "X", "[$1]", "あ[$1]あ"},
		{"キャプチャなしの$<name>はそのまま", "あXあ", "X", "[$<g>]", "あ[$<g>]あ"},
		{"末尾の孤立した$はそのまま", "あXあ", "X", "Y$", "あY$あ"},
		{"普通の文字はそのまま", "あXあ", "X", "Y", "あYあ"},
		{"最初の1回だけ置換", "X-X", "X", "$&$&", "XX-X"},
		{"マッチしない", "あいう", "X", "$&", "あいう"},
		{"空の検索文字列は先頭に挿入", "abc", "", "x", "xabc"},
		{"空マッチでの$'は全体", "abc", "", "$'", "abcabc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := call(tt.s, tt.from, tt.to); got != tt.want {
				t.Errorf("単置換(%q, %q, %q) = %q, want %q", tt.s, tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// 『連続表示』『連続無改行表示』も本家は a.join('') してから表示するので、
// 出力内容に null/undefined の名前が残らないことを確かめる。
func TestContinuousPrintNullUndefined(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()

	e, ok := r.Lookup("連続表示")
	if !ok || e.Fn == nil {
		t.Fatal("連続表示 が登録されていない")
	}
	if _, err := e.Fn(ctx, []value.Value{value.String("a"), value.Null(), value.String("b")}); err != nil {
		t.Fatal(err)
	}
	e2, ok := r.Lookup("連続無改行表示")
	if !ok || e2.Fn == nil {
		t.Fatal("連続無改行表示 が登録されていない")
	}
	if _, err := e2.Fn(ctx, []value.Value{value.Undefined(), value.String("c")}); err != nil {
		t.Fatal(err)
	}
	if got, want := ctx.out, []string{"ab", "c"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("出力 = %v, want %v", got, want)
	}
}
