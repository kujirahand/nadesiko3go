package main

import (
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

// formatNakoCode は整形した結果を返す（#118・#120）。ファイルへは
// 書き戻さず、呼び出し側（エディタ）が返された文字列でバッファを置き換える。
// `gonako format` と同じく、整形後に構文構造が変わってしまう場合は拒否する。
// colonがtrueなら、ブロックをコロン記法に書き換える。
func formatNakoCode(code, filename string, colon bool) FormatResult {
	if filename == "" {
		filename = "gui.nako3"
	}
	formatted, err := format.Program(code, filename, vm.ParseProgram, format.Options{Colon: colon})
	if err != nil {
		return FormatResult{OK: false, Error: err.Error()}
	}
	return FormatResult{OK: true, Formatted: formatted, Changed: formatted != code}
}
