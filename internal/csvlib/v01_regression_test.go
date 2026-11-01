package csvlib

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// V01: CSV の空区切りで停止不能、複数文字区切りで値が欠落する。
//
// 区切り文字の契約は「UTF-8で1文字」。設定境界（CSVオプション設定）で拒否し、
// 拒否後も既定のカンマ設定が壊れないことを、コマンドの戻り値だけで確かめる。

func v01Call(t *testing.T, p *Plugin, name string, args ...value.Value) (value.Value, error) {
	t.Helper()
	return p.Impls()[name](nil, args)
}

func v01SetDelimiter(t *testing.T, p *Plugin, delimiter string) error {
	t.Helper()
	opt := value.NewDict()
	opt.Set("delimiter", value.String(delimiter))
	_, err := v01Call(t, p, "CSVオプション設定", value.DictValue(opt))
	return err
}

// v01Fetch は取得コマンドの結果を、セルを |、行を / で連結した文字列にする。
// 空セルや引用セルの中身がそのまま見えるので、期待値をliteralで書ける。
func v01Fetch(t *testing.T, p *Plugin, command, csv string) string {
	t.Helper()
	v, err := v01Call(t, p, command, value.String(csv))
	if err != nil {
		t.Fatalf("%s(%q): %v", command, csv, err)
	}
	rows, ok := v.Array()
	if !ok || rows == nil {
		t.Fatalf("%s(%q) が配列を返しませんでした: %v", command, csv, v)
	}
	var b strings.Builder
	for i := 0; i < rows.Len(); i++ {
		if i > 0 {
			b.WriteString("/")
		}
		row, ok := rows.Get(i).Array()
		if !ok || row == nil {
			t.Fatalf("%s(%q) の%d行目が配列ではありません", command, csv, i)
		}
		for j := 0; j < row.Len(); j++ {
			if j > 0 {
				b.WriteString("|")
			}
			b.WriteString(value.ToString(row.Get(j)))
		}
	}
	return b.String()
}

func v01Stringify(t *testing.T, p *Plugin, cells ...string) string {
	t.Helper()
	row := make([]value.Value, len(cells))
	for i, cell := range cells {
		row[i] = value.String(cell)
	}
	v, err := v01Call(t, p, "CSV変換", value.ArrayValue(value.NewArray(value.ArrayValue(value.NewArray(row...)))))
	if err != nil {
		t.Fatalf("CSV変換: %v", err)
	}
	return value.ToString(v)
}

func TestV01EmptyDelimiterIsRejected(t *testing.T) {
	p := New()
	err := v01SetDelimiter(t, p, "")
	if err == nil {
		t.Fatal("空の区切り文字が設定できてしまいました")
	}
	if !strings.Contains(err.Error(), "1文字") {
		t.Fatalf("エラーに区切り文字の条件がありません: %v", err)
	}
	// 拒否した設定は残らず、通常のカンマ区切りで読み書きできる
	if got := v01Fetch(t, p, "CSV取得", "a,b"); got != "a|b" {
		t.Fatalf("空区切り拒否後のCSV取得 = %q, want %q", got, "a|b")
	}
	if got := v01Stringify(t, p, "a", "b"); got != "a,b\r\n" {
		t.Fatalf("空区切り拒否後のCSV変換 = %q, want %q", got, "a,b\r\n")
	}
}

func TestV01MultiCharDelimiterIsRejected(t *testing.T) {
	// 受理しない理由が同じなら、ASCIIでも日本語でも拒否する
	for _, delimiter := range []string{"||", "あい"} {
		t.Run(delimiter, func(t *testing.T) {
			p := New()
			if err := v01SetDelimiter(t, p, delimiter); err == nil {
				t.Fatalf("複数文字の区切り文字 %q が設定できてしまいました", delimiter)
			}
			// 値は欠落せず、既定のカンマ区切りでは1セルのまま残る
			in := "a" + delimiter + "b"
			if got := v01Fetch(t, p, "CSV取得", in); got != in {
				t.Fatalf("複数文字拒否後のCSV取得(%q) = %q, want %q", in, got, in)
			}
		})
	}
}

func TestV01SingleRuneDelimiterKeepsCells(t *testing.T) {
	p := New()
	if err := v01SetDelimiter(t, p, "｜"); err != nil {
		t.Fatalf("1文字の区切り文字が拒否されました: %v", err)
	}
	if got := v01Fetch(t, p, "CSV取得", "あ｜い"); got != "あ|い" {
		t.Fatalf("CSV取得 = %q, want %q", got, "あ|い")
	}
	if got := v01Stringify(t, p, "あ", "い"); got != "あ｜い\r\n" {
		t.Fatalf("CSV変換 = %q, want %q", got, "あ｜い\r\n")
	}
}

func TestV01CellParsingIsPreserved(t *testing.T) {
	tests := []struct {
		name string
		csv  string
		want string
	}{
		{"通常のカンマ", "a,b", "a|b"},
		{"引用セル内の区切り", `"a,b",c`, "a,b|c"},
		{"引用セル内の引用符", `a,"b""c"`, `a|b"c`},
		{"引用終了後の区切り", `"a",b`, "a|b"},
		{"空の引用セル", `"",a`, "|a"},
		{"空セルと空行", "a,,b\n\nc", "a||b//c"},
		{"引用セル内の改行", "a,\"b\nc\",d", "a|b\nc|d"},
		{"CRLFと日本語", "名前,値\r\nあ,1\r\n", "名前|値/あ|1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			if got := v01Fetch(t, p, "CSV取得", tt.csv); got != tt.want {
				t.Fatalf("CSV取得(%q) = %q, want %q", tt.csv, got, tt.want)
			}
		})
	}
}

func TestV01TSVKeepsTabDelimiter(t *testing.T) {
	p := New()
	if err := v01SetDelimiter(t, p, ";"); err != nil {
		t.Fatalf("区切り文字の設定に失敗しました: %v", err)
	}
	if got := v01Fetch(t, p, "TSV取得", "x\ty"); got != "x|y" {
		t.Fatalf("TSV取得 = %q, want %q", got, "x|y")
	}
	// TSV取得 は設定を書き換えない
	if got := v01Fetch(t, p, "CSV取得", "a;b"); got != "a|b" {
		t.Fatalf("CSV取得 = %q, want %q", got, "a|b")
	}
}
