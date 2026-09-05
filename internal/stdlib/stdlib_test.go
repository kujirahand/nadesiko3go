package stdlib_test

import (
	"math"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// fakeContext collects output and system variables for the tests.
type fakeContext struct {
	out    []string
	sysVar map[string]value.Value
	state  map[string]value.Value
}

func newContext() *fakeContext {
	return &fakeContext{sysVar: map[string]value.Value{}, state: map[string]value.Value{}}
}

func (c *fakeContext) Print(s string)                    { c.out = append(c.out, s) }
func (c *fakeContext) Write(s string)                    { c.out = append(c.out, s) }
func (c *fakeContext) SysVar(name string) value.Value    { return c.sysVar[name] }
func (c *fakeContext) SetSysVar(n string, v value.Value) { c.sysVar[n] = v }

// The commands that call back into the VM or the timer queue are covered end
// to end in internal/vm; here they only need to satisfy the interface.
func (c *fakeContext) CallFunc(*value.Func, []value.Value) (value.Value, error) {
	return value.Undefined(), nil
}
func (c *fakeContext) FindFunc(string) *value.Func  { return nil }
func (c *fakeContext) FindValue(string) value.Value { return value.Undefined() }
func (c *fakeContext) GlobalFuncNames() []string    { return nil }
func (c *fakeContext) CallCommand(string, []value.Value) (value.Value, error) {
	return value.Undefined(), nil
}
func (c *fakeContext) CommandState(name string) value.Value       { return c.state[name] }
func (c *fakeContext) SetCommandState(name string, v value.Value) { c.state[name] = v }

func (c *fakeContext) SetTimer(*value.Func, float64, bool) (float64, error) { return 0, nil }
func (c *fakeContext) PostFunc(*value.Func, []value.Value) error            { return nil }
func (c *fakeContext) CancelTimer(float64) bool                             { return false }
func (c *fakeContext) CancelAllTimers()                                     {}
func (c *fakeContext) Wait(float64) error                                   { return nil }
func (c *fakeContext) ReadLine() (string, error)                            { return "", nil }
func (c *fakeContext) Exit(int)                                             {}
func (c *fakeContext) Args() []string                                       { return nil }
func (c *fakeContext) ReadResource(string) ([]byte, bool)                   { return nil, false }
func (c *fakeContext) Now() time.Time {
	return time.Date(2026, 9, 1, 12, 34, 56, 0, time.Local)
}
func (c *fakeContext) CurrentSourcePos() (string, int) { return "", 0 }

// TestEveryImplementationIsDeclared guards against an implementation whose name
// does not appear in the signature table, which would never be reachable.
func TestEveryImplementationIsDeclared(t *testing.T) {
	r := stdlib.NewRegistry()
	for id := 0; id < r.Len(); id++ {
		e := r.Entry(id)
		if e.Name == "" || e.Item == nil {
			t.Errorf("ID %d の項目が壊れている: %#v", id, e)
		}
		if got, ok := r.Lookup(e.Name); !ok || got.ID != id {
			t.Errorf("%s を名前で引くとIDが違う", e.Name)
		}
	}
}

// TestEveryArrayCommandIsImplemented keeps the Go registry in sync with
// plugin_system_array.mts. A command being present in ParserFuncList is not
// enough: a nil Fn is accepted by the parser but fails only when it is called.
func TestEveryArrayCommandIsImplemented(t *testing.T) {
	r := stdlib.NewRegistry()
	names := []string{
		"配列結合", "配列只結合", "配列検索", "配列要素数", "要素数", "LEN",
		"配列挿入", "配列一括挿入", "配列ソート", "配列数値変換", "配列数値ソート",
		"配列カスタムソート", "配列逆順", "配列シャッフル", "配列削除", "配列切取",
		"配列取出", "配列ポップ", "配列プッシュ", "配列追加", "配列複製", "配列範囲コピー",
		"参照", "配列参照", "配列足", "配列最大値", "配列最小値", "配列合計", "配列入替",
		"配列連番作成", "配列要素作成", "配列関数適用", "配列マップ", "配列フィルタ",
		"表ソート", "表数値ソート", "表ピックアップ", "表完全一致ピックアップ", "表検索",
		"表列数", "表行数", "表行列交換", "表右回転", "表重複削除", "表列取得",
		"表列挿入", "表列削除", "表列合計", "表曖昧検索", "表正規表現ピックアップ",
	}
	for _, name := range names {
		e, ok := r.Lookup(name)
		if !ok {
			t.Errorf("配列命令『%s』が登録されていない", name)
			continue
		}
		if e.Fn == nil {
			t.Errorf("配列命令『%s』に実装がない", name)
		}
	}
}

// TestEveryRegisteredCommandHasAnImplementation prevents signatures from
// getting ahead of the runtime. Commands intentionally unsupported by the Go
// backend still have an explicit implementation that returns that decision.
func TestEveryRegisteredCommandHasAnImplementation(t *testing.T) {
	r := stdlib.NewRegistry()
	for id := 0; id < r.Len(); id++ {
		e := r.Entry(id)
		if e.Fn == nil {
			t.Errorf("登録命令『%s』に実装がない", e.Name)
		}
	}
}

// TestRegistryIDsAreStable pins that the IDs the IR stores do not depend on map
// iteration order.
func TestRegistryIDsAreStable(t *testing.T) {
	a, b := stdlib.NewRegistry(), stdlib.NewRegistry()
	if a.Len() != b.Len() {
		t.Fatalf("命令数が違う: %d と %d", a.Len(), b.Len())
	}
	for id := 0; id < a.Len(); id++ {
		if a.Entry(id).Name != b.Entry(id).Name {
			t.Fatalf("ID %d の命令名が違う: %s と %s", id, a.Entry(id).Name, b.Entry(id).Name)
		}
	}
}

func TestCoreCommands(t *testing.T) {
	r := stdlib.NewRegistry()
	num, str := value.Number, value.String

	tests := []struct {
		name string
		args []value.Value
		want string
	}{
		{"足", []value.Value{num(1), num(2)}, "3"},
		{"足", []value.Value{num(1), str("2")}, "3"},
		{"足", []value.Value{num(1), str("あ")}, "NaN"},
		{"掛", []value.Value{num(3), num(4)}, "12"},
		{"AND", []value.Value{num(12), num(10)}, "8"},
		{"OR", []value.Value{num(12), num(10)}, "14"},
		{"XOR", []value.Value{num(12), num(10)}, "6"},
		{"整数変換", []value.Value{str("123")}, "123"},
		{"整数変換", []value.Value{str("12.7")}, "12"},
		{"整数変換", []value.Value{str("0xFF")}, "255"},
		{"整数変換", []value.Value{num(1e21)}, "1"},
		{"整数変換", []value.Value{num(math.Inf(1))}, "NaN"},
		{"整数変換", []value.Value{str("あ")}, "NaN"},
		{"実数変換", []value.Value{str("12.5")}, "12.5"},
		{"文字列変換", []value.Value{num(123)}, "123"},
		{"変数型確認", []value.Value{num(1)}, "number"},
		{"変数型確認", []value.Value{str("a")}, "string"},
		// 真偽判定は真偽値ではなく『真』『偽』を返す
		{"真偽判定", []value.Value{str("")}, "偽"},
		{"真偽判定", []value.Value{num(0)}, "偽"},
		{"真偽判定", []value.Value{num(1)}, "真"},
		{"数列判定", []value.Value{str("123")}, "true"},
		{"数列判定", []value.Value{str("あ")}, "false"},
		{"TOSTR", []value.Value{num(123)}, "123"},
		{"TOINT", []value.Value{str("123")}, "123"},
		{"INT", []value.Value{str("123")}, "123"},
		{"TOFLOAT", []value.Value{str("12.5")}, "12.5"},
		{"FLOAT", []value.Value{str("12.5")}, "12.5"},
		{"TYPEOF", []value.Value{num(1)}, "number"},
		{"ください", nil, "undefined"},
		{"お願", nil, "undefined"},
		{"です", nil, "undefined"},
	}
	for _, tt := range tests {
		e, ok := r.Lookup(tt.name)
		if !ok || e.Fn == nil {
			t.Fatalf("%s が実装されていない", tt.name)
		}
		got, err := e.Fn(newContext(), tt.args)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if value.ToString(got) != tt.want {
			t.Errorf("%s%v = %q, want %q", tt.name, tt.args, value.ToString(got), tt.want)
		}
	}
}

func TestPrintAndError(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()

	show, _ := r.Lookup("表示")
	if _, err := show.Fn(ctx, []value.Value{value.Number(3)}); err != nil {
		t.Fatal(err)
	}
	if len(ctx.out) != 1 || ctx.out[0] != "3" {
		t.Errorf("表示の出力 = %v, want [3]", ctx.out)
	}

	raise, _ := r.Lookup("エラー発生")
	_, err := raise.Fn(ctx, []value.Value{value.String("わざとエラー")})
	if err == nil || err.Error() != "わざとエラー" {
		t.Errorf("エラー発生 = %v, want わざとエラー", err)
	}
}

func TestRangeObject(t *testing.T) {
	r := stdlib.NewRegistry()
	e, _ := r.Lookup("範囲")
	got, err := e.Fn(newContext(), []value.Value{value.Number(1), value.Number(3)})
	if err != nil {
		t.Fatal(err)
	}
	d, ok := got.Dict()
	if !ok {
		t.Fatalf("範囲 = %v, want 辞書", got.Kind())
	}
	head, _ := d.Get("先頭")
	tail, _ := d.Get("末尾")
	if value.ToString(head) != "1" || value.ToString(tail) != "3" {
		t.Errorf("範囲 = {先頭:%s, 末尾:%s}, want {1, 3}", value.ToString(head), value.ToString(tail))
	}
}

func TestConstants(t *testing.T) {
	r := stdlib.NewRegistry()
	tests := []struct{ name, want string }{
		{"はい", "true"},
		{"いいえ", "false"},
		{"オン", "true"},
		{"オフ", "false"},
		{"改行", "\n"},
		{"タブ", "\t"},
		{"空", ""},
		// 空とNULLと未定義は別物。空は空文字列、NULLはnull、未定義はundefined。
		{"NULL", "null"},
		{"未定義", "undefined"},
		{"undefined", "undefined"},
		{"戻値無", "0"},
		{"戻値有", "1"},
	}
	for _, tt := range tests {
		v, ok := r.Const(tt.name)
		if !ok {
			t.Errorf("定数 %s がない", tt.name)
			continue
		}
		if got := value.ToString(v); got != tt.want {
			t.Errorf("定数 %s = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestNewlyAddedCommands(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()

	// 空辞書
	for _, name := range []string{"空辞書", "空ハッシュ", "空オブジェクト"} {
		e, ok := r.Lookup(name)
		if !ok {
			t.Fatalf("%s が登録されていない", name)
		}
		v, err := e.Fn(ctx, nil)
		if err != nil {
			t.Fatalf("%s error: %v", name, err)
		}
		if _, ok := v.Dict(); !ok {
			t.Errorf("%s = %v, want dict", name, v.Kind())
		}
	}

	// 拡張子抽出
	ext, _ := r.Lookup("拡張子抽出")
	v, err := ext.Fn(ctx, []value.Value{value.String("path/to/test.txt")})
	if err != nil || value.ToString(v) != ".txt" {
		t.Errorf("拡張子抽出 = %v (err: %v), want .txt", value.ToString(v), err)
	}

	// 表示ログクリア / 言 / コンソール表示
	ctx.SetSysVar("表示ログ", value.String("hello\n"))
	clearLog, _ := r.Lookup("表示ログクリア")
	clearLog.Fn(ctx, nil)
	if s := value.ToString(ctx.SysVar("表示ログ")); s != "" {
		t.Errorf("表示ログ = %q, want empty", s)
	}

	say, _ := r.Lookup("言")
	say.Fn(ctx, []value.Value{value.String("test")})
	if len(ctx.out) == 0 || ctx.out[len(ctx.out)-1] != "test" {
		t.Errorf("言 output = %v, want 'test'", ctx.out)
	}

	// 時間ミリ秒取得
	ms, _ := r.Lookup("時間ミリ秒取得")
	v, err = ms.Fn(ctx, nil)
	if err != nil || v.Kind() != value.KindNumber {
		t.Errorf("時間ミリ秒取得 = %v, err: %v", v, err)
	}
}
