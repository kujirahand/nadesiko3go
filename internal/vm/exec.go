package vm

import (
	"io"
	"strings"
	"unicode"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Collector is a Host that keeps everything printed, which is what the compat
// runner compares against the fixtures.
//
// 『表示』 appends the value and a newline to the 『表示ログ』 system variable,
// and the log is read back with trailing whitespace stripped. Printing three
// empty strings therefore leaves an empty log, not two blank lines.
type Collector struct {
	buf strings.Builder
}

func (c *Collector) Print(s string) {
	c.buf.WriteString(s)
	c.buf.WriteByte('\n')
}

// Write is not part of 『表示ログ』, so the compat runner drops it.
func (c *Collector) Write(string) {}

// ReadLine has nothing to read: the compat fixtures never ask for input.
func (c *Collector) ReadLine() (string, error) { return "", io.EOF }

// Exit does nothing here; the VM stops the program either way.
func (c *Collector) Exit(int) {}

// Args reports no arguments.
func (c *Collector) Args() []string { return nil }

// ReadResource has nothing packed with it.
func (c *Collector) ReadResource(string) ([]byte, bool) { return nil, false }

// Log returns what was printed, with trailing whitespace removed.
func (c *Collector) Log() string {
	return strings.TrimRightFunc(c.buf.String(), isJSSpace)
}

// isJSSpace matches the characters JavaScript's \s does, which is what strips
// the end of the log.
func isJSSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v', '\f', 0x00A0, 0xFEFF, 0x2028, 0x2029:
		return true
	}
	return unicode.Is(unicode.Zs, r)
}

// Result is what running a program produced.
type Result struct {
	Log string
	// Vars holds the final value of the variables the caller asked about.
	Vars map[string]value.Value
}

// VarPrefix is the module namespace a top-level variable ends up under. The
// fixtures name variables without it.
const VarPrefix = "main__"

// RunSource compiles and runs one nadesiko program, and reports what it printed
// along with the final value of the named variables. A nadesiko error comes
// back as *errs.NakoError.
func RunSource(code, filename string, wantVars []string) (*Result, error) {
	registry := stdlib.NewRegistry()
	tree, err := parser.ParseSource(code, filename, registry.FuncList())
	if err != nil {
		return nil, err
	}
	prog, err := compiler.Compile(tree, filename, registry)
	if err != nil {
		return nil, err
	}
	out := &Collector{}
	machine := New(prog, registry, out, DefaultOptions())
	runErr := machine.Run()
	result := &Result{Log: out.Log(), Vars: machine.Vars(VarPrefix, wantVars)}
	if runErr != nil {
		return result, runErr
	}
	return result, nil
}
