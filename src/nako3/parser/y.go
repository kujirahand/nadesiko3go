// Code generated by goyacc _parser_generated.y. DO NOT EDIT.

//line _parser_generated.y:3
//
// なでしこ3 --- 文法定義 (goyaccを利用)
//
// Lexerはgoyaccが要求する形にするため
// nako3/lexerをラップしてこのユニットで使用
//
package parser

import __yyfmt__ "fmt"

//line _parser_generated.y:9
import (
	"fmt"
	"nako3/core"
	"nako3/lexer"
	"nako3/node"
	"nako3/token"
	"nako3/value"
)

//line _parser_generated.y:20
type yySymType struct {
	yys   int
	token *token.Token // lval *yySymType
	node  node.Node
}

const FUNC = 57346
const EOF = 57347
const LF = 57348
const EOS = 57349
const COMMA = 57350
const NUMBER = 57351
const STRING = 57352
const STRING_EX = 57353
const WORD = 57354
const WORD_REF = 57355
const IF = 57356
const THEN = 57357
const ELSE = 57358
const BEGIN = 57359
const END = 57360
const FOR = 57361
const REPEAT = 57362
const FOREACH = 57363
const LET = 57364
const LET_BEGIN = 57365
const EQ = 57366
const PLUS = 57367
const MINUS = 57368
const NOT = 57369
const ASTERISK = 57370
const SLASH = 57371
const PERCENT = 57372
const EQEQ = 57373
const NTEQ = 57374
const GT = 57375
const GTEQ = 57376
const LT = 57377
const LTEQ = 57378
const LPAREN = 57379
const RPAREN = 57380
const LBRACKET = 57381
const RBRACKET = 57382
const LBRACE = 57383
const RBRACE = 57384

var yyToknames = [...]string{
	"$end",
	"error",
	"$unk",
	"FUNC",
	"EOF",
	"LF",
	"EOS",
	"COMMA",
	"NUMBER",
	"STRING",
	"STRING_EX",
	"WORD",
	"WORD_REF",
	"IF",
	"THEN",
	"ELSE",
	"BEGIN",
	"END",
	"FOR",
	"REPEAT",
	"FOREACH",
	"LET",
	"LET_BEGIN",
	"EQ",
	"PLUS",
	"MINUS",
	"NOT",
	"ASTERISK",
	"SLASH",
	"PERCENT",
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

//line _parser_generated.y:237

var haltError error = nil

type Lexer struct {
	sys       *core.Core
	lexer     *lexer.Lexer
	tokens    token.Tokens
	index     int
	lastToken *token.Token
	result    node.Node
}

func NewLexerWrap(sys *core.Core, src string, fileno int) *Lexer {
	haltError = nil
	lex := Lexer{}
	lex.sys = sys
	lex.lexer = lexer.NewLexer(src, fileno)
	lex.tokens = lex.lexer.Split()
	lex.index = 0
	lex.result = nil
	return &lex
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
		v := l.sys.Globals.Get(t.Literal)
		if v != nil && v.Type == value.Function {
			result = FUNC
			t.Type = token.FUNC
		}
	}
	l.lastToken = t
	if l.sys.IsDebug {
		println("- Lex:", t.ToString())
	}
	return result
}

// エラーを報告する
func (l *Lexer) Error(e string) {
	msg := e
	if msg == "syntax error" {
		msg = "文法エラー"
	}
	t := l.lastToken
	lineno := t.FileInfo.Line
	desc := t.ToString()
	haltError = fmt.Errorf("(%d) %s 理由:"+msg, lineno, desc)
}

// 構文解析を実行する
func Parse(sys *core.Core, src string, fno int) (*node.Node, error) {
	l := NewLexerWrap(sys, src, fno)
	yyParse(l)
	if haltError != nil {
		return nil, haltError
	}
	return &l.result, nil
}

// 以下 extract_token.nako3 により自動生成
func getTokenNo(token_type token.TType) int {
	switch token_type {
	case token.FUNC:
		return FUNC
	case token.EOF:
		return EOF
	case token.LF:
		return LF
	case token.EOS:
		return EOS
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
	case token.IF:
		return IF
	case token.THEN:
		return THEN
	case token.ELSE:
		return ELSE
	case token.BEGIN:
		return BEGIN
	case token.END:
		return END
	case token.FOR:
		return FOR
	case token.REPEAT:
		return REPEAT
	case token.FOREACH:
		return FOREACH
	case token.LET:
		return LET
	case token.LET_BEGIN:
		return LET_BEGIN
	case token.EQ:
		return EQ
	case token.PLUS:
		return PLUS
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

const yyLast = 145

var yyAct = [...]int{

	84, 3, 6, 19, 25, 11, 82, 77, 62, 18,
	22, 23, 24, 33, 73, 32, 72, 30, 44, 46,
	12, 49, 50, 63, 52, 79, 40, 78, 91, 11,
	5, 10, 9, 54, 22, 23, 24, 14, 21, 16,
	66, 42, 88, 61, 87, 64, 65, 83, 15, 47,
	48, 53, 51, 70, 71, 67, 32, 68, 69, 17,
	10, 9, 21, 89, 45, 75, 76, 85, 2, 80,
	11, 8, 81, 9, 41, 22, 23, 24, 14, 7,
	16, 22, 23, 24, 33, 86, 13, 25, 20, 15,
	90, 1, 0, 0, 55, 56, 57, 58, 59, 60,
	0, 0, 0, 21, 22, 23, 24, 33, 43, 21,
	74, 31, 22, 23, 24, 33, 22, 23, 24, 33,
	34, 35, 36, 37, 38, 39, 0, 4, 0, 0,
	0, 0, 21, 26, 27, 28, 29, 0, 0, 0,
	21, 0, 0, 0, 21,
}
var yyPact = [...]int{

	25, -1000, 25, -1000, -1000, 54, 54, 54, 54, -1000,
	-1000, -20, 107, 89, 2, 95, 103, 24, -7, -1000,
	-1000, 1, -1000, -1000, -1000, -1000, -1000, -1000, -1000, -1000,
	103, -1000, -1000, -1000, 103, 103, 103, 103, 103, 103,
	103, -16, 103, 103, 27, 40, -1000, 103, 103, 103,
	103, -22, -24, 72, -1000, 24, 24, 24, 24, 24,
	24, -1000, 103, 103, -33, 5, 3, 66, -7, -7,
	-1000, -1000, -1000, -1000, -1000, -1000, -34, -1000, -1000, -1000,
	31, 25, -1000, 25, 26, 25, -1000, -1000, 57, 25,
	10, -1000,
}
var yyPgo = [...]int{

	0, 91, 67, 1, 127, 30, 20, 2, 88, 86,
	59, 9, 3, 79, 74, 71, 64, 0,
}
var yyR1 = [...]int{

	0, 1, 2, 2, 3, 3, 3, 3, 3, 4,
	4, 13, 13, 13, 13, 14, 14, 5, 5, 5,
	6, 6, 8, 8, 8, 8, 7, 9, 9, 9,
	9, 9, 9, 9, 10, 10, 10, 11, 11, 11,
	12, 12, 12, 15, 15, 15, 15, 16, 17,
}
var yyR2 = [...]int{

	0, 1, 1, 2, 1, 2, 2, 2, 2, 1,
	1, 3, 4, 4, 4, 3, 4, 1, 2, 4,
	1, 2, 1, 1, 1, 1, 1, 1, 3, 3,
	3, 3, 3, 3, 1, 3, 3, 1, 3, 3,
	1, 3, 3, 4, 6, 6, 9, 1, 1,
}
var yyChk = [...]int{

	-1000, -1, -2, -3, -4, -5, -7, -13, -15, 7,
	6, 4, -6, -9, 12, 23, 14, -10, -11, -12,
	-8, 37, 9, 10, 11, -3, -4, -4, -4, -4,
	37, 4, -7, 12, 31, 32, 33, 34, 35, 36,
	24, -14, 39, 13, -7, -16, -7, 25, 26, 28,
	29, -5, -7, -6, -7, -10, -10, -10, -10, -10,
	-10, -7, 24, 39, -7, -7, 13, 15, -11, -11,
	-12, -12, 38, 38, 38, -7, -7, 40, 22, 22,
	-3, 6, 40, 16, -17, -2, -3, 18, 16, 6,
	-17, 18,
}
var yyDef = [...]int{

	0, -2, 1, 2, 4, 0, 20, 0, 0, 9,
	10, 17, 0, 26, 25, 0, 0, 27, 34, 37,
	40, 0, 22, 23, 24, 3, 5, 6, 7, 8,
	0, 18, 21, 25, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 47, 0, 0, 0,
	0, 0, 20, 0, 20, 28, 29, 30, 31, 32,
	33, 11, 0, 0, 0, 0, 0, 0, 35, 36,
	38, 39, 41, 42, 19, 12, 0, 15, 13, 14,
	43, 10, 16, 0, 0, 48, 44, 45, 0, 0,
	0, 46,
}
var yyTok1 = [...]int{

	1,
}
var yyTok2 = [...]int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 16, 17, 18, 19, 20, 21,
	22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
	32, 33, 34, 35, 36, 37, 38, 39, 40, 41,
	42,
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
//line _parser_generated.y:34
		{
			yyVAL.node = yyDollar[1].node
			yylex.(*Lexer).result = yyVAL.node
		}
	case 2:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:41
		{
			n := node.NewNodeSentence(yyDollar[1].node.GetFileInfo())
			n.Append(yyDollar[1].node)
			yyVAL.node = n
		}
	case 3:
		yyDollar = yyS[yypt-2 : yypt+1]
//line _parser_generated.y:47
		{
			n, _ := yyDollar[1].node.(node.NodeSentence)
			n.Append(yyDollar[2].node)
			yyVAL.node = n
		}
	case 9:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:62
		{
			yyVAL.node = node.NewNodeNop(yyDollar[1].token)
		}
	case 10:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:66
		{
			yyVAL.node = node.NewNodeNop(yyDollar[1].token)
		}
	case 11:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:72
		{
			yyVAL.node = node.NewNodeLet(yyDollar[1].token, node.NewNodeList(), yyDollar[3].node)
		}
	case 12:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:76
		{
			n := yyDollar[2].node.(node.NodeList)
			yyVAL.node = node.NewNodeLet(yyDollar[1].token, n, yyDollar[4].node)
		}
	case 13:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:81
		{
			yyVAL.node = node.NewNodeLet(yyDollar[2].token, node.NewNodeList(), yyDollar[3].node)
		}
	case 14:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:85
		{
			yyVAL.node = node.NewNodeLet(yyDollar[3].token, node.NewNodeList(), yyDollar[2].node)
		}
	case 15:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:91
		{
			n := node.NodeList{yyDollar[2].node}
			yyVAL.node = n
		}
	case 16:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:96
		{
			n := yyDollar[1].node.(node.NodeList)
			yyVAL.node = append(n, yyDollar[3].node)
		}
	case 17:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:103
		{
			yyVAL.node = node.NewNodeCallFunc(yyDollar[1].token)
		}
	case 18:
		yyDollar = yyS[yypt-2 : yypt+1]
//line _parser_generated.y:107
		{
			n := node.NewNodeCallFunc(yyDollar[2].token)
			n.Args, _ = yyDollar[1].node.(node.NodeList)
			yyVAL.node = n
		}
	case 19:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:113
		{
			n := node.NewNodeCallFunc(yyDollar[1].token)
			n.Args, _ = yyDollar[3].node.(node.NodeList)
			yyVAL.node = n
		}
	case 20:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:121
		{
			n := node.NodeList{yyDollar[1].node}
			yyVAL.node = n
		}
	case 21:
		yyDollar = yyS[yypt-2 : yypt+1]
//line _parser_generated.y:126
		{
			args, _ := yyDollar[1].node.(node.NodeList)
			n := append(args, yyDollar[2].node)
			yyVAL.node = n
		}
	case 22:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:134
		{
			yyVAL.node = node.NewNodeConst(value.Float, yyDollar[1].token)
		}
	case 23:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:138
		{
			yyVAL.node = node.NewNodeConst(value.Str, yyDollar[1].token)
		}
	case 24:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:142
		{
			yyVAL.node = node.NewNodeConst(value.Str, yyDollar[1].token)
		}
	case 25:
		yyDollar = yyS[yypt-1 : yypt+1]
//line _parser_generated.y:146
		{
			yyVAL.node = node.NewNodeWord(yyDollar[1].token)
		}
	case 28:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:156
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 29:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:160
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 30:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:164
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 31:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:168
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 32:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:172
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 33:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:176
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 35:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:183
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 36:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:187
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 38:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:194
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 39:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:198
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 41:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:205
		{
			yyVAL.node = node.NewNodeCalc(yyDollar[3].token, yyDollar[2].node)
		}
	case 42:
		yyDollar = yyS[yypt-3 : yypt+1]
//line _parser_generated.y:209
		{
			yyVAL.node = node.NewNodeCalc(yyDollar[3].token, yyDollar[2].node)
		}
	case 43:
		yyDollar = yyS[yypt-4 : yypt+1]
//line _parser_generated.y:215
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[4].node, node.NewNodeNop(yyDollar[1].token))
		}
	case 44:
		yyDollar = yyS[yypt-6 : yypt+1]
//line _parser_generated.y:219
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[4].node, yyDollar[6].node)
		}
	case 45:
		yyDollar = yyS[yypt-6 : yypt+1]
//line _parser_generated.y:223
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[5].node, node.NewNodeNop(yyDollar[1].token))
		}
	case 46:
		yyDollar = yyS[yypt-9 : yypt+1]
//line _parser_generated.y:227
		{
			yyVAL.node = node.NewNodeIf(yyDollar[1].token, yyDollar[2].node, yyDollar[5].node, yyDollar[8].node)
		}
	}
	goto yystack /* stack new state and value */
}
