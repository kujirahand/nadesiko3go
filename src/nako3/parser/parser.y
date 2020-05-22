// __TOP_COMMENT__
%{
//
// なでしこ3 --- 文法定義 (goyaccを利用)
// 
// Lexerはgoyaccが要求する形にするため
// nako3/lexerをラップしてこのユニットで使用
//
package parser
import (
	"nako3/core"
	"nako3/value"
	"nako3/node"
	"nako3/lexer"
	"nako3/token"
	"fmt"
  "strings"
)
%}

%union{
	token *token.Token // lval *yySymType
	node  node.Node
}

%type<node> program sentences sentence end_sentence callfunc args 
%type<node> expr value comp factor term primary_expr and_or_expr
%type<node> let_stmt varindex 
%type<node> if_stmt if_comp then_block else_block block 
%type<node> repeat_stmt
%type<node> for_stmt while_stmt
%token<token> __TOKENS_LIST__

%%

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
  : end_sentence
  | callfunc end_sentence
  | expr end_sentence
  | let_stmt end_sentence
  | if_stmt 
  | repeat_stmt 
  | for_stmt 
  | while_stmt 
  | COMMENT
  {
    $$ = node.NewNodeNop($1)
  }

end_sentence
  : EOS
  {
    $$ = node.NewNodeNop($1)
  }
  | LF
  {
    $$ = node.NewNodeNop($1)
  }

let_stmt
  : WORD EQ expr
  {
    $$ = node.NewNodeLet($1, node.NewNodeList(), $3)
  }
  | WORD varindex EQ expr
  {
    n := $2.(node.NodeList)
    $$ = node.NewNodeLet($1, n, $4)
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
    n := node.NodeList{$2}
    $$ = n
  }
  | varindex LBRACKET expr RBRACKET
  {
    n := $1.(node.NodeList)
    $$ = append(n, $3)
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

value
  : NUMBER    { $$ = node.NewNodeConst(value.Float, $1) }
  | STRING    { $$ = node.NewNodeConst(value.Str, $1) }
  | STRING_EX { $$ = node.NewNodeConst(value.Str, $1) }
  | WORD      { $$ = node.NewNodeWord($1) }

expr
  : and_or_expr

and_or_expr
  : comp
  | and_or_expr AND comp
  | and_or_expr OR comp

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
  | factor PLUS term
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | factor MINUS term
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }

term
  : primary_expr
  | term ASTERISK primary_expr
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | term SLASH primary_expr
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }
  | term PERCENT primary_expr
  {
    $$ = node.NewNodeOperator($2, $1, $3)
  }

primary_expr
  : value
  | LPAREN expr RPAREN
  {
    $$ = node.NewNodeCalc($3, $2)
  }

if_stmt
  : IF if_comp then_block else_block
  {
    $$ = node.NewNodeIf($1, $2, $3, $4)
  }
  | IF if_comp then_block
  {
    $$ = node.NewNodeIf($1, $2, $3, node.NewNodeNop($1))
  }

then_block
  : THEN_SINGLE sentence 
  {
    $$ = $2
  }
  | THEN block END
  {
    $$ = $2
  }
else_block
  : ELSE_SINGLE sentence 
  {
    $$ = $2
  }
  | ELSE block END
  {
    $$ = $2
  }

if_comp
  : expr 

block
  : sentences 

repeat_stmt
  : expr KAI_SINGLE sentence
  {
    $$ = node.NewNodeRepeat($2, $1, $3)
  }
  | expr KAI LF block END
  {
    $$ = node.NewNodeRepeat($2, $1, $4)
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

%%

var haltError error = nil

type Lexer struct {
  sys     *core.Core
	lexer   *lexer.Lexer
  tokens  token.Tokens
  index   int
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
  return &lex
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
    v := l.sys.Globals.Get(t.Literal)
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
  haltError = fmt.Errorf("(%d) %s 理由:" + msg, lineno, desc)
}

// 構文解析を実行する
func Parse(sys *core.Core, src string, fno int) (*node.Node, error) {
	l := NewLexerWrap(sys, src, fno)
	if sys.IsDebug {
		yyDebug = 1
    yyErrorVerbose = true
	}
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
