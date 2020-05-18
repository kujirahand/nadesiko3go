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
)
%}

%union{
	token *token.Token // lval *yySymType
	node  node.Node
}

%type<node> program sentences sentence callfunc args 
%type<node> expr value term primary_expr

//__def_token:begin__
%token<token> FUNC EOF LF EOS COMMA NUMBER STRING STRING_EX WORD EQ PLUS MINUS NOT ASTERISK SLASH PERCENT EQEQ NTEQ GT GTEQ LT LTEQ LPAREN RPAREN
//__def_token:end__

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
  : callfunc 
  {
    $$ = $1
  }
  | LF
  {
    $$ = node.NewNodeNop($1)
  }
  | EOS
  {
    $$ = node.NewNodeNop($1)
  }

callfunc
  : args FUNC 
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
	: NUMBER
	{
		$$ = node.NewNodeConst(value.Float, $1)
	}
  | STRING
  {
    $$ = node.NewNodeConst(value.Str, $1)
  }
  | STRING_EX
  {
    $$ = node.NewNodeConst(value.Str, $1)
  }
  | WORD
  {
    $$ = node.NewNodeWord($1)
  }

expr
  : term
  {
    $$ = $1
  }
	| expr PLUS term
	{
		$$ = node.NewNodeOperator("+", $1, $3)
	}
	| expr MINUS term
	{
		$$ = node.NewNodeOperator("-", $1, $3)
	}

term
  : primary_expr
  {
    $$ = $1
  }
  | term ASTERISK primary_expr
  {
		$$ = node.NewNodeOperator("*", $1, $3)
  }
  | term SLASH primary_expr
  {
		$$ = node.NewNodeOperator("/", $1, $3)
  }

primary_expr
  : value
  {
    $$ = $1
  }
  | LPAREN callfunc RPAREN
  {
    $$ = node.NewNodeCalc($3, $2)
  }
  | LPAREN expr RPAREN
  {
    $$ = node.NewNodeCalc($3, $2)
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
  haltError = fmt.Errorf("(%d) %s 理由:" + msg, lineno, desc)
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
  case token.FUNC: return FUNC
  case token.EOF: return EOF
  case token.LF: return LF
  case token.EOS: return EOS
  case token.COMMA: return COMMA
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


