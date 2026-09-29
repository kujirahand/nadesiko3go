package lexer_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// Issue #216: 「たら」「なら」分岐で正規化した助詞名を RawJosi に代入し、
// 挿入トークンの Offset と元トークンの Length を壊していた。
// RawJosi には生の助詞文字列が入ることを、各形で確認する。
func TestTararebaTokenPositions(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		wantLen    int    // 置換後の語句トークンの Length
		wantNareba int    // 挿入された「ならば」トークンの Offset
		wantValue  string // 挿入トークンの値(正規化名)
	}{
		{"なら", `AならB`, 1, 1, "ならば"},
		{"たら", `AたらB`, 1, 1, "ならば"},
		{"れば", `AればB`, 1, 1, "ならば"},
		{"ならば", `AならばB`, 1, 1, "ならば"},
		{"なければ", `AなければB`, 1, 1, "でなければ"},
		{"でなければ", `AでなければB`, 1, 1, "でなければ"},
		// 助詞の前の空白は語句トークンの長さに残る(本家と同じ)
		{"空白+たら", `A たらB`, 2, 2, "ならば"},
		// 「もの」構文では正規化後の助詞の先頭を指す(本家と同じ)
		{"ものなら", `AものならB`, 3, 3, "ならば"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustReplace(t, lexer.NewLexer(), tt.code)
			got = got[:len(got)-2] // 末尾の eol と eof を外す
			if len(got) != 3 {
				t.Fatalf("トークン数 = %d, want 3\n%v", len(got), summarize(got))
			}
			word, naraba := got[0], got[1]
			if word.Type != lexer.TypeWord || naraba.Type != "ならば" {
				t.Fatalf("トークン列 = %v", summarize(got))
			}
			if word.Offset != 0 || word.Length != tt.wantLen {
				t.Errorf("語句トークンの位置 = (Offset:%d Length:%d), want (Offset:0 Length:%d)",
					word.Offset, word.Length, tt.wantLen)
			}
			if naraba.Offset != tt.wantNareba {
				t.Errorf("ならばトークンの Offset = %d, want %d", naraba.Offset, tt.wantNareba)
			}
			if naraba.StringValue() != tt.wantValue {
				t.Errorf("ならばトークンの値 = %q, want %q", naraba.StringValue(), tt.wantValue)
			}
		})
	}
}

// Issue #216 の再現コード。数値トークン「1」の Length と、挿入された
// 「ならば」トークンの Offset が壊れないことを確認する。
func TestTararebaNumberPosition(t *testing.T) {
	got := mustReplace(t, lexer.NewLexer(), `値が1なら「大」を表示`)
	got = got[:len(got)-2]
	// word(値) number(1) ならば string(大) word(表示) の5トークン
	if len(got) != 5 {
		t.Fatalf("トークン数 = %d, want 5\n%v", len(got), summarize(got))
	}
	num := got[1]
	if num.Type != lexer.TypeNumber {
		t.Fatalf("[1] の型 = %q, want number\n%v", num.Type, summarize(got))
	}
	if num.Offset != 2 || num.Length != 1 {
		t.Errorf("数値トークンの位置 = (Offset:%d Length:%d), want (Offset:2 Length:1)",
			num.Offset, num.Length)
	}
	naraba := got[2]
	if naraba.Type != "ならば" || naraba.Offset != 3 {
		t.Errorf("ならばトークン = (%q Offset:%d), want (ならば Offset:3)", naraba.Type, naraba.Offset)
	}
}

// 「は」「とは」分岐と同じく、元トークンの Josi/RawJosi は挿入後に空になる。
func TestTararebaClearsJosi(t *testing.T) {
	got := mustReplace(t, lexer.NewLexer(), `AならB`)
	got = got[:len(got)-2]
	if got[0].Josi != "" || got[0].RawJosi != "" {
		t.Errorf("語句トークンの助詞 = (Josi:%q RawJosi:%q), want 空", got[0].Josi, got[0].RawJosi)
	}
}

// 本家はトークン化の時点で rawJosi に書かれた助詞を保持する。
// replaceWord の `if t.RawJosi == ""` フォールバックが正しく働くには、
// Tokenize が生の助詞を入れておく必要がある。
func TestRawJosiKeptFromTokenize(t *testing.T) {
	raw, err := lexer.Tokenize(prepare.Text(prepare.Convert(`AをB`)), 0, "main.nako3")
	if err != nil {
		t.Fatal(err)
	}
	if raw[0].Type != lexer.TypeWord || raw[0].Josi != "を" {
		t.Fatalf("[0] = (%q, josi:%q), want (word, を)", raw[0].Type, raw[0].Josi)
	}
	if raw[0].RawJosi != "を" {
		t.Errorf("RawJosi = %q, want %q", raw[0].RawJosi, "を")
	}
}
