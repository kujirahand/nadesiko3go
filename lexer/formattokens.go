package lexer

import (
	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/token"
)

// formatTokenList : トークン列を整形する - goyacc対策のため
func (p *Lexer) formatTokenList(tt token.Tokens) token.Tokens {
	if len(tt) == 0 {
		return tt
	}
	man := NewTokensManager(tt)
	p.removeEOS(man)
	p.checkDefFunc(man)
	p.checkWord(man)
	p.insertSyntaxMarker(man)
	p.checkIF(man)
	p.checkBeginFunc(man)
	p.checkLetFunc(man)
	return man.GetTokens()
}

func (p *Lexer) isEOS(t *token.Token) bool {
	if t == nil {
		return false
	}
	if t.Type == token.EOS || t.Type == token.LF {
		return true
	}
	return false
}

// EOSの連続を削除 --- 構文解析の邪魔になるので
func (p *Lexer) removeEOS(f *TokensManager) {
	var last *token.Token = nil
	f.MoveTo(0)
	for f.IsLive() {
		t := f.Peek()
		if p.isEOS(t) && p.isEOS(last) {
			f.Delete(f.GetIndex())
			continue
		}
		last = t
		f.Next()
	}
}

func (p *Lexer) checkLetFunc(f *TokensManager) {
}

func (p *Lexer) checkWord(f *TokensManager) {
	sys := core.GetSystem()
	f.MoveTo(0)
	for f.IsLive() {
		t := f.Peek()
		if t.Type == token.WORD {
			funcName := t.Literal
			uf, ok := p.FuncNames[funcName]
			if ok && uf {
				t.Type = token.FUNC
			} else {
				v := sys.Global.Get(funcName)
				if v == nil {
					f.Next()
					continue
				}
				if v.IsFunction() {
					t.Type = token.FUNC
				}
			}
			if t.Type == token.FUNC && t.Josi != "" {
				t.Type = token.FUNC_JOSI
			}
		}
		f.Next()
	}
}

// checkBeginFunc : 関数定義を調べる
func (p *Lexer) checkBeginFunc(f *TokensManager) {
	// FUNC LPAREN => CFUNC LPAREN
	// ref: insertSyntaxMarker <--- LET(代入)
	f.MoveTo(0)
	for f.IsLive() {
		t := f.Peek()
		// 代入文やもし文での関数呼び出しをチェック
		if t.Type == token.EQ || t.Type == token.IF {
			markerPos := f.GetIndex() + 1
			// 次のトークンに助詞があれば、それは呼び出し
			f.Next() // skip "=" or "もし"
			// カッコがあればその中は飛ばす
			if f.PeekType() == token.LPAREN {
				f.Next() // skip '('
				lv := 1
				for f.IsLive() {
					if f.PeekType() == token.RPAREN {
						lv--
						if lv == 0 {
							break
						}
					} else if f.PeekType() == token.LPAREN {
						lv++
					}
					f.Next()
				}
			}
			// VALUE or ")"
			t = f.Peek()
			if t != nil {
				// もし、FUNC()ならば
				// IF . FUNC ( )
				if t.Josi != "" || f.PeekNextType() == token.LPAREN {
					f.Insert(markerPos, p.newMarker(t, token.BEGIN_CALLFUNC))
					f.MoveTo(2)
					continue
				}
			}
		}
		// EOS + WORD + LPARENの組み合わせは関数呼び出しである
		if t.Type == token.WORD || t.Type == token.FUNC {
			markerPos := f.GetIndex()
			if f.PeekNextType() == token.LPAREN {
				prevT := f.PeekPrevType()
				if prevT == token.UNKNOWN || prevT == token.EOS {
					t2 := f.Get(markerPos)
					if t2.Type != token.BEGIN_CALLFUNC {
						f.Insert(markerPos, p.newMarker(t, token.BEGIN_CALLFUNC))
						f.MoveTo(2)
					}
				}
			}
		}
		f.Next()
	}
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
		// たらればを確認
		if p.tararebaJosi[t.Josi] {
			then := p.newMarker(t, token.THEN)
			then.Literal = t.Josi
			t.Josi = ""
			f.Insert(f.GetIndex()+1, then)
			continue
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
	markerPos := 0
	f.MoveTo(0)
	for f.IsLive() {
		t := f.Peek()
		p.line = t.FileInfo.Line
		switch t.Type {
		case token.LF, token.EOS:
			markerPos = f.GetIndex() + 1
		case token.WORD:
			if t.Josi == "に" || t.Josi == "へ" {
				tLetWord = t
			}
		case token.LET:
			tLetWord.Type = token.WORD_REF
			f.Insert(markerPos, p.newMarker(t, token.LET_BEGIN))
			f.Move(2)
			continue
		case token.AIDA:
			f.Insert(markerPos, p.newMarker(t, token.WHILE_BEGIN))
			f.Move(2)
			markerPos = f.GetIndex()
			continue
		case token.FOREACH:
			// 単文・複文の確認
			if f.PeekNextType() != token.LF {
				t.Type = token.FOREACH_SINGLE
			}
			// マーカーを挿入
			f.Insert(markerPos, p.newMarker(t, token.FOREACH_BEGIN))
			f.Move(2)
			markerPos = f.GetIndex()
			continue
		case token.FOR:
			if f.PeekNextType() != token.LF {
				t.Type = token.FOR_SINGLE
			}
			// 繰り返し文で変数が指定されている場合
			tRef := f.Get(markerPos)
			if tRef.Type == token.WORD &&
				(tRef.Josi == "を" || tRef.Josi == "で") {
				tRef.Type = token.WORD_REF
			}
			// マーカーを挿入
			f.Insert(markerPos, p.newMarker(t, token.FOR_BEGIN))
			f.Move(2)
			markerPos = f.GetIndex()
			continue
		case token.KAI:
			if f.PeekNextType() != token.LF {
				t.Type = token.KAI_SINGLE
			}
			// マーカーを挿入
			f.Insert(markerPos, p.newMarker(t, token.KAI_BEGIN))
			f.Move(2)
			markerPos = f.GetIndex()
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
