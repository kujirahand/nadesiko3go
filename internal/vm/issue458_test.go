package vm_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// Issue #458: JSONの値として、関数が使えない問題がある。
// 無引数関数(今・今日などのシステム関数や、引数を取らないユーザー関数)が
// 配列・辞書の要素として先頭や途中にあると、後ろのカンマや閉じ括弧が
// 「値の終わり」として認識されず、文法エラーになっていた。
// 原因は parser の yValue が一語関数の区切りトークン一覧に
// comma と } を含んでいなかったこと。
//
// テストは次の観点で網羅する:
//   - Issue の再現そのもの(全角の ｘ＝[今,0])
//   - 決定的な戻り値を持つユーザー定義の無引数関数での位置・入れ子の網羅
//   - システム関数(今/今日/今年/システム時間など)はパターンで検証
//   - C風呼び出しの引数位置(MAX(関数,数値))
//   - 従来から動いていた書き方の回帰防止(末尾・単独・助詞付き・演算子付き)
//   - 閉じ括弧やカンマの欠落が引き続きエラーになること

// 決定的な戻り値を返す無引数関数の定義。テストコードの前に置く。
const issue458Funcs = "●テストとは\n「ok」を戻す\nここまで\n" +
	"●テスト数とは\n10を戻す\nここまで\n" +
	"●サブ数とは\n3を戻す\nここまで\n"

func TestIssue458ArrayWithFuncValue(t *testing.T) {
	tests := []struct{ name, code, want string }{
		// 配列内での位置の網羅
		{"先頭", "A=[テスト,0]\nAを表示", "ok,0"},
		{"末尾(従来から動作)", "A=[0,テスト]\nAを表示", "0,ok"},
		{"真ん中", "A=[0,テスト,1]\nAを表示", "0,ok,1"},
		{"単独(従来から動作)", "A=[テスト]\nAを表示", "ok"},
		{"連続", "A=[テスト,テスト]\nAを表示", "ok,ok"},
		{"数値関数が先頭", "A=[テスト数,0]\nAを表示", "10,0"},
		{"複数の関数", "A=[テスト,テスト数,サブ数]\nAを表示", "ok,10,3"},
		{"関数のみ3つ", "A=[テスト数,サブ数,テスト]\nAを表示", "10,3,ok"},
		// 入れ子
		{"入れ子の配列", "A=[[テスト,0],[1,テスト]]\nAを表示", "ok,0,1,ok"},
		{"入れ子の奥", "A=[[[テスト,0]]]\nAを表示", "ok,0"},
		{"配列の中の辞書", "A=[{\"a\":テスト},0]\nAをJSONエンコードして表示", `[{"a":"ok"},0]`},
		// 区切りまわり
		{"末尾カンマ", "A=[テスト,0,]\nAを表示", "ok,0"},
		{"改行区切り", "A=[\nテスト,\n0\n]\nAを表示", "ok,0"},
		// 関数値への演算など、従来から動いていた形の回帰防止
		{"演算子付き", "A=[テスト数+1,テスト数*2]\nAを表示", "11,20"},
		{"C風呼び出しの要素", "A=[テスト数(),0]\nAを表示", "10,0"},
		{"助詞付き呼び出しの要素", "A=[1を文字列変換,0]\nAを表示", "1,0"},
		{"組み込みC風関数の要素", "A=[LEN(\"abc\"),0]\nAを表示", "3,0"},
		// 配列としての利用
		{"要素数", "A=[テスト,0]\nAの要素数を表示", "2"},
		{"添字アクセス", "A=[テスト,0]\nA[0]を表示\nA[1]を表示", "ok\n0"},
		{"複数代入", "X,Y=[テスト,0]\nXを表示\nYを表示", "ok\n0"},
		{"JSONエンコード", "A=[テスト,テスト数]\nAをJSONエンコードして表示", `["ok",10]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, issue458Funcs+tt.code); got != tt.want {
				t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestIssue458DictWithFuncValue(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{"単独の値", "A={\"a\":テスト}\nAをJSONエンコードして表示", `{"a":"ok"}`},
		{"先頭の値", "A={\"a\":テスト,\"b\":1}\nAをJSONエンコードして表示", `{"a":"ok","b":1}`},
		{"末尾の値", "A={\"a\":1,\"b\":テスト}\nAをJSONエンコードして表示", `{"a":1,"b":"ok"}`},
		{"全部が関数", "A={\"a\":テスト,\"b\":テスト数}\nAをJSONエンコードして表示", `{"a":"ok","b":10}`},
		{"辞書の中の配列", "A={\"list\":[テスト,0]}\nAをJSONエンコードして表示", `{"list":["ok",0]}`},
		{"辞書の中の辞書", "A={\"d\":{\"t\":テスト}}\nAをJSONエンコードして表示", `{"d":{"t":"ok"}}`},
		{"複合", "A={\"list\":[テスト,0],\"d\":{\"t\":テスト数}}\nAをJSONエンコードして表示", `{"list":["ok",0],"d":{"t":10}}`},
		{"末尾カンマ", "A={\"a\":テスト,}\nAをJSONエンコードして表示", `{"a":"ok"}`},
		{"改行区切り", "A={\n\"a\":テスト,\n\"b\":1\n}\nAをJSONエンコードして表示", `{"a":"ok","b":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, issue458Funcs+tt.code); got != tt.want {
				t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestIssue458FuncValueInCStyleArgs(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{"第1引数", "A=MAX(テスト数,5)\nAを表示", "10"},
		{"第2引数", "A=MAX(5,テスト数)\nAを表示", "10"},
		{"両方の引数", "A=MAX(テスト数,サブ数)\nAを表示", "10"},
		{"C風呼び出し同士", "A=MAX(テスト数(),サブ数())\nAを表示", "10"},
		// システム時間は現在時刻のUNIX秒なので、5より大きいことは決定的
		{"システム関数が第1引数", "A=(MAX(システム時間,5)>5)\nAを表示", "true"},
		{"システム関数が第2引数", "A=(MAX(5,システム時間)>5)\nAを表示", "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, issue458Funcs+tt.code); got != tt.want {
				t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// システム関数は戻り値が時刻依存なので、書式パターンで検証する。
func TestIssue458SystemFuncValues(t *testing.T) {
	tests := []struct{ name, code, wantPattern string }{
		// Issue の再現そのもの(全角代入・全角変数名)
		{"再現:ｘ＝[今,0]", "ｘ＝[今,0]\nｘを表示", `^\d{2}:\d{2}:\d{2},0$`},
		{"今が先頭", "A=[今,0]\nAを表示", `^\d{2}:\d{2}:\d{2},0$`},
		{"今が末尾", "A=[0,今]\nAを表示", `^0,\d{2}:\d{2}:\d{2}$`},
		{"今が連続", "A=[今,今]\nAを表示", `^\d{2}:\d{2}:\d{2},\d{2}:\d{2}:\d{2}$`},
		{"今日が先頭", "A=[今日,0]\nAを表示", `^\d{4}/\d{2}/\d{2},0$`},
		{"今日と明日", "A=[今日,明日]\nAを表示", `^\d{4}/\d{2}/\d{2},\d{4}/\d{2}/\d{2}$`},
		{"今年が先頭", "A=[今年,0]\nAを表示", `^\d{4},0$`},
		{"システム時間が先頭", "A=[システム時間,0]\nAを表示", `^\d+,0$`},
		{"システム時間ミリ秒が真ん中", "A=[0,システム時間ミリ秒,0]\nAを表示", `^0,\d+,0$`},
		{"辞書の値が今だけ", "A={\"時刻\":今}\nAをJSONエンコードして表示", `^\{"時刻":"\d{2}:\d{2}:\d{2}"\}$`},
		{"辞書の先頭の値が今", "A={\"時刻\":今,\"x\":1}\nAをJSONエンコードして表示", `^\{"時刻":"\d{2}:\d{2}:\d{2}","x":1\}$`},
		{"辞書の末尾の値が今", "A={\"x\":1,\"時刻\":今}\nAをJSONエンコードして表示", `^\{"x":1,"時刻":"\d{2}:\d{2}:\d{2}"\}$`},
		{"複合JSON", "A={\"now\":今,\"date\":今日,\"list\":[今,今日]}\nAをJSONエンコードして表示",
			`^\{"now":"\d{2}:\d{2}:\d{2}","date":"\d{4}/\d{2}/\d{2}","list":\["\d{2}:\d{2}:\d{2}","\d{4}/\d{2}/\d{2}"\]}$`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := run(t, tt.code)
			if !regexp.MustCompile(tt.wantPattern).MatchString(got) {
				t.Errorf("%q = %q, want pattern %q", tt.code, got, tt.wantPattern)
			}
		})
	}
}

// 文として直接使う形が壊れていないこと(回帰防止)。
func TestIssue458FuncValueRegression(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{"代入", "A=テスト\nAを表示", "ok"},
		{"表示の引数", "テストを表示", "ok"},
		{"単独の文", "テスト数\nそれを表示", "10"},
		{"配列の外の演算", "A=テスト数+1\nAを表示", "11"},
		{"もし式の中", "A=もし1ならテスト、違えばテスト数\nAを表示", "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, issue458Funcs+tt.code); got != tt.want {
				t.Errorf("%q = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// カンマや閉じ括弧の欠落は、修正後も文法エラーのままであること。
// 区切りとして受理する範囲を広げても、不正な入力を通してはいけない。
func TestIssue458StillRejectsBrokenContainers(t *testing.T) {
	tests := []struct{ name, code, wantErr string }{
		{"配列が閉じていない", "A=[今,0", "閉じられていません"},
		{"ユーザー関数の配列が閉じていない", issue458Funcs + "A=[テスト,0", "閉じられていません"},
		{"辞書が閉じていない", "A={\"a\":今", "末尾の『}』がありません"},
		{"配列でカンマ欠落", "A=[今 0]", "閉じられていません"},
		{"辞書でコロン欠落", "A={\"a\" 今}", "末尾の『}』がありません"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := vm.RunSource(tt.code, "main.nako3", nil)
			if err == nil {
				t.Fatalf("%q はエラーになるべき", tt.code)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("%q のエラー = %v, want %q を含む", tt.code, err, tt.wantErr)
			}
		})
	}
}
