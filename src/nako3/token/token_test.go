package token

import (
	"testing"
)

func TestTokenInsert(t *testing.T) {
	tt := Tokens{}
	t1 := Token{Type: UNKNOWN, Literal: "0"}
	t2 := Token{Type: UNKNOWN, Literal: "1"}
	t3 := Token{Type: UNKNOWN, Literal: "2"}
	t4 := Token{Type: UNKNOWN, Literal: "3"}
	tt = append(tt, &t1)
	tt = append(tt, &t2)
	tt = append(tt, &t3)
	tt = TokensInsert(tt, 1, &t4)
	s := TokensToString(tt, " ")
	if s != "0 3 1 2" {
		t.Fatal("TestTokenInsert")
	}
}
