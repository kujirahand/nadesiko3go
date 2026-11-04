package vm_test

// Issue #374: エンコーディング周りのテスト強化。
// エンコーディング変換・エンコーディング取得の名前の表記ゆれ、入力別の復号、
// SJISで表せない文字、ISO-2022-JP、UTF-16のCSV/TSV（Excelの「Unicodeテキスト」）を、
// なでしこプログラム経由で検証する。期待値はすべて実測で確認した現行挙動。
// バイト列は参照エンコーダ（x/text と unicode/utf16）で作り、なでしこ側へは
// 配列リテラルで渡す。コードを変えて期待値が変わる場合は、理由をコメントに残すこと。

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
)

// encTestLit はバイト列をなでしこの配列リテラル（およびJSONエンコード結果）の形にする。
func encTestLit(b []byte) string {
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprint(v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// encTestJSON は文字列をJSONエンコードした期待値にする（<>& を使わない前提）。
func encTestJSON(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("encTestJSON: %v", err)
	}
	return string(b)
}

// encTestUTF16 は文字列をUTF-16のバイト列にする。bom が真なら先頭にU+FEFFを付ける。
func encTestUTF16(s string, order binary.ByteOrder, bom bool) []byte {
	var out []byte
	put := func(u uint16) {
		var b [2]byte
		order.PutUint16(b[:], u)
		out = append(out, b[:]...)
	}
	if bom {
		put(0xFEFF)
	}
	for _, u := range utf16.Encode([]rune(s)) {
		put(u)
	}
	return out
}

// encTestGet はバイト列を指定の名前で「エンコーディング取得」し、結果をJSON文字列で返す。
func encTestGet(t *testing.T, data []byte, name string) (string, error) {
	t.Helper()
	code := "B=" + encTestLit(data) + "\nS=Bを" + nakoString(name) + "からエンコーディング取得\nSをJSONエンコードして表示"
	return runCUICommands(t, code)
}

// encTestConv は文字列を指定の名前で「エンコーディング変換」し、バイト列のJSON表記を返す。
func encTestConv(t *testing.T, text, name string) (string, error) {
	t.Helper()
	code := "B=「" + text + "」を" + nakoString(name) + "へエンコーディング変換\nBをJSONエンコードして表示"
	return runCUICommands(t, code)
}

// 名前の表記ゆれ。グループ内のどの名前でも同じバイト列になり、同じ文字列へ戻る。
func TestEncodingNameVariants(t *testing.T) {
	groups := []struct {
		group string
		enc   encoding.Encoding
		names []string
	}{
		{"SJIS系", japanese.ShiftJIS, []string{"sjis", "shift_jis", "shift-jis", "cp932", "ms932", "windows-31j", "Shift_JIS", "SJIS", "CP932", "Windows-31J"}},
		{"EUC系", japanese.EUCJP, []string{"euc-jp", "euc_jp", "eucjp", "EUC-JP", "EUCJP"}},
		{"ISO-2022-JP系", japanese.ISO2022JP, []string{"iso-2022-jp", "iso2022jp", "jis", "ISO-2022-JP", "JIS"}},
		{"UTF-8系", nil, []string{"utf-8", "utf8", "UTF-8", "UTF8"}},
	}
	const text = "あ"
	for _, g := range groups {
		var ref []byte
		if g.enc == nil {
			ref = []byte(text)
		} else {
			ref = encodeForTest(t, g.enc, text)
		}
		for _, name := range g.names {
			t.Run(g.group+"/"+name, func(t *testing.T) {
				got, err := encTestConv(t, text, name)
				if err != nil {
					t.Fatalf("変換 err=%v", err)
				}
				if want := encTestLit(ref); got != want {
					t.Errorf("変換 got=%q want=%q", got, want)
				}
				got, err = encTestGet(t, ref, name)
				if err != nil {
					t.Fatalf("取得 err=%v", err)
				}
				if want := encTestJSON(t, text); got != want {
					t.Errorf("取得 got=%q want=%q", got, want)
				}
			})
		}
	}
}

// 未知の名前は、変換・取得のどちらも同じ文面のエラーになる。
func TestEncodingUnknownName(t *testing.T) {
	const want = "未対応のエンコーディング『unknown-xyz』です。"
	got, err := encTestConv(t, "あ", "unknown-xyz")
	if err == nil {
		t.Fatalf("変換: エラーになるはずが got=%q", got)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("変換 err=%v", err)
	}
	got, err = encTestGet(t, []byte{0x41}, "unknown-xyz")
	if err == nil {
		t.Fatalf("取得: エラーになるはずが got=%q", got)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("取得 err=%v", err)
	}
}

func TestEncodingSupportCheck(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"utf-16", "true"},
		{"utf-16le", "true"},
		{"cp932", "true"},
		{"utf-32", "false"},
		{"unknown-xyz", "false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runCUICommands(t, "S="+nakoString(tt.name)+"を文字コード変換サポート判定\nSをJSONエンコードして表示")
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if got != tt.want {
				t.Errorf("got=%q want=%q", got, tt.want)
			}
		})
	}
}

// エンコーディング取得の入力別の挙動。
func TestEncodingGetInputs(t *testing.T) {
	invalid := []byte{0xFF, 0xFF, 0xFF}
	tests := []struct {
		name string
		data []byte
		enc  string
		want string
	}{
		{"SJIS", encodeForTest(t, japanese.ShiftJIS, "日本語ABC"), "shift_jis", "日本語ABC"},
		{"EUC", encodeForTest(t, japanese.EUCJP, "日本語ABC"), "euc-jp", "日本語ABC"},
		{"UTF-8", []byte("日本語ABC"), "utf-8", "日本語ABC"},
		{"ISO-2022-JP", encodeForTest(t, japanese.ISO2022JP, "日本語ABC"), "iso-2022-jp", "日本語ABC"},
		{"ASCIIのみ/SJIS", []byte("Hello, World 123"), "sjis", "Hello, World 123"},
		{"ASCIIのみ/EUC", []byte("Hello, World 123"), "euc-jp", "Hello, World 123"},
		{"ASCIIのみ/UTF-8", []byte("Hello, World 123"), "utf-8", "Hello, World 123"},
		{"空/SJIS", []byte{}, "sjis", ""},
		{"空/EUC", []byte{}, "euc-jp", ""},
		{"空/UTF-8", []byte{}, "utf-8", ""},
		// 不正なバイト列はエラーにならず、不正な1バイトごとに U+FFFD へ置換される。
		// （エラーにするかどうかは別途判断。ここでは現行挙動を固定する。）
		{"不正バイト/SJIS", invalid, "sjis", "���"},
		{"不正バイト/EUC", invalid, "euc-jp", "���"},
		{"不正バイト/UTF-8", invalid, "utf-8", "���"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encTestGet(t, tt.data, tt.enc)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if want := encTestJSON(t, tt.want); got != want {
				t.Errorf("got=%q want=%q", got, want)
			}
		})
	}
}

// SJISで表せない文字は、SJIS変換・エンコーディング変換のどちらもエラーになる。
// （波ダッシュ U+301C と U+2212 は data_conversion_test.go で固定済み）
func TestEncodingSJISUnsupportedChars(t *testing.T) {
	for _, tt := range []struct{ name, text string }{
		{"絵文字", "😀"},
		{"𠮷U+20BB7", "𠮷"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runCUICommands(t, "B=「"+tt.text+"」をSJIS変換\nBをJSONエンコードして表示")
			if err == nil {
				t.Fatalf("SJIS変換: エラーになるはずが got=%q", got)
			}
			if !strings.Contains(err.Error(), "rune not supported") {
				t.Errorf("SJIS変換 err=%v", err)
			}
			got, err = encTestConv(t, tt.text, "shift_jis")
			if err == nil {
				t.Fatalf("エンコーディング変換: エラーになるはずが got=%q", got)
			}
			if !strings.Contains(err.Error(), "rune not supported") {
				t.Errorf("エンコーディング変換 err=%v", err)
			}
		})
	}
}

// ISO-2022-JP は、日本語とASCIIの混在や複数行（エスケープシーケンスが行をまたぐ）でも往復できる。
func TestEncodingISO2022JPRoundtrip(t *testing.T) {
	tests := []struct{ name, text string }{
		{"日本語", "日本語"},
		{"日本語とASCII混在", "abc日本語def漢字ghi"},
		{"複数行", "日本語abc\nテスト\nEnd日本\n終わり"},
		{"行頭と行末が日本語", "あ\nい\nう"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := encodeForTest(t, japanese.ISO2022JP, tt.text)
			for _, b := range ref {
				if b >= 0x80 {
					t.Fatalf("参照エンコーダの出力が7ビットではない: % X", ref)
				}
			}
			got, err := encTestConv(t, tt.text, "iso-2022-jp")
			if err != nil {
				t.Fatalf("変換 err=%v", err)
			}
			if want := encTestLit(ref); got != want {
				t.Errorf("変換 got=%q want=%q", got, want)
			}
			code := "B=「" + tt.text + "」を「iso-2022-jp」へエンコーディング変換\n" +
				"S=Bを「iso-2022-jp」からエンコーディング取得\nSをJSONエンコードして表示"
			got, err = runCUICommands(t, code)
			if err != nil {
				t.Fatalf("往復 err=%v", err)
			}
			if want := encTestJSON(t, tt.text); got != want {
				t.Errorf("往復 got=%q want=%q", got, want)
			}
		})
	}
}

// UTF-16 のCSV: "utf-16" へ変換するとBOM付きBE、取得するとBOMを解釈して元の表に戻る。
func TestEncodingUTF16CSV(t *testing.T) {
	const csv = "氏名,値\n山田,1"
	ref := encTestUTF16(csv, binary.BigEndian, true)
	if ref[0] != 0xFE || ref[1] != 0xFF {
		t.Fatalf("参照のBOMが想定と違う: % X", ref[:2])
	}
	got, err := encTestConv(t, csv, "utf-16")
	if err != nil {
		t.Fatalf("変換 err=%v", err)
	}
	if want := encTestLit(ref); got != want {
		t.Errorf("変換 got=%q want=%q", got, want)
	}
	code := "B=「" + csv + "」を「utf-16」へエンコーディング変換\n" +
		"S=Bを「utf-16」からエンコーディング取得\nT=SをCSV取得\nTをJSONエンコードして表示"
	got, err = runCUICommands(t, code)
	if err != nil {
		t.Fatalf("往復 err=%v", err)
	}
	if want := `[["氏名","値"],["山田",1]]`; got != want {
		t.Errorf("往復 got=%q want=%q", got, want)
	}
}

// "utf-16" の取得は、BOMでLE/BEを判別し、BOMを結果に残さない。
func TestEncodingUTF16GetBOM(t *testing.T) {
	const text = "氏名\t値"
	for _, tt := range []struct {
		name  string
		order binary.ByteOrder
	}{
		{"LE", binary.LittleEndian},
		{"BE", binary.BigEndian},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encTestGet(t, encTestUTF16(text, tt.order, true), "utf-16")
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if want := encTestJSON(t, text); got != want {
				t.Errorf("got=%q want=%q", got, want)
			}
		})
	}
}

// Excelの「Unicodeテキスト」形式（UTF-16LE + BOM、タブ区切り）を読む。
func TestEncodingUTF16ExcelUnicodeText(t *testing.T) {
	data := encTestUTF16("氏名\t値\n山田\t1", binary.LittleEndian, true)
	if data[0] != 0xFF || data[1] != 0xFE {
		t.Fatalf("BOMが想定と違う: % X", data[:2])
	}
	code := "B=" + encTestLit(data) + "\nS=Bを「utf-16」からエンコーディング取得\nT=SをTSV取得\nTをJSONエンコードして表示"
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := `[["氏名","値"],["山田",1]]`; got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

// "utf-16le" で復号するとBOMは取り除かれず、先頭セルに U+FEFF が残る。
// UTF-8 のBOM付き復号で U+FEFF が残る現行挙動（TestDataConvBOMCSV）と同じ扱い。
// BOMを除去する方針に変えた場合は、この期待値を更新すること。
func TestEncodingUTF16LEKeepsBOM(t *testing.T) {
	data := encTestUTF16("氏名\t値\n山田\t1", binary.LittleEndian, true)
	code := "B=" + encTestLit(data) + "\nS=Bを「utf-16le」からエンコーディング取得\nT=SをTSV取得\nTをJSONエンコードして表示"
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := "[[\"\ufeff氏名\",\"値\"],[\"山田\",1]]"; got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

// "utf-16le" への変換はBOMを付けない（実バイトで比較する）。
func TestEncodingUTF16LEEncodeNoBOM(t *testing.T) {
	const text = "あ氏A"
	ref := encTestUTF16(text, binary.LittleEndian, false)
	got, err := encTestConv(t, text, "utf-16le")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if want := encTestLit(ref); got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
	// 先頭は「あ」(U+3042) の下位バイト・上位バイトであり、BOM(FF FE) ではない
	if want := "[66,48,"; !strings.HasPrefix(got, want) {
		t.Errorf("先頭がBOMなしのLEではない: got=%q", got)
	}
}

const encTestFixtureDir = "testdata/data_conversion"

// encTestFixture は fixture ファイルを読み、参照エンコーダの出力とバイト一致することを確認し、
// 絶対パスを返す。fixture は scripts で生成する（テキスト編集では作れないため）。
func encTestFixture(t *testing.T, name string, want []byte) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join(encTestFixtureDir, name))
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture %s を読めません（生成スクリプトで作成してください）: %v", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("fixture %s が参照エンコーダの出力と一致しません: got=% X want=% X", name, got, want)
	}
	return path
}

// testdata の実ファイルを、各命令で読んで CSV/TSV にする。
func TestEncodingFixtures(t *testing.T) {
	const csvText = "氏名,住所\n山田,東京都\n"
	const csvWant = `[["氏名","住所"],["山田","東京都"]]`

	t.Run("cp932.csv", func(t *testing.T) {
		path := encTestFixture(t, "cp932.csv", encodeForTest(t, japanese.ShiftJIS, csvText))
		for _, code := range []string{
			"P=" + nakoString(path) + "\nS=PをSJISファイル読\nT=SをCSV取得\nTをJSONエンコードして表示",
			"P=" + nakoString(path) + "\nB=Pをバイナリ読\nS=Bを「cp932」からエンコーディング取得\nT=SをCSV取得\nTをJSONエンコードして表示",
		} {
			got, err := runCUICommands(t, code)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if got != csvWant {
				t.Errorf("got=%q want=%q", got, csvWant)
			}
		}
	})

	t.Run("eucjp.csv", func(t *testing.T) {
		path := encTestFixture(t, "eucjp.csv", encodeForTest(t, japanese.EUCJP, csvText))
		for _, code := range []string{
			"P=" + nakoString(path) + "\nS=PをEUCファイル読\nT=SをCSV取得\nTをJSONエンコードして表示",
			"P=" + nakoString(path) + "\nB=Pをバイナリ読\nS=Bを「euc-jp」からエンコーディング取得\nT=SをCSV取得\nTをJSONエンコードして表示",
		} {
			got, err := runCUICommands(t, code)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if got != csvWant {
				t.Errorf("got=%q want=%q", got, csvWant)
			}
		}
	})

	// BOM付きUTF-8は、通常の「読」でもエンコーディング取得でも U+FEFF が先頭セルに残る。
	t.Run("utf8bom.csv", func(t *testing.T) {
		path := encTestFixture(t, "utf8bom.csv", []byte("\ufeffa,b\n1,2\n"))
		want := "[[\"\ufeffa\",\"b\"],[1,2]]"
		for _, code := range []string{
			"P=" + nakoString(path) + "\nS=Pを読む\nT=SをCSV取得\nTをJSONエンコードして表示",
			"P=" + nakoString(path) + "\nB=Pをバイナリ読\nS=Bを「utf-8」からエンコーディング取得\nT=SをCSV取得\nTをJSONエンコードして表示",
		} {
			got, err := runCUICommands(t, code)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if got != want {
				t.Errorf("got=%q want=%q", got, want)
			}
		}
	})

	t.Run("utf16le_bom.tsv", func(t *testing.T) {
		path := encTestFixture(t, "utf16le_bom.tsv", encTestUTF16("氏名\t値\n山田\t1", binary.LittleEndian, true))
		code := "P=" + nakoString(path) + "\nB=Pをバイナリ読\nS=Bを「utf-16」からエンコーディング取得\nT=SをTSV取得\nTをJSONエンコードして表示"
		got, err := runCUICommands(t, code)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if want := `[["氏名","値"],["山田",1]]`; got != want {
			t.Errorf("got=%q want=%q", got, want)
		}
	})
}
