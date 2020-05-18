package main

import (
	"fmt"
	"nako3/core"
	"nako3/func/io"
	"nako3/node"
	"nako3/parser"
	"nako3/runner"
	"os"
)

func main() {
	sys := core.New()
	io.RegisterFunction(sys.GlobalVars)
	// parser
	println("=== cnako3.Parse ===")
	n, err := parser.Parse(sys, "1+2を表示", 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[文法エラー] %s\n", err.Error())
		return
	}
	if n == nil {
		panic("[文法エラー] 不明")
	}
	fmt.Printf("[parser.raw] %#v\n", *n)
	fmt.Printf("[parser]\n%s", node.NodeToString(*n, 0))
	// run
	println("=== cnako3.Run ===")
	runner.SetCore(sys)
	runner.Run(n)

}
