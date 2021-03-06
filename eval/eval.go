package eval

import (
	"fmt"

	"github.com/kujirahand/nadesiko3go/compiler"
	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/func/io"
	"github.com/kujirahand/nadesiko3go/func/system"
	"github.com/kujirahand/nadesiko3go/node"
	"github.com/kujirahand/nadesiko3go/parser"
	"github.com/kujirahand/nadesiko3go/runner"
	"github.com/kujirahand/nadesiko3go/value"
)

var sys *core.Core = nil

// InitSystem : システムを初期化
func InitSystem() *core.Core {
	if sys != nil {
		return sys
	}
	sys := core.GetSystemAndClear()
	io.RegisterFunction(sys)
	system.RegisterFunction(sys)
	return sys
}

// Eval : コードを評価して返す
func Eval(code string) (*value.Value, error) {
	sys := InitSystem()
	sys.Code = code
	sys.MainFile = "-e"
	return ExecCode(sys, sys.Code)
}

// ExecCode : コードを実行する
func ExecCode(sys *core.Core, code string) (*value.Value, error) {
	if sys.IsDebug {
		println("[Lexer]")
	}
	n, err := parser.Parse(sys, code, 0)
	if err != nil {
		return nil, fmt.Errorf("[文法エラー] %s", err.Error())
	}
	if n == nil {
		panic("[文法エラー] 不明")
	}
	// fmt.Printf("[parser.raw] %#v\n", *n)
	if sys.IsDebug {
		fmt.Printf("[parser]\n%s\n", node.ToString(*n, 0))
		println("[run]")
	}
	// run
	sys.SetSoreLink() // よく使う変数を毎回取得しなくても良いようにリンクを張る
	return runner.Run(n)
}

// Eval2 : コードを評価して返す
func Eval2(code string) (*value.Value, error) {
	sys := InitSystem()
	sys.Code = code
	sys.MainFile = "-e"
	return ExecBytecode(sys, sys.Code)
}

// ExecBytecode : コードを実行する
func ExecBytecode(sys *core.Core, code string) (*value.Value, error) {
	if sys.IsDebug {
		println("[Lexer]")
	}
	n, err := parser.Parse(sys, code, 0)
	if err != nil {
		return nil, fmt.Errorf("[文法エラー] %s", err.Error())
	}
	if n == nil {
		panic("[文法エラー] 不明")
	}
	// fmt.Printf("[parser.raw] %#v\n", *n)
	if sys.IsDebug {
		fmt.Printf("[parser]\n%s\n", node.ToString(*n, 0))
		println("[Compile]")
	}
	// run
	return compiler.Compile(sys, n)
}
