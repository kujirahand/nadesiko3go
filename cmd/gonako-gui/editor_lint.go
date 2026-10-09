package main

import (
	"sync"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/format"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// LintResult は文法チェックの結果をJavaScript側へ返すJSON構造。
type LintResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// FormatResult は自動整形の結果をJavaScript側へ返すJSON構造。
type FormatResult struct {
	OK        bool   `json:"ok"`
	Formatted string `json:"formatted,omitempty"`
	Changed   bool   `json:"changed,omitempty"`
	Error     string `json:"error,omitempty"`
}

// 文法チェック・自動整形で使う命令セット（#301）。実行モードに合わせる。
const (
	lintModeGonako = "gonako" // gonakoのランタイム命令（既定）
	lintModeWNako  = "wnako"  // 本家ブラウザ版(wnako3)の命令
)

// parseFuncFor は命令セットに合った構文解析関数を返す。
func parseFuncFor(mode string) format.ParseFunc {
	if mode == lintModeWNako {
		return parseWNakoProgram
	}
	return vm.ParseProgram
}

// parseWNakoProgram はwnako3の命令一覧で構文解析する（#301）。
// wnakoモードのプログラムは本家ブラウザ版で実行されるので、gonakoの
// 命令一覧で調べるとDOM操作などのwnako専用命令が未定義の単語になってしまう。
func parseWNakoProgram(code, filename string) (*ast.Node, error) {
	return parser.ParseSource(code, filename, wnakoFuncList())
}

// wnakoSharedPlugins は、gonakoも同じ命令を実装しているwnako3のプラグイン。
// これらの命令はgonakoのレジストリの定義を使う。plugin_browser などの
// ブラウザ用プラグインは、plugin_system と同名の命令（RGBなど）を別の助詞で
// 定義し直しているので、wnako3の一覧の定義を使う。
var wnakoSharedPlugins = map[string]bool{
	"plugin_system":   true,
	"plugin_math":     true,
	"plugin_datetime": true,
	"plugin_csv":      true,
	"plugin_promise":  true,
	"plugin_toml":     true,
}

var (
	wnakoCommandsOnce sync.Once
	wnakoCommands     []CommandItem
)

// wnakoFuncList はwnako3の命令一覧（command-list-wnako.json）から構文解析用の
// 命令表を作る。構文解析は利用者定義の関数を命令表へ書き足すので、
// 呼び出すたびに新しい表を返す。
//
// command-list-wnako.json の助詞はマニュアルの書式から抜き出した近似なので、
// gonakoと共通のプラグイン（wnakoSharedPlugins）の命令は、gonakoのレジストリの
// 定義（plugin_system は互換保証の対象で、助詞や可変長引数の指定が正確）を使う。
// wnakoの一覧に無いgonako専用命令（ファイル操作やGUIなど）は含めない。
func wnakoFuncList() lexer.FuncList {
	wnakoCommandsOnce.Do(func() {
		wnakoCommands = getWNakoCommandList()
	})
	runtime := vm.RuntimeFuncList()
	list := make(lexer.FuncList, len(wnakoCommands))
	for _, cmd := range wnakoCommands {
		if item, ok := runtime[cmd.Name]; ok && item.Type == cmd.Type && wnakoSharedPlugins[cmd.Plugin] {
			list[cmd.Name] = item
			continue
		}
		josi := make([][]string, len(cmd.Josi))
		for i, group := range cmd.Josi {
			josi[i] = append([]string(nil), group...)
		}
		list[cmd.Name] = &lexer.FuncItem{
			Name:       cmd.Name,
			Type:       cmd.Type,
			Josi:       josi,
			ReturnNone: cmd.ReturnNone,
		}
	}
	return list
}

// checkNakoSyntax はプログラムを実行せず構文解析だけ行う（#118）。
// `gonako lint` と同じくコンパイル・実行はしないため、実行に時間がかかる
// プログラムや副作用のあるプログラムでも安全に呼び出せる。
// modeは命令セット（lintModeGonako / lintModeWNako）を指定する（#301）。
func checkNakoSyntax(code, filename, mode string) LintResult {
	if filename == "" {
		filename = "gui.nako3"
	}
	if _, err := parseFuncFor(mode)(code, filename); err != nil {
		return LintResult{OK: false, Error: err.Error()}
	}
	return LintResult{OK: true}
}

// formatNakoCode は整形した結果を返す（#118・#120）。ファイルへは
// 書き戻さず、呼び出し側（エディタ）が返された文字列でバッファを置き換える。
// `gonako format` と同じく、整形後に構文構造が変わってしまう場合は拒否する。
// colonがtrueなら、ブロックをコロン記法に書き換える。
// modeは構文解析に使う命令セットを指定する（#301）。
func formatNakoCode(code, filename string, colon bool, mode string) FormatResult {
	if filename == "" {
		filename = "gui.nako3"
	}
	formatted, err := format.Program(code, filename, parseFuncFor(mode), format.Options{Colon: colon})
	if err != nil {
		return FormatResult{OK: false, Error: err.Error()}
	}
	return FormatResult{OK: true, Formatted: formatted, Changed: formatted != code}
}
