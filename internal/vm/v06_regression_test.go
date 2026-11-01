package vm_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// TestV06RegexpLiveCases は、Issue V06 の再現を再現ファイルと同じ書き方でVMに
// 通す。各期待値はTypeScript実装がNodeのRegExpで出す値。
func TestV06RegexpLiveCases(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{"区切の捕獲", `「a,b」を「/(,)/」で正規表現区切をJSONエンコードして表示`, `["a",",","b"]`},
		{"未参加の捕獲はundefined", `「a1b2c」を「/(\d)|(x)/」で正規表現区切をJSONエンコードして表示`, `["a","1",null,"b","2",null,"c"]`},
		{"空マッチの前進", `「abc」を「/(?:)/」で正規表現区切をJSONエンコードして表示`, `["a","b","c"]`},
		{"空マッチのあとの前進", `「abc」を「/b*/」で正規表現区切をJSONエンコードして表示`, `["a","c"]`},
		{"日本語のrune基準", `「あいう」を「/(い)/」で正規表現区切をJSONエンコードして表示`, `["あ","い","う"]`},
		{"stickyは先頭のみ", `A=「ab」を「/b/y」で正規表現マッチ。AをJSONエンコードして表示`, `null`},
		{"stickyの先頭一致", `A=「ba」を「/b/y」で正規表現マッチ。AをJSONエンコードして表示`, `"b"`},
		{"g+yの連続一致", `B=「abab」を「/ab/gy」で正規表現マッチ。BをJSONエンコードして表示`, `["ab","ab"]`},
		{"g+yは途切れたら止まる", `C=「ab ab」を「/ab/gy」で正規表現マッチ。CをJSONエンコードして表示`, `["ab"]`},
		{"sticky置換の開始位置", `「aa」の「/a/y」を「X」に正規表現置換して表示`, `Xa`},
		{"sticky置換は先頭のみ", `「ba」の「/a/y」を「X」に正規表現置換して表示`, `ba`},
		{"g+y置換の連続範囲", `「aba」の「/a/gy」を「X」に正規表現置換して表示`, `Xba`},
		// GoのFindAllが落とす「直前の一致に隣接する空一致」も、stickyでは一致として数える
		{"nullableの空一致", `D=「ab」を「/a*|b/gy」で正規表現マッチ。DをJSONエンコードして表示`, `["a","",""]`},
		{"nullableの空一致の連続", `E=「abc」を「/b*/gy」で正規表現マッチ。EをJSONエンコードして表示`, `["","b","",""]`},
		{"nullableの置換", `「ab」の「/a*|b/gy」を「X」に正規表現置換して表示`, `XXbX`},
		{"stickyの文脈", `F=「a b」を「/\b./gy」で正規表現マッチ。FをJSONエンコードして表示`, `["a"," ","b"]`},
		{"一致しない残りはそのまま", `「ab ab」の「/ab/gy」を「X」に正規表現置換して表示`, `X ab`},
		{"2件目以降の部分一致", `「1a2b」の「/(\d)([a-z])/gy」を「X$1」に正規表現置換して表示`, `X1X2`},
		{"非BMPはrune基準", `G=「𩸽あ」を「/./gy」で正規表現マッチ。GをJSONエンコードして表示`, `["𩸽","あ"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// TestV06FlagErrors は、重複・未知・同時指定不可のフラグが出すメッセージを
// 固定する。Goのエラーがそのまま漏れるとフラグ名が出ない。
func TestV06FlagErrors(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{
			"重複フラグ",
			`「a」を「/a/gg」で正規表現マッチを表示。`,
			"[実行時エラー]main.nako3(1行目): 正規表現『/a/gg』のフラグ『g』が重複しています。",
		},
		{
			"不正フラグ",
			`「a」を「/a/zz」で正規表現マッチを表示。`,
			"[実行時エラー]main.nako3(1行目): 正規表現『/a/zz』のフラグ『z』は使えません。",
		},
		{
			"uとvの同時指定",
			`「a」を「/a/uv」で正規表現マッチを表示。`,
			"[実行時エラー]main.nako3(1行目): 正規表現『/a/uv』のフラグ『u』と『v』は同時に使えません。",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := vm.RunSource(tt.code, "main.nako3", nil); err == nil {
				t.Fatal("エラーになるはずが成功した")
			} else if err.Error() != tt.want {
				t.Errorf("Error()\n got %q\nwant %q", err.Error(), tt.want)
			}
		})
	}
}
