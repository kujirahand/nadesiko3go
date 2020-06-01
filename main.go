package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/eval"
	"github.com/kujirahand/nadesiko3go/value"
	"github.com/pkg/profile"
)

func main() {
	defer profile.Start().Stop()
	// check arguments
	if len(os.Args) < 2 {
		println("# nadesiko3go ver." + core.NadesikoVersion)
		println("[USAGE]")
		println("  nadesiko3go -e \"source\"")
		println("  nadesiko3go file.nako3")
		println("[Options]")
		println("  -d\tDebug Mode")
		println("  -e (source)\tEval Mode")
		println("  -c\tOpCode Mode")
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
			if v == "-e" {
				sys.RunMode = core.EvalCode
				continue
			}
			if v == "-c" {
				sys.IsOpMode = true
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
	ret, err := eval.ExecCode(sys, string(code))
	outputResult(ret, err)
}

func runEvalCode(sys *core.Core) {
	sys.MainFile = "-e"
	ret, err := eval.ExecCode(sys, sys.Code)
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
