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
