package csvlib

import (
	"strconv"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// Issue #373: CSVパースの境界ケースを固定するテスト群。
// 期待値は本家 nako_csv.mts の挙動と実測で突き合わせて決めた。
// 本家と異なる挙動を固定するケースには、その旨をコメントで明記する。

func edgeCall(t *testing.T, p *Plugin, name string, args ...value.Value) value.Value {
	t.Helper()
	v, err := p.Impls()[name](nil, args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return v
}

// edgeFetch は取得結果を、数値セルは "N:<値>"、文字列セルは "S:<生文字列>" とし、
// セルを |、行を / で連結した文字列にする。空セルや型の違いがそのまま見える。
func edgeFetch(t *testing.T, p *Plugin, command, input string) string {
	t.Helper()
	v := edgeCall(t, p, command, value.String(input))
	rows, ok := v.Array()
	if !ok || rows == nil {
		t.Fatalf("%s(%q) が配列を返しませんでした", command, input)
	}
	renderedRows := make([]string, 0, rows.Len())
	for i := 0; i < rows.Len(); i++ {
		row, ok := rows.Get(i).Array()
		if !ok || row == nil {
			t.Fatalf("%s(%q) の %d 行目が配列ではありません", command, input, i)
		}
		cells := make([]string, 0, row.Len())
		for j := 0; j < row.Len(); j++ {
			cell := row.Get(j)
			switch cell.Kind() {
			case value.KindNumber:
				f, _ := cell.Number()
				cells = append(cells, "N:"+strconv.FormatFloat(f, 'g', -1, 64))
			case value.KindString:
				s, _ := cell.String()
				cells = append(cells, "S:"+s)
			default:
				t.Fatalf("%s(%q) に想定外の型のセルがあります: %v", command, input, cell.Kind())
			}
		}
		renderedRows = append(renderedRows, strings.Join(cells, "|"))
	}
	return strings.Join(renderedRows, "/")
}

func edgeSetOption(t *testing.T, p *Plugin, key string, val value.Value) {
	t.Helper()
	dict := value.NewDict()
	dict.Set(key, val)
	edgeCall(t, p, "CSVオプション設定", value.DictValue(dict))
}

func TestParseEdgeNewlines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"空文字列", "", "S:"},
		{"改行のみ", "\n", "S:"},
		{"連続改行のみ", "\n\n\n", "S:"},
		{"末尾改行なし", "a,b", "S:a|S:b"},
		{"末尾改行あり", "a,b\n", "S:a|S:b"},
		{"改行コード混在", "a,b\r\nc,d\re,f\ng,h", "S:a|S:b/S:c|S:d/S:e|S:f/S:g|S:h"},
		{"引用符内改行の正規化", "\"a\r\nb\",c", "S:a\nb|S:c"},
		{"空行を保持", "a,b\n\n\nc,d", "S:a|S:b/S:/S:/S:c|S:d"},
		// 空白だけの行は空セル1個の行になる。本家は行ごと消える差異だが、Go仕様として固定する。
		{"空白だけの行", "a,b\n   \nc,d", "S:a|S:b/S:/S:c|S:d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := edgeFetch(t, New(), "CSV取得", tt.input)
			if got != tt.want {
				t.Errorf("CSV取得(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseEdgeQuotes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// 閉じていない引用符以降は捨てられる。本家も同じ互換仕様である。
		{"閉じていない引用符", "\"abc", ""},
		{"途中から閉じていない引用符", "a,\"bc", "S:a"},
		{"エスケープ途中で閉じていない引用符", "a,\"b\"\"c", "S:a"},
		{"空の引用セル", "\"\"", "S:"},
		// 閉じ引用符の直後が区切り・改行でない場合は、本家も結果が空になる互換仕様である。
		{"閉じ引用符の後ろに空白", "\"a\" ,b", ""},
		{"エスケープ引用符", "\"a\"\"\"", "S:a\""},
		// 本家と同じ互換仕様である。
		{"余分なエスケープ引用符", "\"a\"\"\"\"", ""},
		{"引用符三個", "\"\"\"", ""},
		{"引用符四個", "\"\"\"\"", "S:\""},
		{"引用符内の区切りと改行", "\"a,\"\"b\nc\"\",d\",e", "S:a,\"b\nc\",d|S:e"},
		{"セル途中の引用符", "a\"b,c", "S:a\"b|S:c"},
		{"引用符一個", "\"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := edgeFetch(t, New(), "CSV取得", tt.input)
			if got != tt.want {
				t.Errorf("CSV取得(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseEdgeDelimiters(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		delimiter string
		input     string
		want      string
	}{
		{"列数不揃い", "CSV取得", "", "a,b,c\nd,e\nf", "S:a|S:b|S:c/S:d|S:e/S:f"},
		{"CSV末尾空セル", "CSV取得", "", "a,b,", "S:a|S:b|S:"},
		{"連続CSV区切り", "CSV取得", "", "a,,b,,,c", "S:a|S:|S:b|S:|S:|S:c"},
		{"区切りのみ", "CSV取得", "", ",,,", "S:|S:|S:|S:"},
		{"TSV末尾空セル", "TSV取得", "", "a\tb\t", "S:a|S:b|S:"},
		{"セミコロン区切り", "CSV取得", ";", "a;b;", "S:a|S:b|S:"},
		{"全角縦線区切り", "CSV取得", "｜", "あ｜い｜", "S:あ|S:い|S:"},
		{"空白区切り", "CSV取得", " ", "a b ", "S:a|S:b|S:"},
		{"先頭空白", "CSV取得", "", " a, b", "S:a|S:b"},
		// 行中の末尾空白は残る非対称を固定する。
		{"行中の末尾空白", "CSV取得", "", "a ,b ", "S:a |S:b"},
		{"空白付き数値", "CSV取得", "", "12 ,13", "S:12 |N:13"},
		{"空白付き数値と文字列", "CSV取得", "", "a, 12 ,b", "S:a|S:12 |S:b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			if tt.delimiter != "" {
				edgeSetOption(t, p, "delimiter", value.String(tt.delimiter))
			}
			got := edgeFetch(t, p, tt.command, tt.input)
			if got != tt.want {
				t.Errorf("%s(%q) = %q, want %q", tt.command, tt.input, got, tt.want)
			}
		})
	}
}

func TestParseEdgeNumbers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"先頭ゼロ", "007", "N:7"},
		{"ゼロ", "0", "N:0"},
		{"負のゼロ", "-0", "N:-0"},
		{"指数表記", "1e3", "N:1000"},
		{"負の指数表記", "1E-3", "N:0.001"},
		{"正の指数表記", "1e+3", "N:1000"},
		{"負の小数", "-42.5", "N:-42.5"},
		// ParseFloatの範囲外は文字列のまま。本家はInfinityになるが、Go版はInfにしない仕様である。
		{"範囲外の指数表記", "1e400", "S:1e400"},
		{"小数点前省略", ".5", "S:.5"},
		{"小数点後省略", "5.", "S:5."},
		{"小数点複数", "1.2.3", "S:1.2.3"},
		{"先頭プラス", "+12", "S:+12"},
		{"全角数値", "１２３", "S:１２３"},
		{"十六進数風", "0x10", "S:0x10"},
		{"アンダースコア付き数値", "1_000", "S:1_000"},
		{"引用された桁区切り数値", "\"1,000\"", "S:1,000"},
		{"先頭空白付き数値", " 12", "N:12"},
		{"末尾空白付き数値", "12 ", "N:12"},
		{"大きな整数", "12345678901234567890", "N:1.2345678901234567e+19"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := edgeFetch(t, New(), "CSV取得", tt.input)
			if got != tt.want {
				t.Errorf("CSV取得(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseEdgeNumberRoundTrip(t *testing.T) {
	p := New()
	parsed := edgeCall(t, p, "CSV取得", value.String("007,-0,1e3,12345678901234567890"))
	got := value.ToString(edgeCall(t, p, "CSV変換", parsed))
	want := "7,0,1000,12345678901234567000\r\n"
	if got != want {
		t.Errorf("CSV変換(CSV取得(...)) = %q, want %q", got, want)
	}
}

func TestParseEdgeNumberOption(t *testing.T) {
	p := New()

	edgeSetOption(t, p, "auto_convert_number", value.Bool(false))
	if got, want := edgeFetch(t, p, "CSV取得", "007,1e3,12"), "S:007|S:1e3|S:12"; got != want {
		t.Errorf("数値自動変換無効時 = %q, want %q", got, want)
	}

	edgeSetOption(t, p, "auto_convert_number", value.Bool(true))
	if got, want := edgeFetch(t, p, "CSV取得", "007,1e3,12"), "N:7|N:1000|N:12"; got != want {
		t.Errorf("数値自動変換有効時 = %q, want %q", got, want)
	}
}

func TestParseEdgeExcelAndBytes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"Excel方言の数値と文字列", "=\"001\",=\"abc\"", "N:1|S:abc"},
		{"Excel方言の区切り文字", "=\"a,b\"", "S:a,b"},
		{"空白付きExcel方言", "= \"x\"", "S:= \"x\""},
		// BOMは先頭セルに残る。本家は除去する差異だが、Go仕様として固定する。
		{"BOMを保持", "\ufeffa,b", "S:\ufeffa|S:b"},
		{"NUL文字", "a\x00b,c", "S:a\x00b|S:c"},
		{"不正UTF8の非引用セル", "a\xffb,c", "S:a\xffb|S:c"},
		{"不正UTF8の引用セル", "\"a\xffb\",c", "S:a\xffb|S:c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := edgeFetch(t, New(), "CSV取得", tt.input)
			if got != tt.want {
				t.Errorf("CSV取得(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
