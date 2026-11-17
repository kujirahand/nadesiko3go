package nodelib

import (
	"errors"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// ajaxFailureContext はシステム変数の記憶領域を持つ実行環境を模す。
// stdlib.Context は nil で埋め込むため、オーバーライドしたメソッド以外を呼ぶと panic する。
type ajaxFailureContext struct {
	stdlib.Context
	vars   map[string]value.Value
	state  map[string]value.Value
	called []*value.Func
}

func (c *ajaxFailureContext) SysVar(name string) value.Value             { return c.vars[name] }
func (c *ajaxFailureContext) SetSysVar(name string, v value.Value)       { c.vars[name] = v }
func (c *ajaxFailureContext) CommandState(name string) value.Value       { return c.state[name] }
func (c *ajaxFailureContext) SetCommandState(name string, v value.Value) { c.state[name] = v }
func (c *ajaxFailureContext) CallFunc(fn *value.Func, args []value.Value) (value.Value, error) {
	c.called = append(c.called, fn)
	return value.Undefined(), nil
}

func TestAjaxFailureDirectSysVar(t *testing.T) {
	for _, clearSysVar := range []bool{false, true} {
		name := "直接代入"
		if clearSysVar {
			name = "未定義でクリア"
		}
		t.Run(name, func(t *testing.T) {
			ctx := &ajaxFailureContext{vars: map[string]value.Value{}, state: map[string]value.Value{}}
			commands := map[string]command{}
			netCommands(commands)
			original, replacement := &value.Func{ID: 1}, &value.Func{ID: 2}
			if _, err := commands["AJAX失敗時"].fn(ctx, []value.Value{value.FuncValue(original)}); err != nil {
				t.Fatal(err)
			}
			current := value.FuncValue(replacement)
			if clearSysVar {
				current = value.Undefined()
			}
			ctx.SetSysVar("AJAX:ONERROR", current)
			originalErr := errors.New("通信失敗")
			_, err := ajaxFailure(ctx, originalErr)
			if clearSysVar {
				if !errors.Is(err, originalErr) || len(ctx.called) != 0 {
					t.Fatalf("クリア後: error=%v, 呼び出し=%v", err, ctx.called)
				}
			} else if err != nil || len(ctx.called) != 1 || ctx.called[0] != replacement {
				t.Fatalf("直接代入後: error=%v, 呼び出し=%v", err, ctx.called)
			}
		})
	}
}
