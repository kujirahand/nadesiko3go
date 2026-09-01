// Package errs defines nadesiko-compatible error kinds and source locations.
package errs

import (
	"fmt"
	"regexp"
)

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

// mainPrefixRE matches a name quoted in an error message that still carries the
// main module's namespace prefix.
var mainPrefixRE = regexp.MustCompile(`『main__(.+?)』`)

func (e *NakoError) Error() string {
	// 『main__関数名』のように名前空間が付いたままだと読みにくいので、
	// メインモジュールの接頭辞だけは省いて表示する (#1223)
	msg := mainPrefixRE.ReplaceAllString(e.Msg, "『$1』")
	return fmt.Sprintf("[%sエラー]%s(%d行目): %s", e.Kind.String(), e.File, e.Line+1, msg)
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
