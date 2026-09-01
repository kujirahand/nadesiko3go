package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kujirahand/nadesiko3go/internal/compat"
)

const usage = `gonako - なでしこ3 Go言語版

使い方:
  gonako compat run [--cases DIR] [--out DIR]
`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return nil
	}
	if len(args) < 2 || args[0] != "compat" || args[1] != "run" {
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("不明なサブコマンドです: %s", args[0])
	}

	flags := flag.NewFlagSet("compat run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	casesDir := flags.String("cases", "./testdata/compat/cases", "compat case JSONのディレクトリ")
	outDir := flags.String("out", "./out", "結果JSONの出力ディレクトリ")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("余分な引数があります: %s", flags.Arg(0))
	}

	summary, err := compat.Run(*casesDir, *outDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%dグループ・%dケースを構文解析し、実行未実装結果として%sへ出力しました\n", summary.Groups, summary.Cases, *outDir)
	return nil
}
