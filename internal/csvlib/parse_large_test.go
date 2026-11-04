package csvlib

import (
	"strings"
	"testing"
)

func TestParseLargeManyRows(t *testing.T) {
	if testing.Short() {
		t.Skip("大きなCSV入力のテストを省略します")
	}

	const rows = 100000
	const line = "a,b,c,d,e,f,g,h,i,j"

	var input strings.Builder
	input.Grow(rows * (len(line) + 1))
	for i := 0; i < rows; i++ {
		if i > 0 {
			input.WriteByte('\n')
		}
		input.WriteString(line)
	}

	p := New()
	result, err := p.parse(input.String(), ",")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.Len() != rows {
		t.Fatalf("行数 = %d, want %d", result.Len(), rows)
	}

	cellCount := 0
	for i := 0; i < result.Len(); i++ {
		row, ok := result.Get(i).Array()
		if !ok {
			t.Fatalf("行 %d が配列ではありません", i)
		}
		cellCount += row.Len()
	}
	if cellCount != rows*10 {
		t.Fatalf("全セル数 = %d, want %d", cellCount, rows*10)
	}

	last, ok := result.Get(rows - 1).Array()
	if !ok {
		t.Fatal("最終行が配列ではありません")
	}
	first, ok := last.Get(0).String()
	if !ok || first != "a" {
		t.Fatalf("最終行の先頭セル = %q, want %q", first, "a")
	}
	end, ok := last.Get(9).String()
	if !ok || end != "j" {
		t.Fatalf("最終行の末尾セル = %q, want %q", end, "j")
	}
}

func TestParseLargeQuotedCell(t *testing.T) {
	if testing.Short() {
		t.Skip("大きなCSV入力のテストを省略します")
	}

	text := strings.Repeat("a", 1<<20)
	p := New()
	result, err := p.parse(`"`+text+`",x`, ",")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.Len() != 1 {
		t.Fatalf("行数 = %d, want 1", result.Len())
	}

	row, ok := result.Get(0).Array()
	if !ok {
		t.Fatal("行が配列ではありません")
	}
	if row.Len() != 2 {
		t.Fatalf("セル数 = %d, want 2", row.Len())
	}
	got, ok := row.Get(0).String()
	if !ok || got != text {
		t.Fatal("引用符付きセルの内容が元の1MiB文字列と一致しません")
	}
}

func TestParseLargeDelimiterRun(t *testing.T) {
	if testing.Short() {
		t.Skip("大きなCSV入力のテストを省略します")
	}

	p := New()
	result, err := p.parse(strings.Repeat(",", 100000), ",")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.Len() != 1 {
		t.Fatalf("行数 = %d, want 1", result.Len())
	}

	row, ok := result.Get(0).Array()
	if !ok {
		t.Fatal("行が配列ではありません")
	}
	if row.Len() != 100001 {
		t.Fatalf("セル数 = %d, want 100001", row.Len())
	}
}

func TestParseLargeUnclosedQuote(t *testing.T) {
	if testing.Short() {
		t.Skip("大きなCSV入力のテストを省略します")
	}

	p := New()
	result, err := p.parse(`"`+strings.Repeat("a", 1<<20), ",")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.Len() != 0 {
		t.Fatalf("行数 = %d, want 0", result.Len())
	}
}
