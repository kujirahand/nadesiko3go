package parser

import (
	"fmt"
	"strings"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/lexer"
	"github.com/kujirahand/nadesiko3go/node"
	"github.com/kujirahand/nadesiko3go/token"
)

// NewLexerWrap : LexeWrap を生成
func NewLexerWrap(sys *core.Core, src string, fileno int) *Lexer {
	haltError = nil
	lex := Lexer{}
	lex.sys = sys
	lex.lexer = lexer.NewLexer(src, fileno)
	lex.tokens = lex.lexer.Split()
	if sys.IsDebug {
		println("[lexer.Split]")
		println(token.TokensToStringDebug(lex.tokens))
	}
	lex.index = 0
	lex.result = nil
	lex.loopId = 0
	return &lex
}

func (l *Lexer) getID() int {
	l.loopId++
	return l.loopId
}

// Parse : 構文解析を実行する
func Parse(sys *core.Core, src string, fno int) (*node.Node, error) {
	l := NewLexerWrap(sys, src, fno)

	yyDebug = 1
	yyErrorVerbose = true
	yyParse(l)

	if haltError != nil {
		return nil, haltError
	}
	return &l.result, nil
}

// Error : エラーを報告する
func (l *Lexer) Error(e string) {
	msg := e
	msg = strings.Replace(msg, "syntax error", "文法エラー", -1)
	msg = strings.Replace(msg, "unexpected", "不正な語句:", -1)
	msg = strings.Replace(msg, "expecting", "期待する語句:", -1)
	t := l.lastToken
	lineno := t.FileInfo.Line
	desc := t.ToString()
	haltError = fmt.Errorf("(%d) %s 理由:"+msg, lineno, desc)
}

// Lex : 字句解析の結果をgoyaccに伝える
func (l *Lexer) Lex(lval *yySymType) int {
	if l.index >= len(l.tokens) {
		return -1
	} // last
	if haltError != nil {
		return -1
	}
	// next
	t := l.tokens[l.index]
	l.index++
	lval.token = t
	// return
	result := getTokenNo(t.Type)
	if result == WORD {
		// go func
		v, _ := l.sys.Scopes.Find(t.Literal)
		if v != nil && v.IsFunction() {
			result = FUNC
			t.Type = token.FUNC
		} else if l.lexer.FuncNames[t.Literal] {
			result = FUNC
			t.Type = token.FUNC
		}
		// 助詞のある関数？
		if result == FUNC && t.Josi != "" {
			result = FUNC_JOSI
		}
	}
	l.lastToken = t
	if l.sys.IsDebug {
		fmt.Printf("- Lex (%03d) %s\n",
			t.FileInfo.Line, t.ToString())
	}
	return result
}
