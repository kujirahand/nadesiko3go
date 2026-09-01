package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"path/filepath"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/compat"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

const usage = `gonako - なでしこ3 Go言語版

使い方:
  gonako run <ファイル> [引数...]   なでしこのプログラムを実行する
  gonako -e <プログラム>            その場でプログラムを実行する
  gonako build <ファイル> [オプション] 単一の実行ファイルに固める
  gonako compat run [--cases DIR] [--out DIR]

ファイル名に - を指定すると標準入力から読み込みます。

build のオプション:
  --out NAME       出力する実行ファイル名 (既定: ソース名から作る)
  --resource DIR   同梱するリソースのフォルダ
  --runtime PATH   土台にするランタイム (既定: 実行中のgonako)
                   他のOS向けのランタイムを指定すれば、そのOS向けに固められる
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

// buildBundle packs a program, and optionally a folder of resources, onto the
// end of a runtime executable.
func buildBundle(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "", "出力する実行ファイル名")
	resource := flags.String("resource", "", "同梱するリソースのフォルダ")
	runtimePath := flags.String("runtime", "", "土台にするランタイム")
	// ファイル名はオプションの前でも後ろでも書けるようにする。flagは最初の
	// 非フラグ引数で解析を止めるので、先に取り除いておく。
	source, rest := splitSource(args)
	if err := flags.Parse(rest); err != nil {
		return err
	}
	if source == "" || flags.NArg() > 0 {
		return errors.New("固めるファイルを1つ指定してください")
	}

	code, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("ファイル『%s』を読み込めません: %w", source, err)
	}
	prog, err := vm.CompileProgram(string(code), source)
	if err != nil {
		return err
	}

	if *runtimePath == "" {
		self, err := os.Executable()
		if err != nil {
			return fmt.Errorf("ランタイムの場所が分かりません: %w", err)
		}
		*runtimePath = self
	}
	if *out == "" {
		*out = defaultOutputName(source, *runtimePath)
	}

	if err := bundle.Build(*out, *runtimePath, prog, source, *resource); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s を作りました\n", *out)
	return nil
}

// splitSource pulls the first argument that is not a flag, or a flag's value,
// out of the argument list.
func splitSource(args []string) (source string, rest []string) {
	valueExpected := false
	for _, a := range args {
		switch {
		case valueExpected:
			valueExpected = false
		case strings.HasPrefix(a, "-"):
			// 『--out NAME』の形なら次の引数は値
			valueExpected = !strings.Contains(a, "=")
		case source == "":
			source = a
			continue
		}
		rest = append(rest, a)
	}
	return source, rest
}

// defaultOutputName drops the source extension, keeping the .exe suffix when
// the runtime being packed is a Windows one.
func defaultOutputName(source, runtimePath string) string {
	name := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	if strings.HasSuffix(runtimePath, ".exe") {
		return name + ".exe"
	}
	return name
}

// runBundled runs the program packed into this executable, if there is one.
// It reports whether it ran, so main can fall back to the ordinary commands.
func runBundled() (ran bool, err error) {
	self, err := os.Executable()
	if err != nil {
		return false, nil
	}
	packed, err := bundle.Open(self)
	if err != nil {
		return false, nil // 同梱されていない普通のgonako
	}
	defer packed.Close()

	host := vm.NewCUIHost(os.Stdout, os.Stdin, os.Args[1:])
	host.Bundle = packed
	return true, vm.RunCompiled(packed.Program, host)
}

func main() {
	// 自分自身にプログラムが同梱されていれば、それを実行する
	if ran, err := runBundled(); ran {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
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
	case "build":
		return buildBundle(args[1:], stdout, stderr)
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
