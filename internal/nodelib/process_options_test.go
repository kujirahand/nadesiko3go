package nodelib

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

func TestCommandOptionsChild(t *testing.T) {
	if os.Getenv("GONAKO_COMMAND_TEST_CHILD") != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			switch args[i+1] {
			case "output":
				os.Stdout.WriteString(args[i+2])
				os.Stderr.WriteString("標準エラー")
				os.Exit(7)
			case "wait":
				time.Sleep(5 * time.Second)
				os.Exit(0)
			}
		}
	}
	os.Exit(2)
}

func TestCommandOptions(t *testing.T) {
	t.Setenv("GONAKO_COMMAND_TEST_CHILD", "1")
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	options := value.NewDict()
	options.Set("実行ファイル", value.String(bin))
	literal := "空白 ' \" $(echo injected) & %PATH%"
	options.Set("引数", value.ArrayValue(value.NewArray(value.String("-test.run=TestCommandOptionsChild"), value.String("--"), value.String("output"), value.String(literal))))
	got, err := runCommandOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	result, _ := got.Dict()
	stdout, _ := result.Get("標準出力")
	stderr, _ := result.Get("標準エラー")
	code, _ := result.Get("終了コード")
	if value.ToString(stdout) != literal || value.ToString(stderr) != "標準エラー" || value.ToNumber(code) != 7 {
		t.Fatalf("出力と終了コード: %s", value.ToString(got))
	}
	options.Set("引数", value.ArrayValue(value.NewArray(value.String("-test.run=TestCommandOptionsChild"), value.String("--"), value.String("wait"))))
	options.Set("秒", value.Number(0.05))
	got, err = runCommandOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	result, _ = got.Dict()
	timedOut, _ := result.Get("時間切れ")
	if !value.ToBool(timedOut) {
		t.Fatal("時間切れになりませんでした")
	}
	options.Set("実行ファイル", value.String("gonako-missing-command-for-test"))
	got, err = runCommandOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	result, _ = got.Dict()
	code, _ = result.Get("終了コード")
	message, _ := result.Get("エラー")
	if value.ToNumber(code) != -1 || strings.TrimSpace(value.ToString(message)) == "" {
		t.Fatal("起動失敗を取得できませんでした")
	}
	options.Set("秒", value.Number(-1))
	if _, err := runCommandOptions(options); err == nil {
		t.Fatal("不正な秒を受け付けました")
	}
}
