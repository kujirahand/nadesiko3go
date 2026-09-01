package vm

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/csvlib"
	"github.com/kujirahand/nadesiko3go/internal/ir"
	"github.com/kujirahand/nadesiko3go/internal/mathlib"
	"github.com/kujirahand/nadesiko3go/internal/nodelib"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/sqlitelib"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

// CUIHost runs a program against a real terminal: output goes to a writer,
// 『尋ねる』 reads a line of input, and 『終了』 stops the process.
type CUIHost struct {
	Out     io.Writer
	In      *bufio.Reader
	CmdArgs []string
	// Bundle is the payload packed into the executable, if any. Its files are
	// looked at before the real file system, so a program reads a bundled
	// resource with the same code it used during development.
	Bundle *bundle.Bundle

	// ExitCode is what 『終了』 asked for, and Exited says whether it was
	// called. The caller decides what to do with them, so that a test does
	// not have to end the test process.
	ExitCode int
	Exited   bool
}

// NewCUIHost wires a host to the process's own streams.
func NewCUIHost(out io.Writer, in io.Reader, args []string) *CUIHost {
	return &CUIHost{Out: out, In: bufio.NewReader(in), CmdArgs: args}
}

func (h *CUIHost) Print(s string) {
	fmt.Fprintln(h.Out, s)
	if f, ok := h.Out.(interface{ Flush() error }); ok {
		_ = f.Flush()
	} else if f, ok := h.Out.(interface{ Flush() }); ok {
		f.Flush()
	}
}

func (h *CUIHost) Write(s string) {
	fmt.Fprint(h.Out, s)
	if f, ok := h.Out.(interface{ Flush() error }); ok {
		_ = f.Flush()
	} else if f, ok := h.Out.(interface{ Flush() }); ok {
		f.Flush()
	}
}

func (h *CUIHost) ReadLine() (string, error) {
	line, err := h.In.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	// 末尾の改行を落とす。Windowsの CRLF も考える。
	line = trimNewline(line)
	return line, nil
}

func (h *CUIHost) Exit(code int) {
	h.ExitCode = code
	h.Exited = true
}

func (h *CUIHost) Args() []string { return h.CmdArgs }

func (h *CUIHost) ReadResource(name string) ([]byte, bool) {
	if h.Bundle == nil {
		return nil, false
	}
	return h.Bundle.ReadResource(name)
}

func (h *CUIHost) Now() time.Time { return time.Now() }

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// RunFile reads a program from a file and runs it against the host.
func RunFile(path string, h *CUIHost) error {
	code, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("ファイル『%s』を読み込めません: %w", path, err)
	}
	return RunProgram(string(code), path, h)
}

// RunProgram compiles and runs a program against the host.
//
// This is the path the CUI takes, so nodelib is available. The compat runner
// takes RunSource instead, which stays inside plugin_system.
func RunProgram(code, filename string, h *CUIHost) error {
	prog, err := CompileProgram(code, filename)
	if err != nil {
		return err
	}
	return RunCompiled(prog, h)
}

// CompileProgram compiles a program to IR without running it, which is what
// `gonako build` needs.
func CompileProgram(code, filename string) (*ir.Program, error) {
	registry := runtimeRegistry()
	tree, err := parser.ParseSource(code, filename, registry.FuncList())
	if err != nil {
		return nil, err
	}
	return compiler.Compile(tree, filename, registry)
}

// RunCompiled runs IR that was compiled earlier, which is how a bundled
// executable starts: the program is already compiled inside it.
func RunCompiled(prog *ir.Program, h *CUIHost) error {
	opts := DefaultOptions()
	opts.RealSleep = true
	return New(prog, runtimeRegistry(), h, opts).Run()
}

// RunWithHost compiles and runs a program against any host, which the doctest
// runner uses to collect the output instead of printing it.
func RunWithHost(code, filename string, h Host) error {
	registry := runtimeRegistry()
	tree, err := parser.ParseSource(code, filename, registry.FuncList())
	if err != nil {
		return err
	}
	prog, err := compiler.Compile(tree, filename, registry)
	if err != nil {
		return err
	}
	options := DefaultOptions()
	// 本家のcnako DocTestは同期実行の終了時点で比較し、予約した
	// setTimeoutの完了までは待たない。同じ境界で表示ログを比較する。
	options.DrainPendingCallbacks = false
	return New(prog, registry, h, options).Run()
}

func runtimeRegistry() *stdlib.Registry {
	return stdlib.NewRegistry(nodelib.New(), csvlib.New(), mathlib.New(), sqlitelib.New())
}
