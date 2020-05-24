// Code generated by goyacc _parser_generated.y. DO NOT EDIT.

//line _parser_generated.y:3
//
// なでしこ3 --- 文法定義 (goyaccを利用)
//
// Lexerはgoyaccが要求する形にするため
// github.com/kujirahand/nadesiko3go/lexerをラップしてこのユニットで使用
//
package parser

import __yyfmt__ "fmt"

//line _parser_generated.y:9
import (
	"fmt"
	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/lexer"
	"github.com/kujirahand/nadesiko3go/node"
	"github.com/kujirahand/nadesiko3go/token"
	"github.com/kujirahand/nadesiko3go/value"
	"strings"
)

//line _parser_generated.y:21
type yySymType struct {
	yys   int
	token *token.Token // lval *yySymType
	node  node.Node
}

const UNKNOWN = 57346
const COMMENT = 57347
const FUNC = 57348
const EOF = 57349
const LF = 57350
const EOS = 57351
const EOS4ELSE = 57352
const COMMA = 57353
const NUMBER = 57354
const STRING = 57355
const STRING_EX = 57356
const WORD = 57357
const WORD_REF = 57358
const AND = 57359
const OR = 57360
const IF = 57361
const THEN = 57362
const THEN_SINGLE = 57363
const ELSE = 57364
const ELSE_SINGLE = 57365
const BEGIN = 57366
const END = 57367
const WHILE_BEGIN = 57368
const FOR_BEGIN = 57369
const FOR = 57370
const FOR_SINGLE = 57371
const KAI_BEGIN = 57372
const KAI = 57373
const KAI_SINGLE = 57374
const AIDA = 57375
const SAKINI = 57376
const TUGINI = 57377
const FOREACH = 57378
const BREAK = 57379
const CONTINUE = 57380
const RETURN = 57381
const TIKUJI = 57382
const LET = 57383
const HENSU = 57384
const TEISU = 57385
const INCLUDE = 57386
const LET_BEGIN = 57387
const ERROR_TRY = 57388
const ERROR = 57389
const DEF_FUNC = 57390
const EQ = 57391
const PLUS = 57392
const STR_PLUS = 57393
const MINUS = 57394
const NOT = 57395
const ASTERISK = 57396
const SLASH = 57397
const PERCENT = 57398
const CIRCUMFLEX = 57399
const EQEQ = 57400
const NTEQ = 57401
const GT = 57402
const GTEQ = 57403
const LT = 57404
const LTEQ = 57405
const LPAREN = 57406
const RPAREN = 57407
const LBRACKET = 57408
const RBRACKET = 57409
const LBRACE = 57410
const RBRACE = 57411

var yyToknames = [...]string{
	"$end",
	"error",
	"$unk",
	"UNKNOWN",
	"COMMENT",
	"FUNC",
	"EOF",
	"LF",
	"EOS",
	"EOS4ELSE",
	"COMMA",
	"NUMBER",
	"STRING",
	"STRING_EX",
	"WORD",
	"WORD_REF",
	"AND",
	"OR",
	"IF",
	"THEN",
	"THEN_SINGLE",
	"ELSE",
	"ELSE_SINGLE",
	"BEGIN",
	"END",
	"WHILE_BEGIN",
	"FOR_BEGIN",
	"FOR",
	"FOR_SINGLE",
	"KAI_BEGIN",
	"KAI",
	"KAI_SINGLE",
	"AIDA",
	"SAKINI",
	"TUGINI",
	"FOREACH",
	"BREAK",
	"CONTINUE",
	"RETURN",
	"TIKUJI",
	"LET",
	"HENSU",
	"TEISU",
	"INCLUDE",
	"LET_BEGIN",
	"ERROR_TRY",
	"ERROR",
	"DEF_FUNC",
	"EQ",
	"PLUS",
	"STR_PLUS",
	"MINUS",
	"NOT",
	"ASTERISK",
	"SLASH",
	"PERCENT",
	"CIRCUMFLEX",
	"EQEQ",
	"NTEQ",
	"GT",
	"GTEQ",
	"LT",
	"LTEQ",
	"LPAREN",
	"RPAREN",
	"LBRACKET",
	"RBRACKET",
	"LBRACE",
	"RBRACE",
}
var yyStatenames = [...]string{}

const yyEofCode = 1
const yyErrCode = 2
const yyInitialStackSize = 16

//line _parser_generated.y:309

var haltError error = nil

type Lexer struct {
	sys       *core.Core
	lexer     *lexer.Lexer
	tokens    token.Tokens
	index     int
	loopId    int
	lastToken *token.Token
	result    node.Node
}

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

func (l *Lexer) getId() int {
	l.loopId++
	return l.loopId
}

// 字句解析の結果をgoyaccに伝える
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
		v, _ := l.sys.Scopes.Find(t.Literal)
		if v != nil && v.Type == value.Function {
			result = FUNC
			t.Type = token.FUNC
		}
	}
	l.lastToken = t
	if l.sys.IsDebug {
		fmt.Printf("- Lex (%03d) %s\n",
			t.FileInfo.Line, t.ToString())
	}
	return result
}

// エラーを報告する
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

// 構文解析を実行する
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

// 以下 extract_token.nako3 により自動生成
func getTokenNo(token_type token.TType) int {
	switch token_type {
	case token.UNKNOWN:
		return UNKNOWN
	case token.COMMENT:
		return COMMENT
	case token.FUNC:
		return FUNC
	case token.EOF:
		return EOF
	case token.LF:
		return LF
	case token.EOS:
		return EOS
	case token.EOS4ELSE:
		return EOS4ELSE
	case token.COMMA:
		return COMMA
	case token.NUMBER:
		return NUMBER
	case token.STRING:
		return STRING
	case token.STRING_EX:
		return STRING_EX
	case token.WORD:
		return WORD
	case token.WORD_REF:
		return WORD_REF
	case token.AND:
		return AND
	case token.OR:
		return OR
	case token.IF:
		return IF
	case token.THEN:
		return THEN
	case token.THEN_SINGLE:
		return THEN_SINGLE
	case token.ELSE:
		return ELSE
	case token.ELSE_SINGLE:
		return ELSE_SINGLE
	case token.BEGIN:
		return BEGIN
	case token.END:
		return END
	case token.WHILE_BEGIN:
		return WHILE_BEGIN
	case token.FOR_BEGIN:
		return FOR_BEGIN
	case token.FOR:
		return FOR
	case token.FOR_SINGLE:
		return FOR_SINGLE
	case token.KAI_BEGIN:
		return KAI_BEGIN
	case token.KAI:
		return KAI
	case token.KAI_SINGLE:
		return KAI_SINGLE
	case token.AIDA:
		return AIDA
	case token.SAKINI:
		return SAKINI
	case token.TUGINI:
		return TUGINI
	case token.FOREACH:
		return FOREACH
	case token.BREAK:
		return BREAK
	case token.CONTINUE:
		return CONTINUE
	case token.RETURN:
		return RETURN
	case token.TIKUJI:
		return TIKUJI
	case token.LET:
		return LET
	case token.HENSU:
		return HENSU
	case token.TEISU:
		return TEISU
	case token.INCLUDE:
		return INCLUDE
	case token.LET_BEGIN:
		return LET_BEGIN
	case token.ERROR_TRY:
		return ERROR_TRY
	case token.ERROR:
		return ERROR
	case token.DEF_FUNC:
		return DEF_FUNC
	case token.EQ:
		return EQ
	case token.PLUS:
		return PLUS
	case token.STR_PLUS:
		return STR_PLUS
	case token.MINUS:
		return MINUS
	case token.NOT:
		return NOT
	case token.ASTERISK:
		return ASTERISK
	case token.SLASH:
		return SLASH
	case token.PERCENT:
		return PERCENT
	case token.CIRCUMFLEX:
		return CIRCUMFLEX
	case token.EQEQ:
		return EQEQ
	case token.NTEQ:
		return NTEQ
	case token.GT:
		return GT
	case token.GTEQ:
		return GTEQ
	case token.LT:
		return LT
	case token.LTEQ:
		return LTEQ
	case token.LPAREN:
		return LPAREN
	case token.RPAREN:
		return RPAREN
	case token.LBRACKET:
		return LBRACKET
	case token.RBRACKET:
		return RBRACKET
	case token.LBRACE:
		return LBRACE
	case token.RBRACE:
		return RBRACE

	}
	panic("[SYSTEM ERROR] parser.y + extract_token.nako3")
	return -1
}

// (メモ) これより下にコードを書かないようにする
// → 行番号が変わらないための対策

//line yacctab:1
var yyExca = [...]int{
	-1, 1,
	1, -1,
	-2, 0,
}

const yyPrivate = 57344

const yyLast = 167

var yyAct = [...]int{

	110, 30, 6, 3, 118, 31, 37, 13, 16, 106,
	15, 14, 29, 77, 34, 35, 36, 19, 47, 102,
	43, 21, 41, 51, 53, 54, 55, 57, 24, 23,
	78, 70, 22, 108, 107, 49, 71, 27, 88, 26,
	25, 67, 68, 69, 73, 17, 139, 20, 85, 84,
	76, 137, 79, 80, 34, 35, 36, 44, 86, 87,
	34, 35, 36, 44, 56, 136, 33, 126, 127, 98,
	99, 100, 65, 64, 66, 43, 101, 95, 96, 97,
	104, 105, 132, 74, 75, 28, 109, 72, 112, 117,
	116, 34, 35, 36, 44, 50, 42, 128, 34, 35,
	36, 44, 34, 35, 36, 44, 33, 103, 114, 115,
	121, 120, 33, 122, 123, 37, 119, 83, 82, 125,
	45, 46, 131, 129, 130, 133, 81, 15, 14, 134,
	124, 135, 113, 111, 2, 138, 58, 59, 60, 61,
	62, 63, 11, 33, 89, 90, 91, 92, 93, 94,
	33, 4, 10, 12, 33, 9, 52, 38, 39, 40,
	8, 48, 7, 18, 32, 5, 1,
}
var yyPact = [...]int{

	2, -1000, 2, -1000, -1000, 119, 119, 119, -1000, -1000,
	-1000, -1000, -1000, -1000, -1000, -1000, -42, 90, 103, -31,
	79, 86, 86, 48, 86, -1000, -1000, 78, 22, -13,
	-26, -1000, -1000, 86, -1000, -1000, -1000, -1000, -1000, -1000,
	-1000, 86, -1000, -1000, -1000, 86, 86, 86, -36, 86,
	86, 110, 97, -1000, 17, 86, 86, 5, 86, 86,
	86, 86, 86, 86, 86, 86, 86, 86, 86, 86,
	86, -46, 42, -1000, 78, 78, -1000, 86, 86, -58,
	-7, -8, 2, 2, 2, 124, 80, 86, 2, 22,
	22, 22, 22, 22, 22, -13, -13, -13, -26, -26,
	-26, -1000, -1000, -1000, -1000, -63, -1000, -1000, -1000, 93,
	88, 2, -1000, 2, 122, 2, 39, 72, -1000, 2,
	2, 2, -1000, 57, 2, -1000, 121, 2, -1000, -1000,
	-1000, 40, -1000, 26, 2, -1000, -1000, -1000, 21, -1000,
}
var yyPgo = [...]int{

	0, 166, 133, 3, 151, 165, 45, 2, 164, 37,
	85, 12, 1, 5, 163, 162, 161, 160, 156, 0,
	155, 153, 152, 142,
}
var yyR1 = [...]int{

	0, 1, 2, 2, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 4, 4, 15, 15, 15, 15,
	16, 16, 5, 5, 5, 6, 6, 8, 8, 8,
	8, 7, 14, 14, 14, 9, 9, 9, 9, 9,
	9, 9, 10, 10, 10, 10, 11, 11, 11, 11,
	12, 12, 13, 13, 17, 17, 17, 17, 17, 18,
	19, 21, 21, 20, 20, 23, 22, 22, 22, 22,
}
var yyR2 = [...]int{

	0, 1, 1, 2, 1, 2, 2, 2, 1, 1,
	1, 1, 1, 1, 1, 1, 3, 4, 4, 4,
	3, 4, 1, 2, 4, 1, 2, 1, 1, 1,
	1, 1, 1, 3, 3, 1, 3, 3, 3, 3,
	3, 3, 1, 3, 3, 3, 1, 3, 3, 3,
	1, 3, 1, 3, 6, 4, 6, 7, 5, 1,
	1, 1, 1, 4, 6, 5, 7, 8, 5, 6,
}
var yyChk = [...]int{

	-1000, -1, -2, -3, -4, -5, -7, -15, -17, -20,
	-22, -23, -21, 5, 9, 8, 6, -6, -14, 15,
	45, 19, 30, 27, 26, 38, 37, -9, -10, -11,
	-12, -13, -8, 64, 12, 13, 14, -3, -4, -4,
	-4, 64, 6, -7, 15, 17, 18, 49, -16, 66,
	16, -7, -18, -7, -7, -7, 16, -7, 58, 59,
	60, 61, 62, 63, 51, 50, 52, 54, 55, 56,
	57, -7, -6, -7, -9, -9, -7, 49, 66, -7,
	-7, 16, 21, 20, 32, 31, -7, -7, 33, -10,
	-10, -10, -10, -10, -10, -11, -11, -11, -12, -12,
	-12, -13, 65, 65, -7, -7, 67, 41, 41, -3,
	-19, -2, -3, 8, 28, 29, -7, -19, 67, 23,
	23, 22, 25, -19, 8, -3, 28, 29, 25, -3,
	-3, -19, 25, -19, 8, -3, 25, 25, -19, 25,
}
var yyDef = [...]int{

	0, -2, 1, 2, 4, 0, 25, 0, 8, 9,
	10, 11, 12, 13, 14, 15, 22, 0, 31, 30,
	0, 0, 0, 0, 0, 61, 62, 32, 35, 42,
	46, 50, 52, 0, 27, 28, 29, 3, 5, 6,
	7, 0, 23, 26, 30, 0, 0, 0, 0, 0,
	0, 0, 0, 59, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 25, 33, 34, 16, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 36,
	37, 38, 39, 40, 41, 43, 44, 45, 47, 48,
	49, 51, 53, 24, 17, 0, 20, 18, 19, 55,
	0, 60, 63, 0, 0, 0, 0, 0, 21, 0,
	0, 0, 58, 0, 0, 68, 0, 0, 65, 54,
	56, 0, 64, 0, 0, 69, 57, 66, 0, 67,
}
var yyTok1 = [...]int{

	1,
}
var yyTok2 = [...]int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 16, 17, 18, 19, 20, 21,
	22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
	32, 33, 34, 35, 36, 37, 38, 39, 40, 41,
	42, 43, 44, 45, 46, 47, 48, 49, 50, 51,
	52, 53, 54, 55, 56, 57, 58, 59, 60, 61,
	62, 63, 64, 65, 66, 67, 68, 69,
}
var yyTok3 = [...]int{
	0,
}

var yyErrorMessages = [...]struct {
	state int
	token int
	msg   string
}{}

//line yaccpar:1

/*	parser for yacc output	*/

var (
	yyDebug        = 0
	yyErrorVerbose = false
)

type yyLexer interface {
	Lex(lval *yySymType) int
	Error(s string)
}

type yyParser interface {
	Parse(yyLexer) int
	Lookahead() int
}

type yyParserImpl struct {
	lval  yySymType
	stack [yyInitialStackSize]yySymType
	char  int
}

func (p *yyParserImpl) Lookahead() int {
	return p.char
}

func yyNewParser() yyParser {
	return &yyParserImpl{}
}

const yyFlag = -1000

func yyTokname(c int) string {
	if c >= 1 && c-1 < len(yyToknames) {
		if yyToknames[c-1] != "" {
			return yyToknames[c-1]
		}
	}
	return __yyfmt__.Sprintf("tok-%v", c)
}

func yyStatname(s int) string {
	if s >= 0 && s < len(yyStatenames) {
		if yyStatenames[s] != "" {
			return yyStatenames[s]
		}
	}
	return __yyfmt__.Sprintf("state-%v", s)
}

func yyErrorMessage(state, lookAhead int) string {
	const TOKSTART = 4

	if !yyErrorVerbose {
		return "syntax error"
	}

	for _, e := range yyErrorMessages {
		if e.state == state && e.token == lookAhead {
			return "syntax error: " + e.msg
		}
	}

	res := "syntax error: unexpected " + yyTokname(lookAhead)

	// To match Bison, suggest at most four expected tokens.
	expected := make([]int, 0, 4)

	// Look for shiftable tokens.
	base := yyPact[state]
	for tok := TOKSTART; tok-1 < len(yyToknames); tok++ {
		if n := base + tok; n >= 0 && n < yyLast && yyChk[yyAct[n]] == tok {
			if len(expected) == cap(expected) {
				return res
			}
			expected = append(expected, tok)
		}
	}

	if yyDef[state] == -2 {
		i := 0
		for yyExca[i] != -1 || yyExca[i+1] != state {
			i += 2
		}

		// Look for tokens that we accept or reduce.
		for i += 2; yyExca[i] >= 0; i += 2 {
			tok := yyExca[i]
			if tok < TOKSTART || yyExca[i+1] == 0 {
				continue
			}
			if len(expected) == cap(expected) {
				return res
			}
			expected = append(expected, tok)
		}

		// If the default action is to accept or reduce, give up.
		if yyExca[i+1] != 0 {
			return res
		}
	}

	for i, tok := range expected {
		if i == 0 {
			res += ", expecting "
		} else {
			res += " or "
		}
		res += yyTokname(tok)
	}
	return res
}

func yylex1(lex yyLexer, lval *yySymType) (char, token int) {
	token = 0
	char = lex.Lex(lval)
	if char <= 0 {
		token = yyTok1[0]
		goto out
	}
	if char < len(yyTok1) {
		token = yyTok1[char]
		goto out
	}
	if char >= yyPrivate {
		if char < yyPrivate+len(yyTok2) {
			token = yyTok2[char-yyPrivate]
			goto out
		}
	}
	for i := 0; i < len(yyTok3); i += 2 {
		token = yyTok3[i+0]
		if token == char {
			token = yyTok3[i+1]
			goto out
		}
	}

out:
	if token == 0 {
		token = yyTok2[1] /* unknown char */
	}
	if yyDebug >= 3 {
		__yyfmt__.Printf("lex %s(%d)\n", yyTokname(token), uint(char))
	}
	return char, token
}

func yyParse(yylex yyLexer) int {
	return yyNewParser().Parse(yylex)
}

func (yyrcvr *yyParserImpl) Parse(yylex yyLexer) int {
	var yyn int
	var yyVAL yySymType
	var yyDollar []yySymType
	_ = yyDollar // silence set and not used
	yyS := yyrcvr.stack[:]

	Nerrs := 0   /* number of errors */
	Errflag := 0 /* error recovery flag */
	yystate := 0
	yyrcvr.char = -1
	yytoken := -1 // yyrcvr.char translated into internal numbering
	defer func() {
		// Make sure we report no lookahead when not parsing.
		yystate = -1
		yyrcvr.char = -1
		yytoken = -1
	}()
	yyp := -1
	goto yystack

ret0:
	return 0

ret1:
	return 1

yystack:
	/* put a state and value onto the stack */
	if yyDebug >= 4 {
		__yyfmt__.Printf("char %v in %v\n", yyTokname(yytoken), yyStatname(yystate))
	}

	yyp++
	if yyp >= len(yyS) {
		nyys := make([]yySymType, len(yyS)*2)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyS[yyp] = yyVAL
	yyS[yyp].yys = yystate

yynewstate:
	yyn = yyPact[yystate]
	if yyn <= yyFlag {
		goto yydefault /* simple state */
	}
	if yyrcvr.char < 0 {
		yyrcvr.char, yytoken = yylex1(yylex, &yyrcvr.lval)
	}
	yyn += yytoken
	if yyn < 0 || yyn >= yyLast {
		goto yydefault
	}
	yyn = yyAct[yyn]
	if yyChk[yyn] == yytoken { /* valid shift */
		yyrcvr.char = -1
		yytoken = -1
		yyVAL = yyrcvr.lval
		yystate = yyn
		if Errflag > 0 {
			Errflag--
		}
		goto yystack
	}

yydefault:
	/* default state action */
	yyn = yyDef[yystate]
	if yyn == -2 {
		if yyrcvr.char < 0 {
			yyrcvr.char, yytoken = yylex1(yylex, &yyrcvr.lval)
		}

		/* look through exception table */
		xi := 0
		for {
			if yyExca[xi+0] == -1 && yyExca[xi+1] == yystate {
				break
			}
			xi += 2
		}
		for xi += 2; ; xi += 2 {
			yyn = yyExca[xi+0]
			if yyn < 0 || yyn == yytoken {
				break
			}
		}
		yyn = yyExca[xi+1]
		if yyn < 0 {
			goto ret0
		}
	}
	if yyn == 0 {
		/* error ... attempt to resume parsing */
		switch Errflag {
		case 0: /* brand new error */
			yylex.Error(yyErrorMessage(yystate, yytoken))
			Nerrs++
			if yyDebug >= 1 {
				__yyfmt__.Printf("%s", yyStatname(yystate))
				__yyfmt__.Printf(" saw %s\n", yyTokname(yytoken))
			}
			fallthrough

		case 1, 2: /* incompletely recovered error ... try again */
			Errflag = 3

			/* find a state where "error" is a legal shift action */
			for yyp >= 0 {
				yyn = yyPact[yyS[yyp].yys] + yyErrCode
				if yyn >= 0 && yyn < yyLast {
					yystate = yyAct[yyn] /* simulate a shift of "error" */
					if yyChk[yystate] == yyErrCode {
						goto yystack
					}
				}

				/* the current p has no shift on "error", pop stack */
				if yyDebug >= 2 {
					__yyfmt__.Printf("error recovery pops state %d\n", yyS[yyp].yys)
				}
				yyp--
			}
			/* there is no state on the stack with an error shift ... abort */
			goto ret1

		case 3: /* no shift yet; clobber input char */
			if yyDebug >= 2 {
				__yyfmt__.Printf("error recovery discards %s\n", yyTokname(yytoken))
			}
			if yytoken == yyEofCode {
				goto ret1
			}
			yyrcvr.char = -1
			yytoken = -1
			goto yynewstate /* try again in the same state */
		}
	}

	/* reduction by production yyn */
	if yyDebug >= 2 {
		__yyfmt__.Printf("reduce %v in:\n\t%v\n", yyn, yyStatname(yystate))
	}

	yynt := yyn
	yypt := yyp
	_ = yypt // guard against "declared and not used"

	yyp -= yyR2[yyn]
	// yyp is now the index of $0. Perform the default action. Iff the
	// reduced production is ε, $1 is possibly out of range.
	if yyp+1 >= len(yyS) {
		nyys := make([]yySymType, len(yyS)*2)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyVAL = yyS[yyp+1]

	/* consult goto table to find next state */
	yyn = yyR1[yyn]
	yyg := yyPgo[yyn]
	yyj := yyg + yyS[yyp].yys + 1

	if yyj >= yyLast {
		yystate = yyAct[yyg]
	} else {
		yystate = yyAct[yyj]
		if yyChk[yystate] != -yyn {
			yystate = yyAct[yyg]
		}
	}
	// dummy call; replaced with literal code
	switch yynt {

	case 1:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:39
		{
			yyVAL.node = yyDollar[1].node
			yylex.(*Lexer).result = yyVAL.node
		}
	case 2:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:46
		{
			n := node.NewNodeSentence(yyDollar[1].node.GetFileInfo())
			n.Append(yyDollar[1].node)
			yyVAL.node = n
		}
	case 3:
		yyDollar = yyS[yypt-2 : yypt+1]
//line _parser_generated.y:52
		{
			n, _ := yyDollar[1].node.(node.NodeSentence)
			n.Append(yyDollar[2].node)
			yyVAL.node = n
		}
	case 13:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:69
		{
			yyVAL.node = node.NewNodeNop(yyDollar[1].token)
		}
	case 14:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:75
		{
			yyVAL.node = node.NewNodeNop(yyDollar[1].token)
		}
	case 15:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:79
		{
			yyVAL.node = node.NewNodeNop(yyDollar[1].token)
		}
	case 16:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:85
		{
			yyVAL.node = node.NewNodeLet(yyDollar[1].token, node.NewNodeList(), yyDollar[3].node)
		}
	case 17:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:89
		{
			n := yyDollar[2].node.(node.NodeList)
			yyVAL.node = node.NewNodeLet(yyDollar[1].token, n, yyDollar[4].node)
		}
	case 18:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:94
		{
			yyVAL.node = node.NewNodeLet(yyDollar[2].token, node.NewNodeList(), yyDollar[3].node)
		}
	case 19:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:98
		{
			yyVAL.node = node.NewNodeLet(yyDollar[3].token, node.NewNodeList(), yyDollar[2].node)
		}
	case 20:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:104
		{
			n := node.NodeList{yyDollar[2].node}
			yyVAL.node = n
		}
	case 21:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:109
		{
			n := yyDollar[1].node.(node.NodeList)
			yyVAL.node = append(n, yyDollar[3].node)
		}
	case 22:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:116
		{
			yyVAL.node = node.NewNodeCallFunc(yyDollar[1].token)
		}
	case 23:
		yyDollar = yyS[yypt-2 : yypt+1]
//line _parser_generated.y:120
		{
			n := node.NewNodeCallFunc(yyDollar[2].token)
			n.Args, _ = yyDollar[1].node.(node.NodeList)
			yyVAL.node = n
		}
	case 24:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:126
		{
			n := node.NewNodeCallFunc(yyDollar[1].token)
			n.Args, _ = yyDollar[3].node.(node.NodeList)
			yyVAL.node = n
		}
	case 25:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:134
		{
			n := node.NodeList{yyDollar[1].node}
			yyVAL.node = n
		}
	case 26:
		yyDollar = yyS[yypt-2 : yypt+1]
//line _parser_generated.y:139
		{
			args, _ := yyDollar[1].node.(node.NodeList)
			n := append(args, yyDollar[2].node)
			yyVAL.node = n
		}
	case 27:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:147
		{
			yyVAL.node = node.NewNodeConst(value.Float, yyDollar[1].token)
		}
	case 28:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:148
		{
			yyVAL.node = node.NewNodeConst(value.Str, yyDollar[1].token)
		}
	case 29:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:149
		{
			yyVAL.node = node.NewNodeConst(value.Str, yyDollar[1].token)
		}
	case 30:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:150
		{
			yyVAL.node = node.NewNodeWord(yyDollar[1].token)
		}
	case 33:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:158
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 34:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:162
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 36:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:169
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 37:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:173
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 38:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:177
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 39:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:181
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 40:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:185
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 41:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:189
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 43:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:196
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 44:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:200
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 45:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:204
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 47:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:211
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 48:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:215
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 49:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:219
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 51:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:226
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 53:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:233
		{
			yyVAL.node = node.NewNodeCalc(yyDollar[3].token, yyDollar[2].node)
		}
	case 54:
		yyDollar = yyS[yypt-6 : yypt+1]
//line _parser_generated.y:240
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[4].node, yyDollar[6].node)
		}
	case 55:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:244
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[4].node, node.NewNodeNop(yyDollar[1].token))
		}
	case 56:
		yyDollar = yyS[yypt-6 : yypt+1]
//line _parser_generated.y:248
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[4].node, yyDollar[6].node)
		}
	case 57:
		yyDollar = yyS[yypt-7 : yypt+1]
//line _parser_generated.y:252
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[4].node, yyDollar[6].node)
		}
	case 58:
		yyDollar = yyS[yypt-5 : yypt+1]
//line _parser_generated.y:256
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[4].node, node.NewNodeNop(yyDollar[1].token))
		}
	case 61:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:268
		{
			yyVAL.node = node.NewNodeContinue(yyDollar[1].token, 0)
		}
	case 62:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:271
		{
			yyVAL.node = node.NewNodeBreak(yyDollar[1].token, 0)
		}
	case 63:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:277
		{
			yyVAL.node = node.NewNodeRepeat(yyDollar[3].token, yyDollar[2].node, yyDollar[4].node)
		}
	case 64:
		yyDollar = yyS[yypt-6 : yypt+1]
//line _parser_generated.y:281
		{
			yyVAL.node = node.NewNodeRepeat(yyDollar[3].token, yyDollar[2].node, yyDollar[5].node)
		}
	case 65:
		yyDollar = yyS[yypt-5 : yypt+1]
//line _parser_generated.y:287
		{
			yyVAL.node = node.NewNodeWhile(yyDollar[3].token, yyDollar[2].node, yyDollar[4].node)
		}
	case 66:
		yyDollar = yyS[yypt-7 : yypt+1]
//line _parser_generated.y:293
		{
			yyVAL.node = node.NewNodeFor(yyDollar[4].token, "", yyDollar[2].node, yyDollar[3].node, yyDollar[6].node)
		}
	case 67:
		yyDollar = yyS[yypt-8 : yypt+1]
//line _parser_generated.y:297
		{
			yyVAL.node = node.NewNodeFor(yyDollar[5].token, yyDollar[2].token.Literal, yyDollar[3].node, yyDollar[4].node, yyDollar[7].node)
		}
	case 68:
		yyDollar = yyS[yypt-5 : yypt+1]
//line _parser_generated.y:301
		{
			yyVAL.node = node.NewNodeFor(yyDollar[4].token, "", yyDollar[2].node, yyDollar[3].node, yyDollar[5].node)
		}
	case 69:
		yyDollar = yyS[yypt-6 : yypt+1]
//line _parser_generated.y:305
		{
			yyVAL.node = node.NewNodeFor(yyDollar[5].token, yyDollar[2].token.Literal, yyDollar[3].node, yyDollar[4].node, yyDollar[6].node)
		}
	}
	goto yystack /* stack new state and value */
}