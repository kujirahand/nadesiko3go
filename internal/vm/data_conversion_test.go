package vm_test

// Issue #374: データ変換テストの強化。
// 文字コード変換（SJIS/EUC）とCSV、JSON、TOML の変換を、なでしこプログラム
// 経由で組み合わせて検証する。期待値はすべて実測で確認した現行挙動。
// キー順が実装依存で変わりうる箇所（JSONエンコード整形・TOMLエンコード）は、
// 出力文字列を直接比べず、デコードし直して値で比べる。
//
// 注意: CSVプラグインはプロセス内で共有され、CSVオプション設定の内容は
// 実行をまたいで残る。このファイルではオプションを変更せず、既定値
// （区切り文字「,」・数値の自動変換あり）を前提にする。他のテストが
// オプションを変更する場合は、v01_regression_test.go のように
// t.Cleanup で既定値へ戻すこと。

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// encodeForTest は参照用エンコーダで期待バイト列を作る。
// 命令で保存したファイルの実バイトと比較することで、保存と読み取りの
// 両方が同じ誤りをして往復だけ通るケースを検出する。
func encodeForTest(t *testing.T, enc encoding.Encoding, s string) []byte {
	t.Helper()
	b, err := io.ReadAll(transform.NewReader(strings.NewReader(s), enc.NewEncoder()))
	if err != nil {
		t.Fatalf("encodeForTest: %v", err)
	}
	return b
}

func TestDataConvSJISFileCSV(t *testing.T) {
	dir := t.TempDir()
	content := "氏名,住所\n山田,東京都\n"
	want := `[["氏名","住所"],["山田","東京都"]]`
	wantBytes := encodeForTest(t, japanese.ShiftJIS, content)

	// 命令で保存したファイルが、実際に Shift_JIS のバイト列であること
	path := filepath.Join(dir, "input.csv")
	code := "P=" + nakoString(path) + "\n「" + content + "」をPにSJISファイル保存\nS=PをSJISファイル読\nT=SをCSV取得\nTをJSONエンコードして表示"
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !bytes.Equal(saved, wantBytes) {
		t.Errorf("保存されたバイト列が Shift_JIS ではありません: got=% X want=% X", saved, wantBytes)
	}

	// 外部で作られた Shift_JIS のバイト列（ファイル）を正しく読めること
	path2 := filepath.Join(dir, "external.csv")
	if err := os.WriteFile(path2, wantBytes, 0o644); err != nil {
		t.Fatalf("err=%v", err)
	}
	got, err = runCUICommands(t, "P="+nakoString(path2)+"\nS=PをSJISファイル読\nT=SをCSV取得\nTをJSONエンコードして表示")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != want {
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
		// SJIS変換がエラーになる。Windows の cp932 との既知の差異。
		// cp932 互換の対応（U+301C→0x8160 など）を入れたら、この2件は
		// 「エラー」ではなく往復で一致する期待値に置き換えること。
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
				// 文法ミス等の別原因のエラーで合格しないよう、
				// SJIS変換由来のエラーであることを文面で確認する。
				if !strings.Contains(err.Error(), "rune not supported") {
					t.Fatalf("SJIS変換のエラーではない: err=%v", err)
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
	dir := t.TempDir()
	content := "名前,値\n桜,1\n"
	want := `[["名前","値"],["桜",1]]`
	wantBytes := encodeForTest(t, japanese.EUCJP, content)

	path := filepath.Join(dir, "input.csv")
	code := "P=" + nakoString(path) + "\n「" + content + "」をPにEUCファイル保存\nS=PをEUCファイル読\nT=SをCSV取得\nTをJSONエンコードして表示"
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !bytes.Equal(saved, wantBytes) {
		t.Errorf("保存されたバイト列が EUC-JP ではありません: got=% X want=% X", saved, wantBytes)
	}

	// 外部で作られた EUC-JP のバイト列（ファイル）を正しく読めること
	path2 := filepath.Join(dir, "external.csv")
	if err := os.WriteFile(path2, wantBytes, 0o644); err != nil {
		t.Fatalf("err=%v", err)
	}
	got, err = runCUICommands(t, "P="+nakoString(path2)+"\nS=PをEUCファイル読\nT=SをCSV取得\nTをJSONエンコードして表示")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != want {
		t.Errorf("got=%q want=%q", got, want)
	}

	// JIS X 0212 の補助漢字（鷗 U+9DD7）を含む行の往復
	path3 := filepath.Join(dir, "x0212.csv")
	code = "P=" + nakoString(path3) + "\n「鳥,鷗\n」をPにEUCファイル保存\nS=PをEUCファイル読\nT=SをCSV取得\nTをJSONエンコードして表示"
	got, err = runCUICommands(t, code)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := `[["鳥","鷗"]]`; got != want {
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

	// BOM付きUTF-8ファイルを「読」で読んで CSV取得 する経路でも同じ。
	path := filepath.Join(t.TempDir(), "bom.csv")
	if err := os.WriteFile(path, []byte("\ufeffa,b\n1,2\n"), 0o644); err != nil {
		t.Fatalf("err=%v", err)
	}
	got, err = runCUICommands(t, "P="+nakoString(path)+"\nS=Pを読む\nT=SをCSV取得\nTをJSONエンコードして表示")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := "[[\"\ufeffa\",\"b\"],[1,2]]"; got != want {
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
	// 整形結果をデコードし直し、キー順に依存しないよう値ごとに確認する。
	// （#349 が直ってキー順が保持されるようになっても、このテストは
	// 値の往復だけを見るので失敗しない。）
	got, err := runCUICommands(t, "A=『{\"b\":1,\"a\":2}』をJSONデコード\nS=AをJSONエンコード整形\nT=SをJSONデコード\nT[\"b\"]を表示\nT[\"a\"]を表示")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := "1\n2"; got != want {
		t.Errorf("整形の往復 got=%q want=%q", got, want)
	}
}

func TestDataConvJSONErrors(t *testing.T) {
	tests := []struct{ name, input string }{
		{"末尾カンマ", `{"a":1,}`},
		{"シングルクォート", `{'a':1}`},
		{"閉じていない配列", `[1,2`},
		{"空文字列", ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runCUICommands(t, fmt.Sprintf("『%s』をJSONデコードして表示", tt.input))
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
	// inf/nan は JSON では null になるため、TOMLエンコードへの往復で確認する。
	// エンコーダのキー順に依存しないよう、1キーずつ往復させる。
	for _, input := range []string{"c = inf", "d = nan", "e = -inf"} {
		got, err := runCUICommands(t, fmt.Sprintf("『%s』をTOMLデコードしてTOMLエンコードして表示", input))
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got != input {
			t.Errorf("got=%q want=%q", got, input)
		}
	}
}

// 範囲外の指数（1e400）は、本家（JSON.parse）では Infinity になるが、
// Go版は float64 の範囲外でエラーになる既知の差異。互換として扱うか
// どうかは未判断のため、ここではエラーになる現行挙動だけを記録する。
func TestDataConvJSONHugeExponent(t *testing.T) {
	got, err := runCUICommands(t, "『1e400』をJSONデコードして表示")
	if err == nil {
		t.Fatalf("エラーになるはずが got=%q", got)
	}
	if !strings.Contains(err.Error(), "JSONデコードに失敗しました。") {
		t.Errorf("err=%q", err)
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

// CSV変換（書き込み方向）と文字コード保存・読み取りの往復。
// クォートが必要なセル（カンマ・引用符・改行を含む）でも壊れないことと、
// 書き出しの改行（既定は CRLF）が読み戻しで揃うことを確認する。
func TestDataConvCSVWriteRoundtrip(t *testing.T) {
	want := `[["a,b","x\"y"],["改\n行","z"]]`
	for _, enc := range []string{"SJIS", "EUC"} {
		t.Run(enc, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "write.csv")
			code := "A=『[[\"a,b\",\"x\\\"y\"],[\"改\\n行\",\"z\"]]』をJSONデコード\n" +
				"S=AをCSV変換\n" +
				"P=" + nakoString(path) + "\n" +
				"SをPに" + enc + "ファイル保存\n" +
				"T=Pを" + enc + "ファイル読\n" +
				"U=TをCSV取得\nUをJSONエンコードして表示"
			got, err := runCUICommands(t, code)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if got != want {
				t.Errorf("got=%q want=%q", got, want)
			}
		})
	}
}

// JSON→CSV の向きの変換。CSV変換の改行は既定で CRLF、末尾にも付く。
func TestDataConvJSONToCSV(t *testing.T) {
	got, err := runCUICommands(t, "A=『[[1,2],[3,4]]』をJSONデコード\nS=AをCSV変換\nSをJSONエンコードして表示")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := `"1,2\r\n3,4\r\n"`; got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}
