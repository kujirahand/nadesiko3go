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
	"os"
  "fmt"
)
%}

%union{
	token *token.Token // lval *yySymType
	node  node.Node
}

%type<node> program sentences sentence callfunc args expr

//__def_token:begin__
%token<token> FUNC EOF LF NUMBER STRING STRING_EX WORD EQ PLUS MINUS NOT ASTERISK SLASH EQEQ NTEQ GT GTEQ LT LTEQ LPAREN RPAREN
//__def_token:end__
%left PLUS MINUS
%left MUL DIV MOD

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
    n := node.NewNodeSentence()
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
  : callfunc 
  {
    $$ = $1
  }
  | LF
  {
    $$ = node.NewNodeNop()
  }

callfunc
  : args FUNC 
  {
    n := node.NewNodeCallFunc($2.Literal)
    n.Args, _ = $1.(node.NodeList)
    $$ = n
  }
  | FUNC LPAREN args RPAREN
  {
    n := node.NewNodeCallFunc($1.Literal)
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

expr
	: NUMBER
	{
		$$ = node.NewNodeConst(value.Float, $1.Literal)
	}
  | STRING
  {
    $$ = node.NewNodeConst(value.Str, $1.Literal)
  }
  | WORD
  {
    $$ = node.NewNodeConst(value.Str, $1.Literal)
  }
  | LPAREN callfunc RPAREN
  {
    $$ = $2
  }
	| expr PLUS expr
	{
		$$ = node.NewNodeOperator("+", $1, $3)
	}
	| expr MUL expr
	{
		$$ = node.NewNodeOperator("*", $1, $3)
	}

%%

type Lexer struct {
  sys     *core.Core
	lexer   *lexer.Lexer
  tokens  token.Tokens
  index   int
	result  node.Node
}

func NewLexerWrap(sys *core.Core, src string, fileno int) *Lexer {
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
  if l.index >= len(l.tokens) { return -1 } // last
  // next
  t := l.tokens[l.index]
  l.index++
  lval.token = t
  // return
  println("-lex:", t.Literal)
  result := getTokenNo(t.Type)
  if result == WORD {
    v := l.sys.GlobalVars.Get(t.Literal)
    if v.Type == value.Function {
      result = FUNC
    }
  }
  return result
}

// エラーを報告する
func (l *Lexer) Error(e string) {
  t := l.tokens[l.index]
	fmt.Fprintln(os.Stderr, e + ":")
  fmt.Fprintln(os.Stderr, "Line ", t.Line, ":", t.Literal)
}

// 構文解析を実行する
func Parse(sys *core.Core, src string, fno int) *node.Node {
	l := NewLexerWrap(sys, src, fno)
	yyParse(l)
	return &l.result
}

// 以下 extract_token.nako3 により自動生成
//__getTokenNo:begin__
func getTokenNo(token_type token.TokenType) int {
  switch token_type {
  case token.FUNC: return FUNC
  case token.EOF: return EOF
  case token.LF: return LF
  case token.NUMBER: return NUMBER
  case token.STRING: return STRING
  case token.STRING_EX: return STRING_EX
  case token.WORD: return WORD
  case token.EQ: return EQ
  case token.PLUS: return PLUS
  case token.MINUS: return MINUS
  case token.NOT: return NOT
  case token.ASTERISK: return ASTERISK
  case token.SLASH: return SLASH
  case token.EQEQ: return EQEQ
  case token.NTEQ: return NTEQ
  case token.GT: return GT
  case token.GTEQ: return GTEQ
  case token.LT: return LT
  case token.LTEQ: return LTEQ
  case token.LPAREN: return LPAREN
  case token.RPAREN: return RPAREN

  }
  panic("[SYSTEM ERROR] parser/extract_token.nako3")
}
//__getTokenNo:end__


