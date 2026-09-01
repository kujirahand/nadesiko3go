// Package stdlib holds the plugin_system commands. This is the only part of
// the runtime whose behaviour is guaranteed compatible with the TypeScript
// implementation (AGENTS.md §3).
package stdlib

import (
	"sort"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Context is what a command may reach while it runs: the host for output and
// the system variables the loops set.
type Context interface {
	Print(s string)
	SysVar(name string) value.Value
	SetSysVar(name string, v value.Value)
}

// Impl is a command implementation. Returning an error raises a nadesiko
// runtime error at the call site.
type Impl func(ctx Context, args []value.Value) (value.Value, error)

// Entry is one command: its signature, and its implementation when there is one.
type Entry struct {
	ID   int
	Name string
	Item *lexer.FuncItem
	Fn   Impl
}

// Registry assigns every command a stable ID so the IR can call it by index
// rather than by name (AGENTS.md §6).
type Registry struct {
	entries []*Entry
	byName  map[string]*Entry
	consts  map[string]value.Value
}

// NewRegistry builds the command table: signatures from the shared list, and
// implementations for the commands that have one.
func NewRegistry() *Registry {
	list := ParserFuncList()
	r := &Registry{
		byName: make(map[string]*Entry, len(list)),
		consts: map[string]value.Value{},
	}

	names := make([]string, 0, len(list))
	for name := range list {
		names = append(names, name)
	}
	// IDが実行ごとに変わらないよう名前順に並べる
	sort.Strings(names)

	impls := implementations()
	for _, name := range names {
		item := list[name]
		if item.Type != "func" {
			r.consts[name] = constValue(item.Value)
			continue
		}
		e := &Entry{ID: len(r.entries), Name: name, Item: item, Fn: impls[name]}
		r.entries = append(r.entries, e)
		r.byName[name] = e
	}
	return r
}

// constValue converts a constant from the signature table into a runtime value.
func constValue(v any) value.Value {
	switch x := v.(type) {
	case nil:
		return value.Null()
	case bool:
		return value.Bool(x)
	case string:
		return value.String(x)
	case float64:
		return value.Number(x)
	case int:
		return value.Number(float64(x))
	}
	return value.Undefined()
}

// Lookup finds a command by name.
func (r *Registry) Lookup(name string) (*Entry, bool) {
	e, ok := r.byName[name]
	return e, ok
}

// Entry returns the command with this ID.
func (r *Registry) Entry(id int) *Entry {
	if id < 0 || id >= len(r.entries) {
		return nil
	}
	return r.entries[id]
}

// Const looks a system constant up by name.
func (r *Registry) Const(name string) (value.Value, bool) {
	v, ok := r.consts[name]
	return v, ok
}

// FuncList returns the metadata the lexer and parser need.
func (r *Registry) FuncList() lexer.FuncList { return ParserFuncList() }

// Len reports how many commands the registry holds.
func (r *Registry) Len() int { return len(r.entries) }
