package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/compat"
	"github.com/kujirahand/nadesiko3go/internal/doctest"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// Version is the current release version of gonako-cui.
var Version = "3.6.0"

const usage = `gonako-cui - なでしこ3 Go言語版 (軽量CUI版)

使い方:
  gonako-cui <ファイル> [引数...]       なでしこのプログラムを実行する
  gonako-cui run <ファイル> [引数...]   なでしこのプログラムを実行する
  gonako-cui -e <プログラム> [引数...]  その場でプログラムを実行する
  gonako-cui build <ファイル> [オプション] 単一の実行ファイルに固める
  gonako-cui doctest [パス...]          DocTestのサンプルを実行して確かめる
  gonako-cui compat run [--cases DIR] [--out DIR]
  gonako-cui version                    バージョン情報を表示する

ファイル名に - を指定すると標準入力から読み込みます。

build のオプション:
  --out NAME       出力する実行ファイル名 (既定: ソース名から作る)
  --resource DIR   同梱するリソースのフォルダ
  --runtime PATH   土台にするランタイム (既定: 実行中のgonako-cui)
                   他のOS向けのランタイムを指定すれば、そのOS向けに固められる
  --list           同梱されているリソースの一覧を表示する

doctest のオプション:
  --max N          失敗の詳細を表示する件数 (既定: 10、0で全件)
  パスを省略すると manual/plugin_system と testdata/doctest を対象にします。
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
	list := flags.Bool("list", false, "同梱されているリソースの一覧を表示する")
	source, rest := splitSource(args)
	if err := flags.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if source == "" && flags.NArg() > 0 {
		source = flags.Arg(0)
	}
	if *list {
		if source == "" {
			return errors.New("一覧を表示する実行ファイルを指定してください")
		}
		packed, err := bundle.Open(source)
		if err != nil {
			return fmt.Errorf("バンドルを読み込めません: %w", err)
		}
		defer packed.Close()
		fmt.Fprintf(stdout, "プログラム: %s\n", packed.Name)
		resources := packed.Resources()
		if len(resources) == 0 {
			fmt.Fprintln(stdout, "同梱リソース: なし")
		} else {
			fmt.Fprintln(stdout, "同梱リソース:")
			for _, r := range resources {
				fmt.Fprintf(stdout, "  %s\n", r)
			}
		}
		return nil
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

func splitSource(args []string) (source string, rest []string) {
	valueExpected := false
	for _, a := range args {
		switch {
		case valueExpected:
			valueExpected = false
		case strings.HasPrefix(a, "-"):
			name := strings.TrimLeft(strings.Split(a, "=")[0], "-")
			valueExpected = !strings.Contains(a, "=") && (name == "out" || name == "resource" || name == "runtime")
		case source == "":
			source = a
			continue
		}
		rest = append(rest, a)
	}
	return source, rest
}

func defaultOutputName(source, runtimePath string) string {
	name := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	if strings.HasSuffix(runtimePath, ".exe") {
		return name + ".exe"
	}
	return name
}

var defaultDocTestTargets = []string{"manual/plugin_system", "testdata/doctest"}

func runDocTests(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("doctest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	max := flags.Int("max", 10, "失敗の詳細を表示する件数 (0で全件)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	targets := flags.Args()
	if len(targets) == 0 {
		targets = existingDocTestTargets(defaultDocTestTargets)
	}

	tests, err := doctest.Collect(targets, doctest.CNako)
	if err != nil {
		return fmt.Errorf("DocTest対象を読み込めません: %w", err)
	}
	if len(tests) == 0 {
		fmt.Fprintln(stdout, "[DocTest] 対象のサンプルコードがありません。")
		return nil
	}
	fmt.Fprintf(stdout, "[DocTest] %d件のサンプルコードを実行します。\n", len(tests))

	root, _ := os.Getwd()
	shown, passed, skipped := 0, 0, 0
	skipReasons := map[string]int{}
	byReason := map[doctest.Failure]int{}
	for _, test := range tests {
		result := doctest.Run(test)
		if result.Skipped {
			skipped++
			skipReasons[result.SkipReason]++
			continue
		}
		if result.OK {
			passed++
			continue
		}
		byReason[doctest.Classify(result)]++
		if *max == 0 || shown < *max {
			shown++
			fmt.Fprintln(stderr, doctest.FormatFailure(test, result, root))
			fmt.Fprintln(stderr)
		}
	}

	failed := len(tests) - passed - skipped
	if failed == 0 {
		fmt.Fprintf(stdout, "[DocTest] %d件成功・%d件省略・失敗なし。\n", passed, skipped)
		for _, reason := range sortedCountKeys(skipReasons) {
			fmt.Fprintf(stdout, "  %s: %d件\n", reason, skipReasons[reason])
		}
		return nil
	}
	if shown < failed {
		fmt.Fprintf(stderr, "(ほか %d件の失敗は省略しました。--max 0 で全件表示します)\n\n", failed-shown)
	}
	fmt.Fprintf(stdout, "[DocTest] %d/%d件成功 (%.1f%%)\n", passed, len(tests),
		float64(passed)/float64(len(tests))*100)
	if skipped > 0 {
		fmt.Fprintf(stdout, "  省略: %d件\n", skipped)
	}
	for _, reason := range []doctest.Failure{
		doctest.FailMissingCommand, doctest.FailMismatch,
		doctest.FailSyntax, doctest.FailRuntime,
	} {
		if n := byReason[reason]; n > 0 {
			fmt.Fprintf(stdout, "  %s: %d件\n", reason, n)
		}
	}
	return fmt.Errorf("DocTestが%d件失敗しました", failed)
}

func existingDocTestTargets(targets []string) []string {
	existing := make([]string, 0, len(targets))
	for _, target := range targets {
		if _, err := os.Stat(target); err == nil || !os.IsNotExist(err) {
			existing = append(existing, target)
		}
	}
	return existing
}

func sortedCountKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func runBundled() (ran bool, err error) {
	self, err := os.Executable()
	if err != nil {
		return false, nil
	}
	packed, err := bundle.Open(self)
	if err != nil {
		return false, nil
	}
	defer packed.Close()

	host := vm.NewCUIHost(os.Stdout, os.Stdin, os.Args[1:])
	host.Bundle = packed
	return true, finish(vm.RunCompiled(packed.Program, host), host)
}

func main() {
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
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "gonako-cui v%s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		return nil
	case "run":
		return runFile(args[1:], stdout)
	case "-e":
		return runInline(args[1:], stdout)
	case "build":
		return buildBundle(args[1:], stdout, stderr)
	case "doctest":
		return runDocTests(args[1:], stdout, stderr)
	}

	if len(args) >= 2 && args[0] == "compat" && args[1] == "run" {
		flags := flag.NewFlagSet("compat run", flag.ContinueOnError)
		flags.SetOutput(stderr)
		casesDir := flags.String("cases", "./testdata/compat/cases", "compat case JSONのディレクトリ")
		outDir := flags.String("out", "./out", "結果JSONの出力ディレクトリ")
		if err := flags.Parse(args[2:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
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

	if args[0] == "-" || strings.HasSuffix(args[0], ".nako3") || strings.HasSuffix(args[0], ".nako") {
		return runFile(args, stdout)
	}
	if _, err := os.Stat(args[0]); err == nil {
		return runFile(args, stdout)
	}

	fmt.Fprint(stderr, usage)
	return fmt.Errorf("不明なサブコマンドです: %s", args[0])
}
