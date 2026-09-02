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
	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/doctest"
	"github.com/kujirahand/nadesiko3go/internal/gogen"
	"github.com/kujirahand/nadesiko3go/internal/imagelib"
	"github.com/kujirahand/nadesiko3go/internal/officelib"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/pdflib"
	"github.com/kujirahand/nadesiko3go/internal/sqlitelib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

func init() {
	vm.RegisterPlugin(sqlitelib.New(), officelib.New(), pdflib.New(), imagelib.New())
}

// Version is the current release version of gonako.
var Version = "3.6.0"

const usage = `gonako - なでしこ3 Go言語版

使い方:
  gonako <ファイル> [引数...]       なでしこのプログラムを実行する
  gonako run <ファイル> [引数...]   なでしこのプログラムを実行する
  gonako -e <プログラム> [引数...]  その場でプログラムを実行する
  gonako build <ファイル> [オプション] 単一の実行ファイルに固める
  gonako gengo <ファイル> [オプション] Goソースに変換する（段階10・gogen）
  gonako doctest [パス...]          DocTestのサンプルを実行して確かめる
  gonako compat run [--cases DIR] [--out DIR]
  gonako version                    バージョン情報を表示する

ファイル名に - を指定すると標準入力から読み込みます。

build のオプション:
  --out NAME       出力する実行ファイル名 (既定: ソース名から作る)
  --resource DIR   同梱するリソースのフォルダ
  --runtime PATH   土台にするランタイム (既定: 実行中のgonako)
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
	// ファイル名はオプションの前でも後ろでも書けるようにする。flagは最初の
	// 非フラグ引数で解析を止めるので、先に取り除いておく。
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

// defaultGoGenPlugins matches this binary's own registry (init above plus
// vm's own defaultPlugins), so a program compiled for the ordinary gonako
// runtime compiles for gengo's output the same way, command for command
// (AGENTS.md §12: the registry gengo compiles the source against and the one
// generated code builds at run time must agree exactly, or a command's ID
// silently resolves to the wrong one).
var defaultGoGenPlugins = []string{"nodelib", "csvlib", "mathlib", "sqlitelib", "officelib", "pdflib", "imagelib"}

// genGo writes a Go source file that, once built with `go build`, is the
// program — no Go toolchain needed to receive it, only to make it (AGENTS.md
// §10, §12).
func genGo(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("gengo", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "", "出力するGoソースファイル名")
	pluginList := flags.String("plugins", strings.Join(defaultGoGenPlugins, ","),
		"生成コードのレジストリに含めるプラグイン（カンマ区切り、空文字で標準命令のみ）")
	pkgName := flags.String("package", "main", "生成するGoソースのパッケージ名")
	source, rest := splitSourceFor(args, "out", "plugins", "package")
	if err := flags.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if source == "" && flags.NArg() > 0 {
		source = flags.Arg(0)
	}
	if source == "" || flags.NArg() > 0 {
		return errors.New("Go変換するファイルを1つ指定してください")
	}

	var plugins []string
	if *pluginList != "" {
		plugins = strings.Split(*pluginList, ",")
	}
	registry, err := gogen.BuildRegistry(plugins)
	if err != nil {
		return err
	}

	code, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("ファイル『%s』を読み込めません: %w", source, err)
	}
	tree, err := parser.ParseSource(string(code), source, registry.FuncList())
	if err != nil {
		return err
	}
	prog, err := compiler.Compile(tree, source, registry)
	if err != nil {
		return err
	}

	src, err := gogen.Generate(prog, gogen.Options{Package: *pkgName, Plugins: plugins})
	if err != nil {
		return err
	}

	if *out == "" {
		*out = strings.TrimSuffix(filepath.Base(source), filepath.Ext(source)) + ".go"
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		return fmt.Errorf("出力ファイル『%s』を書き出せません: %w", *out, err)
	}
	fmt.Fprintf(stdout, "%s を作りました\n", *out)
	fmt.Fprintf(stdout, "ビルドするには、%s と同じ場所に go.mod を用意し、以下を requireとreplaceに書いてください:\n", *out)
	fmt.Fprintf(stdout, "  require github.com/kujirahand/nadesiko3go v0.0.0\n")
	fmt.Fprintf(stdout, "  replace github.com/kujirahand/nadesiko3go => %s\n", mustAbs(mustExecutableModuleHint()))
	return nil
}

// mustExecutableModuleHint is a best-effort guess at this checkout's own root,
// for the replace directive genGo suggests. It is only ever wrong in a way
// that leaves the user with an obviously-not-a-path placeholder to edit, never
// a silently broken build.
func mustExecutableModuleHint() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "/path/to/nadesiko3go"
}

func mustAbs(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// splitSource pulls the first argument that is not a flag, or a flag's value,
// out of the argument list.
func splitSource(args []string) (source string, rest []string) {
	return splitSourceFor(args, "out", "resource", "runtime")
}

// splitSourceFor is splitSource, generalised over which flags take a value —
// buildBundle and genGo each recognise a different set.
func splitSourceFor(args []string, valueFlags ...string) (source string, rest []string) {
	valueExpected := false
	for _, a := range args {
		switch {
		case valueExpected:
			valueExpected = false
		case strings.HasPrefix(a, "-"):
			// 『--out NAME』などの形なら次の引数は値
			name := strings.TrimLeft(strings.Split(a, "=")[0], "-")
			valueExpected = !strings.Contains(a, "=") && containsString(valueFlags, name)
		case source == "":
			source = a
			continue
		}
		rest = append(rest, a)
	}
	return source, rest
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
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

// defaultDocTestTargets are the manual and repository-owned fixtures used when
// no path is given. Keeping fixed tests outside manual makes them available in
// environments where the nadesiko3doc symlink is absent.
var defaultDocTestTargets = []string{"manual/plugin_system", "testdata/doctest"}

// runDocTests runs sample code from the manual and fixed test data, then
// reports what did not match. Failures are summarised by reason and only the
// first few are shown.
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

// existingDocTestTargets lets the optional manual symlink be absent while the
// repository-owned fixtures continue to run. An explicitly supplied missing
// path is still reported by doctest.Collect.
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
	return true, finish(vm.RunCompiled(packed.Program, host), host)
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
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "gonako v%s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		return nil
	case "run":
		return runFile(args[1:], stdout)
	case "-e":
		return runInline(args[1:], stdout)
	case "build":
		return buildBundle(args[1:], stdout, stderr)
	case "gengo":
		return genGo(args[1:], stdout, stderr)
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

	// ファイル直接指定の場合 (例: gonako main.nako3, gonako -)
	if args[0] == "-" || strings.HasSuffix(args[0], ".nako3") || strings.HasSuffix(args[0], ".nako") {
		return runFile(args, stdout)
	}
	if _, err := os.Stat(args[0]); err == nil {
		return runFile(args, stdout)
	}

	fmt.Fprint(stderr, usage)
	return fmt.Errorf("不明なサブコマンドです: %s", args[0])
}
