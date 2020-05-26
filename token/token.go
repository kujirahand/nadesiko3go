package token

import (
	"fmt"
	"strings"

	"github.com/kujirahand/nadesiko3go/core"
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
	UNKNOWN        = "UNKNOWN"
	COMMENT        = "COMMENT"
	FUNC           = "FUNC"
	EOF            = "EOF"
	LF             = "LF"
	EOS            = "EOS"
	EOS4ELSE       = "EOS(違えば)"
	COMMA          = "COMMA"
	NUMBER         = "NUMBER"
	STRING         = "STRING"
	STRING_EX      = "STRING_EX"
	WORD           = "WORD"
	WORD_REF       = "WORD_REF"
	AND            = "かつ"
	OR             = "または"
	IF             = "もし"
	THEN           = "ならば"
	THEN_SINGLE    = "ならば単文"
	ELSE           = "違"
	ELSE_SINGLE    = "違(単文)"
	BEGIN          = "ここから"
	END            = "ここまで"
	AIDA           = "間"
	WHILE_BEGIN    = "ここから間"
	FOREACH_BEGIN  = "ここから反復"
	FOREACH        = "反復"
	FOREACH_SINGLE = "反復(単文)"
	FOR_BEGIN      = "ここから繰返"
	FOR            = "繰返"
	FOR_SINGLE     = "繰返(単文)"
	KAI_BEGIN      = "ここから回"
	KAI            = "回"
	KAI_SINGLE     = "回(単文)"
	SAKINI         = "先"
	TUGINI         = "次"
	BREAK          = "抜"
	CONTINUE       = "続"
	RETURN         = "戻"
	TIKUJI         = "逐次実行"
	LET            = "代入"
	HENSU          = "変数"
	TEISU          = "定数"
	INCLUDE        = "取込"
	LET_BEGIN      = "代入(単文)"
	ERROR_TRY      = "エラー監視"
	ERROR          = "エラー"
	DEF_FUNC       = "関数"
	EQ             = "="
	PLUS           = "+"
	STR_PLUS       = "&"
	MINUS          = "-"
	NOT            = "!"
	ASTERISK       = "*"
	SLASH          = "/"
	PERCENT        = "%"
	CIRCUMFLEX     = "^"
	EQEQ           = "=="
	NTEQ           = "!="
	GT             = ">"
	GTEQ           = ">="
	LT             = "<"
	LTEQ           = "<="
	LPAREN         = "("
	RPAREN         = ")"
	LBRACKET       = "["
	RBRACKET       = "]"
	LBRACE         = "{"
	RBRACE         = "}"
	COLON          = ":"
	//__END_TOKEN__
)

// ReplaceWordToken : WORDを特定のトークンに置換
func ReplaceWordToken(lit string) TType {
	tok := ReservedWord[lit]
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

// TokensToStringDebug : デバッグ用にトークンを出力
func TokensToStringDebug(tt Tokens) string {
	s := ""
	for _, v := range tt {
		value := v.Literal
		switch v.Type {
		case STRING:
			value = "'" + value + "'"
		case STRING_EX:
			value = "「" + value + "」"
		}
		typ := string(v.Type)
		w := fmt.Sprintf(
			"[%s:%s%s]",
			typ, value, v.Josi)
		s += strings.TrimSpace(w)
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

// TokensInsert : トークンの途中に追加
func TokensInsert(tt Tokens, index int, t *Token) Tokens {
	tt2 := append(
		tt[:index],
		append(Tokens{t},
			tt[index:]...)...)
	return tt2
}

func IsOperator(t TType) bool {
	return t == PLUS || t == MINUS ||
		t == ASTERISK || t == SLASH || t == PERCENT ||
		t == EQEQ || t == NTEQ ||
		t == GT || t == GTEQ || t == LT || t == LTEQ
}

func CanUMinus(lt TType) bool {
	return IsOperator(lt) ||
		lt == LF || lt == EOS || lt == UNKNOWN ||
		lt == RPAREN || lt == RBRACKET || lt == RBRACE ||
		lt == EQ
}
