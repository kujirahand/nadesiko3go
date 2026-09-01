// Package mathlib implements commands from the separate TypeScript plugin_math.
package mathlib

import (
	"math"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) FuncList() lexer.FuncList {
	return lexer.FuncList{
		"切上": {Name: "切上", Type: "func", Josi: [][]string{{"を", "の"}}, Pure: true},
		"切捨": {Name: "切捨", Type: "func", Josi: [][]string{{"を", "の"}}, Pure: true},
	}
}

func (p *Plugin) Impls() map[string]stdlib.Impl {
	return map[string]stdlib.Impl{
		"切上": func(_ stdlib.Context, args []value.Value) (value.Value, error) {
			return value.Number(math.Ceil(first(args))), nil
		},
		"切捨": func(_ stdlib.Context, args []value.Value) (value.Value, error) {
			return value.Number(math.Floor(first(args))), nil
		},
	}
}

func first(args []value.Value) float64 {
	if len(args) == 0 {
		return 0
	}
	return value.ToNumber(args[0])
}
