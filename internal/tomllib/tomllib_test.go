package tomllib_test

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

func TestTOMLCommands(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{
			name: "TOMLデコード basic",
			code: `A=『title = "hoge"
num = 42』のTOMLデコード
A["title"]と"/"とA["num"]を文字列連結して表示`,
			want: "hoge/42",
		},
		{
			name: "TOML取得 is alias of decode",
			code: `A=『title = "hoge"』のTOML取得
A["title"]を表示`,
			want: "hoge",
		},
		{
			name: "TOMLエンコード basic",
			code: `A={"name":"けもの", "num":42}
A をTOMLエンコードして表示`,
			want: "name = \"けもの\"\nnum = 42",
		},
		{
			name: "TOML変換 is alias of encode",
			code: `A={"x":1}
A をTOML変換して表示`,
			want: "x = 1",
		},
		{
			name: "TOML往復変換",
			code: `A={"owner":{"name":"foo"}}
B=(AをTOMLエンコード)のTOMLデコード
B["owner"]["name"]を表示`,
			want: "foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runNako(t, tt.code); got != tt.want {
				t.Errorf("code=%q\n got=%q\nwant=%q", tt.code, got, tt.want)
			}
		})
	}
}
