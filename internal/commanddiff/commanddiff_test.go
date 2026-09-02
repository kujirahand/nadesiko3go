package commanddiff

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

func TestCompare(t *testing.T) {
	source := `[
		{"plugin":"plugin_system","type":"関数","name":"共通"},
		{"plugin":"plugin_system","type":"関数","name":"本家のみ"},
		{"plugin":"plugin_system","type":"定数","name":"定数"},
		{"plugin":"plugin_node","type":"関数","name":"別プラグイン"}
	]`
	funcs := lexer.FuncList{
		"共通":   {Name: "共通", Type: "func"},
		"Goのみ": {Name: "Goのみ", Type: "func"},
		"Go定数": {Name: "Go定数", Type: "const"},
	}

	report, err := Compare(strings.NewReader(source), funcs)
	if err != nil {
		t.Fatal(err)
	}
	if report.OfficialCount != 2 || report.GoCount != 2 {
		t.Fatalf("件数が違います: %#v", report)
	}
	if got := strings.Join(report.Missing, ","); got != "本家のみ" {
		t.Fatalf("未実装が違います: %q", got)
	}
	if got := strings.Join(report.GoOnly, ","); got != "Goのみ" {
		t.Fatalf("Go版のみが違います: %q", got)
	}
}

func TestCompareRejectsInputWithoutPluginSystemCommands(t *testing.T) {
	_, err := Compare(strings.NewReader(`[]`), lexer.FuncList{})
	if err == nil || !strings.Contains(err.Error(), "plugin_system") {
		t.Fatalf("想定したエラーではありません: %v", err)
	}
}

func TestWriteText(t *testing.T) {
	var out bytes.Buffer
	WriteText(&out, "commands.json", "abc123", Report{
		OfficialCount: 2,
		GoCount:       2,
		Missing:       []string{"本家のみ"},
		GoOnly:        []string{"Goのみ"},
	})
	got := out.String()
	for _, want := range []string{"比較元: commands.json", "比較元コミット: abc123", "未実装 (1):\n  本家のみ", "Go版のみ (1):\n  Goのみ"} {
		if !strings.Contains(got, want) {
			t.Errorf("出力に%qがありません:\n%s", want, got)
		}
	}
}
