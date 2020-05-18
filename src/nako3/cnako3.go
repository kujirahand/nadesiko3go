package main

import (
	"fmt"
	"nako3/core"
	"nako3/func/io"
	"nako3/parser"
	"nako3/runner"
)

func main() {
	sys := core.New()
	io.RegisterFunction(sys.GlobalVars)
	node := parser.Parse(sys, "1+2を表示\n3を表示", 0)
	fmt.Printf("%#v\n", *node)
	// run
	runner.SetCore(sys)
	runner.Run(node)

}
