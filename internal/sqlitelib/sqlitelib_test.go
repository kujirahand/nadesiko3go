package sqlitelib_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/sqlitelib"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// stubContext は sqlitelib の命令が使う範囲だけを満たす stdlib.Context。
type stubContext struct{ vars map[string]value.Value }

func newStubContext() *stubContext { return &stubContext{vars: map[string]value.Value{}} }

func (c *stubContext) Print(string) {}
func (c *stubContext) Write(string) {}
func (c *stubContext) SysVar(name string) value.Value {
	if v, ok := c.vars[name]; ok {
		return v
	}
	return value.Undefined()
}
func (c *stubContext) SetSysVar(name string, v value.Value) { c.vars[name] = v }
func (c *stubContext) CallFunc(*value.Func, []value.Value) (value.Value, error) {
	return value.Undefined(), nil
}
func (c *stubContext) FindFunc(string) *value.Func  { return nil }
func (c *stubContext) FindValue(string) value.Value { return value.Undefined() }
func (c *stubContext) GlobalFuncNames() []string    { return nil }
func (c *stubContext) CallCommand(string, []value.Value) (value.Value, error) {
	return value.Undefined(), nil
}
func (c *stubContext) CommandState(string) value.Value     { return value.Undefined() }
func (c *stubContext) SetCommandState(string, value.Value) {}
func (c *stubContext) SetTimer(*value.Func, float64, bool) (float64, error) {
	return 0, nil
}
func (c *stubContext) PostFunc(*value.Func, []value.Value) error { return nil }
func (c *stubContext) CancelTimer(float64) bool                  { return false }
func (c *stubContext) CancelAllTimers()                          {}
func (c *stubContext) Wait(float64) error                        { return nil }
func (c *stubContext) ReadLine() (string, error)                 { return "", nil }
func (c *stubContext) Exit(int)                                  {}
func (c *stubContext) Args() []string                            { return nil }
func (c *stubContext) ReadResource(string) ([]byte, bool)        { return nil, false }
func (c *stubContext) Now() time.Time                            { return time.Unix(0, 0) }
func (c *stubContext) CurrentSourcePos() (string, int)           { return "", 0 }

var _ stdlib.Context = (*stubContext)(nil)

func openMemory(t *testing.T, p *sqlitelib.Plugin, ctx stdlib.Context) {
	t.Helper()
	if _, err := p.Impls()["SQLITE3開"](ctx, []value.Value{value.String(":memory:")}); err != nil {
		t.Fatalf("SQLITE3開: %v", err)
	}
}

func TestPluginCloseIsIdempotentAndReopenable(t *testing.T) {
	p := sqlitelib.New()
	ctx := newStubContext()
	openMemory(t, p, ctx)
	if _, err := p.Impls()["SQLITE3実行"](ctx, []value.Value{
		value.String("CREATE TABLE t(v TEXT)"), value.ArrayValue(value.NewArray()),
	}); err != nil {
		t.Fatalf("SQLITE3実行: %v", err)
	}
	// 明示Closeと環境破棄によるCloseを重ねても安全でなければならない。
	if err := p.Close(); err != nil {
		t.Fatalf("Close (1回目): %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close (2回目): %v", err)
	}
	// Close後もプラグインは再利用できる（共有インスタンス経由の逐次実行のため）。
	openMemory(t, p, ctx)
	if _, err := p.Impls()["SQLITE3実行"](ctx, []value.Value{
		value.String("CREATE TABLE u(v TEXT)"), value.ArrayValue(value.NewArray()),
	}); err != nil {
		t.Fatalf("再オープン後のSQLITE3実行: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("再オープン後のClose: %v", err)
	}
}

// 開いたまま参照を捨てたプラグインは、GCで接続が回収され、
// database/sql の常駐goroutineが残らないことを確かめる（Issue #363の再現手順）。
func TestDiscardedPluginReleasesGoroutines(t *testing.T) {
	openAndDiscard := func() {
		p := sqlitelib.New()
		ctx := newStubContext()
		if _, err := p.Impls()["SQLITE3開"](ctx, []value.Value{value.String(":memory:")}); err != nil {
			t.Fatalf("SQLITE3開: %v", err)
		}
		// pへの参照はここで消える。明示Closeはしない。
	}
	// ベースラインを安定させるため、一度捨てた分が回収されるまで待ってから数える。
	openAndDiscard()
	base := settleGoroutineCount(t)
	for i := 0; i < 30; i++ {
		openAndDiscard()
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
		if n := runtime.NumGoroutine(); n <= base+5 {
			return
		} else if time.Now().After(deadline) {
			t.Fatalf("goroutineが回収されない: base=%d now=%d", base, n)
		}
	}
}

// settleGoroutineCount はGCを繰り返し、goroutine数が安定した時点の値を返す。
func settleGoroutineCount(t *testing.T) int {
	t.Helper()
	prev := -1
	for i := 0; i < 40; i++ {
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
		n := runtime.NumGoroutine()
		if n == prev {
			return n
		}
		prev = n
	}
	return prev
}
