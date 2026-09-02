// Package runtime is the public facade the gogen backend generates Go source
// against (AGENTS.md §12).
//
// It exists only because of Go's visibility rule: everything a compiled
// program needs — the value model, the stdlib commands, the VM's operator and
// call-dispatch logic — lives under internal/, which a program outside this
// module cannot import. This package re-exports exactly that surface, mostly
// as type aliases, so a generated program built as its own module can import
//
//	github.com/kujirahand/nadesiko3go/pkg/runtime
//
// and nothing else. It does not reimplement anything: every alias points at
// the same type, and every function is a thin pass-through, so the VM and a
// gogen-built program share one implementation and cannot drift apart.
package runtime

import (
	"encoding/json"
	"io"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/csvlib"
	"github.com/kujirahand/nadesiko3go/internal/imagelib"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/mathlib"
	"github.com/kujirahand/nadesiko3go/internal/nodelib"
	"github.com/kujirahand/nadesiko3go/internal/officelib"
	"github.com/kujirahand/nadesiko3go/internal/pdflib"
	"github.com/kujirahand/nadesiko3go/internal/sqlitelib"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// --- 値モデル (internal/value) ---

// Value is a なでしこ value: undefined, null, bool, number, string, array,
// dict, or func (AGENTS.md §4).
type Value = value.Value

// Cell is one storage slot — a local, a capture, or a global — that a closure
// may share with the frame that created it.
type Cell = value.Cell

// Func is an opaque reference to a なでしこ function value.
type Func = value.Func

var (
	Undefined  = value.Undefined
	Null       = value.Null
	Bool       = value.Bool
	Number     = value.Number
	String     = value.String
	ArrayValue = value.ArrayValue
	DictValue  = value.DictValue
	FuncValue  = value.FuncValue
	NewArray   = value.NewArray
	NewDict    = value.NewDict
	ToBool     = value.ToBool
	ToString   = value.ToString
	ToNumber   = value.ToNumber
)

// --- IR (internal/ir) ---

// Program is the compiled program gogen translated. Generated code embeds one
// (usually as JSON — see MustDecodeProgram) so that the machine has the
// metadata a bytecode interpreter would otherwise carry: constants, function
// signatures and captures, global names, and source positions for error
// messages. A generated function's own body never runs through it — that part
// is native Go — but everything around a call still does.
type Program = ir.Program

// BinaryOp identifies a なでしこ binary operator.
type BinaryOp = ir.BinaryOp

// UnaryOp identifies a なでしこ unary operator.
type UnaryOp = ir.UnaryOp

// Special identifies one of the system values (『それ』『対象』 …).
type Special = ir.Special

// SpecialSore is 『それ』, which a generated function keeps as its own local
// variable rather than reading through the machine (AGENTS.md §4: it belongs
// to the running call, not to the program).
const SpecialSore = ir.SpecialSore

// DecodeProgram parses a Program from the JSON gogen embedded (AGENTS.md §6:
// IR is the versioned, serializable boundary).
func DecodeProgram(data []byte) (*Program, error) {
	var prog Program
	if err := json.Unmarshal(data, &prog); err != nil {
		return nil, err
	}
	return &prog, nil
}

// MustDecodeProgram is DecodeProgram for generated code's init path, where a
// decode failure means the embedded data is corrupt — a build problem, not
// something a running program can recover from.
func MustDecodeProgram(data []byte) *Program {
	prog, err := DecodeProgram(data)
	if err != nil {
		panic(err)
	}
	return prog
}

// --- 標準命令 (internal/stdlib) ---

// Registry assigns every command a stable ID, the way the IR calls one.
type Registry = stdlib.Registry

// Plugin adds commands beyond plugin_system (AGENTS.md §3).
type Plugin = stdlib.Plugin

// NewRegistry builds the command table core なでしこ ships with, plus any
// plugins named.
func NewRegistry(plugins ...Plugin) *Registry { return stdlib.NewRegistry(plugins...) }

// Plugins beyond plugin_system (AGENTS.md §3) — Go-idiomatic, not part of the
// compatibility guarantee. A generated program's registry must be built with
// exactly the plugins the source was compiled against: a stdlib command's ID
// depends on the full, sorted set of commands in the registry, so a mismatch
// between compile time and run time silently calls the wrong command.
func NodeLib() Plugin   { return nodelib.New() }
func CSVLib() Plugin    { return csvlib.New() }
func MathLib() Plugin   { return mathlib.New() }
func SQLiteLib() Plugin { return sqlitelib.New() }
func OfficeLib() Plugin { return officelib.New() }
func PDFLib() Plugin    { return pdflib.New() }
func ImageLib() Plugin  { return imagelib.New() }

// --- 実行 (internal/vm) ---

// Machine runs one program: it is what NativeFunc receives, and what a
// generated function calls back into for anything beyond straight-line Go —
// an operator, an index, a stdlib command, another call.
type Machine = vm.VM

// NativeFunc is a function body gogen compiled to Go, standing in for a
// bytecode Func.
type NativeFunc = vm.NativeFunc

// Host is what a Machine needs from its environment: where output goes,
// where input comes from, and the bundle a packaged program may carry.
type Host = vm.Host

// Options bounds what one run may do.
type Options = vm.Options

// DefaultOptions is generous enough for a program that means it.
func DefaultOptions() Options { return vm.DefaultOptions() }

// NewMachine prepares a Machine for prog. Register every generated function
// with SetNative before calling Run.
func NewMachine(prog *Program, registry *Registry, host Host, options Options) *Machine {
	return vm.New(prog, registry, host, options)
}

// CUIHost runs a program against a real terminal: Print/Write go to out,
// ReadLine reads from in.
type CUIHost = vm.CUIHost

// NewCUIHost wires a host to the given streams — os.Stdout/os.Stdin for an
// ordinary program.
func NewCUIHost(out io.Writer, in io.Reader, args []string) *CUIHost {
	return vm.NewCUIHost(out, in, args)
}

// Handler remembers where a generated function resumes after エラー監視
// catches a runtime error raised inside it (mirrors the interpreter's own
// handler in internal/vm/vm.go).
type Handler struct {
	// Target is the label to resume at — Code[A] on the OpTry instruction that
	// opened this region.
	Target int
}

// Now supplies a Host's initial event-loop clock when the host has no
// meaningful notion of "now" — never a running program.
func Now() time.Time { return time.Now() }
