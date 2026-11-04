package vm_test

// Issue #374: データ変換テストの強化。
// 文字コード変換（SJIS/EUC）とCSV、JSON、TOML の変換を、なでしこプログラム
// 経由で組み合わせて検証する。期待値はすべて実測で確認した現行挙動。
// キー順が実装依存で変わりうる箇所（JSONエンコード整形・TOMLエンコード）は、
// 出力文字列を直接比べず、デコードし直して値で比べる。

import (
	"fmt"
	"strings"
	"testing"
)

func TestDataConvSJISFileCSV(t *testing.T) {
	path := fmt.Sprintf("%s/input.csv", t.TempDir())
	code := fmt.Sprintf("P=「%s」\n「氏名,住所\n山田,東京都\n」をPにSJISファイル保存\nS=PをSJISファイル読\nT=SをCSV取得\nTをJSONエンコードして表示", path)
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := `[["氏名","住所"],["山田","東京都"]]`; got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

func TestDataConvSJISCharsCSV(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"cp932機種依存文字", "①,㈱,№,Ⅲ,髙,﨑", `[["①","㈱","№","Ⅲ","髙","﨑"]]`, false},
		{"半角カナ", "ｱｲｳ,ﾃﾞｰﾀ", `[["ｱｲｳ","ﾃﾞｰﾀ"]]`, false},
		{"2バイト目が0x5Cの文字", "表,ソ,能", `[["表","ソ","能"]]`, false},
		{"全角チルダU+FF5E", "～", `[["～"]]`, false},
		// 波ダッシュ(U+301C)と U+2212 は、x/text の ShiftJIS では扱えず
		// SJIS変換がエラーになる。Windows の cp932 との既知の差異として、
		// エラーになる現行挙動を固定する（黙って文字が消えることはない）。
		{"波ダッシュU+301Cは変換エラー", "〜", "", true},
		{"U+2212は変換エラー", "−", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf("B=「%s」をSJIS変換\nS=BからSJIS取得\nT=SをCSV取得\nTをJSONエンコードして表示", tt.input)
			got, err := runCUICommands(t, code)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("エラーになるはずが got=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if got != tt.want {
				t.Errorf("got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestDataConvEUCFileCSV(t *testing.T) {
	path := fmt.Sprintf("%s/input.csv", t.TempDir())
	code := fmt.Sprintf("P=「%s」\n「名前,値\n桜,1\n」をPにEUCファイル保存\nS=PをEUCファイル読\nT=SをCSV取得\nTをJSONエンコードして表示", path)
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := `[["名前","値"],["桜",1]]`; got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

func TestDataConvBOMCSV(t *testing.T) {
	// 現行実装では BOM は除去されず、1セル目の先頭に残る。
	// その現行挙動を固定する（除去するかどうかは別途判断）。
	got, err := runCUICommands(t, "\"\ufeffa,b\"をCSV取得してJSONエンコードして表示")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := "[[\"\ufeffa\",\"b\"]]"; got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

func TestDataConvJSONDecode(t *testing.T) {
	tests := []struct{ name, input, want string }{
		{"unicodeエスケープ", `"\u3042"`, `"あ"`},
		{"サロゲートペア", `"\uD83D\uDE00"`, `"😀"`},
		{"単独サロゲートはU+FFFD", `"\uD83D"`, `"�"`},
		{"2^53+1はfloat64に丸められる", `9007199254740993`, "9007199254740992"},
		{"nullと真偽値", `[null,true,false]`, `[null,true,false]`},
		{"文字列と数値の型保持", `["1",1,"",0]`, `["1",1,"",0]`},
		{"深いネスト", `[[[[[[[[[[1]]]]]]]]]]`, `[[[[[[[[[[1]]]]]]]]]]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf("『%s』をJSONデコードしてJSONエンコードして表示", tt.input)
			got, err := runCUICommands(t, code)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if got != tt.want {
				t.Errorf("got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestDataConvJSONEncode(t *testing.T) {
	tests := []struct{ name, input, want string }{
		{"非ASCIIと記号をエスケープしない", `{"s":"あ<>&"}`, `{"s":"あ<>&"}`},
		{"マイナスゼロは0になる", `-0`, `0`},
		{"1.0は1になる", `1.0`, `1`},
		{"キー順が保持される", `{"b":1,"a":2,"c":3}`, `{"b":1,"a":2,"c":3}`},
		{"重複キーは後の値が勝つ", `{"a":1,"a":2}`, `{"a":2}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf("『%s』をJSONデコードしてJSONエンコードして表示", tt.input)
			got, err := runCUICommands(t, code)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if got != tt.want {
				t.Errorf("got=%q want=%q", got, tt.want)
			}
		})
	}
	// JSONエンコード整形は現行ではキー順が変わる（Issue #349）ため、
	// 整形結果の文字列ではなく、デコードし直した値で確認する。
	got, err := runCUICommands(t, "A=『{\"b\":1,\"a\":2}』をJSONデコード\nS=AをJSONエンコード整形\nT=SをJSONデコード\nTをJSONエンコードして表示")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := `{"a":2,"b":1}`; got != want {
		t.Errorf("整形の往復 got=%q want=%q", got, want)
	}
}

func TestDataConvJSONErrors(t *testing.T) {
	for _, input := range []string{`{"a":1,}`, `{'a':1}`, `[1,2`, ``, `1e400`} {
		t.Run(fmt.Sprintf("%q", input), func(t *testing.T) {
			got, err := runCUICommands(t, fmt.Sprintf("『%s』をJSONデコードして表示", input))
			if err == nil {
				t.Fatalf("エラーになるはずが got=%q", got)
			}
			if !strings.Contains(err.Error(), "JSONデコードに失敗しました。") {
				t.Errorf("err=%q", err)
			}
		})
	}
}

func TestDataConvTOMLDecode(t *testing.T) {
	tests := []struct{ name, input, want string }{
		{"ドット区切りキー", `a.b.c = 1`, `{"a":{"b":{"c":1}}}`},
		{"テーブル配列", "[[items]]\nname = \"x\"\n[[items]]\nname = \"y\"", `{"items":[{"name":"x"},{"name":"y"}]}`},
		{"引用キー", "\"a b\" = 1\n'a.b' = 2", `{"a b":1,"a.b":2}`},
		{"inline table", `a = {x = 1, y = "z"}`, `{"a":{"x":1,"y":"z"}}`},
		{"複数行文字列", "s = \"\"\"ab\ncd\"\"\"\nt = '''xy'''", "{\"s\":\"ab\\ncd\",\"t\":\"xy\"}"},
		{"日時の4種", "a = 1979-05-27T07:32:00Z\nb = 1979-05-27T07:32:00\nc = 1979-05-27\nd = 07:32:00", `{"a":"1979-05-27T07:32:00Z","b":"1979-05-27T07:32:00","c":"1979-05-27","d":"07:32:00"}`},
		{"数値リテラル", "a = 1_000\nb = 0x1F", `{"a":1000,"b":31}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf("『%s』をTOMLデコードしてJSONエンコードして表示", tt.input)
			got, err := runCUICommands(t, code)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if got != tt.want {
				t.Errorf("got=%q want=%q", got, tt.want)
			}
		})
	}
	// inf/nan は JSON では null になるため、TOMLエンコードへの往復で確認する
	got, err := runCUICommands(t, "『c = inf\nd = nan\ne = -inf』をTOMLデコードしてTOMLエンコードして表示")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := "c = inf\nd = nan\ne = -inf"; got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

func TestDataConvTOMLEncode(t *testing.T) {
	tests := []struct{ name, input, want string }{
		// TOML に null は無いため、現行実装では空文字列になる仕様を固定する
		{"nullは空文字列になる", `{"a":null}`, `a = ""`},
		{"配列内のnullも空文字列", `{"a":[1,null]}`, `a = [1, ""]`},
		{"空配列", `{"a":[]}`, `a = []`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf("『%s』をJSONデコードしてTOMLエンコードして表示", tt.input)
			got, err := runCUICommands(t, code)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if got != tt.want {
				t.Errorf("got=%q want=%q", got, tt.want)
			}
		})
	}
	// TOMLエンコードのキー順は実装依存のため、デコードし直して値で比べる
	got, err := runCUICommands(t, "A=『{\"b\":1,\"a\":\"x\"}』をJSONデコード\nS=AをTOMLエンコード\nT=SをTOMLデコード\nTをJSONエンコードして表示")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := `{"a":"x","b":1}`; got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
	// 最上位が辞書でない場合はエラー
	got, err = runCUICommands(t, "『[1,2]』をJSONデコードしてTOMLエンコードして表示")
	if err == nil {
		t.Fatalf("エラーになるはずが got=%q", got)
	}
	if !strings.Contains(err.Error(), "TOMLエンコードできるのは辞書のみです。") {
		t.Errorf("err=%q", err)
	}
}

func TestDataConvTOMLErrors(t *testing.T) {
	for _, input := range []string{"a = 1\na = 2", "d = 2024-13-01", `s = "abc`} {
		t.Run(fmt.Sprintf("%q", input), func(t *testing.T) {
			got, err := runCUICommands(t, fmt.Sprintf("『%s』をTOMLデコードして表示", input))
			if err == nil {
				t.Fatalf("エラーになるはずが got=%q", got)
			}
			if !strings.Contains(err.Error(), "TOMLデコードに失敗しました。") {
				t.Errorf("err=%q", err)
			}
		})
	}
}

func TestDataConvCSVToJSON(t *testing.T) {
	got, err := runCUICommands(t, "「名前,値\n桜,1\n青,2」をCSV取得してJSONエンコードして表示")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := `[["名前","値"],["桜",1],["青",2]]`; got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}
