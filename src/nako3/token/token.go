package token

import (
	"nako3/core"
	"strings"
)

// TType : Token Type
type TType string

// Token : トークン定義
type Token struct {
	Type     TType
	Literal  string
	Josi     string
	FileInfo core.TFileInfo
}

// Tokens : トークンのスライス
type Tokens []*Token

const (
	//__BEGIN_TOKEN__
	FUNC      = "FUNC"
	EOF       = "EOF"
	LF        = "LF"
	EOS       = "EOS"
	COMMA     = "COMMA"
	NUMBER    = "NUMBER"
	STRING    = "STRING"
	STRING_EX = "STRING_EX"
	WORD      = "WORD"
	WORD_REF  = "WORD_REF"
	IF        = "もし"
	THEN      = "ならば"
	ELSE      = "違"
	BEGIN     = "ここから"
	END       = "ここまで"
	FOR       = "繰返"
	REPEAT    = "回"
	FOREACH   = "反復"
	LET       = "代入"
	LET_BEGIN = "LET_BEGIN"
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
	LBRACKET  = "["
	RBRACKET  = "]"
	LBRACE    = "{"
	RBRACE    = "}"
	//__END_TOKEN__
)

var wordTokens = map[string]TType{
	"もし":   IF,
	"ならば":  THEN,
	"違":    ELSE,
	"ここまで": END,
	"ここから": BEGIN,
	"繰返":   FOR,
	"反復":   FOREACH,
	"回":    REPEAT,
	"代入":   LET,
	"入":    LET,
}

// ReplaceWordToken : WORDを特定のトークンに置換
func ReplaceWordToken(lit string) TType {
	tok := wordTokens[lit]
	if tok == "" {
		return WORD
	}
	return tok
}

// ToString : トークンを文字列に変換
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

// TokensAppend : トークンの末尾に追加
func TokensAppend(tt Tokens, t *Token) Tokens {
	tt2 := append(tt, t)
	return tt2
}

// TonkensInsert : トークンの途中に追加
func TokensInsert(tt Tokens, index int, t *Token) Tokens {
	tt2 := append(
		tt[:index],
		append(Tokens{t},
			tt[index:]...)...)
	return tt2
}
