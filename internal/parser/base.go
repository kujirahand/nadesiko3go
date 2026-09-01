// Package parser turns a token stream into a syntax tree
// (nako_parser3.mts / nako_parser_base.mts equivalent).
package parser

import (
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

// VarInfo records what a name refers to while parsing.
type VarInfo struct {
	Type  string // "var" / "const" / "func"
	Value any
}

// VarScope says where a name was found.
type VarScope string

const (
	ScopeLocal  VarScope = "local"
	ScopeGlobal VarScope = "global"
	ScopeSystem VarScope = "system"
)

// FoundVar is the result of looking a name up.
type FoundVar struct {
	Name  string
	Scope VarScope
	Func  *lexer.FuncItem // グローバル・システムのとき
	Local *VarInfo        // ローカルのとき
}

// Parser holds the cursor, the calculation stack, and the name tables.
type Parser struct {
	tokens []lexer.Token
	index  int

	// stack holds values waiting to be consumed as function arguments.
	// Only pushStack and popStack touch it.
	stack []*ast.Node
	// stackList saves and restores stack across a function definition.
	stackList [][]*ast.Node
	// y holds what accept matched, for the rule that called it.
	y []any

	ModName        string
	ModList        []string
	NamespaceStack []string
	FuncList       lexer.FuncList
	ModuleExport   map[string]bool
	UsedFuncs      map[string]bool

	localvars map[string]*VarInfo
	funcLevel int

	usedAsyncFn bool
	// GenMode selects the code generator (#637). "sync" unless 非同期モード.
	GenMode string

	// arrayIndexFrom and the flags below are set by DNCLモード (#1140).
	arrayIndexFrom        int
	flagReverseArrayIndex bool
	flagCheckArrayInit    bool
	// flagNoPostfixIndex is true while reading the index of 『@』 (#2396).
	// It stops a value from absorbing a following 『@』『[』『$』, so that
	// 『A@B@C』 reads as 『A[B][C]』 rather than 『A[B[C]]』.
	flagNoPostfixIndex bool

	// recentlyCalledFunc is used when reporting leftover stack values.
	recentlyCalledFunc []*lexer.FuncItem
	isReadingCalc      bool
	isExportDefault    bool
	isExportStack      []bool

	// originalVarNames remembers a word token's name before namespace
	// resolution rewrote it, so an assignment can look the original up again.
	// The keys are pointers into tokens, which does not move during a parse.
	originalVarNames map[*lexer.Token]string

	// Warnings collects messages the TypeScript version logs instead of raising.
	Warnings []string
}

func New() *Parser {
	p := &Parser{}
	p.reset()
	p.FuncList = lexer.FuncList{}
	p.ModuleExport = map[string]bool{}
	return p
}

func (p *Parser) reset() {
	p.tokens = nil
	p.index = 0
	p.stack = nil
	p.y = nil
	p.GenMode = "sync" // #637, #1056
	// 次回実行時に持ち越されないように初期化する (#1746)
	p.localvars = map[string]*VarInfo{"それ": {Type: "var", Value: ""}}
	p.UsedFuncs = map[string]bool{}
	p.funcLevel = 0
	p.usedAsyncFn = false
	p.isReadingCalc = false
	p.isExportDefault = true
	p.isExportStack = nil
	p.NamespaceStack = nil
	p.ModList = nil
	p.stackList = nil
	p.recentlyCalledFunc = nil
	p.arrayIndexFrom = 0
	p.flagReverseArrayIndex = false
	p.flagCheckArrayInit = false
	p.flagNoPostfixIndex = false
}

// SetFuncList installs the function table the lexer built.
func (p *Parser) SetFuncList(fl lexer.FuncList) { p.FuncList = fl }

// SetModuleExport installs each module's export default.
func (p *Parser) SetModuleExport(m map[string]bool) { p.ModuleExport = m }

// --- スタック操作 ---

// popStack takes one value off the calculation stack. With josiList it takes
// the topmost value whose particle is in the list; an empty list matches any
// particle. It returns nil when nothing matches.
func (p *Parser) popStack(josiList []string) *ast.Node {
	if josiList == nil {
		if n := len(p.stack); n > 0 {
			t := p.stack[n-1]
			p.stack = p.stack[:n-1]
			return t
		}
		return nil
	}
	for i := len(p.stack) - 1; i >= 0; i-- {
		t := p.stack[i]
		if len(josiList) == 0 || containsString(josiList, t.Josi) {
			p.stack = append(p.stack[:i], p.stack[i+1:]...)
			return t
		}
	}
	return nil
}

func (p *Parser) pushStack(item *ast.Node) { p.stack = append(p.stack, item) }

// saveStack and loadStack are used in pairs so that a function definition does
// not disturb the stack of the code around it.
func (p *Parser) saveStack() {
	p.stackList = append(p.stackList, p.stack)
	p.stack = nil
}

func (p *Parser) loadStack() {
	if n := len(p.stackList); n > 0 {
		p.stack = p.stackList[n-1]
		p.stackList = p.stackList[:n-1]
	}
}

// --- 名前解決 ---

// FindVar looks a name up as a local, then as a global in this module, then in
// the imported modules, and finally as a system name.
func (p *Parser) FindVar(name string) *FoundVar {
	if info, ok := p.localvars[name]; ok {
		return &FoundVar{Name: name, Scope: ScopeLocal, Local: info}
	}
	// モジュール名を含んでいるならそれ以上は探さない
	if strings.Contains(name, "__") {
		if fo, ok := p.FuncList[name]; ok {
			return &FoundVar{Name: name, Scope: ScopeGlobal, Func: fo}
		}
		return nil
	}
	// グローバル変数(自身のモジュール)
	gnameSelf := p.ModName + "__" + name
	if fo, ok := p.FuncList[gnameSelf]; ok {
		return &FoundVar{Name: gnameSelf, Scope: ScopeGlobal, Func: fo}
	}
	// グローバル変数(取り込んだモジュール)
	for _, mod := range p.ModList {
		gname := mod + "__" + name
		fo, ok := p.FuncList[gname]
		if !ok || !p.exported(fo, mod) {
			continue
		}
		return &FoundVar{Name: gname, Scope: ScopeGlobal, Func: fo}
	}
	// システム変数
	if fo, ok := p.FuncList[name]; ok {
		return &FoundVar{Name: name, Scope: ScopeSystem, Func: fo}
	}
	return nil
}

func (p *Parser) exported(fo *lexer.FuncItem, mod string) bool {
	if fo.IsExport != nil {
		return *fo.IsExport
	}
	def, ok := p.ModuleExport[mod]
	return !ok || def
}

// --- カーソル操作 ---

func (p *Parser) isEOF() bool { return p.index >= len(p.tokens) }

// check reports whether the token under the cursor has this type.
func (p *Parser) check(t lexer.TokenType) bool {
	if p.isEOF() {
		return false
	}
	return p.tokens[p.index].Type == t
}

// checkTypes reports whether the token under the cursor has any of these types.
func (p *Parser) checkTypes(types []lexer.TokenType) bool {
	if p.isEOF() {
		return false
	}
	return containsType(types, p.tokens[p.index].Type)
}

// check2 matches several token types starting at the cursor. A nil entry is a
// wildcard, and an entry with several types matches any one of them.
func (p *Parser) check2(seq [][]lexer.TokenType) bool {
	for i, want := range seq {
		idx := i + p.index
		if idx >= len(p.tokens) {
			return false
		}
		if want == nil { // ワイルドカード
			continue
		}
		if !containsType(want, p.tokens[idx].Type) {
			return false
		}
	}
	return true
}

// get returns the token under the cursor and advances past it.
func (p *Parser) get() *lexer.Token {
	if p.isEOF() {
		return nil
	}
	t := &p.tokens[p.index]
	p.index++
	return t
}

func (p *Parser) unget() {
	if p.index > 0 {
		p.index--
	}
}

// peek returns the token i places ahead of the cursor without moving it.
func (p *Parser) peek(offset int) *lexer.Token {
	if p.isEOF() {
		return nil
	}
	idx := p.index + offset
	if idx < 0 || idx >= len(p.tokens) {
		return nil
	}
	return &p.tokens[idx]
}

// peekDef returns the token under the cursor, or an empty token at EOF.
func (p *Parser) peekDef() *lexer.Token {
	if t := p.peek(0); t != nil {
		return t
	}
	return &lexer.Token{Type: "?", Value: "", Indent: -1}
}

// peekSourceMap gives the position of tok, or of the token under the cursor.
func (p *Parser) peekSourceMap(tok *lexer.Token) ast.SourceMap {
	if tok == nil {
		tok = p.peek(0)
	}
	if tok == nil {
		return ast.SourceMap{}
	}
	return ast.SourceMap{
		Line: tok.Line, Column: tok.Column, File: tok.File,
		Offset: tok.Offset, Length: tok.Length,
	}
}

// --- accept ---

// matcher tests one position in an accept sequence. It returns what it matched
// and whether it matched at all.
type matcher func(p *Parser) (any, bool)

// tokenType matches one token of exactly this type.
func tokenType(t lexer.TokenType) matcher {
	return func(p *Parser) (any, bool) {
		tok := p.get()
		if tok == nil || tok.Type != t {
			return nil, false
		}
		return tok, true
	}
}

// anyOf matches one token whose type is in the list.
func anyOf(types ...lexer.TokenType) matcher {
	return func(p *Parser) (any, bool) {
		if !p.checkTypes(types) {
			return nil, false
		}
		return p.get(), true
	}
}

// rule matches whatever a syntax rule accepts. A nil result means no match.
func rule(f func() *ast.Node) matcher {
	return func(*Parser) (any, bool) {
		n := f()
		if n == nil {
			return nil, false
		}
		return n, true
	}
}

// accept runs the matchers in order. If any of them fails it rewinds the cursor
// and reports false, leaving p.y untouched.
func (p *Parser) accept(matchers ...matcher) bool {
	saved := p.index
	y := make([]any, len(matchers))
	for i, m := range matchers {
		if p.isEOF() || m == nil {
			p.index = saved
			return false
		}
		got, ok := m(p)
		if !ok {
			p.index = saved
			return false
		}
		y[i] = got
	}
	p.y = y
	return true
}

// yToken returns the token accept matched at position i.
func (p *Parser) yToken(i int) *lexer.Token {
	if i < 0 || i >= len(p.y) {
		return nil
	}
	t, _ := p.y[i].(*lexer.Token)
	return t
}

// yNode returns the node accept matched at position i.
func (p *Parser) yNode(i int) *ast.Node {
	if i < 0 || i >= len(p.y) {
		return nil
	}
	n, _ := p.y[i].(*ast.Node)
	return n
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func containsType(list []lexer.TokenType, t lexer.TokenType) bool {
	for _, v := range list {
		if v == t {
			return true
		}
	}
	return false
}
