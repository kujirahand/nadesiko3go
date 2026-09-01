package errs

import "testing"

func TestRuntimeErrorFormat(t *testing.T) {
	err := (&NakoError{Kind: Runtime, File: "main.nako3", Line: 0, Msg: "失敗"}).Error()
	want := "[実行時エラー]main.nako3(1行目): 失敗"
	if err != want {
		t.Fatalf("%q != %q", err, want)
	}
}

func TestCompatType(t *testing.T) {
	cases := map[Kind]string{
		Lexer:   "NakoLexerError",
		Syntax:  "NakoSyntaxError",
		Runtime: "NakoRuntimeError",
	}
	for kind, want := range cases {
		if got := (&NakoError{Kind: kind}).CompatType(); got != want {
			t.Errorf("kind %d: %q != %q", kind, got, want)
		}
	}
}

// TestErrorStripsMainPrefix pins #1223: a name that still carries the main
// module's namespace prefix is shown without it.
func TestErrorStripsMainPrefix(t *testing.T) {
	tests := []struct {
		msg  string
		want string
	}{
		{"不完全な文です。単語『main__未定義関数呼』が解決していません。",
			"不完全な文です。単語『未定義関数呼』が解決していません。"},
		{"関数『main__F』と単語『main__A』", "関数『F』と単語『A』"},
		// メイン以外のモジュールは省略しない
		{"単語『lib__A』", "単語『lib__A』"},
		{"接頭辞がないものはそのまま", "接頭辞がないものはそのまま"},
	}
	for _, tt := range tests {
		e := &NakoError{Kind: Syntax, File: "main.nako3", Line: 0, Msg: tt.msg}
		want := "[文法エラー]main.nako3(1行目): " + tt.want
		if got := e.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	}
}
