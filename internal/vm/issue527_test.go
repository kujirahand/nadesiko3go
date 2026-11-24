package vm_test

import "testing"

// Issue #527: 『もし』の条件に「〜の関数」形式の呼び出しを直接書いたときの実行テスト。
func TestIssue527IfCondFuncCall(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{
			name: "Issue再現ケース_等価比較",
			code: "A = [1, 2]\n" +
				"もし、Aの要素数 = 2 ならば\n" +
				"　「OK」を表示\n" +
				"ここまで",
			want: "OK",
		},
		{
			name: "偽分岐",
			code: "A = [1]\n" +
				"もし、Aの要素数 = 2 ならば\n" +
				"　「NG」を表示\n" +
				"違えば\n" +
				"　「OK」を表示\n" +
				"ここまで",
			want: "OK",
		},
		{
			name: "否定形_でなければ",
			code: "A = [1, 2]\n" +
				"もし、Aの要素数 = 3 でなければ\n" +
				"　「OK」を表示\n" +
				"ここまで",
			want: "OK",
		},
		{
			name: "算術演算子との組み合わせ",
			code: "A = [1, 2]\n" +
				"もし、Aの要素数 + 1 = 3 ならば\n" +
				"　「OK」を表示\n" +
				"ここまで",
			want: "OK",
		},
		{
			name: "論理演算子との組み合わせ",
			code: "A = [1, 2]\nB = 10\n" +
				"もし、Aの要素数 = 2 かつ B = 10 ならば\n" +
				"　「OK」を表示\n" +
				"ここまで",
			want: "OK",
		},
		{
			name: "助詞がによる比較",
			code: "A = [1, 2]\n" +
				"もし、Aの要素数が2ならば\n" +
				"　「OK」を表示\n" +
				"ここまで",
			want: "OK",
		},
		{
			name: "助詞がによる否定比較",
			code: "A = [1, 2]\n" +
				"もし、Aの要素数が3でなければ\n" +
				"　「OK」を表示\n" +
				"ここまで",
			want: "OK",
		},
		{
			name: "関数内のイベントハンドラ想定",
			code: "●判定する(配列を)とは\n" +
				"　もし、配列の要素数 = 2 ならば\n" +
				"　　「2枚」を表示\n" +
				"　違えば\n" +
				"　　「その他」を表示\n" +
				"　ここまで\n" +
				"ここまで\n" +
				"[10, 20]を判定する\n" +
				"[10]を判定する",
			want: "2枚\nその他",
		},
		{
			name: "もし式での利用",
			code: "A = [1, 2]\n" +
				"X = もし、Aの要素数 = 2 ならば「OK」違えば「NG」\n" +
				"Xを表示",
			want: "OK",
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
