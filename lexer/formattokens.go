package lexer

import (
	"github.com/kujirahand/nadesiko3go/token"
)

// formatTokenList : トークン列を整形する - goyacc対策のため
func (p *Lexer) formatTokenList(tt token.Tokens) token.Tokens {
	if len(tt) == 0 {
		return tt
	}
	formatter := NewFormatter(tt)
	p.insertSyntaxMarker(formatter)
	p.checkIF(formatter)
	p.checkDefFunc(formatter)
	return formatter.GetTokens()
}

// checkDefFunc : 関数定義を調べる
func (p *Lexer) checkDefFunc(f *TokensManager) {
	isDefFunc, isParen := false, false
	funcName := ""
	p.FuncNames = map[string]bool{}

	f.MoveTo(0)
	for f.IsLive() {
		t := f.Peek()
		switch t.Type {
		case token.LF, token.EOS:
			if isDefFunc {
				isDefFunc = false
				if funcName != "" {
					p.FuncNames[funcName] = true
					funcName = ""
				}
			}
		case token.WORD:
			if isDefFunc && !isParen {
				funcName = t.Literal
			}
		case token.DEF_FUNC:
			isDefFunc = true
		case token.LPAREN:
			isParen = true
		case token.RPAREN:
			isParen = false
		}
		f.Next()
	}
}

// checkIF : check if syntax
func (p *Lexer) checkIF(f *TokensManager) {
	isMosi := false

	f.MoveTo(0)
	for f.IsLive() {
		t := f.Peek()
		switch t.Type {
		case token.IF:
			isMosi = true
		case token.EQ:
			if isMosi {
				t.Type = token.EQEQ
				t.Literal = "=="
			}
		case token.THEN, token.THEN_SINGLE:
			if f.PeekNextType() != token.LF {
				t.Type = token.THEN_SINGLE
			}
			isMosi = false
		case token.ELSE:
			if f.PeekNextType() != token.LF {
				t.Type = token.ELSE_SINGLE
			}
		}
		f.Next()
	}
}

// insertSyntaxMarker : 構文の開始位置にマーカーを追加する
func (p *Lexer) insertSyntaxMarker(f *TokensManager) {
	// goyaccのために、構文の開始位置にMarkerを挿入
	//		WORD(に|へ)exprを代入→LET_BEGIN WORD expr LET
	// 		同じく,FOR_BEGINなど、を挿入
	var tLetWord *token.Token = nil
	makerPos := 0
	f.MoveTo(0)
	for f.IsLive() {
		t := f.Peek()
		p.line = t.FileInfo.Line
		switch t.Type {
		case token.LF, token.EOS:
			makerPos = f.GetIndex() + 1
		case token.WORD:
			if t.Josi == "に" || t.Josi == "へ" {
				tLetWord = t
			}
		case token.LET:
			tLetWord.Type = token.WORD_REF
			f.Insert(makerPos, p.newMarker(t, token.LET_BEGIN))
			f.Move(2)
			continue
		case token.AIDA:
			f.Insert(makerPos, p.newMarker(t, token.WHILE_BEGIN))
			f.Move(2)
			continue
		case token.FOR:
			if f.PeekNextType() != token.LF {
				t.Type = token.FOR_SINGLE
			}
			// 繰り返し文で変数が指定されている場合
			tRef := f.Get(makerPos)
			if tRef.Type == token.WORD &&
				(tRef.Josi == "を" || tRef.Josi == "で") {
				tRef.Type = token.WORD_REF
			}
			// マーカーを挿入
			f.Insert(makerPos, p.newMarker(t, token.FOR_BEGIN))
			f.Move(2)
			continue
		case token.KAI:
			if f.PeekNextType() != token.LF {
				t.Type = token.KAI_SINGLE
			}
			// マーカーを挿入
			f.Insert(makerPos, p.newMarker(t, token.KAI_BEGIN))
			f.Move(2)
			continue
		}
		f.Next()
	}
}

func (p *Lexer) newMarker(base *token.Token, typ token.TType) *token.Token {
	p.line = base.FileInfo.Line
	nt := NewToken(p, typ)
	return nt
}
