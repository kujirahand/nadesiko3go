package main

import (
	"fmt"
	"io/ioutil"
	"nako3/core"
	"nako3/func/io"
	"nako3/node"
	"nako3/parser"
	"nako3/runner"
	"os"
)

func main() {
	// check arguments
	if len(os.Args) < 2 {
		println("# cnako3(go) ver." + core.NadesikoVersion)
		println("[USAGE]")
		println("  cnako3 -e \"source\"")
		println("  cnako3 file.nako3")
		println("[Options]")
		println("  -d\tDebug Mode")
		println("  -e (source)\tEval Mode")
		return
	}
	sys := core.GetSystem()
	io.RegisterFunction(sys.GlobalVars)

	// Analize Command Line
	for _, v := range os.Args {
		if v == "" {
			continue
		}
		// options
		if v[0] == '-' {
			if v == "-d" {
				sys.IsDebug = true
				continue
			}
			if v == "-e" {
				sys.RunMode = core.EvalCode
				continue
			}
		}
		// mainfile or evalcode
		if sys.RunMode == core.EvalCode {
			sys.Code = v
			continue
		}
		if sys.RunMode == core.MainFile {
			sys.MainFile = v
			continue
		}
	}
	// run
	switch sys.RunMode {
	case core.MainFile:
		runMainFile(sys)
	case core.EvalCode:
		runEvalCode(sys)
	}
}

func runMainFile(sys *core.Core) {
	code, err := ioutil.ReadFile(sys.MainFile)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"[IOエラー] ファイルが読めません。(file:%s)",
			sys.MainFile)
		return
	}
	execCode(sys, string(code))
}

func runEvalCode(sys *core.Core) {
	sys.MainFile = "-e"
	execCode(sys, sys.Code)
}

func execCode(sys *core.Core, code string) {
	if sys.IsDebug {
		println("[Lexer]")
	}
	n, err := parser.Parse(sys, code, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[文法エラー] %s\n", err.Error())
		return
	}
	if n == nil {
		panic("[文法エラー] 不明")
	}
	// fmt.Printf("[parser.raw] %#v\n", *n)
	if sys.IsDebug {
		fmt.Printf("[parser]\n%s\n", node.NodeToString(*n, 0))
		println("[run]")
	}
	// run
	runner.Run(n)
}
