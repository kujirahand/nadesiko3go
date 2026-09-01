// Package errs defines nadesiko-compatible error kinds and source locations.
package errs

import "fmt"

type Kind uint8

const (
	Lexer Kind = iota
	Syntax
	Runtime
)

type NakoError struct {
	Kind Kind
	File string
	Line int
	Msg  string
}

func (e *NakoError) Error() string {
	return fmt.Sprintf("[%sエラー]%s(%d行目): %s", e.Kind.String(), e.File, e.Line+1, e.Msg)
}

// CompatType returns the TypeScript error class name used by compat fixtures.
func (e *NakoError) CompatType() string {
	switch e.Kind {
	case Lexer:
		return "NakoLexerError"
	case Syntax:
		return "NakoSyntaxError"
	case Runtime:
		return "NakoRuntimeError"
	default:
		return "NakoError"
	}
}

func (k Kind) String() string {
	switch k {
	case Lexer:
		return "字句解析"
	case Syntax:
		return "文法"
	case Runtime:
		return "実行時"
	default:
		return "不明"
	}
}
