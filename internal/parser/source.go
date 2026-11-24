package parser

import (
	"fmt"
	"os"
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
	var requireWarnings []string
	raw, err = resolveRequires(raw, requireFrom, map[string]bool{}, &modNames, funcList, &requireWarnings)
	if err != nil {
		return nil, err
	}
	// 取り込みの警告は、実行を止めずに標準エラーへ知らせる。
	for _, w := range requireWarnings {
		fmt.Fprintln(os.Stderr, "警告: "+w)
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
	p.Warnings = append(p.Warnings, requireWarnings...)
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
	if sourcePath == UnsavedSourcePath || isRequireURL(sourcePath) || strings.HasPrefix(sourcePath, embeddedPrefix) {
		return sourcePath, nil
	}
	return filepath.Abs(sourcePath)
}

// CheckUnsavedRequires は、まだ保存していないプログラムの取り込み文を調べる。
// 取り込み元のフォルダを基準に解決する取り込み（./ や ../ を付けた相対パス、
// パッケージとして見つからない名前）があれば、先に保存するよう促すエラーを返す。
// 組み込みプラグイン・URL・貯蔵庫・見つかるパッケージは、保存の有無で結果が
// 変わらないので問題にしない。ここでは取り込み先を読み込まない（#542）。
func CheckUnsavedRequires(code string, funcs lexer.FuncList) error {
	raw, err := lexer.Tokenize(prepare.Text(prepare.Convert(code)), 0, "")
	if err != nil {
		return nil // 字句エラーは通常の構文チェックに任せる
	}
	raw, err = indent.ConvertSyntax(raw)
	if err != nil {
		return nil
	}
	for i := range raw {
		if !isRequireStatement(raw, i) {
			continue
		}
		nameTok := raw[i+1]
		if isBuiltinPlugin(nameTok.StringValue(), funcs) {
			continue
		}
		if _, err := resolveRequirePath(nameTok.StringValue(), UnsavedSourcePath, nameTok); err != nil {
			if strings.Contains(err.Error(), unsavedRequireHint) {
				return err
			}
		}
	}
	return nil
}
