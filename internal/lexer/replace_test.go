package lexer_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// Expected values come from NakoLexer.replaceTokens in nako_lexer.mts with an
// empty function table. All 239 compat fixture cases plus 184 hand-written
// inputs were checked this way, including the resulting function table.
func TestReplaceTokens(t *testing.T) {
	tests := []struct {
		name string
		code string
		want []tok
	}{
		{
			name: "助詞「は」はeqトークンに展開する",
			code: "Aは1",
			want: []tok{{"word", "A", ""}, {"eq", nil, ""}, {"number", 1.0, ""}},
		},
		{
			name: "助詞「とは」はとはトークンにする",
			code: "Aとは変数",
			want: []tok{{"word", "A", ""}, {"とは", nil, ""}, {"変数", "変数", ""}},
		},
		{
			name: "助詞「ならば」はならばトークンにする",
			code: "もしAならば",
			want: []tok{{"もし", "もし", ""}, {"word", "A", ""}, {"ならば", "ならば", ""}},
		},
		{
			name: "「でなければ」は値が変わる",
			code: "もしAでなければ",
			want: []tok{{"もし", "もし", ""}, {"word", "A", ""}, {"ならば", "でなければ", ""}},
		},
		{
			name: "N回を数値と回に分ける",
			code: "30回",
			want: []tok{{"number", 30.0, ""}, {"回", "回", ""}},
		},
		{
			name: "回繰返は回に置換する #924",
			code: "10回繰返",
			want: []tok{{"number", 10.0, ""}, {"回", "回繰返", ""}},
		},
		{
			name: "永遠に繰返は永遠の間に置換する #1686",
			code: "永遠に繰返",
			want: []tok{{"word", "永遠", "に"}, {"間", "間", "の"}},
		},
		{
			name: "予約語をトークン型に置換する",
			code: "抜ける",
			want: []tok{{"抜ける", "抜", ""}},
		},
		{
			name: "「そう」は「それ」のエイリアス",
			code: "そうを表示",
			want: []tok{{"word", "それ", "を"}, {"word", "表示", ""}},
		},
		{
			name: "予約語の前方一致では置換しない",
			code: "変数A",
			want: []tok{{"word", "変数A", ""}},
		},
		{
			name: "行頭のマイナスは負数になる",
			code: "A=-1",
			want: []tok{{"word", "A", ""}, {"eq", "=", ""}, {"number", -1.0, ""}},
		},
		{
			name: "演算子の後のマイナスは負数になる",
			code: "1*-3",
			want: []tok{{"number", 1.0, ""}, {"*", "*", ""}, {"number", -3.0, ""}},
		},
		{
			name: "助詞付きの語句の後のマイナスは負数になる",
			code: "Aに-3を足す",
			want: []tok{{"word", "A", "に"}, {"number", -3.0, "を"}, {"word", "足", ""}},
		},
		{
			name: "語句の間のマイナスは引き算のまま",
			code: "5-3",
			want: []tok{{"number", 5.0, ""}, {"-", "-", ""}, {"number", 3.0, ""}},
		},
		{
			name: "行末のコメントは改行トークンに埋め込む",
			code: "A=1 #後ろ\nB=2",
			want: []tok{
				{"word", "A", ""}, {"eq", "=", ""}, {"number", 1.0, ""}, {"eol", "#後ろ", ""},
				{"word", "B", ""}, {"eq", "=", ""}, {"number", 2.0, ""},
			},
		},
		{
			name: "アンダースコアと改行は取り除く",
			code: "A_\n+1",
			want: []tok{{"word", "A_", ""}, {"eol", "", ""}, {"+", "+", ""}, {"number", 1.0, ""}},
		},
		{
			name: "「には」は暗黙の無名関数定義になる",
			code: "Fには",
			want: []tok{{"word", "F", "には"}, {"def_func", "関数", ""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustReplace(t, lexer.NewLexer(), tt.code)
			// 末尾の eol と eof は毎回付くので比較から外す
			got = got[:len(got)-2]
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

func TestReplaceTokensAppendsEOLAndEOF(t *testing.T) {
	got := mustReplace(t, lexer.NewLexer(), "A=1")
	if n := len(got); n < 2 ||
		got[n-2].Type != lexer.TypeEOL || got[n-2].Value != "---" || got[n-2].Indent != -1 ||
		got[n-1].Type != lexer.TypeEOF || got[n-1].Indent != -1 {
		t.Errorf("末尾が eol(---) と eof になっていない: %v", summarize(got))
	}
}

func TestPreDefineFunc(t *testing.T) {
	lx := lexer.NewLexer()
	got := mustReplace(t, lx, "●(AとBを)足すとは\nここまで\n1と2を足す")

	fn, ok := lx.FuncList["main__足"]
	if !ok {
		t.Fatalf("関数が登録されていない: %v", lx.FuncList)
	}
	if fn.Type != "func" {
		t.Errorf("Type = %q, want %q", fn.Type, "func")
	}
	if want := []string{"A", "B"}; !equalStrings(fn.VarNames, want) {
		t.Errorf("VarNames = %v, want %v", fn.VarNames, want)
	}
	if len(fn.Josi) != 2 || !equalStrings(fn.Josi[0], []string{"と"}) || !equalStrings(fn.Josi[1], []string{"を"}) {
		t.Errorf("Josi = %v, want [[と] [を]]", fn.Josi)
	}
	// 定義側も呼び出し側も名前空間付きの func になる
	if got[5].Type != lexer.TypeFunc || got[5].StringValue() != "main__足" {
		t.Errorf("定義側 = (%q, %v), want (func, main__足)", got[5].Type, got[5].Value)
	}
	last := got[len(got)-3]
	if last.Type != lexer.TypeFunc || last.StringValue() != "main__足" {
		t.Errorf("呼び出し側 = (%q, %v), want (func, main__足)", last.Type, last.Value)
	}
}

func TestPreDefineFuncExportAttribute(t *testing.T) {
	tests := []struct {
		code string
		want *bool
	}{
		{"●{公開}(Aを)Fとは\nここまで", ptr(true)},
		{"●{エクスポート}(Aを)Fとは\nここまで", ptr(true)},
		{"●{非公開}(Aを)Fとは\nここまで", ptr(false)},
		{"●(Aを)Fとは\nここまで", nil},
	}
	for _, tt := range tests {
		lx := lexer.NewLexer()
		mustReplace(t, lx, tt.code)
		fn := lx.FuncList["main__F"]
		if fn == nil {
			t.Fatalf("%q: 関数が登録されていない", tt.code)
		}
		switch {
		case tt.want == nil && fn.IsExport != nil:
			t.Errorf("%q: IsExport = %v, want nil", tt.code, *fn.IsExport)
		case tt.want != nil && (fn.IsExport == nil || *fn.IsExport != *tt.want):
			t.Errorf("%q: IsExport = %v, want %v", tt.code, fn.IsExport, *tt.want)
		}
	}
}

func TestPreDefineFuncWarnsOnRedefinition(t *testing.T) {
	lx := lexer.NewLexer()
	mustReplace(t, lx, "●Fとは\nここまで\n●Fとは\nここまで")
	if len(lx.Warnings) != 1 {
		t.Fatalf("警告 = %v, want 二重定義の警告1件", lx.Warnings)
	}
	if want := "関数『F』は既に定義されています。"; lx.Warnings[0] != want {
		t.Errorf("警告 = %q, want %q", lx.Warnings[0], want)
	}
}

func TestFilenameToModName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"main.nako3", "main"},
		{"main.nako", "main"},
		{"/path/to/lib.nako3", "lib"},
		{`C:\path\lib.nako3`, "lib"},
		{"plain", "plain"},
		{"", "main"},
	}
	for _, tt := range tests {
		if got := lexer.FilenameToModName(tt.in); got != tt.want {
			t.Errorf("FilenameToModName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func mustReplace(t *testing.T, lx *lexer.Lexer, code string) []lexer.Token {
	t.Helper()
	raw, err := lexer.Tokenize(prepare.Text(prepare.Convert(code)), 0, "main.nako3")
	if err != nil {
		t.Fatalf("Tokenize(%q) failed: %v", code, err)
	}
	got, err := lx.ReplaceTokens(raw, true, "main.nako3")
	if err != nil {
		t.Fatalf("ReplaceTokens(%q) failed: %v", code, err)
	}
	return got
}

func ptr(b bool) *bool { return &b }

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
