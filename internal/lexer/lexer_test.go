package lexer_test

import (
	"errors"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// tok is a compact expectation: type, value, josi.
type tok struct {
	typ   string
	value any
	josi  string
}

// The expected values come from running the same source through
// NakoLexer.tokenize in nako_lexer.mts. All 239 compat fixture cases plus 121
// hand-written inputs were checked this way; the only divergence is the 🟰
// case that prepare already documents.
func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		code string
		want []tok
	}{
		{
			name: "数値と助詞",
			code: "3を表示",
			want: []tok{{"number", 3.0, "を"}, {"word", "表示", ""}},
		},
		{
			name: "代入と演算子",
			code: "A=1+2",
			want: []tok{{"word", "A", ""}, {"eq", "=", ""}, {"number", 1.0, ""}, {"+", "+", ""}, {"number", 2.0, ""}},
		},
		{
			name: "鉤括弧の文字列は展開ありだが埋め込みがなければstringになる",
			code: "「あ」を表示",
			want: []tok{{"string", "あ", "を"}, {"word", "表示", ""}},
		},
		{
			name: "二重鉤括弧の文字列は展開なし",
			code: "『あ』を表示",
			want: []tok{{"string", "あ", "を"}, {"word", "表示", ""}},
		},
		{
			name: "助詞「は」はトークンにせず助詞として持つ",
			code: "Aは1",
			want: []tok{{"word", "A", "は"}, {"number", 1.0, ""}},
		},
		{
			name: "助詞「とは」",
			code: "Aとは変数",
			want: []tok{{"word", "A", "とは"}, {"word", "変数", ""}},
		},
		{
			name: "「ならば」は助詞として読む",
			code: "もし1=1ならば\n「T」と表示\nここまで",
			want: []tok{
				{"もし", "もし", ""}, {"number", 1.0, ""}, {"eq", "=", ""}, {"number", 1.0, "ならば"},
				{"eol", 0, ""},
				{"string", "T", "と"}, {"word", "表示", ""},
				{"eol", 1, ""},
				{"ここまで", "ここまで", ""},
			},
		},
		{
			name: "セミコロンもeolになる",
			code: "A;B",
			want: []tok{{"word", "A", ""}, {"eol", ";", ""}, {"word", "B", ""}},
		},
		{
			name: "展開あり文字列は連結する式に展開される",
			code: "A=30\n「abc{A}abc」を表示",
			want: []tok{
				{"word", "A", ""}, {"eq", "=", ""}, {"number", 30.0, ""}, {"eol", 0, ""},
				{"(", "(", ""},
				{"string", "abc", ""},
				{"&", "&", ""}, {"(", "(", ""}, {"code", "A", ""}, {")", ")", ""}, {"&", "&", ""},
				{"string", "abc", ""},
				{")", ")", "を"},
				{"word", "表示", ""},
			},
		},
		{
			name: "範囲コメントの前後の空白は片方しか落ちない",
			code: "/* コメント */\n1を表示",
			want: []tok{{"range_comment", "コメント ", ""}, {"eol", 0, ""}, {"number", 1.0, "を"}, {"word", "表示", ""}},
		},
		{
			name: "行コメント",
			code: "# コメント\n1を表示",
			want: []tok{{"line_comment", "# コメント", ""}, {"eol", 0, ""}, {"number", 1.0, "を"}, {"word", "表示", ""}},
		},
		{
			name: "16進数",
			code: "0xFFを表示",
			want: []tok{{"number", 255.0, "を"}, {"word", "表示", ""}},
		},
		{
			name: "アンダースコア区切りの数値",
			code: "1_000を表示",
			want: []tok{{"number", 1000.0, "を"}, {"word", "表示", ""}},
		},
		{
			name: "指数表記",
			code: "1.5e3を表示",
			want: []tok{{"number", 1500.0, "を"}, {"word", "表示", ""}},
		},
		{
			name: "CSSの単位が付くと文字列になる #1811",
			code: "10pxを表示",
			want: []tok{{"string", "10px", "を"}, {"word", "表示", ""}},
		},
		{
			name: "単位は読み飛ばす #994",
			code: "10円を表示",
			want: []tok{{"number", 10.0, "を"}, {"word", "表示", ""}},
		},
		{
			name: "送り仮名を省略する",
			code: "置換する",
			want: []tok{{"word", "置換", ""}},
		},
		{
			name: "末尾のひらがなだけ落とす",
			code: "お願いします",
			want: []tok{{"word", "お願", ""}},
		},
		{
			name: "全てひらがなならそのまま",
			code: "どうぞ",
			want: []tok{{"word", "どうぞ", ""}},
		},
		{
			name: "ひらがなに続く「間」は切り離す #831",
			code: "等しい間",
			want: []tok{{"word", "等", ""}, {"word", "間", ""}},
		},
		{
			name: "「システム時間」の「間」は切り離さない #831",
			code: "システム時間を表示",
			want: []tok{{"word", "システム時間", "を"}, {"word", "表示", ""}},
		},
		{
			name: "「以上」は語句から切り離す #918",
			code: "Aが1以上ならば",
			want: []tok{{"word", "A", "が"}, {"number", 1.0, ""}, {"word", "以上", "ならば"}},
		},
		{
			name: "配列リテラル",
			code: "A=[1,2]",
			want: []tok{
				{"word", "A", ""}, {"eq", "=", ""}, {"[", "[", ""},
				{"number", 1.0, ""}, {"comma", ",", ""}, {"number", 2.0, ""}, {"]", "]", ""},
			},
		},
		// 本家の不具合 kujirahand/nadesiko3#2453。単語では助詞が消え、
		// 文字列では「こと」が残る。本家が統一されたら両方 "" になる。
		{
			name: "助詞「ものこと」は単語だと消える",
			code: "Aものこと",
			want: []tok{{"word", "A", ""}},
		},
		{
			name: "助詞「ものこと」は文字列だと「こと」が残る",
			code: "「あ」ものこと",
			want: []tok{{"string", "あ", "こと"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustTokenize(t, tt.code)
			if len(got) != len(tt.want) {
				t.Fatalf("トークン数 = %d, want %d\ngot %v", len(got), len(tt.want), summarize(got))
			}
			for i, w := range tt.want {
				g := got[i]
				if string(g.Type) != w.typ || g.Josi != w.josi || !sameValue(g.Value, w.value) {
					t.Errorf("[%d] = (%q, %#v, %q), want (%q, %#v, %q)",
						i, g.Type, g.Value, g.Josi, w.typ, w.value, w.josi)
				}
			}
		})
	}
}

func TestTokenizeIndentAndLine(t *testing.T) {
	got := mustTokenize(t, "もし1=1ならば\n  「T」と表示\nここまで")
	// インデントは行頭の空白幅、lineは0起点
	want := []struct {
		typ    string
		line   int
		indent int
	}{
		{"もし", 0, 0}, {"number", 0, 0}, {"eq", 0, 0}, {"number", 0, 0},
		{"eol", 0, 0},
		{"string", 1, 2}, {"word", 1, 2},
		{"eol", 1, 2},
		{"ここまで", 2, 0},
	}
	if len(got) != len(want) {
		t.Fatalf("トークン数 = %d, want %d: %v", len(got), len(want), summarize(got))
	}
	for i, w := range want {
		if string(got[i].Type) != w.typ || got[i].Line != w.line || got[i].Indent != w.indent {
			t.Errorf("[%d] = (%q, line=%d, indent=%d), want (%q, %d, %d)",
				i, got[i].Type, got[i].Line, got[i].Indent, w.typ, w.line, w.indent)
		}
	}
}

func TestTokenizeErrors(t *testing.T) {
	tests := []struct {
		name string
		code string
		msg  string
	}{
		{
			// 差分fixture 09_error の「字句解析エラー-文字列の入れ子」
			name: "文字列の入れ子",
			code: "「あ「い」を表示",
			msg:  "『「』で始めた文字列の中に『「』を含めることはできません。",
		},
		{
			// 『 だけは文面が違う
			name: "二重鉤括弧の入れ子",
			code: "『あ『い』",
			msg:  "「『」で始めた文字列の中に「『」を含めることはできません。",
		},
		{
			name: "閉じられていない文字列",
			code: "「閉じない",
			msg:  "『「』で始めた文字列の終端記号『」』が見つかりません。",
		},
		{
			name: "突然の閉じ記号",
			code: "」だけ",
			msg:  "突然の『」』があります。",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := prepare.Text(prepare.Convert(tt.code))
			_, err := lexer.Tokenize(src, 0, "main.nako3")
			if err == nil {
				t.Fatal("エラーになるはずが成功した")
			}
			var ne *errs.NakoError
			if !errors.As(err, &ne) {
				t.Fatalf("エラー型 = %T, want *errs.NakoError", err)
			}
			if ne.Kind != errs.Lexer {
				t.Errorf("Kind = %v, want errs.Lexer", ne.Kind)
			}
			if ne.Msg != tt.msg {
				t.Errorf("Msg = %q, want %q", ne.Msg, tt.msg)
			}
		})
	}
}

func TestTrimOkurigana(t *testing.T) {
	tests := []struct{ in, want string }{
		{"置換する", "置換"},
		{"お願いします", "お願"},
		{"どうぞ", "どうぞ"},
		{"表示", "表示"},
		{"取り出す", "取出"},
	}
	for _, tt := range tests {
		if got := lexer.TrimOkurigana(tt.in); got != tt.want {
			t.Errorf("TrimOkurigana(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func mustTokenize(t *testing.T, code string) []lexer.Token {
	t.Helper()
	src := prepare.Text(prepare.Convert(code))
	got, err := lexer.Tokenize(src, 0, "main.nako3")
	if err != nil {
		t.Fatalf("Tokenize(%q) failed: %v", code, err)
	}
	return got
}

func sameValue(got, want any) bool {
	switch w := want.(type) {
	case float64:
		g, ok := got.(float64)
		return ok && g == w
	case int:
		g, ok := got.(int)
		return ok && g == w
	case string:
		g, ok := got.(string)
		return ok && g == w
	case nil:
		return got == nil
	}
	return false
}

func summarize(tokens []lexer.Token) []string {
	out := make([]string, 0, len(tokens))
	for _, tk := range tokens {
		out = append(out, string(tk.Type))
	}
	return out
}
