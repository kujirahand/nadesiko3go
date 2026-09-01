package vm

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/nodelib"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

// CUIHost runs a program against a real terminal: output goes to a writer,
// 『尋ねる』 reads a line of input, and 『終了』 stops the process.
type CUIHost struct {
	Out     io.Writer
	In      *bufio.Reader
	CmdArgs []string

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

func (h *CUIHost) Print(s string) { fmt.Fprintln(h.Out, s) }

func (h *CUIHost) Write(s string) { fmt.Fprint(h.Out, s) }

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
	registry := stdlib.NewRegistry(nodelib.New())
	tree, err := parser.ParseSource(code, filename, registry.FuncList())
	if err != nil {
		return err
	}
	prog, err := compiler.Compile(tree, filename, registry)
	if err != nil {
		return err
	}
	return New(prog, registry, h, DefaultOptions()).Run()
}
