package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/eval"
	"github.com/kujirahand/nadesiko3go/value"
	// "github.com/pkg/profile"
)

func main() {
	// pprof /var/xxx/cpu.pprof
	// (pprof) svg
	// defer profile.Start().Stop()

	// check arguments
	if len(os.Args) < 2 {
		println("# nadesiko3go ver." + core.NadesikoVersion)
		println("[USAGE]")
		println("  cnako3go -e \"source\"")
		println("  cnako3go file.nako3")
		println("[Options]")
		println("  -d\tDebug Mode")
		println("  -e (source)\tEval Mode")
		println("  -S\tDo not run")
		return
	}
	sys := eval.InitSystem()

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
			if v == "-c" {
				sys.IsCompile = true
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

var sys *core.Core = nil

func runMainFile(sys *core.Core) {
	code, err := ioutil.ReadFile(sys.MainFile)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"[IOエラー] ファイルが読めません。(file:%s)",
			sys.MainFile)
		return
	}
	ret, err := eval.ExecBytecode(sys, string(code))
	outputResult(ret, err)
}

func runEvalCode(sys *core.Core) {
	sys.MainFile = "-e"
	ret, err := eval.ExecBytecode(sys, sys.Code)
	outputResult(ret, err)
}

func outputResult(ret *value.Value, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
	}
	if ret != nil {
		println(ret.ToString())
	}
}
