// __TOP_COMMENT__
%{
//
// なでしこ3 --- 文法定義 (goyaccを利用)
// 
// Lexerはgoyaccが要求する形にするため
// github.com/kujirahand/nadesiko3go/lexerをラップしてこのユニットで使用
//
package parser
import (
	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/value"
	"github.com/kujirahand/nadesiko3go/node"
	"github.com/kujirahand/nadesiko3go/lexer"
	"github.com/kujirahand/nadesiko3go/token"
	"fmt"
  "strings"
)
%}

%union{
	token     *token.Token // lval *yySymType
	node      node.Node
  jsonkv    node.JSONHashKeyValue
  nodelist  node.TNodeList
}

%type<node> program sentence eos callfunc 
%type<node> expr value 
// comp factor term pri_expr high_expr and_or_expr
%type<node> let_stmt
%type<node> if_stmt if_comp 
%type<node> repeat_stmt loop_ctrl
%type<node> for_stmt while_stmt comment_stmt foreach_stmt
%type<node> def_function
%type<node> json_value variable
%type<jsonkv> json_hash
%type<token> json_key
%type<nodelist> sentences json_array varindex args block def_args
__TOKENS_LIST__

// 演算子の順序
%left AND OR
%left EQEQ NTEQ GT GTEQ LT LTEQ
%left STR_PLUS PLUS MINUS
%left MUL DIV MOD
%left EXP
%%

// --- program ---
program
  : sentences { $$ = $1; yylex.(*Lexer).result = $$ }

sentences
  : sentence            { $$ = node.TNodeList{$1} }
  | sentences sentence  { $$ = append($1, $2)     }

sentence
  : eos
  | callfunc eos
  | expr eos
  | let_stmt
  | if_stmt 
  | repeat_stmt 
  | for_stmt 
  | foreach_stmt
  | while_stmt
  | loop_ctrl
  | comment_stmt
  | def_function

comment_stmt
  : COMMENT { $$ = node.NewNodeNop($1) }

eos
  : EOS { $$ = node.NewNodeNop($1) }
  | LF  { $$ = node.NewNodeNop($1) }

let_stmt
  : WORD EQ expr                { $$ = node.NewNodeLet($1, nil, $3) }
  | WORD varindex EQ expr       { $$ = node.NewNodeLet($1, $2, $4)  }
  | LET_BEGIN WORD_REF expr LET { $$ = node.NewNodeLet($2, nil, $3) }
  | LET_BEGIN expr WORD_REF LET { $$ = node.NewNodeLet($3, nil, $2) }
  | WORD HENSU EQ expr  { $$ = node.NewNodeDefVar($1, $4) }
  | WORD HENSU          { $$ = node.NewNodeDefVar($1, nil) }
  | WORD TEISU EQ expr  { $$ = node.NewNodeDefConst($1, $4) }
  | WORD TEISU          { $$ = node.NewNodeDefConst($1, nil) }

varindex
  : LBRACKET expr RBRACKET          { $$ = node.TNodeList{$2} }
  | varindex LBRACKET expr RBRACKET { $$ = append($1, $3) }

callfunc
  : FUNC                    { $$ = node.NewNodeCallFunc($1, nil) }
  | args FUNC               { $$ = node.NewNodeCallFunc($2, $1) }
  | FUNC LPAREN args RPAREN { $$ = node.NewNodeCallFuncCStyle($1, $3, $4) }

args
  : expr        { $$ = node.TNodeList{ $1 } }
  | args expr   { $$ = append($1, $2)       }

// --- calc ---
value
  : NUMBER    { $$ = node.NewNodeConst(value.Float, $1) }
  | STRING    { $$ = node.NewNodeConst(value.Str, $1) }
  | STRING_EX { $$ = node.NewNodeConstEx(value.Str, $1) }
  | variable
  | json_value

variable
  : WORD          { $$ = node.NewNodeWord($1, nil) }
  | WORD varindex { $$ = node.NewNodeWord($1, $2)  }


expr
  : value
  | expr AND expr       { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr OR expr        { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr EQEQ expr      { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr NTEQ expr      { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr GT expr        { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr GTEQ expr      { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr LT expr        { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr LTEQ expr      { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr PLUS expr      { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr MINUS expr     { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr MUL expr       { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr DIV expr       { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr MOD expr       { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr EXP expr       { $$ = node.NewNodeOperator($2, $1, $3) }
  | expr STR_PLUS expr  { $$ = node.NewNodeOperator($2, $1, $3) }
  | LPAREN expr RPAREN  { $$ = node.NewNodeCalc($3, $2) }
  | LPAREN callfunc RPAREN { $$ = node.NewNodeCalc($3, $2) }
  | BEGIN_CALLFUNC callfunc { $$ = $2 }


json_value
  : LBRACKET json_array RBRACKET { $$ = node.NewNodeJSONArray($1, $2) }
  | LBRACE json_hash RBRACE      { $$ = node.NewNodeJSONHash($1, $2) }

json_array
  : expr            { $$ = node.TNodeList{$1} }
  | json_array expr { $$ = append($1, $2)     }

json_key
  : STRING
  | STRING_EX
  | WORD

json_hash
  :  json_key COLON expr
  {
    kv := node.JSONHashKeyValue {}
    kv[$1.Literal] = $3
    $$ = kv
  }
  | json_hash json_key COLON expr
  {
    $1[$2.Literal] = $4
    $$ = $1
  }

// --- if ---
if_stmt
  : IF if_comp THEN_SINGLE sentence ELSE_SINGLE sentence
  {
    $$ = node.NewNodeIf($1, $2, $4, $6)
  }
  | IF if_comp THEN_SINGLE sentence
  {
    $$ = node.NewNodeIf($1, $2, $4, nil)
  }
  | IF if_comp THEN block ELSE_SINGLE sentence
  {
    $$ = node.NewNodeIf($1, $2, $4, $6)
  }
  | IF if_comp THEN block ELSE block END
  {
    $$ = node.NewNodeIf($1, $2, $4, $6)
  }
  | IF if_comp THEN block END
  {
    $$ = node.NewNodeIf($1, $2, $4, nil)
  }

if_comp
  : expr

block
  : sentences 

// --- loop ---
loop_ctrl
  : CONTINUE    { $$ = node.NewNodeContinue($1, 0) }
  | BREAK       { $$ = node.NewNodeBreak($1, 0) }
  | RETURN      { $$ = node.NewNodeReturn($1, nil, 0) }
  | expr RETURN { $$ = node.NewNodeReturn($2, $1, 0) }

repeat_stmt
  : KAI_BEGIN expr KAI_SINGLE sentence
  {
    $$ = node.NewNodeRepeat($3, $2, $4)
  }
  | KAI_BEGIN expr KAI LF block END
  {
    $$ = node.NewNodeRepeat($3, $2, $5)
  }

while_stmt
  : WHILE_BEGIN expr AIDA block END
  {
    $$ = node.NewNodeWhile($3, $2, $4)
  }

foreach_stmt
  : FOREACH_BEGIN expr FOREACH_SINGLE sentence
  {
    $$ = node.NewNodeForeach($3, $2, $4)
  }
  | FOREACH_BEGIN FOREACH_SINGLE sentence
  {
    $$ = node.NewNodeForeach($2, nil, $3)
  }
  | FOREACH_BEGIN expr FOREACH LF block END
  {
    $$ = node.NewNodeForeach($3, $2, $5)
  }
  | FOREACH_BEGIN FOREACH LF block END
  {
    $$ = node.NewNodeForeach($2, nil, $4)
  }

for_stmt
  : FOR_BEGIN expr expr FOR LF block END
  {
    $$ = node.NewNodeFor($4, "", $2, $3, $6)
  }
  | FOR_BEGIN WORD_REF expr expr FOR LF block END
  {
    $$ = node.NewNodeFor($5, $2.Literal, $3, $4, $7)
  }
  | FOR_BEGIN expr expr FOR_SINGLE sentence
  {
    $$ = node.NewNodeFor($4, "", $2, $3, $5)
  }
  | FOR_BEGIN WORD_REF expr expr FOR_SINGLE sentence
  {
    $$ = node.NewNodeFor($5, $2.Literal, $3, $4, $6)
  }

def_function
  : DEF_FUNC FUNC LF block END
  {
    $$ = node.NewNodeDefFunc($2, node.NewNodeList(), $4)
  }
  | DEF_FUNC LPAREN def_args RPAREN FUNC LF block END
  {
    $$ = node.NewNodeDefFunc($5, $3, $7)
  }
  | DEF_FUNC FUNC LPAREN def_args RPAREN LF block END
  {
    $$ = node.NewNodeDefFunc($2, $4, $7)
  }

def_args
  : WORD          { $$ = node.TNodeList{ node.NewNodeWord($1, nil) } }
  | def_args WORD { $$ = append($1, node.NewNodeWord($2, nil)) }

%%

var haltError error = nil

type Lexer struct {
  sys     *core.Core
	lexer   *lexer.Lexer
  tokens  token.Tokens
  index   int
  loopId  int
  lastToken *token.Token
	result  node.Node
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
  if l.index >= len(l.tokens) { return -1 } // last
  if haltError != nil { return - 1 }
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
  haltError = fmt.Errorf("(%d) %s 理由:" + msg, lineno, desc)
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
__FUNC_GET_TOKEN_NO_CONTNTS__
	}
	panic("[SYSTEM ERROR] parser.y + extract_token.nako3")
	return -1
}
// (メモ) これより下にコードを書かないようにする
// → 行番号が変わらないための対策