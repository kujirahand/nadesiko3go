package lexer

import (
	"github.com/kujirahand/nadesiko3go/token"
)

// TokensManager : トークン列を整形するために使う構造体
type TokensManager struct {
	tokens token.Tokens
	index  int
	length int
}

// NewTokensManager : トークン列を整形するために使う構造体を初期化
func NewTokensManager(tt token.Tokens) *TokensManager {
	formatter := TokensManager{
		tokens: tt,
		index:  0,
		length: len(tt),
	}
	return &formatter
}

// Insert : トークンを挿入
func (p *TokensManager) Insert(index int, t *token.Token) {
	p.tokens = token.TokensInsert(
		p.tokens,
		index,
		t)
	p.length++
}

// Delete : 削除
func (p *TokensManager) Delete(index int) {
	p.tokens = append(p.tokens[:index], p.tokens[index+1:]...)
	p.length--
}

// IsLive : 残りのトークンがあるか
func (p *TokensManager) IsLive() bool {
	return p.index < p.length
}

// Next : 次のトークンを得る
func (p *TokensManager) Next() *token.Token {
	t := p.tokens[p.index]
	p.index++
	return t
}

// Move : 次のトークンを得る
func (p *TokensManager) Move(i int) {
	p.index += i
}

// MoveTo : 指定の位置にカーソルを移動
func (p *TokensManager) MoveTo(i int) {
	p.index = i
}

// SkipTo : 指定の位置にカーソルを移動
func (p *TokensManager) SkipTo(ttype token.TType) {
	for p.IsLive() {
		t := p.Peek()
		if t.Type == ttype {
			p.Next()
			break
		}
		p.Next()
	}
}

// Peek : 現在のトークンを得る
func (p *TokensManager) Peek() *token.Token {
	if len(p.tokens) <= p.index {
		return nil
	}
	t := p.tokens[p.index]
	return t
}

// PeekType : 現在のトークンのタイプを得る
func (p *TokensManager) PeekType() token.TType {
	t := p.Peek()
	return t.Type
}

// GetIndex : 現在位置を得る
func (p *TokensManager) GetIndex() int {
	return p.index
}

// PeekNext : 次のトークンを得る
func (p *TokensManager) PeekNext() *token.Token {
	if p.index+1 < p.length {
		return p.tokens[p.index+1]
	}
	return nil
}

// PeekNextType : 次のトークンを得る
func (p *TokensManager) PeekNextType() token.TType {
	t := p.PeekNext()
	if t == nil {
		return token.UNKNOWN
	}
	return t.Type
}

// PeekPrevType : 前のトークンタイプを得る
func (p *TokensManager) PeekPrevType() token.TType {
	if p.index <= 0 {
		return token.UNKNOWN
	}
	t := p.Get(p.index - 1)
	if t == nil {
		return token.UNKNOWN
	}
	return t.Type
}

// Get : 指定インデックスのトークンを得る
func (p *TokensManager) Get(index int) *token.Token {
	return p.tokens[index]
}

// GetTokens : トークンリストを返す
func (p *TokensManager) GetTokens() token.Tokens {
	return p.tokens
}
