package main

// gonako-gui のサブコマンド (Issue #551)。
//
// 従来の gonako-gui は「引数なし＝エディタ」「ファイル・フォルダ・URLを
// 位置引数で渡す」だけだったが、gonako (CUI版) と同じサブコマンド体系を
// 揃える。GUIを起動しないサブコマンド (help / version / doc / doctest /
// lint / format / build / gengo / install / compat) はこのファイルで処理し、
// GUIを起動するもの (run / edit / -e) は main 側で処理する。
//
// 互換性のため、サブコマンドでない従来の引数 (ファイル・フォルダ・URL・
// 各種 -フラグ) は今までどおり受け付ける。

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/commanddiff"
	"github.com/kujirahand/nadesiko3go/internal/commanddoc"
	"github.com/kujirahand/nadesiko3go/internal/compat"
	"github.com/kujirahand/nadesiko3go/internal/compiler"
	"github.com/kujirahand/nadesiko3go/internal/doctest"
	"github.com/kujirahand/nadesiko3go/internal/format"
	"github.com/kujirahand/nadesiko3go/internal/gogen"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// subcommandNames は help の一覧と未知コマンド判定に使う公開サブコマンド。
var subcommandNames = []string{
	"run", "edit", "help", "version", "doc", "doctest",
	"lint", "format", "build", "gengo", "install", "compat",
}

// subcommandUsage はサブコマンドごとの使い方。`gonako-gui help <名前>` と
// `gonako-gui <名前> --help` で表示する。
var subcommandUsage = map[string]string{
	"run": `gonako-gui run <ファイル.nako3> - GUIプログラムをエディタを開かずに直接実行する

指定したファイルは削除されません。ファイルはちょうど1つだけ指定します。
作業ディレクトリはファイルのあるフォルダになります。
`,
	"edit": `gonako-gui edit [ファイル] - 内蔵エディタで開く

ファイルを指定しない場合は、新規プログラムでエディタを起動します。
従来どおり gonako-gui <ファイル> でも同じ動作になります。
`,
	"version": `gonako-gui version - バージョン情報を表示する (-v / --version / -version も同じ)
`,
	"doc": `gonako-gui doc <キーワード> [オプション] - 命令やマニュアルを検索する

オプション:
  --command, -c    命令一覧(JSON)から検索する (既定)
  --web, -w        Webのマニュアル(https://nadesi.com/v3/doc/)も検索する
  --wnako          本家ブラウザ版(wnako3)の命令一覧から検索する
  --json           結果をJSONで出力する (AI向け)
  --limit N        表示する件数の上限 (既定: 20、0で全件)
`,
	"doctest": `gonako-gui doctest [オプション] [パス...] - DocTestのサンプルを実行して確かめる

オプション:
  --max N          失敗の詳細を表示する件数 (既定: 10、0で全件)
  --runtime PATH   サンプルを実行するランタイム (既定: 内蔵VM)
  --subcommand NAME ランタイムに渡すサブコマンド (例: run)
  --label NAME     対象にする表示結果ラベル (カンマ区切りで複数指定可)
  --json[=FILE]    失敗した結果をJSONで出力する (ファイル名を省略すると標準出力)
  --usage          使い方を表示する
パスを省略すると manual/plugin_system と manual/gonako と testdata/doctest を対象にします。
`,
	"lint": `gonako-gui lint <ファイル> - 文法をチェックする (実行はしません)
`,
	"format": `gonako-gui format <ファイル> [オプション] - コードを整形する

オプション:
  --force, -f      結果をファイルへ書き戻す (省略時は標準出力に表示するだけ)
  --colon          『ここまで』で閉じるブロックをコロン記法に書き換える
`,
	"build": `gonako-gui build <ファイル> [オプション] - 単一の実行ファイルに固める

土台にするランタイムは gonako-gui 自身なので、固めたファイルは
GUIアプリとして起動します (gonako build の GUI版)。

オプション:
  --out NAME       出力する実行ファイル名 (既定: ソース名から作る)
  --resource DIR   同梱するリソースのフォルダ
  --runtime PATH   土台にするランタイム (既定: 実行中の gonako-gui)
  --list           同梱されているリソースの一覧を表示する
  --include-symlink フォルダ外を指すシンボリックリンクも辿って梱包する
`,
	"gengo": `gonako-gui gengo <ファイル> [オプション] - Goソースに変換する

オプション:
  --out NAME       出力するGoソースファイル名 (既定: ソース名から作る)
  --plugins LIST   生成コードに含めるプラグイン (カンマ区切り)
  --package NAME   生成するGoソースのパッケージ名 (既定: main)
`,
	"install": `gonako-gui install <ファイル> [オプション] - bin/に起動用スクリプトを配置する

配置したスクリプトは gonako-gui run <ファイル> で起動します。

オプション:
  --bin DIR        配置先 (既定: カレントディレクトリのbin/)
  --name NAME      コマンド名 (既定: gonako-gui-ソース名)
  --runtime PATH   使用する gonako-gui (既定: 現在の実行ファイル)
  --force          既存の起動用ファイルを置き換える
`,
	"compat": `gonako-gui compat <サブコマンド> - 本家との互換性チェック

  gonako-gui compat run [--cases DIR] [--out DIR]   差分fixtureを実行する
  gonako-gui compat commands [--source FILE]        命令一覧を本家と比較する
`,
	"help": `gonako-gui help [サブコマンド] - ヘルプを表示する
`,
}

// runSubcommand は GUI を起動しないサブコマンドを処理する。
// 処理した場合は handled=true を返し、err が終了コード1に相当する。
// サブコマンドでない引数 (従来のファイル・フォルダ・URL・フラグ) の場合は
// handled=false を返し、呼び出し元 (main) が従来どおり処理する。
func runSubcommand(args []string, stdout, stderr io.Writer) (handled bool, err error) {
	if len(args) == 0 {
		return false, nil
	}
	switch args[0] {
	case "help":
		if len(args) == 1 {
			fmt.Fprint(stdout, usage)
			return true, nil
		}
		if text, ok := subcommandUsage[args[1]]; ok {
			fmt.Fprint(stdout, text)
			return true, nil
		}
		return true, fmt.Errorf("不明なサブコマンドです: %s", args[1])
	case "-h", "--help":
		fmt.Fprint(stdout, usage)
		return true, nil
	case "version", "-v", "--version":
		fmt.Fprintln(stdout, guiVersionText())
		return true, nil
	case "doc":
		return true, searchDocGUI(args[1:], stdout, stderr)
	case "doctest":
		return true, runDocTestsGUI(args[1:], stdout, stderr)
	case "lint":
		return true, lintFileGUI(args[1:], stdout)
	case "format":
		return true, formatFileGUI(args[1:], stdout, stderr)
	case "build":
		return true, buildBundleGUI(args[1:], stdout, stderr)
	case "gengo":
		return true, genGoGUI(args[1:], stdout, stderr)
	case "install":
		return true, installScriptGUI(args[1:], stdout, stderr)
	case "compat":
		return true, runCompatGUI(args[1:], stdout, stderr)
	}
	// 未知の単語は、存在しないファイル名としてではなく、未知の
	// サブコマンドとして知らせる (gonako と同じ)。パス区切りや拡張子を
	// 含むもの・実在するものは従来の引数として main に任せる。
	if !strings.HasPrefix(args[0], "-") && !strings.ContainsAny(args[0], "/\\.") &&
		!strings.HasPrefix(args[0], "http://") && !strings.HasPrefix(args[0], "https://") {
		if _, statErr := os.Stat(args[0]); statErr != nil {
			fmt.Fprint(stderr, usage)
			return true, fmt.Errorf("不明なサブコマンドです: %s", args[0])
		}
	}
	return false, nil
}

// isSubcommandHelp は `gonako-gui <サブコマンド> --help` の形かどうか。
// GUIを起動するサブコマンド (run / edit) の --help もここで受ける。
func isSubcommandHelp(args []string) (name string, ok bool) {
	if len(args) == 2 && (args[1] == "--help" || args[1] == "-h") {
		if _, exists := subcommandUsage[args[0]]; exists {
			return args[0], true
		}
	}
	return "", false
}

// --- doc (gonako doc と同じ検索。internal/commanddoc を使う) ---

type docResultGUI struct {
	Keywords []string               `json:"keywords"`
	Target   string                 `json:"target"`
	Count    int                    `json:"count"`
	Commands []commanddoc.Command   `json:"commands"`
	Web      []commanddoc.WebResult `json:"web,omitempty"`
	WebError string                 `json:"web_error,omitempty"`
}

func searchDocGUI(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("doc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var commandOnly, web, asJSON, wnako bool
	flags.BoolVar(&commandOnly, "command", false, "命令一覧から検索する")
	flags.BoolVar(&commandOnly, "c", false, "命令一覧から検索する")
	flags.BoolVar(&web, "web", false, "Webのマニュアルも検索する")
	flags.BoolVar(&web, "w", false, "Webのマニュアルも検索する")
	flags.BoolVar(&wnako, "wnako", false, "wnako3の命令一覧から検索する")
	flags.BoolVar(&asJSON, "json", false, "結果をJSONで出力する")
	limit := flags.Int("limit", 20, "表示する件数の上限")
	keywords, rest := splitDocKeywords(args, "limit")
	if err := flags.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	keywords = append(keywords, flags.Args()...)
	if len(keywords) == 0 {
		return errors.New("検索するキーワードを指定してください (例: gonako-gui doc 秒待)")
	}
	list, err := commanddoc.Commands()
	if wnako {
		list, err = commanddoc.WNakoCommands()
	}
	if err != nil {
		return err
	}
	found := commanddoc.Search(list, keywords)
	total := len(found)
	if *limit > 0 && len(found) > *limit {
		found = found[:*limit]
	}
	target := "gonako"
	if wnako {
		target = "wnako"
	}
	result := docResultGUI{Keywords: keywords, Target: target, Count: total, Commands: found}
	if result.Commands == nil {
		result.Commands = []commanddoc.Command{}
	}
	if web {
		results, werr := commanddoc.SearchWeb(keywords)
		if werr != nil {
			result.WebError = werr.Error()
		}
		result.Web = results
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(result)
	}
	writeDocTextGUI(stdout, stderr, result, web, total-len(found))
	return nil
}

func writeDocTextGUI(stdout, stderr io.Writer, result docResultGUI, web bool, omitted int) {
	keyword := strings.Join(result.Keywords, " ")
	target := ""
	if result.Target == "wnako" {
		target = "(wnako) "
	}
	if len(result.Commands) == 0 {
		fmt.Fprintf(stdout, "%s『%s』に一致する命令はありませんでした。\n", target, keyword)
	} else {
		fmt.Fprintf(stdout, "%s『%s』に一致する命令: %d件\n\n", target, keyword, result.Count)
	}
	for _, cmd := range result.Commands {
		fmt.Fprintf(stdout, "■ %s%s\n", cmd.Name, formatDocOrigin(cmd))
		if cmd.Template != "" && cmd.Template != cmd.Name {
			fmt.Fprintf(stdout, "  書式: %s\n", cmd.Template)
		}
		if cmd.Yomi != "" {
			fmt.Fprintf(stdout, "  読み: %s\n", cmd.Yomi)
		}
		if cmd.Desc != "" {
			fmt.Fprintf(stdout, "  説明: %s\n", cmd.Desc)
		}
		fmt.Fprintf(stdout, "  マニュアル: %s\n", cmd.DocURL)
		if cmd.File != "" {
			fmt.Fprintf(stdout, "  ソース: %s:%d\n", cmd.File, cmd.Line)
		}
		fmt.Fprintln(stdout)
	}
	if omitted > 0 {
		fmt.Fprintf(stdout, "(ほか %d件は省略しました。--limit 0 で全件表示します)\n\n", omitted)
	}
	if !web {
		return
	}
	if result.WebError != "" {
		fmt.Fprintf(stderr, "Web検索に失敗しました: %s\n", result.WebError)
		return
	}
	if len(result.Web) == 0 {
		fmt.Fprintf(stdout, "Webのマニュアルに『%s』の該当ページはありませんでした。\n", keyword)
		return
	}
	fmt.Fprintf(stdout, "Webのマニュアル (%d件):\n", len(result.Web))
	for _, page := range result.Web {
		fmt.Fprintf(stdout, "  %s\n    %s\n", page.Title, page.URL)
	}
}

func formatDocOrigin(cmd commanddoc.Command) string {
	var parts []string
	if cmd.Plugin != "" {
		parts = append(parts, cmd.Plugin)
	}
	if cmd.Category != "" {
		parts = append(parts, cmd.Category)
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, " / ") + ")"
}

// splitDocKeywords はフラグでない引数 (=キーワード) を先に取り出す。
// flag は最初の非フラグ引数で解析を止めるため。
func splitDocKeywords(args []string, valueFlags ...string) (keywords, rest []string) {
	valueExpected := false
	for _, a := range args {
		switch {
		case valueExpected:
			valueExpected = false
			rest = append(rest, a)
		case strings.HasPrefix(a, "-") && a != "-":
			name := strings.TrimLeft(strings.Split(a, "=")[0], "-")
			valueExpected = !strings.Contains(a, "=") && containsStr(valueFlags, name)
			rest = append(rest, a)
		default:
			keywords = append(keywords, a)
		}
	}
	return keywords, rest
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// splitSourceArg は値を持つフラグを飛ばして最初の位置引数を取り出す。
func splitSourceArg(args []string, valueFlags ...string) (source string, rest []string) {
	valueExpected := false
	for _, a := range args {
		switch {
		case valueExpected:
			valueExpected = false
			rest = append(rest, a)
		case strings.HasPrefix(a, "-"):
			name := strings.TrimLeft(strings.Split(a, "=")[0], "-")
			valueExpected = !strings.Contains(a, "=") && containsStr(valueFlags, name)
			rest = append(rest, a)
		case source == "":
			source = a
		default:
			rest = append(rest, a)
		}
	}
	return source, rest
}

// --- lint / format (エディタ内蔵の checkNakoSyntax / formatNakoCode を使う) ---

func lintFileGUI(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("文法チェックするファイルを1つ指定してください")
	}
	source := args[0]
	code, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("ファイル『%s』を読み込めません: %w", source, err)
	}
	res := checkNakoSyntax(string(code), source, lintModeGonako)
	if !res.OK {
		return errors.New(res.Error)
	}
	fmt.Fprintf(stdout, "%s: 文法エラーはありません\n", source)
	return nil
}

func formatFileGUI(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("format", flag.ContinueOnError)
	flags.SetOutput(stderr)
	force := flags.Bool("force", false, "結果をファイルへ書き戻す")
	flags.BoolVar(force, "f", false, "結果をファイルへ書き戻す")
	colon := flags.Bool("colon", false, "ブロックをコロン記法に書き換える")
	source, rest := splitSourceArg(args)
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
		return errors.New("整形するファイルを1つ指定してください")
	}
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("ファイル『%s』を読み込めません: %w", source, err)
	}
	code, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("ファイル『%s』を読み込めません: %w", source, err)
	}
	res := formatNakoCode(string(code), source, *colon, lintModeGonako)
	if !res.OK {
		if strings.Contains(res.Error, format.ErrStructureChanged.Error()) {
			return fmt.Errorf("『%s』は%w", source, format.ErrStructureChanged)
		}
		return errors.New(res.Error)
	}
	if !*force {
		fmt.Fprint(stdout, res.Formatted)
		return nil
	}
	if res.Formatted == string(code) {
		fmt.Fprintf(stdout, "%s: 変更はありません\n", source)
		return nil
	}
	if err := atomicWriteFileGUI(source, []byte(res.Formatted), info.Mode()); err != nil {
		return fmt.Errorf("ファイル『%s』へ書き戻せません: %w", source, err)
	}
	fmt.Fprintf(stdout, "%s を整形しました\n", source)
	return nil
}

// atomicWriteFileGUI は一時ファイル経由で置き換える (gonako format と同じ理由)。
func atomicWriteFileGUI(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// --- doctest (gonako doctest と同じ。internal/doctest を使う) ---

var defaultDocTestTargetsGUI = []string{"manual/plugin_system", "manual/gonako", "testdata/doctest"}

type jsonFlagGUI struct {
	enabled bool
	file    string
}

func (f *jsonFlagGUI) String() string {
	if !f.enabled {
		return ""
	}
	return f.file
}

func (f *jsonFlagGUI) Set(value string) error {
	f.enabled = true
	if value == "true" || value == "" {
		f.file = ""
	} else {
		f.file = value
	}
	return nil
}

func (f *jsonFlagGUI) IsBoolFlag() bool { return true }

func runDocTestsGUI(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("doctest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	max := flags.Int("max", 10, "失敗の詳細を表示する件数")
	runtimePath := flags.String("runtime", "", "サンプルを実行するランタイムのパス")
	subcommand := flags.String("subcommand", "", "ランタイムに渡すサブコマンド")
	label := flags.String("label", "", "対象にする表示結果ラベル")
	jsonOut := &jsonFlagGUI{}
	flags.Var(jsonOut, "json", "失敗した結果をJSONで出力する")
	usageFlag := flags.Bool("usage", false, "使い方を表示する")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *usageFlag {
		fmt.Fprint(stdout, subcommandUsage["doctest"])
		return nil
	}
	targets := flags.Args()
	if len(targets) == 0 {
		targets = existingTargetsGUI(defaultDocTestTargetsGUI)
	}
	labels := doctest.DefaultLabels
	if *label != "" {
		labels = nil
		for _, p := range strings.Split(*label, ",") {
			if n := doctest.NormalizeLabel(p); n != "" {
				labels = append(labels, n)
			}
		}
	}
	runOne := doctest.Run
	if *runtimePath != "" {
		resolved, err := exec.LookPath(*runtimePath)
		if err != nil {
			return fmt.Errorf("DocTestのランタイム『%s』が見つかりません: %w", *runtimePath, err)
		}
		if *subcommand != "" {
			runOne = doctest.ExternalRunner(resolved, *subcommand)
		} else {
			runOne = doctest.ExternalRunner(resolved)
		}
	} else if *subcommand != "" {
		return errors.New("--subcommand は --runtime と一緒に指定してください")
	}
	tests, err := doctest.CollectByLabel(targets, labels...)
	if err != nil {
		return fmt.Errorf("DocTest対象を読み込めません: %w", err)
	}
	if len(tests) == 0 {
		if jsonOut.enabled {
			return writeJSONReportGUI(stdout, jsonOut.file, 0, 0, 0, 0, nil)
		}
		fmt.Fprintln(stdout, "[DocTest] 対象のサンプルコードがありません。")
		return nil
	}
	if !jsonOut.enabled {
		fmt.Fprintf(stdout, "[DocTest] %d件のサンプルコードを実行します。\n", len(tests))
	}
	root, _ := os.Getwd()
	shown, passed, skipped := 0, 0, 0
	skipReasons := map[string]int{}
	byReason := map[doctest.Failure]int{}
	var jsonResults []doctest.JSONResult
	for _, test := range tests {
		result := runOne(test)
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
		if jsonOut.enabled {
			jsonResults = append(jsonResults, doctest.FormatJSONResult(test, result, root))
		} else if *max == 0 || shown < *max {
			shown++
			fmt.Fprintln(stderr, doctest.FormatFailure(test, result, root))
			fmt.Fprintln(stderr)
		}
	}
	failed := len(tests) - passed - skipped
	if jsonOut.enabled {
		if err := writeJSONReportGUI(stdout, jsonOut.file, len(tests), passed, failed, skipped, jsonResults); err != nil {
			return err
		}
		if failed > 0 {
			return fmt.Errorf("DocTestが%d件失敗しました", failed)
		}
		return nil
	}
	if failed == 0 {
		fmt.Fprintf(stdout, "[DocTest] %d件成功・%d件省略・失敗なし。\n", passed, skipped)
		for _, reason := range sortedKeysGUI(skipReasons) {
			fmt.Fprintf(stdout, "  %s: %d件\n", reason, skipReasons[reason])
		}
		return nil
	}
	if shown < failed {
		fmt.Fprintf(stderr, "(ほか %d件の失敗は省略しました。--max 0 で全件表示します)\n\n", failed-shown)
	}
	fmt.Fprintf(stdout, "[DocTest] %d/%d件成功 (%.1f%%)\n", passed, len(tests), float64(passed)/float64(len(tests))*100)
	if skipped > 0 {
		fmt.Fprintf(stdout, "  省略: %d件\n", skipped)
	}
	for _, reason := range []doctest.Failure{doctest.FailMissingCommand, doctest.FailMismatch, doctest.FailSyntax, doctest.FailRuntime} {
		if n := byReason[reason]; n > 0 {
			fmt.Fprintf(stdout, "  %s: %d件\n", reason, n)
		}
	}
	return fmt.Errorf("DocTestが%d件失敗しました", failed)
}

func writeJSONReportGUI(stdout io.Writer, file string, count, passed, failed, skipped int, results []doctest.JSONResult) error {
	if results == nil {
		results = []doctest.JSONResult{}
	}
	report := doctest.JSONReport{Tool: "gonako-gui-doctest", Count: count, Passed: passed, Failed: failed, Skipped: skipped, Results: results}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON出力に失敗しました: %w", err)
	}
	if file == "" {
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		return fmt.Errorf("JSONファイルを書き込めません: %w", err)
	}
	return nil
}

func existingTargetsGUI(targets []string) []string {
	existing := make([]string, 0, len(targets))
	for _, target := range targets {
		if _, err := os.Stat(target); err == nil || !os.IsNotExist(err) {
			existing = append(existing, target)
		}
	}
	return existing
}

func sortedKeysGUI(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --- build (gonako build の GUI版。土台は gonako-gui 自身) ---

func buildBundleGUI(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "", "出力する実行ファイル名")
	resource := flags.String("resource", "", "同梱するリソースのフォルダ")
	runtimePath := flags.String("runtime", "", "土台にするランタイム")
	list := flags.Bool("list", false, "同梱リソースの一覧を表示する")
	includeSymlink := flags.Bool("include-symlink", false, "フォルダ外のシンボリックリンクも辿って梱包する")
	source, rest := splitSourceArg(args, "out", "resource", "runtime")
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
	// 梱包アプリは gonako-gui のレジストリ (GUI命令を含む) で実行されるので、
	// コンパイルも同じレジストリで行う。
	registry := stdlib.NewRegistry(guiPlugins()...)
	prog, err := vm.CompileWithRegistry(string(code), source, registry)
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
		*out = buildDefaultOutputName(source, *runtimePath)
	}
	outAbs, err := filepath.Abs(*out)
	if err != nil {
		return fmt.Errorf("出力パスを解決できません: %w", err)
	}
	skip := map[string]bool{outAbs: true}
	if err := bundle.BuildSpec(*out, *runtimePath, bundle.Spec{
		Kind: bundle.KindProgram, Program: prog, Name: source,
		ResourceDir: *resource, IncludeSymlink: *includeSymlink, Skip: skip,
		Warn: func(format string, a ...any) { fmt.Fprintf(stderr, "警告: "+format+"\n", a...) },
	}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s を作りました\n", *out)
	return nil
}

func buildDefaultOutputName(source, runtimePath string) string {
	name := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	if strings.HasSuffix(runtimePath, ".exe") {
		return name + ".exe"
	}
	return name
}

// --- gengo (gonako gengo と同じ。internal/gogen を使う) ---

var defaultGoGenPluginsGUI = []string{"nodelib", "csvlib", "mathlib", "tomllib", "inilib", "sqlitelib", "officelib", "pdflib", "imagelib"}

func genGoGUI(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("gengo", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "", "出力するGoソースファイル名")
	pluginList := flags.String("plugins", strings.Join(defaultGoGenPluginsGUI, ","), "生成コードに含めるプラグイン")
	pkgName := flags.String("package", "main", "生成するGoソースのパッケージ名")
	source, rest := splitSourceArg(args, "out", "plugins", "package")
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
	return nil
}

// --- install (gonako install の GUI版。起動は gonako-gui run 経由) ---

func installScriptGUI(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	binDir := flags.String("bin", "bin", "起動用スクリプトの配置先")
	name := flags.String("name", "", "コマンド名")
	runtimePath := flags.String("runtime", "", "使用する gonako-gui")
	force := flags.Bool("force", false, "既存ファイルを置き換える")
	source, rest := splitSourceArg(args, "bin", "name", "runtime")
	if err := flags.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if source == "" || len(flags.Args()) != 0 {
		return errors.New("installにはソースファイルを1つ指定してください")
	}
	ext := filepath.Ext(source)
	if ext != ".nako3" && ext != ".nako" {
		return errors.New("installの対象は.nako3または.nakoファイルです")
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("installの対象は通常のファイルです")
	}
	if *name == "" {
		*name = "gonako-gui-" + strings.TrimSuffix(filepath.Base(source), ext)
	}
	if *name == "." || *name == ".." || strings.ContainsAny(*name, "/\\\r\n\x00") {
		return errors.New("コマンド名にはパスや改行を指定できません")
	}
	if *runtimePath == "" {
		*runtimePath, err = os.Executable()
		if err != nil {
			return err
		}
	}
	*runtimePath, err = filepath.Abs(*runtimePath)
	if err != nil {
		return err
	}
	if info, err := os.Stat(*runtimePath); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("ランタイム『%s』が見つかりません", *runtimePath)
	}
	script, suffix := installedScriptGUI(source, *runtimePath, runtime.GOOS)
	if err := os.MkdirAll(*binDir, 0o755); err != nil {
		return err
	}
	output := filepath.Join(*binDir, *name+suffix)
	if !*force {
		file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
		if err != nil {
			return fmt.Errorf("起動用ファイルを作成できません（上書きは--force）: %w", err)
		}
		_, writeErr := io.WriteString(file, script)
		modeErr := file.Chmod(0o755)
		closeErr := file.Close()
		if err := errors.Join(writeErr, modeErr, closeErr); err != nil {
			os.Remove(output)
			return err
		}
	} else {
		file, err := os.CreateTemp(*binDir, ".gonako-gui-install-*")
		if err != nil {
			return err
		}
		defer os.Remove(file.Name())
		_, writeErr := io.WriteString(file, script)
		modeErr := file.Chmod(0o755)
		closeErr := file.Close()
		if err := errors.Join(writeErr, modeErr, closeErr); err != nil {
			return err
		}
		if err := os.Rename(file.Name(), output); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "起動用ファイルを配置しました: %s\n", output)
	return nil
}

// installedScriptGUI は gonako-gui run で起動するスクリプトを作る。
func installedScriptGUI(source, runtimePath, platform string) (string, string) {
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	if platform == "windows" {
		quote = func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
		return "$ErrorActionPreference = 'Stop'\n" +
			"$runtime = $env:GONAKO_GUI_RUNTIME\nif (-not $runtime) { $runtime = " + quote(runtimePath) + " }\n" +
			"& $runtime run " + quote(source) + " @args\n    $code = $LASTEXITCODE\n" +
			"exit $code\n", ".ps1"
	}
	return "#!/bin/sh\n# gonako-gui installで生成した起動用スクリプト。\nset -eu\n" +
		"runtime=${GONAKO_GUI_RUNTIME:-" + quote(runtimePath) + "}\n" +
		"exec \"$runtime\" run " + quote(source) + " \"$@\"\n", ""
}

// --- compat ---

func runCompatGUI(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("compat のサブコマンド (run / commands) を指定してください")
	}
	switch args[0] {
	case "run":
		flags := flag.NewFlagSet("compat run", flag.ContinueOnError)
		flags.SetOutput(stderr)
		casesDir := flags.String("cases", "./testdata/compat/cases", "compat case JSONのディレクトリ")
		outDir := flags.String("out", "./out", "結果JSONの出力ディレクトリ")
		if err := flags.Parse(args[1:]); err != nil {
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
	case "commands":
		return runCommandDiffGUI(args[1:], stdout, stderr)
	}
	return fmt.Errorf("不明な compat サブコマンドです: %s", args[0])
}

func runCommandDiffGUI(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("compat commands", flag.ContinueOnError)
	flags.SetOutput(stderr)
	source := flags.String("source", "./nadesiko3/doc/command_list.json", "本家のcommand_list.json")
	sourceRef := flags.String("source-ref", "", "比較元のコミットID")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("余分な引数があります: %s", flags.Arg(0))
	}
	f, err := os.Open(*source)
	if err != nil {
		return fmt.Errorf("比較元を開けません: %w", err)
	}
	defer f.Close()
	report, err := commanddiff.Compare(f, stdlib.ParserFuncList())
	if err != nil {
		return err
	}
	ref := *sourceRef
	if ref == "" {
		ref = findGitRevisionGUI(*source)
	}
	commanddiff.WriteText(stdout, *source, ref, report)
	return nil
}

func findGitRevisionGUI(source string) string {
	dir, err := filepath.Abs(filepath.Dir(source))
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
			if err == nil {
				return strings.TrimSpace(string(out))
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
