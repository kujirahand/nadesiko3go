package token

import (
	"nako3/core"
	"strings"
)

// TType : Token Type
type TType string

type Token struct {
	Type     TType
	Literal  string
	Josi     string
	FileInfo core.TFileInfo
}

type Tokens []*Token

const (
	//__BEGIN_TOKEN__
	FUNC      = "FUNC"
	EOF       = "EOF"
	LF        = "LF"
	NUMBER    = "NUMBER"
	STRING    = "STRING"
	STRING_EX = "STRING_EX"
	WORD      = "WORD"
	EQ        = "="
	PLUS      = "+"
	MINUS     = "-"
	NOT       = "!"
	ASTERISK  = "*"
	SLASH     = "/"
	PERCENT   = "%"
	EQEQ      = "=="
	NTEQ      = "!="
	GT        = ">"
	GTEQ      = ">="
	LT        = "<"
	LTEQ      = "<="
	LPAREN    = "("
	RPAREN    = ")"
	//__END_TOKEN__
)

func (t *Token) ToString() string {
	s := t.Literal + "[" + string(t.Type) + "]" + t.Josi
	return s
}

// TokensToString : TokensをStringに変換
func TokensToString(tt Tokens, delimiter string) string {
	s := ""
	for _, v := range tt {
		value := v.Literal
		switch v.Type {
		case STRING:
			value = "'" + value + "'"
		case STRING_EX:
			value = "「" + value + "」"
		}
		s += value + v.Josi + delimiter
	}
	s = strings.TrimSpace(s)
	return s
}

// TokensToTypeString : TokensをTypeに変換
func TokensToTypeString(tt Tokens, delimiter string) string {
	s := ""
	for _, v := range tt {
		s += string(v.Type) + delimiter
	}
	s = strings.TrimSpace(s)
	return s
}
