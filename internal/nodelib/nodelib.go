// Package nodelib holds the commands that reach outside the program: files,
// the operating system, and the process itself (plugin_node's territory).
//
// Unlike internal/stdlib, nothing here is covered by the compatibility
// guarantee (AGENTS.md §3). The command names follow the TypeScript version so
// that existing scripts read the same, but the behaviour is free to be the Go
// one where that is clearer.
package nodelib

import (
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

// Plugin is the nodelib command set, ready to merge into a registry.
type Plugin struct{}

// New creates the plugin.
func New() *Plugin { return &Plugin{} }

// command is one nodelib command: its particles and its implementation.
type command struct {
	josi       [][]string
	returnNone bool
	// variadic marks a command whose last parameter takes any number of
	// arguments, such as 『パス結合』.
	variadic bool
	fn       stdlib.Impl
}

// FuncList gives the signatures the lexer and parser need.
func (p *Plugin) FuncList() lexer.FuncList {
	list := lexer.FuncList{}
	for name, c := range commands() {
		list[name] = &lexer.FuncItem{
			Name: name, Type: "func", Josi: c.josi,
			ReturnNone: c.returnNone, IsVariableJosi: c.variadic, Pure: true,
		}
	}
	for name, v := range constants() {
		list[name] = &lexer.FuncItem{Name: name, Type: "const", Value: v}
	}
	return list
}

// Impls gives the implementations.
func (p *Plugin) Impls() map[string]stdlib.Impl {
	out := map[string]stdlib.Impl{}
	for name, c := range commands() {
		out[name] = c.fn
	}
	return out
}
