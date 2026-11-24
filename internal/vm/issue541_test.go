package vm_test

import "testing"

func TestIssue541CallRange(t *testing.T) {
	curry := "curry_add = 関数(あ)\n  inner = 関数(い)\n    それはあ + い\n  ここまで\n  innerを戻す\nここまで\n"
	tests := []struct{ code, want string }{
		{"INT(1)…3をJSONエンコードして表示", `{"先頭":1,"末尾":3}`},
		{"N=0;INT(1)…3を繰り返す\nN=N+それ;\nここまで;Nを表示", "6"},
		{"N=0;2の二乗…6を繰り返す\nN=N+それ;\nここまで;Nを表示", "15"},
		{"A=INT(1)…3;AをJSONエンコードして表示", `{"先頭":1,"末尾":3}`},
		{curry + "N=0;curry_add(1)(2)…5を繰り返す\nN=N+それ;\nここまで;Nを表示", "12"},
	}
	for _, tt := range tests {
		if got := run(t, tt.code); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
		}
	}
}
