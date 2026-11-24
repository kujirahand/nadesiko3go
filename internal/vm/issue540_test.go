package vm_test

import "testing"

func TestIssue540PropertyIndex(t *testing.T) {
	tests := []struct{ code, want string }{
		{"ホゲは{hoge:[72]}。ホゲ$hoge@0を表示。", "72"},
		{"ホゲは{hoge:[72]}。ホゲ$hoge@0=99。ホゲ$hoge@0を表示。", "99"},
		{"ホゲは{hoge:[72]}。99をホゲ$hoge@0に代入。ホゲ$hoge@0を表示。", "99"},
		{"ホゲは{hoge:[72]}。ホゲ$hoge@0を5増やす。ホゲ$hoge@0を表示。", "77"},
		{"ホゲは{a:[{b:5}]}。ホゲ$a@0$b=50。ホゲ$a@0$bを表示。", "50"},
		{"ホゲは{a:{b:[3,4]}}。ホゲ$a$b@1=40。ホゲ$a$b@1を表示。", "40"},
		{"ホゲは{hoge:[72]}。もし、ホゲ$hoge@0=72ならば「OK」と表示。", "OK"},
		{"ホゲは{a:[{b:5}]}。ホゲ$a@0$bを1増やす。ホゲ$a@0$bを表示。", "6"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}
}
