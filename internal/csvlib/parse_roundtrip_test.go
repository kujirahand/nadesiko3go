package csvlib

import (
	"math/rand"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// 以下の既知の差分はいずれも本家のnako_csv.mtsでも同じになる往復の仕様である。
func TestCSVRoundTripProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(373))
	parts := []string{
		"", "a", "abc", "あ", "なでしこ", "a b", "a,b", "a\"b", "a\nb",
		"x\ty", "007", "1e3", "a=b", "日本 語", ",", "\"", "\n", "a; b", "x=1",
	}

	p := New()
	edgeSetOption(t, p, "auto_convert_number", value.Bool(false))

	for n := 0; n < 300; n++ {
		rows := rng.Intn(6) + 1
		cols := rng.Intn(5) + 1
		table := make([][]string, rows)

		for i := range table {
			table[i] = make([]string, cols)
			for j := range table[i] {
				table[i][j] = parts[rng.Intn(len(parts))]
			}
			// 空セルだけの1セル行は、末尾にあると往復で畳まれる既知の差分
			// （TestCSVRoundTripKnownDiffs で固定）に当たるため、生成対象から外す。
			if len(table[i]) == 1 && table[i][0] == "" {
				table[i] = append(table[i], "a")
			}
		}

		csv := value.ToString(edgeCall(t, p, "CSV変換", roundTripTable(table)))
		got := roundTripStrings(t, edgeCall(t, p, "CSV取得", value.String(csv)))
		roundTripEqual(t, got, table)
	}
}

func TestCSVRoundTripKnownDiffs(t *testing.T) {
	p := New()
	edgeSetOption(t, p, "auto_convert_number", value.Bool(false))

	cases := []struct {
		name string
		in   [][]string
		want [][]string
	}{
		{
			name: "セル先頭の空白",
			// 取得時にセル先頭の空白が切り取られる。
			in:   [][]string{{" a", "x"}},
			want: [][]string{{"a", "x"}},
		},
		{
			name: "空白だけのセル",
			// 空白だけのセルは空文字列になる。
			in:   [][]string{{" ", "x"}},
			want: [][]string{{"", "x"}},
		},
		{
			name: "最終セルの末尾空白",
			// 入力末尾のトリムにより最終セルの末尾空白が切り取られる。
			in:   [][]string{{"x", "a "}},
			want: [][]string{{"x", "a"}},
		},
		{
			name: "CRの正規化",
			// CRは改行として正規化される。
			in:   [][]string{{"a\rb", "x"}},
			want: [][]string{{"a\nb", "x"}},
		},
		{
			name: "空セルだけの複数行",
			// 空セルだけの行は複数行あっても1行に畳まれる。
			in:   [][]string{{""}, {""}},
			want: [][]string{{""}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			csv := value.ToString(edgeCall(t, p, "CSV変換", roundTripTable(tc.in)))
			got := roundTripStrings(t, edgeCall(t, p, "CSV取得", value.String(csv)))
			roundTripEqual(t, got, tc.want)
		})
	}
}

func roundTripTable(table [][]string) value.Value {
	rows := make([]value.Value, len(table))
	for i, row := range table {
		cells := make([]value.Value, len(row))
		for j, cell := range row {
			cells[j] = value.String(cell)
		}
		rows[i] = value.ArrayValue(value.NewArray(cells...))
	}
	return value.ArrayValue(value.NewArray(rows...))
}

func roundTripStrings(t *testing.T, v value.Value) [][]string {
	t.Helper()

	rows, ok := v.Array()
	if !ok {
		t.Fatal("CSV取得の結果が配列ではありません")
	}

	result := make([][]string, rows.Len())
	for i := 0; i < rows.Len(); i++ {
		row, ok := rows.Get(i).Array()
		if !ok {
			t.Fatalf("行 %d が配列ではありません", i)
		}
		result[i] = make([]string, row.Len())
		for j := 0; j < row.Len(); j++ {
			cell, ok := row.Get(j).String()
			if !ok {
				t.Fatalf("行 %d、列 %d が文字列ではありません", i, j)
			}
			result[i][j] = cell
		}
	}
	return result
}

func roundTripEqual(t *testing.T, got, want [][]string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("行数 = %d, want %d: got=%q want=%q", len(got), len(want), got, want)
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("行 %d の列数 = %d, want %d: got=%q want=%q", i, len(got[i]), len(want[i]), got, want)
		}
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Fatalf("行 %d、列 %d = %q, want %q", i, j, got[i][j], want[i][j])
			}
		}
	}
}
