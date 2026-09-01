package parser

import (
	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/indent"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// ParseSource runs the stage-1 source pipeline: prepare, tokenize, token
// replacement, and parsing. The supplied function list is shared by lexer and
// parser because both need command signatures.
func ParseSource(code, filename string, funcList lexer.FuncList) (*ast.Node, error) {
	lx := lexer.NewLexer()
	lx.FuncList = funcList
	raw, err := lexer.Tokenize(prepare.Text(prepare.Convert(code)), 0, filename)
	if err != nil {
		return nil, err
	}
	raw, err = indent.ConvertSyntax(raw)
	if err != nil {
		return nil, err
	}
	tokens, err := lx.ReplaceTokens(raw, true, filename)
	if err != nil {
		return nil, err
	}
	tokens, err = lx.ExpandCodeTokens(tokens, filename)
	if err != nil {
		return nil, err
	}
	p := New()
	p.SetFuncList(lx.FuncList)
	p.SetModuleExport(lx.ModuleExport)
	return p.Parse(tokens, filename)
}
