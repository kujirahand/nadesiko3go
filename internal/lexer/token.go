package lexer

// TokenType names a token kind. The names are the ones used by nako_token.mts,
// because the parser's error messages quote them.
type TokenType string

const (
	TypeEOL           TokenType = "eol"
	TypeEOLUnderscore TokenType = "_eol"
	TypeEOF           TokenType = "eof"
	TypeDecLineNo     TokenType = "dec_lineno"
	TypeLineComment   TokenType = "line_comment"
	TypeRangeComment  TokenType = "range_comment"
	TypeString        TokenType = "string"
	TypeStringEx      TokenType = "string_ex"
	TypeCode          TokenType = "code"
	TypeNumber        TokenType = "number"
	TypeBigInt        TokenType = "bigint"
	TypeWord          TokenType = "word"
	TypeFunc          TokenType = "func"
	TypeComma         TokenType = "comma"
	TypeSpace         TokenType = "space"
	TypeDefTest       TokenType = "def_test"
	TypeDefFunc       TokenType = "def_func"
	TypeNot           TokenType = "not"
	TypeMoshi         TokenType = "もし"
	TypeChigaeba      TokenType = "違えば"
	TypeKokomade      TokenType = "ここまで"
	TypeKokokara      TokenType = "ここから"
)

// Token is one lexical unit.
//
// Offset and Length are rune offsets into the preprocessed source, not byte
// offsets, matching the rest of the Go implementation.
type Token struct {
	Type    TokenType
	Value   any // string, float64 (number), or int (eol carries its line)
	Line    int
	Column  int
	File    string
	Josi    string
	RawJosi string
	Indent  int
	Offset  int
	Length  int
}

// StringValue returns the token value as a string, or "" when it is not one.
func (t Token) StringValue() string {
	s, _ := t.Value.(string)
	return s
}

// NumberValue returns the token value as a float64, or 0 when it is not one.
func (t Token) NumberValue() float64 {
	n, _ := t.Value.(float64)
	return n
}

// TypesString joins token types with sep. It matches NakoLexer.tokensToTypeStr
// and is used in parser error messages.
func TypesString(tokens []Token, sep string) string {
	out := ""
	for i, t := range tokens {
		if i > 0 {
			out += sep
		}
		out += string(t.Type)
	}
	return out
}
