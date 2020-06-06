// Code generated by goyacc -o parser/y.go parser/_parser_generated.y. DO NOT EDIT.

//line parser/_parser_generated.y:3
//
// なでしこ3 --- 文法定義 (goyaccを利用)
//
// Lexerはgoyaccが要求する形にするため
// github.com/kujirahand/nadesiko3go/lexerをラップしてこのユニットで使用
//
package parser

import __yyfmt__ "fmt"

//line parser/_parser_generated.y:9
import (
	"fmt"
	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/lexer"
	"github.com/kujirahand/nadesiko3go/node"
	"github.com/kujirahand/nadesiko3go/token"
	"github.com/kujirahand/nadesiko3go/value"
	"strings"
)

//line parser/_parser_generated.y:21
type yySymType struct {
	yys      int
	token    *token.Token // lval *yySymType
	node     node.Node
	jsonkv   node.JSONHashKeyValue
	nodelist node.TNodeList
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
const AIDA = 57368
const WHILE_BEGIN = 57369
const FOREACH_BEGIN = 57370
const FOREACH = 57371
const FOREACH_SINGLE = 57372
const FOR_BEGIN = 57373
const FOR = 57374
const FOR_SINGLE = 57375
const KAI_BEGIN = 57376
const KAI = 57377
const KAI_SINGLE = 57378
const SAKINI = 57379
const TUGINI = 57380
const BREAK = 57381
const CONTINUE = 57382
const RETURN = 57383
const TIKUJI = 57384
const LET = 57385
const HENSU = 57386
const TEISU = 57387
const INCLUDE = 57388
const LET_BEGIN = 57389
const BEGIN_CALLFUNC = 57390
const ERROR_TRY = 57391
const ERROR = 57392
const DEF_FUNC = 57393
const EQ = 57394
const PLUS = 57395
const STR_PLUS = 57396
const MINUS = 57397
const NOT = 57398
const MUL = 57399
const DIV = 57400
const MOD = 57401
const EXP = 57402
const EQEQ = 57403
const NTEQ = 57404
const GT = 57405
const GTEQ = 57406
const LT = 57407
const LTEQ = 57408
const LPAREN = 57409
const RPAREN = 57410
const LBRACKET = 57411
const RBRACKET = 57412
const LBRACE = 57413
const RBRACE = 57414
const COLON = 57415

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
	"AIDA",
	"WHILE_BEGIN",
	"FOREACH_BEGIN",
	"FOREACH",
	"FOREACH_SINGLE",
	"FOR_BEGIN",
	"FOR",
	"FOR_SINGLE",
	"KAI_BEGIN",
	"KAI",
	"KAI_SINGLE",
	"SAKINI",
	"TUGINI",
	"BREAK",
	"CONTINUE",
	"RETURN",
	"TIKUJI",
	"LET",
	"HENSU",
	"TEISU",
	"INCLUDE",
	"LET_BEGIN",
	"BEGIN_CALLFUNC",
	"ERROR_TRY",
	"ERROR",
	"DEF_FUNC",
	"EQ",
	"PLUS",
	"STR_PLUS",
	"MINUS",
	"NOT",
	"MUL",
	"DIV",
	"MOD",
	"EXP",
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
	"COLON",
}
var yyStatenames = [...]string{}

const yyEofCode = 1
const yyErrCode = 2
const yyInitialStackSize = 16

//line parser/_parser_generated.y:269

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
		// go func
		v, _ := l.sys.Scopes.Find(t.Literal)
		if v != nil && v.IsFunction() {
			result = FUNC
			t.Type = token.FUNC
		} else if l.lexer.FuncNames[t.Literal] {
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
	case token.AIDA:
		return AIDA
	case token.WHILE_BEGIN:
		return WHILE_BEGIN
	case token.FOREACH_BEGIN:
		return FOREACH_BEGIN
	case token.FOREACH:
		return FOREACH
	case token.FOREACH_SINGLE:
		return FOREACH_SINGLE
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
	case token.SAKINI:
		return SAKINI
	case token.TUGINI:
		return TUGINI
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
	case token.BEGIN_CALLFUNC:
		return BEGIN_CALLFUNC
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
	case token.MUL:
		return MUL
	case token.DIV:
		return DIV
	case token.MOD:
		return MOD
	case token.EXP:
		return EXP
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
	case token.COLON:
		return COLON

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

const yyLast = 654

var yyAct = [...]int{

	6, 150, 134, 90, 91, 92, 93, 3, 165, 140,
	42, 71, 72, 115, 114, 73, 164, 132, 164, 69,
	63, 112, 65, 68, 85, 75, 77, 78, 79, 81,
	84, 115, 61, 70, 58, 19, 73, 35, 36, 37,
	64, 88, 55, 56, 57, 58, 94, 95, 96, 97,
	98, 99, 100, 101, 102, 103, 104, 105, 106, 107,
	108, 201, 68, 138, 117, 35, 36, 37, 64, 181,
	113, 163, 116, 22, 118, 119, 133, 148, 168, 193,
	125, 126, 83, 82, 200, 86, 170, 169, 137, 171,
	129, 199, 21, 139, 40, 136, 41, 109, 110, 195,
	194, 22, 45, 46, 191, 187, 180, 53, 59, 54,
	63, 55, 56, 57, 58, 142, 143, 144, 145, 179,
	21, 178, 40, 135, 41, 122, 121, 156, 192, 149,
	189, 152, 159, 160, 161, 157, 162, 173, 53, 59,
	54, 166, 55, 56, 57, 58, 47, 48, 49, 50,
	51, 52, 158, 111, 153, 172, 5, 17, 16, 42,
	177, 130, 87, 174, 182, 89, 183, 38, 35, 36,
	37, 64, 186, 45, 46, 188, 184, 185, 66, 67,
	91, 92, 93, 4, 190, 151, 2, 39, 15, 43,
	44, 196, 11, 14, 197, 198, 12, 10, 13, 9,
	76, 8, 7, 20, 22, 1, 0, 0, 0, 53,
	59, 54, 0, 55, 56, 57, 58, 47, 48, 49,
	50, 51, 52, 21, 0, 40, 0, 41, 33, 18,
	0, 17, 16, 0, 0, 35, 36, 37, 23, 0,
	0, 0, 25, 0, 0, 0, 0, 0, 0, 0,
	29, 28, 0, 0, 27, 0, 0, 26, 0, 45,
	46, 0, 31, 30, 32, 0, 0, 0, 0, 0,
	24, 22, 0, 0, 34, 0, 0, 45, 46, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	21, 0, 40, 0, 41, 53, 59, 54, 0, 55,
	56, 57, 58, 47, 48, 49, 50, 51, 52, 0,
	0, 0, 167, 53, 59, 54, 0, 55, 56, 57,
	58, 47, 48, 49, 50, 51, 52, 17, 16, 0,
	146, 0, 0, 0, 0, 0, 45, 46, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	45, 46, 0, 35, 36, 37, 64, 0, 0, 0,
	60, 0, 0, 0, 0, 175, 176, 35, 36, 37,
	64, 80, 53, 59, 54, 0, 55, 56, 57, 58,
	47, 48, 49, 50, 51, 52, 53, 59, 54, 22,
	55, 56, 57, 58, 47, 48, 49, 50, 51, 52,
	45, 46, 0, 22, 0, 0, 0, 0, 21, 141,
	40, 0, 41, 0, 0, 154, 155, 45, 46, 0,
	0, 0, 21, 0, 40, 0, 41, 0, 0, 128,
	127, 0, 45, 46, 0, 0, 53, 59, 54, 0,
	55, 56, 57, 58, 47, 48, 49, 50, 51, 52,
	124, 123, 0, 53, 59, 54, 0, 55, 56, 57,
	58, 47, 48, 49, 50, 51, 52, 0, 53, 59,
	54, 0, 55, 56, 57, 58, 47, 48, 49, 50,
	51, 52, 45, 46, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 45, 46, 0, 0,
	0, 0, 0, 0, 0, 131, 0, 0, 147, 120,
	45, 46, 0, 35, 36, 37, 64, 74, 53, 59,
	54, 0, 55, 56, 57, 58, 47, 48, 49, 50,
	51, 52, 53, 59, 54, 0, 55, 56, 57, 58,
	47, 48, 49, 50, 51, 52, 53, 59, 54, 22,
	55, 56, 57, 58, 47, 48, 49, 50, 51, 52,
	45, 46, 0, 35, 36, 37, 64, 0, 21, 18,
	40, 0, 41, 0, 62, 35, 36, 37, 64, 0,
	35, 36, 37, 64, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 53, 59, 54, 22,
	55, 56, 57, 58, 47, 48, 49, 50, 51, 52,
	0, 22, 0, 0, 0, 0, 22, 0, 21, 0,
	40, 0, 41, 0, 0, 0, 0, 0, 0, 0,
	21, 0, 40, 0, 41, 21, 0, 40, 0, 41,
	53, 59, 54, 0, 55, 56, 57, 58, 47, 48,
	49, 50, 51, 52,
}
var yyPact = [...]int{

	223, -1000, 223, -1000, -1000, 149, 319, -1000, -1000, -1000,
	-1000, -1000, -1000, -1000, -1000, -1000, -1000, -1000, -35, 568,
	-1000, 563, 563, -33, 501, 551, 551, 355, 53, 551,
	-1000, -1000, -1000, -1000, 18, -1000, -1000, -1000, -1000, -1000,
	551, 167, -1000, -1000, -1000, 551, 551, 551, 551, 551,
	551, 551, 551, 551, 551, 551, 551, 551, 551, 551,
	-1000, 551, -1000, 543, -54, 85, -47, -1000, 543, 551,
	-38, 20, 12, 551, 551, 493, 105, 543, 415, 156,
	551, 400, 223, 153, 479, 9, 108, 25, 543, -9,
	-64, -1000, -1000, -1000, 587, 587, 54, 54, 54, 54,
	54, 54, -15, -15, -26, -26, -26, -1000, -15, 341,
	-56, -1000, -1000, 543, 551, 551, 551, 551, 260, 465,
	34, 223, 223, 223, 146, 383, 156, 223, 144, -1000,
	223, 223, 223, 108, 3, -1000, -1000, 543, -1000, -65,
	551, -1000, 543, 242, 543, 543, -1000, -1000, -1000, 55,
	64, 223, -1000, 223, 129, 223, 333, -1000, 223, 96,
	94, 81, 1, 158, -1000, 551, 543, -1000, 223, 223,
	223, -1000, 80, 223, -1000, 122, 223, 79, -1000, -1000,
	-1000, 120, 71, 543, -1000, -1000, 75, -1000, 74, 223,
	-1000, -1000, 223, 223, -1000, -1000, 66, 59, 36, -1000,
	-1000, -1000,
}
var yyPgo = [...]int{

	0, 205, 7, 183, 156, 0, 203, 202, 201, 200,
	199, 198, 197, 196, 193, 192, 188, 187, 167, 165,
	3, 185, 162, 33, 35, 1, 2,
}
var yyR1 = [...]int{

	0, 1, 21, 21, 2, 2, 2, 2, 2, 2,
	2, 2, 2, 2, 2, 2, 14, 3, 3, 7,
	7, 7, 7, 7, 7, 7, 7, 23, 23, 4,
	4, 4, 24, 24, 6, 6, 6, 6, 6, 18,
	18, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	17, 17, 22, 22, 20, 20, 20, 19, 19, 8,
	8, 8, 8, 8, 9, 25, 11, 11, 11, 11,
	10, 10, 13, 15, 15, 15, 15, 12, 12, 12,
	12, 16, 16, 16, 26, 26,
}
var yyR2 = [...]int{

	0, 1, 1, 2, 1, 2, 2, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 3,
	4, 4, 4, 4, 2, 4, 2, 3, 4, 1,
	2, 4, 1, 2, 1, 1, 1, 1, 1, 1,
	2, 1, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 2,
	3, 3, 1, 2, 1, 1, 1, 3, 4, 6,
	4, 6, 7, 5, 1, 1, 1, 1, 1, 2,
	4, 6, 5, 4, 3, 6, 5, 7, 8, 5,
	6, 5, 8, 8, 1, 2,
}
var yyChk = [...]int{

	-1000, -1, -21, -2, -3, -4, -5, -7, -8, -10,
	-12, -15, -13, -11, -14, -16, 9, 8, 6, -24,
	-6, 67, 48, 15, 47, 19, 34, 31, 28, 27,
	40, 39, 41, 5, 51, 12, 13, 14, -18, -17,
	69, 71, -2, -3, -3, 17, 18, 61, 62, 63,
	64, 65, 66, 53, 55, 57, 58, 59, 60, 54,
	41, 67, 6, -5, 15, -5, -4, -4, -5, 52,
	-23, 44, 45, 69, 16, -5, -9, -5, -5, -5,
	16, -5, 30, 29, -5, 6, 67, -22, -5, -19,
	-20, 13, 14, 15, -5, -5, -5, -5, -5, -5,
	-5, -5, -5, -5, -5, -5, -5, -5, -5, -24,
	-23, 68, 68, -5, 52, 69, 52, 52, -5, -5,
	16, 21, 20, 36, 35, -5, -5, 30, 29, -2,
	8, 26, 8, 67, -26, 15, 70, -5, 72, -20,
	73, 68, -5, -5, -5, -5, 70, 43, 43, -2,
	-25, -21, -2, 8, 32, 33, -5, -2, 8, -25,
	-25, -25, -26, 68, 15, 73, -5, 70, 23, 23,
	22, 25, -25, 8, -2, 32, 33, -25, 25, 25,
	25, 68, 6, -5, -2, -2, -25, 25, -25, 8,
	-2, 25, 8, 8, 25, 25, -25, -25, -25, 25,
	25, 25,
}
var yyDef = [...]int{

	0, -2, 1, 2, 4, 0, 32, 7, 8, 9,
	10, 11, 12, 13, 14, 15, 17, 18, 29, 0,
	41, 0, 0, 39, 0, 0, 0, 0, 0, 0,
	76, 77, 78, 16, 0, 34, 35, 36, 37, 38,
	0, 0, 3, 5, 6, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	79, 0, 30, 33, 39, 32, 0, 59, 32, 0,
	40, 24, 26, 0, 0, 0, 0, 74, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 62, 0,
	0, 64, 65, 66, 42, 43, 44, 45, 46, 47,
	48, 49, 50, 51, 52, 53, 54, 55, 56, 0,
	40, 57, 58, 19, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 84,
	0, 0, 0, 0, 0, 94, 60, 63, 61, 0,
	0, 31, 20, 0, 23, 25, 27, 21, 22, 70,
	0, 75, 80, 0, 0, 0, 0, 83, 0, 0,
	0, 0, 0, 0, 95, 0, 67, 28, 0, 0,
	0, 73, 0, 0, 89, 0, 0, 0, 86, 82,
	91, 0, 0, 68, 69, 71, 0, 81, 0, 0,
	90, 85, 0, 0, 72, 87, 0, 0, 0, 88,
	93, 92,
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
	62, 63, 64, 65, 66, 67, 68, 69, 70, 71,
	72, 73,
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
//line parser/_parser_generated.y:52
		{
			yyVAL.node = yyDollar[1].nodelist
			yylex.(*Lexer).result = yyVAL.node
		}
	case 2:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:55
		{
			yyVAL.nodelist = node.TNodeList{yyDollar[1].node}
		}
	case 3:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser/_parser_generated.y:56
		{
			yyVAL.nodelist = append(yyDollar[1].nodelist, yyDollar[2].node)
		}
	case 16:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:73
		{
			yyVAL.node = node.NewNodeNop(yyDollar[1].token)
		}
	case 17:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:76
		{
			yyVAL.node = node.NewNodeNop(yyDollar[1].token)
		}
	case 18:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:77
		{
			yyVAL.node = node.NewNodeNop(yyDollar[1].token)
		}
	case 19:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:80
		{
			yyVAL.node = node.NewNodeLet(yyDollar[1].token, nil, yyDollar[3].node)
		}
	case 20:
		yyDollar = yyS[yypt-4 : yypt+1]
//line parser/_parser_generated.y:81
		{
			yyVAL.node = node.NewNodeLet(yyDollar[1].token, yyDollar[2].nodelist, yyDollar[4].node)
		}
	case 21:
		yyDollar = yyS[yypt-4 : yypt+1]
//line parser/_parser_generated.y:82
		{
			yyVAL.node = node.NewNodeLet(yyDollar[2].token, nil, yyDollar[3].node)
		}
	case 22:
		yyDollar = yyS[yypt-4 : yypt+1]
//line parser/_parser_generated.y:83
		{
			yyVAL.node = node.NewNodeLet(yyDollar[3].token, nil, yyDollar[2].node)
		}
	case 23:
		yyDollar = yyS[yypt-4 : yypt+1]
//line parser/_parser_generated.y:84
		{
			yyVAL.node = node.NewNodeDefVar(yyDollar[1].token, yyDollar[4].node)
		}
	case 24:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser/_parser_generated.y:85
		{
			yyVAL.node = node.NewNodeDefVar(yyDollar[1].token, nil)
		}
	case 25:
		yyDollar = yyS[yypt-4 : yypt+1]
//line parser/_parser_generated.y:86
		{
			yyVAL.node = node.NewNodeDefConst(yyDollar[1].token, yyDollar[4].node)
		}
	case 26:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser/_parser_generated.y:87
		{
			yyVAL.node = node.NewNodeDefConst(yyDollar[1].token, nil)
		}
	case 27:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:90
		{
			yyVAL.nodelist = node.TNodeList{yyDollar[2].node}
		}
	case 28:
		yyDollar = yyS[yypt-4 : yypt+1]
//line parser/_parser_generated.y:91
		{
			yyVAL.nodelist = append(yyDollar[1].nodelist, yyDollar[3].node)
		}
	case 29:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:94
		{
			yyVAL.node = node.NewNodeCallFunc(yyDollar[1].token, nil)
		}
	case 30:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser/_parser_generated.y:95
		{
			yyVAL.node = node.NewNodeCallFunc(yyDollar[2].token, yyDollar[1].nodelist)
		}
	case 31:
		yyDollar = yyS[yypt-4 : yypt+1]
//line parser/_parser_generated.y:96
		{
			yyVAL.node = node.NewNodeCallFuncCStyle(yyDollar[1].token, yyDollar[3].nodelist, yyDollar[4].token)
		}
	case 32:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:99
		{
			yyVAL.nodelist = node.TNodeList{yyDollar[1].node}
		}
	case 33:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser/_parser_generated.y:100
		{
			yyVAL.nodelist = append(yyDollar[1].nodelist, yyDollar[2].node)
		}
	case 34:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:104
		{
			yyVAL.node = node.NewNodeConst(value.Float, yyDollar[1].token)
		}
	case 35:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:105
		{
			yyVAL.node = node.NewNodeConst(value.Str, yyDollar[1].token)
		}
	case 36:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:106
		{
			yyVAL.node = node.NewNodeConstEx(value.Str, yyDollar[1].token)
		}
	case 39:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:111
		{
			yyVAL.node = node.NewNodeWord(yyDollar[1].token, nil)
		}
	case 40:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser/_parser_generated.y:112
		{
			yyVAL.node = node.NewNodeWord(yyDollar[1].token, yyDollar[2].nodelist)
		}
	case 42:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:117
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 43:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:118
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 44:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:119
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 45:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:120
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 46:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:121
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 47:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:122
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 48:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:123
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 49:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:124
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 50:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:125
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 51:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:126
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 52:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:127
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 53:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:128
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 54:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:129
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 55:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:130
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 56:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:131
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 57:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:132
		{
			yyVAL.node = node.NewNodeCalc(yyDollar[3].token, yyDollar[2].node)
		}
	case 58:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:133
		{
			yyVAL.node = node.NewNodeCalc(yyDollar[3].token, yyDollar[2].node)
		}
	case 59:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser/_parser_generated.y:134
		{
			yyVAL.node = yyDollar[2].node
		}
	case 60:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:138
		{
			yyVAL.node = node.NewNodeJSONArray(yyDollar[1].token, yyDollar[2].nodelist)
		}
	case 61:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:139
		{
			yyVAL.node = node.NewNodeJSONHash(yyDollar[1].token, yyDollar[2].jsonkv)
		}
	case 62:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:142
		{
			yyVAL.nodelist = node.TNodeList{yyDollar[1].node}
		}
	case 63:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser/_parser_generated.y:143
		{
			yyVAL.nodelist = append(yyDollar[1].nodelist, yyDollar[2].node)
		}
	case 67:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:152
		{
			kv := node.JSONHashKeyValue{}
			kv[yyDollar[1].token.Literal] = yyDollar[3].node
			yyVAL.jsonkv = kv
		}
	case 68:
		yyDollar = yyS[yypt-4 : yypt+1]
//line parser/_parser_generated.y:158
		{
			yyDollar[1].jsonkv[yyDollar[2].token.Literal] = yyDollar[4].node
			yyVAL.jsonkv = yyDollar[1].jsonkv
		}
	case 69:
		yyDollar = yyS[yypt-6 : yypt+1]
//line parser/_parser_generated.y:166
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[4].node, yyDollar[6].node)
		}
	case 70:
		yyDollar = yyS[yypt-4 : yypt+1]
//line parser/_parser_generated.y:170
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[4].node, nil)
		}
	case 71:
		yyDollar = yyS[yypt-6 : yypt+1]
//line parser/_parser_generated.y:174
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[4].nodelist, yyDollar[6].node)
		}
	case 72:
		yyDollar = yyS[yypt-7 : yypt+1]
//line parser/_parser_generated.y:178
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[4].nodelist, yyDollar[6].nodelist)
		}
	case 73:
		yyDollar = yyS[yypt-5 : yypt+1]
//line parser/_parser_generated.y:182
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[4].nodelist, nil)
		}
	case 76:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:194
		{
			yyVAL.node = node.NewNodeContinue(yyDollar[1].token, 0)
		}
	case 77:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:195
		{
			yyVAL.node = node.NewNodeBreak(yyDollar[1].token, 0)
		}
	case 78:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:196
		{
			yyVAL.node = node.NewNodeReturn(yyDollar[1].token, nil, 0)
		}
	case 79:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser/_parser_generated.y:197
		{
			yyVAL.node = node.NewNodeReturn(yyDollar[2].token, yyDollar[1].node, 0)
		}
	case 80:
		yyDollar = yyS[yypt-4 : yypt+1]
//line parser/_parser_generated.y:201
		{
			yyVAL.node = node.NewNodeRepeat(yyDollar[3].token, yyDollar[2].node, yyDollar[4].node)
		}
	case 81:
		yyDollar = yyS[yypt-6 : yypt+1]
//line parser/_parser_generated.y:205
		{
			yyVAL.node = node.NewNodeRepeat(yyDollar[3].token, yyDollar[2].node, yyDollar[5].nodelist)
		}
	case 82:
		yyDollar = yyS[yypt-5 : yypt+1]
//line parser/_parser_generated.y:211
		{
			yyVAL.node = node.NewNodeWhile(yyDollar[3].token, yyDollar[2].node, yyDollar[4].nodelist)
		}
	case 83:
		yyDollar = yyS[yypt-4 : yypt+1]
//line parser/_parser_generated.y:217
		{
			yyVAL.node = node.NewNodeForeach(yyDollar[3].token, yyDollar[2].node, yyDollar[4].node)
		}
	case 84:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser/_parser_generated.y:221
		{
			yyVAL.node = node.NewNodeForeach(yyDollar[2].token, nil, yyDollar[3].node)
		}
	case 85:
		yyDollar = yyS[yypt-6 : yypt+1]
//line parser/_parser_generated.y:225
		{
			yyVAL.node = node.NewNodeForeach(yyDollar[3].token, yyDollar[2].node, yyDollar[5].nodelist)
		}
	case 86:
		yyDollar = yyS[yypt-5 : yypt+1]
//line parser/_parser_generated.y:229
		{
			yyVAL.node = node.NewNodeForeach(yyDollar[2].token, nil, yyDollar[4].nodelist)
		}
	case 87:
		yyDollar = yyS[yypt-7 : yypt+1]
//line parser/_parser_generated.y:235
		{
			yyVAL.node = node.NewNodeFor(yyDollar[4].token, "", yyDollar[2].node, yyDollar[3].node, yyDollar[6].nodelist)
		}
	case 88:
		yyDollar = yyS[yypt-8 : yypt+1]
//line parser/_parser_generated.y:239
		{
			yyVAL.node = node.NewNodeFor(yyDollar[5].token, yyDollar[2].token.Literal, yyDollar[3].node, yyDollar[4].node, yyDollar[7].nodelist)
		}
	case 89:
		yyDollar = yyS[yypt-5 : yypt+1]
//line parser/_parser_generated.y:243
		{
			yyVAL.node = node.NewNodeFor(yyDollar[4].token, "", yyDollar[2].node, yyDollar[3].node, yyDollar[5].node)
		}
	case 90:
		yyDollar = yyS[yypt-6 : yypt+1]
//line parser/_parser_generated.y:247
		{
			yyVAL.node = node.NewNodeFor(yyDollar[5].token, yyDollar[2].token.Literal, yyDollar[3].node, yyDollar[4].node, yyDollar[6].node)
		}
	case 91:
		yyDollar = yyS[yypt-5 : yypt+1]
//line parser/_parser_generated.y:253
		{
			yyVAL.node = node.NewNodeDefFunc(yyDollar[2].token, node.NewNodeList(), yyDollar[4].nodelist)
		}
	case 92:
		yyDollar = yyS[yypt-8 : yypt+1]
//line parser/_parser_generated.y:257
		{
			yyVAL.node = node.NewNodeDefFunc(yyDollar[5].token, yyDollar[3].nodelist, yyDollar[7].nodelist)
		}
	case 93:
		yyDollar = yyS[yypt-8 : yypt+1]
//line parser/_parser_generated.y:261
		{
			yyVAL.node = node.NewNodeDefFunc(yyDollar[2].token, yyDollar[4].nodelist, yyDollar[7].nodelist)
		}
	case 94:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser/_parser_generated.y:266
		{
			yyVAL.nodelist = node.TNodeList{node.NewNodeWord(yyDollar[1].token, nil)}
		}
	case 95:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser/_parser_generated.y:267
		{
			yyVAL.nodelist = append(yyDollar[1].nodelist, node.NewNodeWord(yyDollar[2].token, nil))
		}
	}
	goto yystack /* stack new state and value */
}
