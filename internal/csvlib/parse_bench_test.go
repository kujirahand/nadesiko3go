package csvlib

import (
	"strings"
	"testing"
)

// benchCSV はベンチマーク用のCSVテキストを作る（引用符付きセル・空引用符セル・数値を混在させる）
func benchCSV(rows int) string {
	var sb strings.Builder
	for i := 0; i < rows; i++ {
		sb.WriteString(`123,"なでしこ,三","",abc,"改行
あり","""引用""",4.5` + "\n")
	}
	return sb.String()
}

func BenchmarkParse(b *testing.B) {
	txt := benchCSV(2000)
	p := New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.parse(txt, ",")
	}
}
