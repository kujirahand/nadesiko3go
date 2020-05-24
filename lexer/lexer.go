package lexer

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/runeutil"
	"github.com/kujirahand/nadesiko3go/token"
	"github.com/kujirahand/nadesiko3go/zenhan"
)

// Lexer : Lexer struct
type Lexer struct {
	src        []rune
	index      int
	line       int
	fileNo     int
	autoHalf   bool
	renbunJosi map[string]bool
	tokens     *token.Tokens
	FuncNames  map[string]bool
}

// NewLexer : NewLexer
func NewLexer(source string, fileNo int) *Lexer {
	p := Lexer{}
	p.src = []rune(source)
	p.index = 0
	p.line = 0
	p.fileNo = fileNo
	p.autoHalf = true
	p.FuncNames = map[string]bool{}
	// 連文に使う助詞を初期化
	p.renbunJosi = map[string]bool{}
	for _, josi := range token.JosiRenbun {
		p.renbunJosi[josi] = true
	}
	return &p
}

// NewToken : NewToken
func NewToken(lexer *Lexer, ttype token.TType) *token.Token {
	t := token.Token{}
	t.Type = ttype
	t.Literal = string(ttype)
	t.FileInfo = core.TFileInfo{
		Line:   lexer.line,
		FileNo: lexer.fileNo,
	}
	return &t
}

func (p *Lexer) lastToken() *token.Token {
	if len(*p.tokens) == 0 {
		return nil
	}
	return (*p.tokens)[len(*p.tokens)-1]
}
func (p *Lexer) lastTokenType() token.TType {
	t := p.lastToken()
	if t == nil {
		return token.UNKNOWN
	}
	return t.Type
}

// Split : Split tokens
func (p *Lexer) Split() token.Tokens {
	tt := token.Tokens{}
	p.tokens = &tt
	lastIndex := -1
	for p.isLive() {
		if lastIndex == p.index {
			println("[Lexer error] " + string(p.peek()))
			p.index++
			continue
		} else {
			lastIndex = p.index
		}
		t := p.GetToken()
		if t == nil {
			c := p.next()
			if core.GetSystem().IsDebug {
				fmt.Printf(
					"[警告] (%d) 不明な文字[%c:%d]\n", p.line, c, c)
			}
			continue
		}
		// 連文に対処
		if t.Josi != "" {
			if p.renbunJosi[t.Josi] == true {
				p.line = t.FileInfo.Line
				tt = append(tt, t)
				tt = append(tt, NewToken(p, token.EOS))
				continue
			}
		}
		if t.Type == token.WORD {
			// WORDは → WORD EQ
			if t.Josi == "は" {
				t.Josi = ""
				p.line = t.FileInfo.Line
				tt = append(tt, t)
				tt = append(tt, NewToken(p, token.EQ))
				continue
			}
		}
		// EOS LF → LF
		if t.Type == token.LF {
			if p.lastTokenType() == token.EOS {
				p.lastToken().Type = token.LF
				continue
			}
		}
		// -1が後ろの数値と結びつくか判定
		if t.Type == token.MINUS {
			lt := p.lastTokenType()
			if token.CanUMinus(lt) {
				nt := p.GetToken()
				if nt.Type == token.NUMBER {
					nt.Literal = "-" + nt.Literal
				}
				tt = append(tt, nt)
				continue
			}
		}
		// 行頭のコメント`だけ`トークンに追加
		// その他のコメントは構文解析を邪魔するので。
		// TODO: 将来的に DocTest などに使う
		if t.Type == token.COMMENT {
			if p.lastTokenType() != token.LF {
				continue
			}
		}
		// その他、普通に追加
		tt = append(tt, t)
	}
	// 最後にEOSを足す
	lt := p.lastToken()
	if lt != nil {
		p.line = lt.FileInfo.Line
	}
	tt = append(tt, NewToken(p, token.EOS))

	// goyaccで文法エラー起こさないためにマーカーを入れる
	tt = p.formatTokenList(tt)

	return tt
}

// GetToken : トークンを一つ取得
func (p *Lexer) GetToken() *token.Token {
	// skip space
	p.skipSpaceN()
	c := p.peek()

	// LF
	if c == '\n' {
		p.move(1)
		p.line++
		return NewToken(p, token.LF)
	}

	// flag
	t := p.checkFlagToken(c)
	if t != nil {
		return t
	}
	// number
	if runeutil.IsDigit(c) {
		return p.getNumber()
	}
	// word
	if runeutil.IsWordRune(c) {
		return p.getWord()
	}

	return nil
}

// checkFlagToken : 記号から始まるトークンをチェックする
func (p *Lexer) checkFlagToken(c rune) *token.Token {
	switch c {
	// string
	case '\'':
		p.next()
		return p.getString(c)
	case '"':
		p.next()
		return p.getStringEx(c)
	case '「':
		p.next()
		return p.getStringEx('」')
	case '『':
		p.next()
		return p.getString('」')
	// 不等号
	case '=':
		switch p.peekStr(2) {
		case "==":
			p.move(2)
			return NewToken(p, token.EQEQ)
		case "=>":
			p.move(2)
			return NewToken(p, token.GTEQ)
		case "=<":
			p.move(2)
			return NewToken(p, token.LTEQ)
		}
		p.move(1)
		return NewToken(p, token.EQ)
	case '>':
		switch p.peekStr(2) {
		case ">=":
			p.move(2)
			return NewToken(p, token.GTEQ)
		case "><":
			p.move(2)
			return NewToken(p, token.NTEQ)
		}
		p.move(1)
		return NewToken(p, token.GT)
	case '<':
		switch p.peekStr(2) {
		case "<=":
			p.move(2)
			return NewToken(p, token.LTEQ)
		case "<>":
			p.move(2)
			return NewToken(p, token.NTEQ)
		}
		p.move(1)
		return NewToken(p, token.LT)
	case '≧':
		p.move(1)
		return NewToken(p, token.GTEQ)
	case '≦':
		p.move(1)
		return NewToken(p, token.LTEQ)
	case '!':
		if p.peekStr(2) == "!=" {
			p.move(2)
			return NewToken(p, token.NTEQ)
		}
		p.move(1)
		return NewToken(p, token.NOT)
	// 算術演算子
	case '&':
		p.move(1)
		return NewToken(p, token.PLUS)
	case '+':
		p.move(1)
		return NewToken(p, token.PLUS)
	case '-':
		p.move(1)
		return NewToken(p, token.MINUS)
	case '*':
		p.move(1)
		return NewToken(p, token.ASTERISK)
	case '/':
		// comment
		ch2 := p.peekStr(2)
		if ch2 == "//" {
			p.move(2)
			return p.getLineComment()
		} else if ch2 == "/*" {
			p.move(2)
			return p.getRangeComment()
		}
		p.move(1)
		return NewToken(p, token.SLASH)
	case '%':
		p.move(1)
		return NewToken(p, token.PERCENT)
	case '^':
		p.move(1)
		return NewToken(p, token.CIRCUMFLEX)
	// カッコ
	case '(':
		p.move(1)
		return NewToken(p, token.LPAREN)
	case ')':
		p.move(1)
		rp := NewToken(p, token.RPAREN)
		rp.Josi = p.getJosi(true)
		return rp
	case '{':
		p.move(1)
		return NewToken(p, token.LBRACE)
	case '}':
		p.move(1)
		rp := NewToken(p, token.RBRACE)
		rp.Josi = p.getJosi(true)
		return rp
	case '[':
		p.move(1)
		return NewToken(p, token.LBRACKET)
	case ']':
		p.move(1)
		rp := NewToken(p, token.RBRACKET)
		rp.Josi = p.getJosi(true)
		return rp
	// 句点など
	case '。':
		p.move(1)
		return NewToken(p, token.EOS)
	case ';':
		p.move(1)
		return NewToken(p, token.EOS)
	case ':':
		p.move(1)
		return NewToken(p, token.EOS)
	// その他の記号
	case '●':
		p.move(1)
		return NewToken(p, token.DEF_FUNC)
	}

	return nil
}

// formatTokenList : トークン
func (p *Lexer) formatTokenList(tt token.Tokens) token.Tokens {
	if len(tt) == 0 {
		return tt
	}
	// goyaccのために Markerを挿入
	// WORD(に|へ)exprを代入→LET_BEGIN WORD expr LET
	// 同じく,FOR_BEGINを挿入
	var tWord *token.Token = nil
	i, mk, isMosi := 0, 0, false
	isDefFunc, isParen, funcName := false, false, ""
	p.FuncNames = map[string]bool{}
	nextType := func() token.TType {
		if (i + 1) < len(tt) {
			return tt[i+1].Type
		}
		return token.UNKNOWN
	}
	for i < len(tt) {
		t := tt[i]
		switch t.Type {
		case token.LF, token.EOS:
			mk = i + 1
			if isDefFunc {
				isDefFunc = false
				if funcName != "" {
					p.FuncNames[funcName] = true
					funcName = ""
				}
			}
		case token.WORD:
			if t.Josi == "に" || t.Josi == "へ" {
				tWord = t
			}
			if isDefFunc && !isParen {
				funcName = t.Literal
			}
		case token.LET:
			p.line = t.FileInfo.Line
			tWord.Type = token.WORD_REF
			tt = token.TokensInsert(tt, mk,
				NewToken(p, token.LET_BEGIN))
			i += 2
			continue
		case token.AIDA:
			p.line = t.FileInfo.Line
			tt = token.TokensInsert(tt, mk,
				NewToken(p, token.WHILE_BEGIN))
			i += 2
			continue
		case token.FOR:
			p.line = t.FileInfo.Line
			if nextType() != token.LF {
				t.Type = token.FOR_SINGLE
			}
			// 繰り返し変数が指定されている場合
			tRef := tt[mk]
			if tRef.Type == token.WORD &&
				(tRef.Josi == "を" || tRef.Josi == "で") {
				tRef.Type = token.WORD_REF
			}
			// マーカーを挿入
			tt = token.TokensInsert(tt, mk,
				NewToken(p, token.FOR_BEGIN))
			i += 2
			continue
		case token.KAI:
			if nextType() != token.LF {
				t.Type = token.KAI_SINGLE
			}
			// マーカーを挿入
			p.line = t.FileInfo.Line
			tt = token.TokensInsert(tt, mk,
				NewToken(p, token.KAI_BEGIN))
			i += 2
			continue
		case token.IF:
			isMosi = true
		case token.EQ:
			if isMosi {
				t.Type = token.EQEQ
				t.Literal = "=="
			}
		case token.THEN, token.THEN_SINGLE:
			if nextType() != token.LF {
				t.Type = token.THEN_SINGLE
			}
			isMosi = false
		case token.ELSE:
			if nextType() != token.LF {
				t.Type = token.ELSE_SINGLE
			}
		case token.DEF_FUNC:
			isDefFunc = true
		case token.LPAREN:
			isParen = true
		case token.RPAREN:
			isParen = false
		}
		//
		i++
	}
	return tt
}

// SetAutoHalf : Set AutoHalf
func (p *Lexer) SetAutoHalf(v bool) {
	p.autoHalf = v
}

func (p *Lexer) isLive() bool {
	return (p.index < len(p.src))
}

func (p *Lexer) skipSpace() {
	for p.isLive() {
		c := p.peek()
		if c == ' ' || c == '\t' {
			p.index++
			continue
		}
		break
	}
}

func (p *Lexer) skipSpaceN() {
	for p.isLive() {
		c := p.peek()
		if c == ' ' || c == '\t' || c == '\r' || c == ',' {
			p.move(1)
			continue
		}
		break
	}
}

func (p *Lexer) next() rune {
	if p.isLive() {
		c := p.src[p.index]
		p.index++
		if p.autoHalf {
			c = zenhan.EncodeRune(c)
		}
		return c
	}
	return rune(0)
}

func (p *Lexer) peek() rune {
	if p.isLive() {
		c := p.src[p.index]
		if p.autoHalf {
			return zenhan.EncodeRune(c)
		}
		return c
	}
	return rune(0)
}

func (p *Lexer) peekRaw() rune {
	if p.isLive() {
		c := p.src[p.index]
		return c
	}
	return rune(0)
}

func (p *Lexer) peekStr(n int) string {
	i2 := p.index + n
	if i2 >= len(p.src) {
		i2 = len(p.src)
	}
	s := p.src[p.index:i2]
	if p.autoHalf {
		s = zenhan.EncodeRunes(s)
	}
	return string(s)
}

func (p *Lexer) move(n int) {
	p.index += n
	if p.index < 0 {
		p.index = 0
	}
}

func (p *Lexer) peekCur(i int) rune {
	if (p.index + i) < len(p.src) {
		return p.src[p.index+i]
	}
	return rune(0)
}

// GetStringToRune : endRuneまでの文字列を返す(endRuneは含まない)
func (p *Lexer) GetStringToRune(endRune rune) string {
	s := ""
	for p.isLive() {
		c := p.peekRaw()
		if c == '\n' {
			p.move(1)
			p.line++
			if endRune == c {
				break
			}
			continue
		}
		if c == endRune {
			p.move(1)
			break
		}
		s += string(c)
		p.move(1)
	}
	return s
}

// GetStringToRunes : endRunesのいずれかの文字までの文字列を返す
func (p *Lexer) GetStringToRunes(endRunes []rune) string {
	s := ""
	for p.isLive() {
		c := p.peekRaw()
		if c == '\n' {
			p.move(1)
			p.line++
			if runeutil.HasRune(endRunes, c) {
				break
			}
		}
		if runeutil.HasRune(endRunes, c) {
			p.move(1)
			break
		}
		s += string(c)
		p.move(1)
	}
	return s
}

func (p *Lexer) getLineComment() *token.Token {
	t := NewToken(p, token.COMMENT)
	s := p.GetStringToRune('\n')
	t.Literal = s
	p.line++
	return t
}

func (p *Lexer) getRangeComment() *token.Token {
	t := NewToken(p, token.COMMENT)
	s := ""
	for p.isLive() {
		if p.peekStr(2) == "*/" {
			p.move(2)
			break
		}
		if p.peek() == '\n' {
			p.line++
		}
		s += string(p.next())
	}
	t.Literal = strings.TrimSpace(s)
	return t
}

func (p *Lexer) getString(endRune rune) *token.Token {
	t := NewToken(p, token.STRING)
	t.Literal = p.GetStringToRune(endRune)
	t.Josi = p.getJosi(true)
	return t
}

func replaceStringEx(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if s == "改行" || s == "~" || s == "\\n" {
		return "\n"
	}
	if s == "\\r" {
		return "\r"
	}
	if len(s) == 2 && s[0] == '\\' {
		return string(s[1])
	}
	return "{" + s + "}"
}

func (p *Lexer) getStringEx(endRune rune) *token.Token {
	t := NewToken(p, token.STRING_EX)
	s := ""
	for p.isLive() {
		c := p.peekRaw()
		if c == endRune {
			p.move(1)
			break
		}
		// 基本的な変換以外、文字列の展開は後から行う
		// 但し、{ .. } は半角に揃える
		if c == '{' || c == '｛' {
			p.move(1)
			ss := p.GetStringToRunes([]rune{'}', '｝'})
			s += replaceStringEx(ss)
			continue
		}
		s += string(c)
		p.move(1)
	}
	t.Literal = s
	t.Josi = p.getJosi(true)
	return t
}

// getWord : 単語を得る
func (p *Lexer) getWord() *token.Token {
	t := NewToken(p, token.WORD)

	// 予約語を確認
	for _, rt := range token.ReservedToken {
		rtlen := runeutil.Length(rt)
		if p.peekStr(rtlen) == rt {
			p.move(rtlen)
			t.Literal = rt
			t.Josi = p.getJosi(true)
			t.Type = token.ReplaceWordToken(t.Literal)
			return t
		}
	}

	// 一文字ずつ確認
	s := ""
	s += string(p.next())
	for p.isLive() {
		c := p.peek()

		// check Josi
		if runeutil.IsHiragana(c) {
			josi := p.getJosi(true)
			if josi != "" {
				t.Josi = josi
				break
			}
		}

		// word ...
		if runeutil.IsWordRune(c) {
			s += string(c)
			p.move(1)
			continue
		}
		break
	}
	t.Literal = s

	// 送り仮名を省略
	t.Literal = DeleteOkurigana(t.Literal)
	// replace
	t.Type = token.ReplaceWordToken(t.Literal)
	return t
}

// DeleteOkurigana : 送り仮名を省略
func DeleteOkurigana(s string) string {
	if s == "" {
		return s
	}
	// ひらがなから始まる単語
	ss := []rune(s)
	k := ""
	if runeutil.IsHiragana(ss[0]) {
		// (ex) すごく青い → すごく青
		stat := 0
		for _, c := range ss {
			bHira := runeutil.IsHiragana(c)
			switch stat {
			case 0:
				if bHira { // すごく
					k += string(c)
					continue
				}
				stat = 1
				k += string(c)
				continue
			case 1:
				if !bHira { // 青
					k += string(c)
					continue
				}
				stat = 2
			case 2:
				if bHira {
					continue
				}
				k += string(c)
			}
		}
		return k
	}
	// 漢字カタカナのみ取り出す
	for i, c := range ss {
		c = ss[i]
		if !runeutil.IsHiragana(rune(c)) {
			k += string(c)
		}
	}
	return k
}

func (p *Lexer) getNumber() *token.Token {
	t := NewToken(p, token.NUMBER)
	s := ""
	for p.isLive() {
		c := p.peek()
		if runeutil.IsDigit(c) || c == '.' {
			s += string(c)
			p.move(1)
			continue
		}
		break
	}
	t.Literal = s
	t.Josi = p.getJosi(true)
	return t
}

func (p *Lexer) getJosi(moveCur bool) string {
	for _, j := range token.Josi {
		jLen := utf8.RuneCountInString(j)
		if p.peekStr(jLen) == j {
			if moveCur {
				p.move(jLen)
				return j
			}
		}
	}
	return ""
}
