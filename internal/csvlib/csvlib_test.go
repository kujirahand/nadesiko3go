package csvlib_test

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

func runNako(t *testing.T, code string) string {
	t.Helper()
	var out strings.Builder
	h := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	if err := vm.RunProgram(code, "main.nako3", h); err != nil {
		t.Fatalf("run error: %v", err)
	}
	return strings.TrimRight(out.String(), " \t\r\n")
}

func TestCSVCommands(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{
			name: "CSV取得 basic",
			code: `a=「1,2,3
4,5,6」のCSV取得。a[1][2]を表示`,
			want: "6",
		},
		{
			name: "CSV取得 quotes and newlines",
			code: `a=「"a",b,c
""a,b,c
a,""b,c
a,b,c""
"a,
b",c,d
a,"b,
c",d
a,b,"c,
d"」のCSV取得。a[5][1]を表示`,
			want: "b,\nc",
		},
		{
			name: "CSV取得 escaped quote 1",
			code: `a=「1,"a""a",2」のCSV取得。a[0][1]を表示`,
			want: `a"a`,
		},
		{
			name: "CSV取得 escaped quote 2",
			code: `a=「1,"2""2",3
4,5,6」のCSV取得。a[0][1]を表示`,
			want: `2"2`,
		},
		{
			name: "CSV取得 empty cell",
			code: `a=「1,,3
4,5,6」のCSV取得。a[0][2]を表示`,
			want: "3",
		},
		{
			name: "CSV取得 trailing comma",
			code: `a=「1,2,3,
4,5,6」のCSV取得。a[1][0]を表示`,
			want: "4",
		},
		{
			name: "TSV取得 basic",
			code: "a=「1\t2\t3\n4\t5\t6」のTSV取得。a[1][2]を表示",
			want: "6",
		},
		{
			name: "TSV取得 quotes and newlines",
			code: "a=「\"a\"\tb\tc\n\"\"a\tb\tc\na\t\"\"b\tc\na\tb\tc\"\"\n\"a\t\nb\"\tc\td\na\t\"b\t\nc\"\td\na\tb\t\"c\t\nd\"」のTSV取得。a[5][1]を表示",
			want: "b\t\nc",
		},
		{
			name: "TSV取得 escaped quote 1",
			code: "a=「1\t\"a\"\"a\"\t2」のTSV取得。a[0][1]を表示",
			want: `a"a`,
		},
		{
			name: "TSV取得 escaped quote 2",
			code: "a=「1\t\"2\"\"2\"\t3\n4\t5\t6」のTSV取得。a[0][1]を表示",
			want: `2"2`,
		},
		{
			name: "TSV取得 empty cell",
			code: "a=「1\t\t3\n4\t5\t6」のTSV取得。a[0][2]を表示",
			want: "3",
		},
		{
			name: "表CSV変換 basic",
			code: `[[1,2,3],[4,5,6]]を表CSV変換して表示`,
			want: "1,2,3\r\n4,5,6",
		},
		{
			name: "表CSV変換 quotes and comma",
			code: "[[1,2,\"3\r\n,\"],[4,5,6]]を表CSV変換して表示",
			want: "1,2,\"3\r\n,\"\r\n4,5,6",
		},
		{
			name: "CSV変換 alias",
			code: `[[1,2,3],[4,5,6]]をCSV変換して表示`,
			want: "1,2,3\r\n4,5,6",
		},
		{
			name: "CSV変換 quotes and comma",
			code: "[[1,2,\"3\r\n,\"],[4,5,6]]をCSV変換して表示",
			want: "1,2,\"3\r\n,\"\r\n4,5,6",
		},
		{
			name: "表TSV変換 basic",
			code: `[[1,2,3],[4,5,6]]を表TSV変換して表示`,
			want: "1\t2\t3\r\n4\t5\t6",
		},
		{
			name: "表TSV変換 quotes",
			code: "[[1,2,\"3\r\n\t\"],[4,5,6]]を表TSV変換して表示",
			want: "1\t2\t\"3\r\n\t\"\r\n4\t5\t6",
		},
		{
			name: "TSV変換 basic",
			code: `[[1,2,3],[4,5,6]]をTSV変換して表示`,
			want: "1\t2\t3\r\n4\t5\t6",
		},
		{
			name: "TSV変換 quotes",
			code: "[[1,2,\"3\r\n\t\"],[4,5,6]]をTSV変換して表示",
			want: "1\t2\t\"3\r\n\t\"\r\n4\t5\t6",
		},
		{
			name: "日付形式を実数として誤判定しない #1910",
			code: `a=「2024.01.01,200,300
4,5,6」のCSV取得。a[0][0]を表示`,
			want: "2024.01.01",
		},
		{
			name: "小数は実数として判定される",
			code: `a=「3.14,200,300
4,5,6」のCSV取得。a[0][0]を表示`,
			want: "3.14",
		},
		{
			name: "TYPEOF 判定",
			code: `a=「3.14,200,300
4,5,6」のCSV取得。TYPEOF(a[0][0])を表示`,
			want: "number",
		},
		{
			name: "日付形式文字列のTYPEOF判定",
			code: `a=「2010.1.5,200,300
4,5,6」のCSV取得。TYPEOF(a[0][0])を表示`,
			want: "string",
		},
		{
			name: "CSVオプション設定 auto_convert_number false",
			code: `{"auto_convert_number": 0}をCSVオプション設定;a=「2024.01,200,300
4,5,6」のCSV取得。TYPEOF(a[0][0])を表示`,
			want: "string",
		},
		{
			name: "データからCSV変換 (から助詞)",
			code: `データ = [
  ["名前", "点数"],
  ["太郎", 85],
  ["次郎", 92],
  ["花子", 78]
]
CsvText = データからCSV変換
CsvTextを表示`,
			want: "名前,点数\r\n太郎,85\r\n次郎,92\r\n花子,78",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runNako(t, tt.code)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

