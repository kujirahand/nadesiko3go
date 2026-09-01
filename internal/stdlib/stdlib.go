// Package stdlib holds the plugin_system commands. This is the only part of
// the runtime whose behaviour is guaranteed compatible with the TypeScript
// implementation (AGENTS.md §3).
package stdlib

import (
	"sort"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Context is what a command may reach while it runs: the host for output and
// the system variables the loops set.
type Context interface {
	Print(s string)
	// Write writes without ending the line, for a prompt.
	Write(s string)
	SysVar(name string) value.Value
	SetSysVar(name string, v value.Value)
	// CallFunc runs a function value, which the commands that take a function
	// argument need (『配列マップ』 and 『配列フィルタ』).
	CallFunc(fn *value.Func, args []value.Value) (value.Value, error)
	// FindFunc resolves a user function by the name form accepted by commands
	// such as 『実行』 and 『配列フィルタ』.
	FindFunc(name string) *value.Func
	CommandState(name string) value.Value
	SetCommandState(name string, v value.Value)

	// SetTimer schedules fn and reports the timer id. The commands keep the
	// id as a number, because that is what 『対象』 holds.
	SetTimer(fn *value.Func, seconds float64, repeat bool) (float64, error)
	// PostFunc queues a one-shot callback at the current virtual time. Database
	// callback commands use it to match Node.js setImmediate ordering.
	PostFunc(fn *value.Func, args []value.Value) error
	// CancelTimer stops one timer and reports whether there was one.
	CancelTimer(id float64) bool
	// CancelAllTimers stops every timer.
	CancelAllTimers()
	// Wait advances time by the given number of seconds, running the
	// callbacks that come due in between.
	Wait(seconds float64) error

	// ReadLine reads one line from the host's input, for 『尋ねる』.
	ReadLine() (string, error)
	// Exit ends the program with the given status.
	Exit(code int)
	// Args reports the arguments the program was started with.
	Args() []string
	// ReadResource reads a file packed into the executable. It reports false
	// when the program is not bundled, or has no such file, in which case the
	// caller falls back to the real file system.
	ReadResource(name string) ([]byte, bool)
	// Now is owned by the VM event loop so tests and timer callbacks observe
	// one deterministic clock.
	Now() time.Time
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
	list    lexer.FuncList
}

// Plugin adds commands beyond plugin_system. Only plugin_system is covered by
// the compatibility guarantee (AGENTS.md §3); a plugin such as nodelib is free
// to be designed the Go way.
type Plugin interface {
	// FuncList gives the signatures the lexer and parser need.
	FuncList() lexer.FuncList
	// Impls gives the implementations, keyed by command name.
	Impls() map[string]Impl
}

// NewRegistry builds the command table: signatures from the shared list, and
// implementations for the commands that have one. Plugins are merged in, and a
// plugin may not replace a plugin_system command.
func NewRegistry(plugins ...Plugin) *Registry {
	list := ParserFuncList()
	impls := implementations()
	for _, plugin := range plugins {
		for name, item := range plugin.FuncList() {
			if _, taken := list[name]; taken {
				continue // plugin_system の命令は上書きさせない
			}
			list[name] = item
		}
		for name, fn := range plugin.Impls() {
			if _, taken := impls[name]; taken {
				continue
			}
			impls[name] = fn
		}
	}
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

	r.list = list
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
	case nil, undefinedConst:
		return value.Undefined()
	case nullConst:
		return value.Null()
	case emptyArrayConst:
		return value.ArrayValue(value.NewArray())
	case eraDataConst:
		items := make([]value.Value, 0, len(eras))
		for _, era := range eras {
			d := value.NewDict()
			d.Set("元号", value.String(era.name))
			d.Set("改元日", value.String(era.start))
			items = append(items, value.DictValue(d))
		}
		return value.ArrayValue(value.NewArray(items...))
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

// FuncList returns the metadata the lexer and parser need, including whatever
// the plugins contributed.
func (r *Registry) FuncList() lexer.FuncList { return r.list }

// Len reports how many commands the registry holds.
func (r *Registry) Len() int { return len(r.entries) }
