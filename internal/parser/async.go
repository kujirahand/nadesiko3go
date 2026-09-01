package parser

import (
	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

// checkContext carries whether the walk rewrote the tree, plus what anonymous
// functions have been assigned to which variables.
type checkContext struct {
	modified bool
	// funcObjVarScopes maps a variable name to whether the anonymous function
	// it holds is async.
	//
	// An anonymous function assigned to a variable (`F=●()...ここまで`) never
	// reaches funclist, so without remembering the assignment a later `F()`
	// would not be marked async. One map per function definition, so an inner
	// assignment shadows an outer one.
	//
	// Synchronous assignments are recorded too: without them, assigning a
	// synchronous function to the same name in an inner scope would still see
	// the outer async assignment.
	funcObjVarScopes []map[string]bool
}

// CheckAsyncFn walks the tree and marks the functions that need to await.
//
// The information spreads outward — a function that calls an async function is
// itself async — so one pass is not enough. Callers repeat until it returns
// false (nako_parser_async.mts equivalent).
func CheckAsyncFn(node *ast.Node, funclist lexer.FuncList) bool {
	ctx := &checkContext{funcObjVarScopes: []map[string]bool{{}}}
	checkNode(node, funclist, ctx)
	return ctx.modified
}

// isAsyncVar looks the name up from the innermost scope outward, so an inner
// assignment shadows an outer one.
func isAsyncVar(ctx *checkContext, name string) bool {
	for i := len(ctx.funcObjVarScopes) - 1; i >= 0; i-- {
		if async, ok := ctx.funcObjVarScopes[i][name]; ok {
			return async
		}
	}
	return false
}

// rememberFuncObjAssign records an anonymous function assigned to a variable.
// Both let and def_local_var keep the assigned value in blocks[0].
//
// Only anonymous functions are recorded. Treating an assignment like
// 『F=Gの参照』 as synchronous would risk a missing await, which is the worse
// mistake.
func rememberFuncObjAssign(node *ast.Node, ctx *checkContext) {
	if node.Name == "" {
		return
	}
	v := node.Block(0)
	if v == nil || v.Type != ast.FuncObj {
		return
	}
	ctx.funcObjVarScopes[len(ctx.funcObjVarScopes)-1][node.Name] = v.AsyncFn
}

// checkNode reports whether the node contains async work, rewriting the tree
// where it finds out something new.
func checkNode(node *ast.Node, funclist lexer.FuncList, ctx *checkContext) bool {
	if node == nil {
		return false
	}
	switch node.Type {
	case ast.DefFunc, ast.DefTest, ast.FuncObj:
		// 非同期と分かっていても中身の走査は省略しない。途中で打ち切ると
		// それより後ろにある呼び出しに asyncFn が付かなくなる。
		ctx.funcObjVarScopes = append(ctx.funcObjVarScopes, map[string]bool{})
		isAsync := false
		for _, n := range node.Blocks {
			if checkNode(n, funclist, ctx) {
				isAsync = true
			}
		}
		ctx.funcObjVarScopes = ctx.funcObjVarScopes[:len(ctx.funcObjVarScopes)-1]

		if isAsync && !node.AsyncFn {
			node.AsyncFn = true
			if node.Meta != nil {
				node.Meta.AsyncFn = true
			}
			ctx.modified = true
		}
		if !node.AsyncFn {
			return false
		}
		// 後から判明した非同期情報を、呼び出し側の判定に使う関数一覧にも反映する
		if fn, ok := funclist[node.Name]; ok && !fn.AsyncFn {
			fn.AsyncFn = true
			ctx.modified = true
		}
		return true

	case ast.Let, ast.DefLocalVar:
		containsAsync := false
		for _, n := range node.Blocks {
			if checkNode(n, funclist, ctx) {
				containsAsync = true
			}
		}
		rememberFuncObjAssign(node, ctx)
		return containsAsync

	case ast.Func:
		if node.AsyncFn {
			return true
		}
		// 引数に非同期処理があるか。2つ目以降にも asyncFn が要るので
		// 最初の1つで打ち切らない。
		hasAsyncArg := false
		for _, n := range node.Blocks {
			if checkNode(n, funclist, ctx) {
				hasAsyncArg = true
			}
		}
		if hasAsyncArg {
			node.AsyncFn = true
			ctx.modified = true
			return true
		}
		if fn, ok := funclist[node.Name]; ok && fn.AsyncFn {
			node.AsyncFn = true
			ctx.modified = true
			return true
		}
		if isAsyncVar(ctx, node.Name) {
			node.AsyncFn = true
			ctx.modified = true
			return true
		}
		return false

	case ast.Renbun:
		// 連文は現在、効率は悪いが非同期で実行することになっている
		return true
	}

	containsAsync := false
	for _, n := range node.Blocks {
		if checkNode(n, funclist, ctx) {
			containsAsync = true
		}
	}
	return containsAsync
}
