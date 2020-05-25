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
  nodelist  node.NodeList
}

%type<node> program sentences sentence eos callfunc args 
%type<node> expr value comp factor term pri_expr high_expr and_or_expr
%type<node> let_stmt
%type<node> if_stmt if_comp block 
%type<node> repeat_stmt loop_ctrl
%type<node> for_stmt while_stmt comment_stmt
%type<node> def_function def_args
%type<node> json_value variable
%type<jsonkv> json_hash
%type<token> json_key
%type<nodelist> json_array varindex
__TOKENS_LIST__

%%

// --- program ---
program
	: sentences
	{
		$$ = $1
		yylex.(*Lexer).result = $$    
	}

sentences
  : sentence
  {
    n := node.NewNodeSentence($1.GetFileInfo())
    n.Append($1)
    $$ = n
  }
  | sentences sentence
  {
    n, _ := $1.(node.NodeSentence)
    n.Append($2)
    $$ = n
  }

sentence
  : eos
  | callfunc eos
  | expr eos
  | let_stmt eos
  | if_stmt 
  | repeat_stmt 
  | for_stmt 
  | while_stmt
  | loop_ctrl
  | comment_stmt
  | def_function

comment_stmt : COMMENT { $$ = node.NewNodeNop($1) }

eos
  : EOS { $$ = node.NewNodeNop($1) }
  | LF  { $$ = node.NewNodeNop($1) }

let_stmt
  : WORD EQ expr
  {
    $$ = node.NewNodeLet($1, node.NewNodeList(), $3)
  }
  | WORD varindex EQ expr
  {
    $$ = node.NewNodeLet($1, $2, $4)
  }
  | LET_BEGIN WORD_REF expr LET
  {
    $$ = node.NewNodeLet($2, node.NewNodeList(), $3)
  }
  | LET_BEGIN expr WORD_REF LET
  {
    $$ = node.NewNodeLet($3, node.NewNodeList(), $2)
  }

varindex
  : LBRACKET expr RBRACKET
  {
    $$ = node.NodeList{$2}
  }
  | varindex LBRACKET expr RBRACKET
  {
    $$ = append($1, $3)
  }

callfunc
  : FUNC
  {
    $$ = node.NewNodeCallFunc($1)
  }
  | args FUNC
  {
    n := node.NewNodeCallFunc($2)
    n.Args, _ = $1.(node.NodeList)
    $$ = n
  }
  | FUNC LPAREN args RPAREN
  {
    n := node.NewNodeCallFunc($1)
    n.Args, _ = $3.(node.NodeList)
    $$ = n
  }

args
  : expr
  {
    n := node.NodeList{ $1 }
    $$ = n
  }
  | args expr
  {
    args, _ := $1.(node.NodeList)
    n := append(args, $2)
    $$ = n
  }

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
  : and_or_expr

and_or_expr
  : comp
  | and_or_expr AND comp
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | and_or_expr OR comp
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }

comp
  : factor
  | comp EQEQ factor
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | comp NTEQ factor
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | comp GT factor
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | comp GTEQ factor
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | comp LT factor
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | comp LTEQ factor
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }

factor
  : term
  | factor STR_PLUS term
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | factor PLUS term
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | factor MINUS term
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }

term
  : pri_expr
  | term ASTERISK pri_expr
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | term SLASH pri_expr
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | term PERCENT pri_expr
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }

pri_expr
  : high_expr
  | pri_expr CIRCUMFLEX high_expr
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }

high_expr
  : value
  | LPAREN expr RPAREN
  {
    $$ = node.NewNodeCalc($3, $2)
  }

json_value
  : LBRACKET json_array RBRACKET { $$ = node.NewNodeJSONArray($1, $2) }
  | LBRACE json_hash RBRACE      { $$ = node.NewNodeJSONHash($1, $2) }

json_array
  : expr
  {
    $$ = node.NewNodeList()
    $$ = append($$, $1)
  }
  | json_array expr
  {
    $1 = append($1, $2)
    $$ = $1
  }

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
    $$ = node.NewNodeIf($1, $2, $4, node.NewNodeNop($1))
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
    $$ = node.NewNodeIf($1, $2, $4, node.NewNodeNop($1))
  }

if_comp
  : expr 

block
  : sentences 

// --- loop ---
loop_ctrl
  : CONTINUE {
    $$ = node.NewNodeContinue($1, 0)
  }
  | BREAK {
    $$ = node.NewNodeBreak($1, 0)
  }
  | RETURN {
    $$ = node.NewNodeReturn($1, nil, 0)
  }
  | expr RETURN {
    $$ = node.NewNodeReturn($2, $1, 0)
  }

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

def_args
  : WORD
  {
    w := node.NewNodeWord($1, nil)
    nl := node.NewNodeList()
    nl = append(nl, w)
    $$ = nl
  }
  | def_args WORD
  {
    w := node.NewNodeWord($2, nil)
    nl := $1.(node.NodeList)
    nl = append(nl, w)
    $$ = nl
  }

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
