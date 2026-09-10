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

// resolveRequires は tok に含まれる `!「file」を取込` 文を展開する。
//
// 取り込むファイルは、取り込む側のファイルのディレクトリからの相対パスで
// 読み込み、単独で字句解析・インデント変換してから（本家TypeScript版の
// NakoTokenizer.rawtokenize / nako_require.mts と同じ方式）、取込文3トー
// クンの位置に、名前空間スコープ（『modName』に名前空間設定;『modName』
// にプラグイン名設定; ... ;名前空間ポップ;）で包んで差し込む。こうすると
// lexerのreplaceWordとparserのプラグイン名設定/名前空間ポップの処理
// （どちらも既存実装）が正しくファイルをまたいだ関数呼び出しを解決できる。
//
// 同じファイルは、どこから何度取り込まれても一度しか読み込まない。guard
// にはこれまでに差し込んだファイルの絶対パスを入れておき、
// NakoRequireLoader.replaceRequireStatements のグローバルなinclude guard
// と同じ挙動にする（循環取込の無限再帰も同時に防げる）。modNames には、
// こうして読み込んだモジュール名を集めておき、呼び出し元が残りのトークン
// 置換パスを実行する前に Lexer.ModList にまとめて設定できるようにする。
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
		i += 2 // not/string/取込の3トークン分を読み進める
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

// isRequireStatement は tok[i:i+3] が `not (string|string_ex) 取込` かどうかを返す。
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
	// 相対パスと絶対パスで同じファイルを指定しても取込ガードのキーが
	// 一致するように、必ず絶対パスへ正規化する（同一ファイルの二重取込を防ぐ）
	full, err := filepath.Abs(full)
	if err != nil {
		return "", requireErr(tok, fmt.Sprintf("ファイル『%s』のパスを解決できません。%s", name, err))
	}
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

// wrapNamespace は children を名前空間スコープで包む。nako_require.mts が
// 組み立てるソース文字列 `『modName』に名前空間設定;『modName』にプラグイン
// 名設定;` + code + `;名前空間ポップ;` と同じ構造をトークン列で再現する。
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
