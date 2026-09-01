package parser

import (
	"github.com/kujirahand/nadesiko3go/internal/ast"
)

// opPriority gives each operator its binding power (nako_parser_const.mts).
var opPriority = map[string]int{
	// and or
	"and": 1, "or": 1,
	// compare
	"eq": 2, "noteq": 2, "===": 2, "!==": 2, "gt": 2, "gteq": 2, "lt": 2, "lteq": 2,
	"&": 3,
	// + - << >> >>>
	"+": 4, "-": 4, "shift_l": 4, "shift_r": 4, "shift_r0": 4,
	// * /
	"*": 5, "/": 5, "÷": 5, "÷÷": 5, "%": 5,
	// ^
	"^": 6, "**": 6,
}

// operatorList names every operator, in the order opPriority declares them.
var operatorList = []string{
	"and", "or",
	"eq", "noteq", "===", "!==", "gt", "gteq", "lt", "lteq",
	"&",
	"+", "-", "shift_l", "shift_r", "shift_r0",
	"*", "/", "÷", "÷÷", "%",
	"^", "**",
}

// RenbunJosi lists the particles that chain one sentence onto the next.
var RenbunJosi = []string{
	"いて", "えて", "きて", "けて", "して", "って", "にて", "みて", "めて", "ねて", "には", "んで",
}

// isOperator reports whether a node is an operator placeholder in an infix list.
func isOperator(n *ast.Node) bool {
	_, ok := opPriority[string(n.Type)]
	return ok
}

// infixToPolish rewrites an infix list (values and operators alternating) into
// reverse Polish order.
func infixToPolish(list []*ast.Node) []*ast.Node {
	priority := func(t *ast.Node) int {
		if p, ok := opPriority[string(t.Type)]; ok {
			return p
		}
		return 10
	}
	var stack, polish []*ast.Node
	for _, t := range list {
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			if priority(t) > priority(top) {
				break
			}
			stack = stack[:len(stack)-1]
			polish = append(polish, top)
		}
		stack = append(stack, t)
	}
	for len(stack) > 0 {
		polish = append(polish, stack[len(stack)-1])
		stack = stack[:len(stack)-1]
	}
	return polish
}

// infixToAST builds a tree of op nodes from an infix list. The particle of the
// last element becomes the particle of every op node, because that is what
// binds the whole expression to the function that consumes it.
func (p *Parser) infixToAST(list []*ast.Node) *ast.Node {
	if len(list) == 0 {
		return nil
	}
	last := list[len(list)-1]
	josi := last.Josi

	polish := infixToPolish(list)
	var stack []*ast.Node
	for _, t := range polish {
		if !isOperator(t) {
			stack = append(stack, t)
			continue
		}
		if len(stack) < 2 {
			p.failNode("計算式でエラー", last)
		}
		b := stack[len(stack)-1]
		a := stack[len(stack)-2]
		stack = stack[:len(stack)-2]
		op := &ast.Node{
			Type:      ast.Op,
			Operator:  string(t.Type),
			Blocks:    []*ast.Node{a, b},
			Josi:      josi,
			SourceMap: a.SourceMap,
		}
		stack = append(stack, op)
	}
	if len(stack) == 0 {
		return nil
	}
	return stack[len(stack)-1]
}
