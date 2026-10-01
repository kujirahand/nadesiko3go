package parser

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/kujirahand/nadesiko3go" // 直下のパッケージを埋め込み登録する。
	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/indent"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/nakopackage"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// resolveRequires は tok に含まれる `!「file」を取込` 文を展開する。
//
// 取り込むファイルは、URL・相対パス・パッケージ探索順を解決して
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
// requireHTTPClient はURL形式の取込に使うHTTPクライアントです。テストでは transport を差し替えます。
var requireHTTPClient = &http.Client{Timeout: 15 * time.Second}

func resolveRequires(tok []lexer.Token, filename string, guard map[string]bool, modNames *[]string, funcs lexer.FuncList) ([]lexer.Token, error) {
	out := make([]lexer.Token, 0, len(tok))
	for i := 0; i < len(tok); i++ {
		if !isRequireStatement(tok, i) {
			out = append(out, tok[i])
			continue
		}
		nameTok := tok[i+1]
		name := nameTok.StringValue()
		i += 2 // not/string/取込の3トークン分を読み進める
		if isBuiltinPlugin(name, funcs) {
			continue // 実行ファイルに組み込み済みのプラグイン名は読み飛ばす
		}
		filePath, err := resolveRequirePath(name, filename, nameTok)
		if err != nil {
			return nil, err
		}
		if guard[filePath] {
			continue // 同じファイルは一度だけ取り込む
		}
		guard[filePath] = true

		children, err := loadRequireFile(filePath, nameTok)
		if err != nil {
			return nil, err
		}
		children, err = resolveRequires(children, filePath, guard, modNames, funcs)
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

// builtinPluginSentinels は、実行ファイルに組み込まれうるプラグイン名と、
// 組み込み済みかどうかを FuncList で確かめるための代表命令名の対応表。
// 本家TypeScript版はこれらを外部ファイルとして読み込むが、Go版は
// ランタイムに組み込んで配布するため、『取込』は「既に読み込み済み」
// として読み飛ばす。代表命令がFuncListにない（＝そのバイナリには
// 組み込まれていない）ときは従来どおりファイル解決のエラーになる。
var builtinPluginSentinels = map[string]string{
	"plugin_system":     "表示",
	"plugin_node":       "開",
	"plugin_math":       "SIN",
	"plugin_csv":        "CSV取得",
	"plugin_toml":       "TOMLデコード",
	"nadesiko3-toml":    "TOMLデコード",
	"nadesiko3-sqlite3": "SQLITE3開",
	"nadesiko3-office":  "OFFICEバージョン",
}

// isBuiltinPlugin は name が組み込みプラグインの取込名で、そのプラグインが
// この実行ファイルに組み込み済みかを返す。`.js`/`.mjs`/`.mts` 付きの名前も
// 素の名前として扱う（`.nako3` を指す名前はここでは拾わず、実ファイルが
// 優先する）。
func isBuiltinPlugin(name string, funcs lexer.FuncList) bool {
	base := name
	ext := path.Ext(base)
	if ext == ".js" || ext == ".mjs" || ext == ".mts" {
		base = strings.TrimSuffix(base, ext)
	}
	sentinel, ok := builtinPluginSentinels[base]
	return ok && funcs[sentinel] != nil
}

func resolveRequirePath(name, fromFile string, tok lexer.Token) (string, error) {
	name = strings.Replace(name, "貯蔵庫：", "貯蔵庫:", 1)
	if strings.HasPrefix(name, "貯蔵庫:") {
		module := strings.TrimPrefix(name, "貯蔵庫:")
		if module == "" || filepath.Base(module) != module || strings.ContainsAny(module, `\\/?#`) || !strings.HasSuffix(module, ".nako3") {
			return "", requireErr(tok, fmt.Sprintf("URLのファイル名『%s』が不正です。拡張子.nako3のファイル名を指定してください。", module))
		}
		return "https://n3s.nadesi.com/plain/" + module, nil
	}
	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") {
		return resolveRequireURL(name, "", tok)
	}
	if strings.HasPrefix(name, "拡張プラグイン:") || strings.HasPrefix(name, "拡張プラグイン：") {
		return "", requireErr(tok, fmt.Sprintf("『%s』の取り込みは未対応です。ローカルの.nako3ファイルのみ取り込めます。", name))
	}
	// パッケージ名だけの指定は、そのパッケージの入口へ展開する。
	if filepath.Ext(name) == "" && !filepath.IsAbs(name) && !strings.HasPrefix(name, ".") {
		name = filepath.ToSlash(name) + ".nako3"
	}
	if !strings.HasSuffix(name, ".nako3") && !strings.HasSuffix(name, ".nako") {
		return "", requireErr(tok, fmt.Sprintf("ファイル『%s』を取り込めません。取り込めるのは拡張子.nako3または.nakoのファイルだけです。", name))
	}
	explicit := filepath.IsAbs(name) || strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") || strings.HasPrefix(name, `.\`) || strings.HasPrefix(name, `..\`)
	if explicit {
		if isRequireURL(fromFile) && !filepath.IsAbs(name) {
			return resolveRequireURL(name, fromFile, tok)
		}
		if strings.HasPrefix(fromFile, embeddedPrefix) && !filepath.IsAbs(name) {
			candidate := path.Join(path.Dir(strings.TrimPrefix(fromFile, embeddedPrefix)), filepath.ToSlash(name))
			if full, ok := embeddedRequirePath(candidate); ok {
				return full, nil
			}
		} else {
			full := name
			if !filepath.IsAbs(name) {
				full = filepath.Join(filepath.Dir(fromFile), name)
			}
			if full, ok := localRequirePath(full); ok {
				return full, nil
			}
		}
	} else {
		for _, dir := range filepath.SplitList(os.Getenv("GONAKO_PACKAGE_PATH")) {
			if dir == "" {
				continue
			}
			if full, ok := localPackageRequirePath(dir, name); ok {
				return full, nil
			}
		}
		if exe, err := runtimeExecutable(); err == nil {
			// binのリンク配置よりも、実体のランタイムの位置を基準にする。
			if real, err := filepath.EvalSymlinks(exe); err == nil {
				exe = real
			}
			for _, dir := range []string{filepath.Dir(exe), filepath.Dir(filepath.Dir(exe))} {
				if full, ok := localPackageRequirePath(filepath.Join(dir, "gonako-package"), name); ok {
					return full, nil
				}
			}
		}
		candidate := path.Join("gonako-package", filepath.ToSlash(name))
		if full, ok := embeddedRequirePath(candidate); ok {
			return full, nil
		}
	}
	return "", requireErr(tok, fmt.Sprintf("ファイル『%s』が見つかりません。", name))
}

const embeddedPrefix = "embed:"

// テストでも実行ファイルの配置と埋め込み内容を検証できるようにする。
var runtimeExecutable = os.Executable
var packageFiles fs.FS = nakopackage.Files

func localRequirePath(name string) (string, bool) {
	full, err := filepath.Abs(name)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(full)
	return full, err == nil && !info.IsDir()
}

// localPackageRequirePath は字面とリンク解決後の両方で探索ルート内に限定する。
func localPackageRequirePath(dir, name string) (string, bool) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	candidate := filepath.Join(root, name)
	if !withinRequireRoot(root, candidate) {
		return "", false
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", false
	}
	realCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil || !withinRequireRoot(realRoot, realCandidate) {
		return "", false
	}
	return localRequirePath(candidate)
}

func withinRequireRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// embeddedRequirePath は仮想FSの構成が増えてもパッケージ領域の外を読まない。
func embeddedRequirePath(candidate string) (string, bool) {
	if !fs.ValidPath(candidate) || !strings.HasPrefix(candidate, "gonako-package/") {
		return "", false
	}
	info, err := fs.Stat(packageFiles, candidate)
	return embeddedPrefix + candidate, err == nil && !info.IsDir()
}

func isRequireURL(name string) bool {
	return strings.HasPrefix(name, "https://") || strings.HasPrefix(name, "http://")
}

func resolveRequireURL(name, fromFile string, tok lexer.Token) (string, error) {
	u, err := url.Parse(name)
	if err == nil && fromFile != "" {
		base, baseErr := url.Parse(fromFile)
		if baseErr != nil {
			err = baseErr
		} else {
			u = base.ResolveReference(u)
		}
	}
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || (!strings.HasSuffix(u.Path, ".nako3") && !strings.HasSuffix(u.Path, ".nako")) {
		return "", requireErr(tok, fmt.Sprintf("URL『%s』を取り込めません。拡張子.nako3または.nakoのHTTP URLを指定してください。", name))
	}
	u.Fragment = ""
	return u.String(), nil
}

func loadRequireFile(filePath string, tok lexer.Token) ([]lexer.Token, error) {
	var data []byte
	var err error
	if isRequireURL(filePath) {
		resp, requestErr := requireHTTPClient.Get(filePath)
		if requestErr != nil {
			return nil, requireErr(tok, fmt.Sprintf("URLのファイルを取得できません。%s", requestErr))
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, requireErr(tok, fmt.Sprintf("URLのファイルを取得できません。HTTP %d", resp.StatusCode))
		}
		data, err = io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
		if err == nil && len(data) > 8<<20 {
			return nil, requireErr(tok, "URLのファイルが大きすぎます（上限8MiB）。")
		}
	} else if strings.HasPrefix(filePath, embeddedPrefix) {
		data, err = fs.ReadFile(packageFiles, strings.TrimPrefix(filePath, embeddedPrefix))
	} else {
		data, err = os.ReadFile(filePath)
	}
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
