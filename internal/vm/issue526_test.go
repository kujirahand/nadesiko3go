package vm_test

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// 「ずっと（比較式）の間…ここまで」構文のテスト (#526 / 本家 #2694)
func TestIssue526ZuttoWhile(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{
			name: "基本と等号比較(Issue発端ケース)",
			code: "●試すとは\n" +
				"　済み = オフ\n" +
				"　N = 0\n" +
				"　ずっと、済み = オフ の間\n" +
				"　　N = N + 1\n" +
				"　　もし、N >= 3 ならば\n" +
				"　　　済み = オン\n" +
				"　　ここまで\n" +
				"　ここまで\n" +
				"　Nを表示\n" +
				"ここまで\n" +
				"試す",
			want: "3",
		},
		{
			name: "スペース・読点なしと全角イコール",
			code: "N=5\n" +
				"ずっとN＝5の間\n" +
				"　Nを表示\n" +
				"　N=N+1\n" +
				"ここまで\n" +
				"「OK」と表示",
			want: "5\nOK",
		},
		{
			name: "等号比較 ==",
			code: "N=1\nずっと N == 1 の間\nNを表示\nN=2\nここまで",
			want: "1",
		},
		{
			name: "厳密等号比較 ===",
			code: "N=1\nずっと N === 1 の間\nNを表示\nN=2\nここまで",
			want: "1",
		},
		{
			name: "括弧付き条件",
			code: "N=1\nずっと(N = 1)の間\nNを表示\nN=2\nここまで",
			want: "1",
		},
		{
			name: "末尾: 間繰り返す",
			code: "N=5\nずっとN＝5の間繰り返す\nNを表示\nN=N+1\nここまで",
			want: "5",
		},
		{
			name: "末尾: 間、繰り返す",
			code: "N=5\nずっとN＝5の間、繰り返す\nNを表示\nN=N+1\nここまで",
			want: "5",
		},
		{
			name: "末尾: 間、ここから",
			code: "N=5\nずっとN＝5の間、ここから\nNを表示\nN=N+1\nここまで",
			want: "5",
		},
		{
			name: "末尾: 間ここから",
			code: "N=5\nずっとN＝5の間ここから\nNを表示\nN=N+1\nここまで",
			want: "5",
		},
		{
			name: "複合条件(かつ)",
			code: "A=1;B=2;C=0\n" +
				"ずっと A = 1 かつ B = 2 の間\n" +
				"　C=C+1\n" +
				"　もし C = 2 ならば A = 0\n" +
				"ここまで\n" +
				"Cを表示",
			want: "2",
		},
		{
			name: "複合条件(または)",
			code: "A=1;B=0;C=0\n" +
				"ずっと A = 1 または B = 1 の間\n" +
				"　C=C+1\n" +
				"　もし C = 1 ならば\n" +
				"　　A = 0; B = 1\n" +
				"　違えば\n" +
				"　　B = 0\n" +
				"　ここまで\n" +
				"ここまで\n" +
				"Cを表示",
			want: "2",
		},
		{
			name: "大小比較演算子",
			code: "N=0\nずっと N < 3 の間\nNを表示\nN=N+1\nここまで",
			want: "0\n1\n2",
		},
		{
			name: "C風関数呼び出し",
			code: "X=0\nずっと INT(X) = 0 の間\nX=1\n「OK」を表示\nここまで",
			want: "OK",
		},
		{
			name: "日本語比較(未満)",
			code: "i=0\nずっと iが3未満の間\niを表示\ni=i+1\nここまで",
			want: "0\n1\n2",
		},
		{
			name: "日本語比較(等しい)",
			code: "a=1;i=0\n" +
				"ずっと aと1が等しい間\n" +
				"　iを表示\n" +
				"　i=i+1\n" +
				"　もし i >= 2 ならば a = 0\n" +
				"ここまで",
			want: "0\n1",
		},
		{
			name: "抜ける・続ける",
			code: "A=1;N=0\n" +
				"ずっと A = 1 の間\n" +
				"　N=N+1\n" +
				"　もし N = 2 ならば 続ける\n" +
				"　Nを表示\n" +
				"　もし N = 3 ならば 抜ける\n" +
				"ここまで",
			want: "1\n3",
		},
		{
			name: "単一変数を条件にする",
			code: "フラグ=オン;N=0\n" +
				"ずっと フラグの間\n" +
				"　N=N+1\n" +
				"　フラグ=オフ\n" +
				"ここまで\n" +
				"Nを表示",
			want: "1",
		},
		{
			name: "ずっとの入れ子",
			code: "A=1;N=0\n" +
				"ずっと A = 1 の間\n" +
				"　B=1\n" +
				"　ずっと B = 1 の間\n" +
				"　　B=0\n" +
				"　　N=N+1\n" +
				"　ここまで\n" +
				"　A=0\n" +
				"ここまで\n" +
				"Nを表示",
			want: "1",
		},
		{
			name: "インラインインデント構文",
			code: "N=5\n" +
				"ずっとN＝5の間繰り返す：\n" +
				"　　「N={N}」と表示\n" +
				"　　N=N+1\n" +
				"「OK」と表示。",
			want: "N=5\nOK",
		},
		{
			name: "!インデント構文",
			code: "!インデント構文\n" +
				"N=5\n" +
				"ずっと、N=5の間\n" +
				"　　「N={N}」と表示\n" +
				"　　N=N+1\n" +
				"「OK」と表示。",
			want: "N=5\nOK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// 「ずっと（比較式）の間…ここまで」構文のエラーケース (#526 / 本家 #2694)
func TestIssue526ZuttoWhileErrors(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantSub string
	}{
		{
			name:    "条件なし(ずっとの間)",
			code:    "ずっとの間\n1を表示\nここまで",
			wantSub: "条件がありません",
		},
		{
			name:    "条件なし(ずっと間)",
			code:    "ずっと間\n1を表示\nここまで",
			wantSub: "条件がありません",
		},
		{
			name:    "間が欠落",
			code:    "N=1\nずっと N = 1\n1を表示\nここまで",
			wantSub: "『間』がありません",
		},
		{
			name:    "ここまでが欠落",
			code:    "N=1\nずっと N = 1 の間\n1を表示",
			wantSub: "『ここまで』がありません",
		},
		{
			name:    "間の直後に改行がない",
			code:    "N=1\nずっと N = 1 の間 1を表示\nここまで",
			wantSub: "改行が必要",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := vm.RunSource(tt.code, "main.nako3", nil)
			if err == nil {
				t.Fatalf("エラーを期待したが成功した: %q", tt.code)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("エラー文面に %q が含まれない: got %q", tt.wantSub, err.Error())
			}
		})
	}
}
