package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/indent"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// resolveRequires expands `!「file」を取込` statements found in tok.
//
// A required file is read relative to the directory of the file that
// requires it, tokenized and indent-converted on its own (mirroring
// NakoTokenizer.rawtokenize / nako_require.mts in the TypeScript version),
// then spliced in place of the three-token require statement, wrapped in a
// namespace scope (『modName』に名前空間設定;『modName』にプラグイン名設定;
// ... ;名前空間ポップ;) so that the existing namespace machinery in
// replaceWord (lexer) and the プラグイン名設定/名前空間ポップ handling in
// the parser resolve cross-file function calls correctly.
//
// Each file is loaded at most once no matter how many times, or from where,
// it is required; guard tracks every file path already spliced in, matching
// NakoRequireLoader.replaceRequireStatements's global include guard (this
// also prevents infinite recursion on circular requires). modNames collects
// every module loaded this way so the caller can seed Lexer.ModList before
// running the rest of the token replacement passes.
func resolveRequires(tok []lexer.Token, filename string, guard map[string]bool, modNames *[]string) ([]lexer.Token, error) {
	out := make([]lexer.Token, 0, len(tok))
	for i := 0; i < len(tok); i++ {
		if !isRequireStatement(tok, i) {
			out = append(out, tok[i])
			continue
		}
		nameTok := tok[i+1]
		name := nameTok.StringValue()
		filePath, err := resolveRequirePath(name, filename, nameTok)
		if err != nil {
			return nil, err
		}
		i += 2 // consume the not/string/取込 span
		if guard[filePath] {
			continue // 同じファイルは一度だけ取り込む
		}
		guard[filePath] = true

		children, err := loadRequireFile(filePath, nameTok)
		if err != nil {
			return nil, err
		}
		children, err = resolveRequires(children, filePath, guard, modNames)
		if err != nil {
			return nil, err
		}
		modName := lexer.FilenameToModName(filePath)
		*modNames = append(*modNames, modName)
		out = append(out, wrapNamespace(modName, children, nameTok)...)
	}
	return out, nil
}

// isRequireStatement reports whether tok[i:i+3] is `not (string|string_ex) 取込`.
func isRequireStatement(tok []lexer.Token, i int) bool {
	if i+2 >= len(tok) {
		return false
	}
	if tok[i].Type != lexer.TypeNot {
		return false
	}
	strType := tok[i+1].Type
	if strType != lexer.TypeString && strType != lexer.TypeStringEx {
		return false
	}
	return tok[i+2].Type == lexer.TypeWord && tok[i+2].StringValue() == "取込"
}

func resolveRequirePath(name, fromFile string, tok lexer.Token) (string, error) {
	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") {
		return "", requireErr(tok, fmt.Sprintf("URL『%s』からの取り込みは未対応です。", name))
	}
	if strings.HasPrefix(name, "貯蔵庫:") || strings.HasPrefix(name, "貯蔵庫：") ||
		strings.HasPrefix(name, "拡張プラグイン:") || strings.HasPrefix(name, "拡張プラグイン：") {
		return "", requireErr(tok, fmt.Sprintf("『%s』の取り込みは未対応です。ローカルの.nako3ファイルのみ取り込めます。", name))
	}
	if !strings.HasSuffix(name, ".nako3") && !strings.HasSuffix(name, ".nako") {
		return "", requireErr(tok, fmt.Sprintf("ファイル『%s』を取り込めません。取り込めるのは拡張子.nako3または.nakoのファイルだけです。", name))
	}
	full := name
	if !filepath.IsAbs(full) {
		dir := "."
		if fromFile != "" {
			dir = filepath.Dir(fromFile)
		}
		full = filepath.Join(dir, name)
	}
	full = filepath.Clean(full)
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return "", requireErr(tok, fmt.Sprintf("ファイル『%s』が見つかりません。", name))
	}
	return full, nil
}

func loadRequireFile(filePath string, tok lexer.Token) ([]lexer.Token, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, requireErr(tok, fmt.Sprintf("ファイル『%s』を読み込めません。%s", filePath, err))
	}
	raw, err := lexer.Tokenize(prepare.Text(prepare.Convert(string(data))), 0, filePath)
	if err != nil {
		return nil, err
	}
	return indent.ConvertSyntax(raw)
}

// wrapNamespace brackets children in a namespace scope, matching the literal
// source `『modName』に名前空間設定;『modName』にプラグイン名設定;` + code +
// `;名前空間ポップ;` built in nako_require.mts.
func wrapNamespace(modName string, children []lexer.Token, at lexer.Token) []lexer.Token {
	word := func(v string) lexer.Token {
		return lexer.Token{Type: lexer.TypeWord, Value: v, Line: at.Line, File: at.File, Indent: -1}
	}
	str := func() lexer.Token {
		return lexer.Token{Type: lexer.TypeString, Value: modName, Josi: "に", Line: at.Line, File: at.File, Indent: -1}
	}
	eol := func() lexer.Token {
		return lexer.Token{Type: lexer.TypeEOL, Value: "---", Line: at.Line, File: at.File, Indent: -1}
	}
	out := make([]lexer.Token, 0, len(children)+9)
	out = append(out,
		str(), word("名前空間設定"), eol(),
		str(), word("プラグイン名設定"), eol(),
	)
	out = append(out, children...)
	out = append(out, eol(), word("名前空間ポップ"), eol())
	return out
}

func requireErr(tok lexer.Token, msg string) error {
	return &errs.NakoError{Kind: errs.Lexer, File: tok.File, Line: tok.Line, Msg: msg}
}
