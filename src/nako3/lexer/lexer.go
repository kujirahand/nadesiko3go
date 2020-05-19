package lexer

import (
	"nako3/core"
	"nako3/token"
	"nako3/zenhan"
	"strings"
	"unicode/utf8"
)

// Lexer : Lexer struct
type Lexer struct {
	src      []rune
	index    int
	line     int
	fileNo   int
	autoHalf bool
}

// NewLexer : NewLexer
func NewLexer(source string, fileNo int) *Lexer {
	p := Lexer{}
	p.src = []rune(source)
	p.index = 0
	p.line = 0
	p.fileNo = fileNo
	p.autoHalf = true
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

// SetAutoHalf : Set AutoHalf
func (p *Lexer) SetAutoHalf(v bool) {
	p.autoHalf = v
}

// Split : Split tokens
func (p *Lexer) Split() token.Tokens {
	tt := token.Tokens{}
	for p.isLive() {
		t := p.getToken()
		if t == nil {
			continue
		}
		tt = append(tt, t)
	}
	return tt
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

func (p *Lexer) getToken() *token.Token {
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
	if IsDigit(c) {
		return p.getNumber()
	}
	// word
	if IsLetter(c) || c == '_' || IsMultibytes(c) {
		return p.getWord()
	}

	return nil
}

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
		p.move(1)
		return NewToken(p, token.SLASH)
	case '%':
		p.move(1)
		return NewToken(p, token.PERCENT)
	case '(':
		p.move(1)
		return NewToken(p, token.LPAREN)
	case ')':
		p.move(1)
		rp := NewToken(p, token.RPAREN)
		rp.Josi = p.getJosi(true)
		return rp
	case '!':
		if p.peekStr(2) == "!=" {
			p.move(2)
			return NewToken(p, token.NTEQ)
		}
		p.move(1)
		return NewToken(p, token.NOT)
	case '。':
		p.move(1)
		return NewToken(p, token.EOS)
	case ';':
		p.move(1)
		return NewToken(p, token.EOS)
	case ':':
		p.move(1)
		return NewToken(p, token.EOS)
	}

	return nil
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
			if HasRune(endRunes, c) {
				break
			}
		}
		if HasRune(endRunes, c) {
			p.move(1)
			break
		}
		s += string(c)
		p.move(1)
	}
	return s
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

func (p *Lexer) getWord() *token.Token {
	t := NewToken(p, token.WORD)
	s := ""
	s += string(p.next())
	for p.isLive() {
		c := p.peek()

		// check Josi
		if IsHira(c) {
			josi := p.getJosi(true)
			if josi != "" {
				t.Josi = josi
				break
			}
		}

		// word ...
		if IsLetter(c) || c == '_' || IsMultibytes(c) {
			s += string(c)
			p.move(1)
			continue
		}
		break
	}
	t.Literal = DeleteOkurigana(s)
	// 送り仮名を省略
	return t
}

// DeleteOkurigana : 送り仮名を省略
func DeleteOkurigana(s string) string {
	if s == "" {
		return s
	}
	// ひらがなから始まる単語
	ss := []rune(s)
	if IsHira(ss[0]) {
		// (ex) すごく青い → すごく青
		stat := 0
		for j, c := range ss {
			bHira := IsHira(c)
			switch stat {
			case 0:
				if bHira { // すごく
					continue
				}
				stat = 1
				continue
			case 1:
				if !bHira { // 青
					continue
				}
				stat = 2
			case 2:
				return string(ss[0:j])
			}
			if IsHira(c) {
				stat++
			}
		}
		return s
	}
	// 漢字カタカナのみ取り出す
	for i, c := range ss {
		c = ss[i]
		if IsHira(rune(c)) {
			return string(ss[0:i])
		}
	}
	return s
}

func (p *Lexer) getNumber() *token.Token {
	t := NewToken(p, token.NUMBER)
	s := ""
	for p.isLive() {
		c := p.peek()
		if IsDigit(c) || c == '.' {
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
	for _, j := range Josi {
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

// IsLower : Is rune lower case?
func IsLower(c rune) bool {
	return rune('a') <= c && c <= rune('z')
}

// IsUpper : Is rune upper case?
func IsUpper(c rune) bool {
	return rune('A') <= c && c <= rune('Z')
}

// IsLetter : Is rune alphabet
func IsLetter(c rune) bool {
	return IsLower(c) || IsUpper(c)
}

// IsDigit : Is rune Digit?
func IsDigit(c rune) bool {
	return rune('0') <= c && c <= rune('9')
}

// IsFlag : Is rune Flag?
func IsFlag(c rune) bool {
	return rune(0x21) <= c && c <= rune(0x2F) ||
		rune(0x3A) <= c && c <= rune(0x40) ||
		rune(0x5B) <= c && c <= rune(0x60) ||
		rune(0x7B) <= c && c <= rune(0x7E)
}

// IsMultibytes : Is rune multibytes?
func IsMultibytes(c rune) bool {
	return (c > 0xFF)
}

// IsHira : Is rune Hiragana?
func IsHira(c rune) bool {
	return ('ぁ' <= c && c <= 'ん')
}

// HasRune : Has rune in runes?
func HasRune(runes []rune, c rune) bool {
	for _, v := range runes {
		if v == c {
			return true
		}
	}
	return false
}
