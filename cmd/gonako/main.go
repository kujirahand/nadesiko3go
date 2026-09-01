package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kujirahand/nadesiko3go/internal/compat"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

const usage = `gonako - なでしこ3 Go言語版

使い方:
  gonako run <ファイル> [引数...]   なでしこのプログラムを実行する
  gonako -e <プログラム>            その場でプログラムを実行する
  gonako compat run [--cases DIR] [--out DIR]

ファイル名に - を指定すると標準入力から読み込みます。
`

// runFile runs a program from a file. Everything after the file name is passed
// on to the program as its arguments.
func runFile(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("実行するファイルを指定してください")
	}
	host := vm.NewCUIHost(stdout, os.Stdin, args[1:])
	if args[0] == "-" {
		code, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("標準入力を読み込めません: %w", err)
		}
		return finish(vm.RunProgram(string(code), "main.nako3", host), host)
	}
	return finish(vm.RunFile(args[0], host), host)
}

// runInline runs a program given on the command line.
func runInline(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("実行するプログラムを指定してください")
	}
	host := vm.NewCUIHost(stdout, os.Stdin, args[1:])
	return finish(vm.RunProgram(args[0], "main.nako3", host), host)
}

// finish reports the program's outcome. 『終了』 with a non-zero status ends the
// process with that status rather than printing an error.
func finish(err error, host *vm.CUIHost) error {
	if err != nil {
		return err
	}
	if host.Exited && host.ExitCode != 0 {
		os.Exit(host.ExitCode)
	}
	return nil
}

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
	switch args[0] {
	case "run":
		return runFile(args[1:], stdout)
	case "-e":
		return runInline(args[1:], stdout)
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
	fmt.Fprintf(stdout, "%dグループ・%dケースを実行し、結果を%sへ出力しました\n", summary.Groups, summary.Cases, *outDir)
	return nil
}
