package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestCheckNakoSyntaxOK(t *testing.T) {
	res := checkNakoSyntax("「こんにちは」と表示。", "", lintModeGonako)
	if !res.OK {
		t.Errorf("OK = false, error = %q", res.Error)
	}
	if res.Error != "" {
		t.Errorf("Error = %q, want empty", res.Error)
	}
}

func TestCheckNakoSyntaxError(t *testing.T) {
	res := checkNakoSyntax("もし「A」ならば\n「B」と表示。", "test.nako3", lintModeGonako)
	if res.OK {
		t.Fatal("文法エラーがあるのにOK = trueでした")
	}
	if res.Error == "" {
		t.Error("Error が空でした")
	}
}

func TestFormatNakoCodeChanged(t *testing.T) {
	code := "3回\n「A」と表示。\nここまで\n"
	res := formatNakoCode(code, "test.nako3", false, lintModeGonako)
	if !res.OK {
		t.Fatalf("OK = false, error = %q", res.Error)
	}
	if !res.Changed {
		t.Error("Changed = false, want true")
	}
	if !strings.Contains(res.Formatted, "    「A」と表示。") {
		t.Errorf("インデントが整形されませんでした: %q", res.Formatted)
	}
}

func TestFormatNakoCodeUnchanged(t *testing.T) {
	code := "3回\n    「A」と表示。\nここまで\n"
	res := formatNakoCode(code, "test.nako3", false, lintModeGonako)
	if !res.OK {
		t.Fatalf("OK = false, error = %q", res.Error)
	}
	if res.Changed {
		t.Error("Changed = true, want false")
	}
}

func TestFormatNakoCodeColonSyntax(t *testing.T) {
	code := "3回:\n  「A」と表示。\n「終」と表示。\n"
	res := formatNakoCode(code, "colon.nako3", false, lintModeGonako)
	if !res.OK {
		t.Fatalf("OK = false, error = %q", res.Error)
	}
	want := "3回:\n    「A」と表示。\n「終」と表示。\n"
	if res.Formatted != want || !res.Changed {
		t.Errorf("Formatted = %q, Changed = %v", res.Formatted, res.Changed)
	}
}

func TestFormatNakoCodeToColon(t *testing.T) {
	code := "3回\n    「A」と表示。\nここまで\n"
	res := formatNakoCode(code, "test.nako3", true, lintModeGonako)
	if !res.OK {
		t.Fatalf("OK = false, error = %q", res.Error)
	}
	want := "3回:\n    「A」と表示。\n"
	if res.Formatted != want || !res.Changed {
		t.Errorf("Formatted = %q, Changed = %v", res.Formatted, res.Changed)
	}
}

func TestFormatNakoCodeSyntaxError(t *testing.T) {
	res := formatNakoCode("もし「A」ならば\n「B」と表示。", "test.nako3", false, lintModeGonako)
	if res.OK {
		t.Fatal("文法エラーがあるのにOK = trueでした")
	}
	if res.Error == "" {
		t.Error("Error が空でした")
	}
}

// wnakoモードではwnako3専用の命令（DOM操作など）を文法エラーにしない（#301）。
func TestCheckNakoSyntaxWNakoCommands(t *testing.T) {
	code := "「div」のDOM_HTML取得して表示\n「style」のDOM_HTML取得して表示\n"
	if res := checkNakoSyntax(code, "wnako.nako3", lintModeWNako); !res.OK {
		t.Errorf("wnakoモードでエラーになりました: %q", res.Error)
	}
	if res := checkNakoSyntax(code, "wnako.nako3", lintModeGonako); res.OK {
		t.Error("gonakoモードでwnako専用命令がエラーになりませんでした")
	}
}

// wnakoモードではgonako専用の命令（外部プロセスの起動など）を文法エラーにする（#301）。
func TestCheckNakoSyntaxWNakoRejectsGonakoOnly(t *testing.T) {
	code := "「ls」を起動して表示\n"
	if res := checkNakoSyntax(code, "gonako.nako3", lintModeGonako); !res.OK {
		t.Fatalf("gonakoモードでエラーになりました: %q", res.Error)
	}
	if res := checkNakoSyntax(code, "wnako.nako3", lintModeWNako); res.OK {
		t.Error("wnakoモードでgonako専用命令がエラーになりませんでした")
	}
}

// 利用者定義の関数が命令表へ書き足されても、次のチェックへ持ち越さない。
func TestCheckNakoSyntaxWNakoDoesNotLeakUserFuncs(t *testing.T) {
	def := "●(Aを)独自命令とは\n    Aを表示\nここまで\n「x」を独自命令\n"
	if res := checkNakoSyntax(def, "def.nako3", lintModeWNako); !res.OK {
		t.Fatalf("OK = false, error = %q", res.Error)
	}
	if res := checkNakoSyntax("「x」を独自命令\n", "use.nako3", lintModeWNako); res.OK {
		t.Error("前回の利用者定義関数が残っています")
	}
}

func TestFormatNakoCodeWNakoCommands(t *testing.T) {
	code := "3回\n「div」のDOM_HTML取得して表示\nここまで\n"
	res := formatNakoCode(code, "wnako.nako3", false, lintModeWNako)
	if !res.OK {
		t.Fatalf("OK = false, error = %q", res.Error)
	}
	want := "3回\n    「div」のDOM_HTML取得して表示\nここまで\n"
	if res.Formatted != want {
		t.Errorf("Formatted = %q, want %q", res.Formatted, want)
	}
	if res := formatNakoCode(code, "wnako.nako3", true, lintModeWNako); !res.OK {
		t.Errorf("コロン記法: OK = false, error = %q", res.Error)
	}
}

// wnako3の命令一覧の挿入用テンプレートが、wnakoモードの文法チェックを
// 通ることを確かめる。command-list-wnako.json の助詞はマニュアルの書式から
// 抜き出した近似なので、抜け漏れがあると正しいプログラムを誤ってエラーにする。
func TestCheckNakoSyntaxWNakoTemplates(t *testing.T) {
	argRe := regexp.MustCompile(`【[^】]*】`)
	for _, cmd := range getWNakoCommandList() {
		var code string
		switch cmd.Type {
		case "func":
			if cmd.Template == "" {
				continue
			}
			code = argRe.ReplaceAllString(cmd.Template, "「x」") + "\n"
		default:
			code = cmd.Name + "を表示\n"
		}
		if res := checkNakoSyntax(code, "template.nako3", lintModeWNako); !res.OK {
			t.Errorf("%s (%s): %q: %s", cmd.Name, cmd.Plugin, code, res.Error)
		}
	}
}
