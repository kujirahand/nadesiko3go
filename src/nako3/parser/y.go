// Code generated by goyacc parser.y. DO NOT EDIT.

//line parser.y:2
//
// なでしこ3 --- 文法定義 (goyaccを利用)
//
// Lexerはgoyaccが要求する形にするため
// nako3/lexerをラップしてこのユニットで使用
//
package parser

import __yyfmt__ "fmt"

//line parser.y:8
import (
	"fmt"
	"nako3/core"
	"nako3/lexer"
	"nako3/node"
	"nako3/token"
	"nako3/value"
)

//line parser.y:19
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
const EQ = 57355
const PLUS = 57356
const MINUS = 57357
const NOT = 57358
const ASTERISK = 57359
const SLASH = 57360
const PERCENT = 57361
const EQEQ = 57362
const NTEQ = 57363
const GT = 57364
const GTEQ = 57365
const LT = 57366
const LTEQ = 57367
const LPAREN = 57368
const RPAREN = 57369

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
}
var yyStatenames = [...]string{}

const yyEofCode = 1
const yyErrCode = 2
const yyInitialStackSize = 16

//line parser.y:156

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
//__getTokenNo:begin__
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

	}
	panic("[SYSTEM ERROR] parser/extract_token.nako3")
}

//__getTokenNo:end__

//line yacctab:1
var yyExca = [...]int{
	-1, 1,
	1, -1,
	-2, 0,
}

const yyPrivate = 57344

const yyLast = 61

var yyAct = [...]int{

	8, 33, 5, 6, 21, 14, 15, 16, 17, 14,
	15, 16, 17, 8, 24, 25, 22, 23, 14, 15,
	16, 17, 13, 19, 9, 11, 13, 35, 14, 15,
	16, 17, 20, 7, 10, 13, 22, 23, 27, 14,
	15, 16, 17, 12, 3, 13, 4, 18, 2, 34,
	31, 32, 1, 20, 0, 28, 13, 29, 30, 0,
	26,
}
var yyPact = [...]int{

	-4, -1000, -4, -1000, -1000, -1000, -1000, 19, -22, 2,
	-3, -1000, -1000, 9, -1000, -1000, -1000, -1000, -1000, -1000,
	2, 30, 30, 30, 30, 30, -26, 22, 0, -3,
	-3, -1000, -1000, -1000, -1000, -1000,
}
var yyPgo = [...]int{

	0, 52, 48, 44, 46, 33, 24, 43, 34, 25,
}
var yyR1 = [...]int{

	0, 1, 2, 2, 3, 3, 3, 4, 4, 5,
	5, 7, 7, 7, 7, 6, 6, 6, 8, 8,
	8, 9, 9, 9,
}
var yyR2 = [...]int{

	0, 1, 1, 2, 1, 1, 1, 2, 4, 1,
	2, 1, 1, 1, 1, 1, 3, 3, 1, 3,
	3, 1, 3, 3,
}
var yyChk = [...]int{

	-1000, -1, -2, -3, -4, 6, 7, -5, 4, -6,
	-8, -9, -7, 26, 9, 10, 11, 12, -3, 4,
	-6, 26, 14, 15, 17, 18, -4, -6, -5, -8,
	-8, -9, -9, 27, 27, 27,
}
var yyDef = [...]int{

	0, -2, 1, 2, 4, 5, 6, 0, 0, 9,
	15, 18, 21, 0, 11, 12, 13, 14, 3, 7,
	10, 0, 0, 0, 0, 0, 0, 9, 0, 16,
	17, 19, 20, 22, 23, 8,
}
var yyTok1 = [...]int{

	1,
}
var yyTok2 = [...]int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 16, 17, 18, 19, 20, 21,
	22, 23, 24, 25, 26, 27,
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
//line parser.y:35
		{
			yyVAL.node = yyDollar[1].node
			yylex.(*Lexer).result = yyVAL.node
		}
	case 2:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.y:42
		{
			n := node.NewNodeSentence(yyDollar[1].node.GetFileInfo())
			n.Append(yyDollar[1].node)
			yyVAL.node = n
		}
	case 3:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser.y:48
		{
			n, _ := yyDollar[1].node.(node.NodeSentence)
			n.Append(yyDollar[2].node)
			yyVAL.node = n
		}
	case 4:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.y:56
		{
			yyVAL.node = yyDollar[1].node
		}
	case 5:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.y:60
		{
			yyVAL.node = node.NewNodeNop(yyDollar[1].token)
		}
	case 6:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.y:64
		{
			yyVAL.node = node.NewNodeNop(yyDollar[1].token)
		}
	case 7:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser.y:70
		{
			n := node.NewNodeCallFunc(yyDollar[2].token)
			n.Args, _ = yyDollar[1].node.(node.NodeList)
			yyVAL.node = n
		}
	case 8:
		yyDollar = yyS[yypt-4 : yypt+1]
//line parser.y:76
		{
			n := node.NewNodeCallFunc(yyDollar[1].token)
			n.Args, _ = yyDollar[3].node.(node.NodeList)
			yyVAL.node = n
		}
	case 9:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.y:84
		{
			n := node.NodeList{yyDollar[1].node}
			yyVAL.node = n
		}
	case 10:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser.y:89
		{
			args, _ := yyDollar[1].node.(node.NodeList)
			n := append(args, yyDollar[2].node)
			yyVAL.node = n
		}
	case 11:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.y:97
		{
			yyVAL.node = node.NewNodeConst(value.Float, yyDollar[1].token)
		}
	case 12:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.y:101
		{
			yyVAL.node = node.NewNodeConst(value.Str, yyDollar[1].token)
		}
	case 13:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.y:105
		{
			yyVAL.node = node.NewNodeConst(value.Str, yyDollar[1].token)
		}
	case 14:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.y:109
		{
			yyVAL.node = node.NewNodeWord(yyDollar[1].token)
		}
	case 15:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.y:115
		{
			yyVAL.node = yyDollar[1].node
		}
	case 16:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.y:119
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 17:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.y:123
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 18:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.y:129
		{
			yyVAL.node = yyDollar[1].node
		}
	case 19:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.y:133
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 20:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.y:137
		{
			yyVAL.node = node.NewNodeOperator(yyDollar[2].token, yyDollar[1].node, yyDollar[3].node)
		}
	case 21:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.y:143
		{
			yyVAL.node = yyDollar[1].node
		}
	case 22:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.y:147
		{
			yyVAL.node = node.NewNodeCalc(yyDollar[3].token, yyDollar[2].node)
		}
	case 23:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.y:151
		{
			yyVAL.node = node.NewNodeCalc(yyDollar[3].token, yyDollar[2].node)
		}
	}
	goto yystack /* stack new state and value */
}
