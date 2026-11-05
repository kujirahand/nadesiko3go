package parser

import (
	"path/filepath"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/indent"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// ParseSource runs the stage-1 source pipeline: prepare, tokenize, token
// replacement, and parsing. The supplied function list is shared by lexer and
// parser because both need command signatures.
func ParseSource(code, filename string, funcList lexer.FuncList) (*ast.Node, error) {
	return ParseSourceFile(code, filename, "", funcList)
}

// ParseSourceFile は ParseSource と同じ解析を行うが、表示やエラー帰属に
// 使う filename と、相対的な『取り込む』の解決基準にする sourcePath を
// 分けて受け取る。sourcePath はコードを読み込んだ主ソースファイル自身の
// パスであり、フォルダ名ではない。空文字の場合は従来どおり filename を
// 解決基準にも使う。
//
// 表示名だけを短くしても include の基準が失われないようにするための
// 入口で、GUI のビルド経路が使う。字句解析・トークン置換・構文解析には
// filename を渡し、取り込み解決の起点だけを sourcePath にする。取り込んだ
// 依存ファイルは、従来どおり解決済みの実パスを自らの解決基準と表示名に
// 使う（#340・#351）。
func ParseSourceFile(code, filename, sourcePath string, funcList lexer.FuncList) (*ast.Node, error) {
	requireFrom, err := requireSourcePath(filename, sourcePath)
	if err != nil {
		return nil, err
	}
	lx := lexer.NewLexer()
	lx.FuncList = funcList
	raw, err := lexer.Tokenize(prepare.Text(prepare.Convert(code)), 0, filename)
	if err != nil {
		return nil, err
	}
	raw, err = indent.ConvertSyntax(raw)
	if err != nil {
		return nil, err
	}
	var modNames []string
	raw, err = resolveRequires(raw, requireFrom, map[string]bool{}, &modNames, funcList)
	if err != nil {
		return nil, err
	}
	lx.ModList = append(lx.ModList, modNames...)
	tokens, err := lx.ReplaceTokens(raw, true, filename)
	if err != nil {
		return nil, err
	}
	tokens, err = lx.ExpandCodeTokens(tokens, filename)
	if err != nil {
		return nil, err
	}
	p := New()
	p.SetFuncList(lx.FuncList)
	p.SetModuleExport(lx.ModuleExport)
	return p.Parse(tokens, filename)
}

// requireSourcePath は取り込み解決の起点を決める。ローカルファイルは
// 絶対パスにしておくことで、解決の途中でプロセスの作業フォルダに
// 依存しないようにする。URL と埋め込みファイルはファイルシステムの
// パスではないため、そのまま返す。
func requireSourcePath(filename, sourcePath string) (string, error) {
	if sourcePath == "" {
		return filename, nil
	}
	if isRequireURL(sourcePath) || strings.HasPrefix(sourcePath, embeddedPrefix) {
		return sourcePath, nil
	}
	return filepath.Abs(sourcePath)
}
