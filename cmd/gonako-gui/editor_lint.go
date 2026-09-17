package main

import (
	"fmt"

	"github.com/kujirahand/nadesiko3go/internal/format"
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

// checkNakoSyntax はプログラムを実行せず構文解析だけ行う（#118）。
// `gonako lint` と同じくコンパイル・実行はしないため、実行に時間がかかる
// プログラムや副作用のあるプログラムでも安全に呼び出せる。
func checkNakoSyntax(code, filename string) LintResult {
	if filename == "" {
		filename = "gui.nako3"
	}
	if _, err := vm.ParseProgram(code, filename); err != nil {
		return LintResult{OK: false, Error: err.Error()}
	}
	return LintResult{OK: true}
}

// formatNakoCode はインデントを整えた結果を返す（#118）。ファイルへは
// 書き戻さず、呼び出し側（エディタ）が返された文字列でバッファを置き換える。
// `gonako format` と同じく、整形後に構文構造が変わってしまう場合は拒否する。
func formatNakoCode(code, filename string) FormatResult {
	if filename == "" {
		filename = "gui.nako3"
	}
	tree, err := vm.ParseProgram(code, filename)
	if err != nil {
		return FormatResult{OK: false, Error: err.Error()}
	}
	formatted := format.Source(code, filename, tree)
	if formatted == code {
		return FormatResult{OK: true, Formatted: formatted, Changed: false}
	}
	formattedTree, err := vm.ParseProgram(formatted, filename)
	if err != nil {
		return FormatResult{OK: false, Error: fmt.Sprintf("整形結果が構文解析できなくなるため中止しました: %s", err.Error())}
	}
	if format.Structure(formattedTree) != format.Structure(tree) {
		return FormatResult{OK: false, Error: "整形すると構文構造が変わってしまうため中止しました。" +
			"インデントでブロックを表すファイル(『!インデント構文』や『3回:』のようなコロン記法)は整形できません"}
	}
	return FormatResult{OK: true, Formatted: formatted, Changed: true}
}
