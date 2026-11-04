package vm_test

import (
	"strings"
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

// TestV06HybridLiveCases は、920がpanicしていた入力を同じ入口（なでしこの
// 正規表現命令）から通す。閉じた引用の3件はimmutable archiveのe526/baseが
// 返した成功literal、深さ998はNodeで固定したliteral。
func TestV06HybridLiveCases(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{"閉じた引用の空一致", `A=「a」を「/(?:\Q\\E)?/gy」で正規表現マッチ。AをJSONエンコードして表示`, `["",""]`},
		{"閉じた引用と語境界", `A=「\a」を「/\Q\\E\b/gy」で正規表現マッチ。AをJSONエンコードして表示`, `["\\"]`},
		{"名前付き抽出の閉じた引用", `A=「a」を「/(?<n>(?:\Q\\E)?)/gy」で正規表現抽出。AをJSONエンコードして表示`, `["",""]`},
		{"名前付き組の位置補正", `「あいう」の「/(あ)?(?<n>い?)/gy」を「<$1:$<n>>」に正規表現置換して表示`, `<あ:い><:>う<:>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// TestV06DepthContextFreeLiveCase は、元がvalidな深さ998のcontext-freeパターンが
// 命令の入口から成功することを固定する。920は初回の位置指定照合でpanicし、
// e526は深さを理由にerrorにしていた。
func TestV06DepthContextFreeLiveCase(t *testing.T) {
	body := strings.Repeat("(", 998) + "a?" + strings.Repeat(")", 998)
	code := `A=「a」を「/` + body + `/gy」で正規表現マッチ。AをJSONエンコードして表示`
	if got, want := run(t, code), `["a",""]`; got != want {
		t.Errorf("深さ998の正規表現マッチ = %q, want %q", got, want)
	}
}

// TestV06DeepContextLiveError は、文脈を見るアサーションを含む深いパターンを、
// 同じ入口から明示的な言語エラーとして返すことを固定する。yフラグの位置指定
// 照合にだけ使えないのであって、パターン自体は正しい。
func TestV06DeepContextLiveError(t *testing.T) {
	body := strings.Repeat("(", 997) + "a?$" + strings.Repeat(")", 997)
	code := `A=「a」を「/` + body + `/gy」で正規表現マッチ。Aを表示`
	want := "[実行時エラー]main.nako3(1行目): 正規表現『/" + body + "/gy』はyフラグの位置指定照合には深すぎます。"
	if _, err := vm.RunSource(code, "main.nako3", nil); err == nil {
		t.Fatal("深い文脈付きパターンはエラーになるべき")
	} else if err.Error() != want {
		t.Errorf("Error()\n got %q\nwant %q", err.Error(), want)
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

// TestV06ParticipationLiveCases は、未参加の組と空文字に一致した参加組の
// 区別を、なでしこのプログラムの入口からJSONで固定する。未参加はundefined、
// 空一致は空文字で、どちらも区切りの結果と『抽出文字列』で同じ扱いになる。
func TestV06ParticipationLiveCases(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{
			"抽出文字列の未参加",
			`「b」を「/(a)|(b)/」で正規表現マッチ。抽出文字列をJSONエンコードして表示`,
			`[null,"b"]`,
		},
		{
			"抽出文字列の空一致",
			`「1」を「/(x*)(\d)/」で正規表現マッチ。抽出文字列をJSONエンコードして表示`,
			`["","1"]`,
		},
		{
			"区切りの未参加",
			`「a1b」を「/(x)?(\d)/」で正規表現区切をJSONエンコードして表示`,
			`["a",null,"1","b"]`,
		},
		{
			"区切りの空一致",
			`「a1b」を「/(x*)(\d)/」で正規表現区切をJSONエンコードして表示`,
			`["a","","1","b"]`,
		},
		{
			"抽出の未参加",
			`「a1b2c」を「/(\d)|(x)/」で正規表現抽出をJSONエンコードして表示`,
			`["1",null,"2",null]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// TestV06NonASCIIFlagLiveError は、非ASCIIのフラグを命令の入口で言語の
// エラーにし、問題の文字を添えることを固定する。修正前は『/a/é』が
// パターンそのものとして受理されていた。
func TestV06NonASCIIFlagLiveError(t *testing.T) {
	code := `「a」を「/a/é」で正規表現マッチを表示。`
	want := "[実行時エラー]main.nako3(1行目): 正規表現『/a/é』のフラグ『é』は使えません。"
	if _, err := vm.RunSource(code, "main.nako3", nil); err == nil {
		t.Fatal("『/a/é』はエラーになるべき")
	} else if err.Error() != want {
		t.Errorf("Error()\n got %q\nwant %q", err.Error(), want)
	}
}
