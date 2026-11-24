package parser

import (
	"errors"
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

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/indent"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
	"github.com/kujirahand/nadesiko3go/pkg/nako3package"
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

func resolveRequires(tok []lexer.Token, filename string, guard map[string]bool, modNames *[]string, funcs lexer.FuncList, warns *[]string) ([]lexer.Token, error) {
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
		filePath, err := resolveRequirePathWarn(name, filename, nameTok, warns)
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
		children, err = resolveRequires(children, filePath, guard, modNames, funcs, warns)
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
	"plugin_ini":        "INI取得",
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

// UnsavedSourcePath は、まだ保存していないプログラム（GUIエディタの新規バッファなど）を
// 取り込み解決の起点として渡すときの目印。取り込み元のフォルダが無いため、
// フォルダを基準にする取り込みはエラーにして、先に保存するよう促す（#542）。
const UnsavedSourcePath = "<unsaved>"

func resolveRequirePath(name, fromFile string, tok lexer.Token) (string, error) {
	return resolveRequirePathWarn(name, fromFile, tok, nil)
}

// resolveRequirePathWarn は resolveRequirePath と同じ解決を行い、取り込み元の
// フォルダのファイルが同名のパッケージを隠したときの警告を warns へ追加する。
func resolveRequirePathWarn(name, fromFile string, tok lexer.Token, warns *[]string) (string, error) {
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
	unsaved := fromFile == UnsavedSourcePath
	explicit := filepath.IsAbs(name) || strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") || strings.HasPrefix(name, `.\`) || strings.HasPrefix(name, `..\`)
	if explicit {
		if unsaved && !filepath.IsAbs(name) {
			return "", unsavedRequireErr(tok, name)
		}
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
		return "", requireErr(tok, requireNotFoundMessage(name, fromFile, true))
	}
	// 接頭辞のない名前は、取り込み元のフォルダ → パッケージの順に探す。
	// 本家TypeScript版と同じく、手元のファイルを優先する（#542）。
	if dir, ok := requireLocalDir(fromFile); ok {
		if full, ok := localRequirePath(filepath.Join(dir, name)); ok {
			if pkg, found := findPackageRequire(name); found && warns != nil && pkg != full {
				*warns = append(*warns, fmt.Sprintf("『%s』は取り込み元フォルダのファイルを使います。同名のパッケージ（%s）は使われません。パッケージを使うには取り込み元フォルダの同名ファイルの名前を変えてください。", name, strings.TrimPrefix(pkg, embeddedPrefix)))
			}
			return full, nil
		}
	}
	if full, ok := findPackageRequire(name); ok {
		return full, nil
	}
	if unsaved {
		return "", unsavedRequireErr(tok, name)
	}
	return "", requireErr(tok, requireNotFoundMessage(name, fromFile, false))
}

// requireLocalDir は取り込み元ファイルのフォルダを返す。URL・埋め込み・
// 保存前の場合は、フォルダを基準にできないので false を返す。
func requireLocalDir(fromFile string) (string, bool) {
	if fromFile == "" || fromFile == UnsavedSourcePath || isRequireURL(fromFile) || strings.HasPrefix(fromFile, embeddedPrefix) {
		return "", false
	}
	return filepath.Dir(fromFile), true
}

// packageSearchDirs はパッケージを探すローカルのフォルダを、探索順に返す。
func packageSearchDirs() []string {
	var dirs []string
	for _, dir := range filepath.SplitList(os.Getenv("GONAKO_PACKAGE_PATH")) {
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	if exe, err := runtimeExecutable(); err == nil {
		// binのリンク配置よりも、実体のランタイムの位置を基準にする。
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		for _, dir := range []string{filepath.Dir(exe), filepath.Dir(filepath.Dir(exe))} {
			dirs = append(dirs, filepath.Join(dir, "gonako-package"))
		}
	}
	return dirs
}

// findPackageRequire は name を、環境変数・ランタイムの隣・埋め込みの順に探す。
func findPackageRequire(name string) (string, bool) {
	for _, dir := range packageSearchDirs() {
		if full, ok := localPackageRequirePath(dir, name); ok {
			return full, true
		}
	}
	return embeddedRequirePath(path.Join("gonako-package", filepath.ToSlash(name)))
}

// unsavedRequireHint は保存前の取り込みエラーに含まれる案内文。
const unsavedRequireHint = "先にこのプログラムをファイルとして保存"

// ErrUnsavedRequire は、保存前のプログラムが取り込み元のフォルダを必要とする
// 取り込みをしたときのエラーが満たす。errors.Is で判定する（#542）。
var ErrUnsavedRequire = errors.New("保存前のプログラムでは取り込み元のフォルダを使えません")

// unsavedRequireError は、通常の取り込みエラー（なでしこ形式の文面）に
// ErrUnsavedRequire を結び付ける。
type unsavedRequireError struct{ err error }

func (e *unsavedRequireError) Error() string { return e.err.Error() }

func (e *unsavedRequireError) Unwrap() []error { return []error{e.err, ErrUnsavedRequire} }

func unsavedRequireErr(tok lexer.Token, name string) error {
	return &unsavedRequireError{err: requireErr(tok, fmt.Sprintf("!「%s」を取り込むには、取り込み元のフォルダが必要です。"+unsavedRequireHint+"してから実行してください。", name))}
}

// requireNotFoundMessage は取り込むファイルが見つからないときの文面を作る。
// 探した場所を並べ、拡張子違い・大文字小文字違い・./ の付けすぎなど、
// よくある間違いを見つけたら案内を添える。
func requireNotFoundMessage(name, fromFile string, explicit bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ファイル『%s』が見つかりません。", name)
	// 取り込み元のフォルダの実パスは、プロジェクトの場所を表示に出さない
	// 方針（#340・#351）に合わせて、文面へ書かない。
	var localCand string
	if explicit {
		if !filepath.IsAbs(name) {
			if _, ok := requireLocalDir(fromFile); ok {
				localCand = filepath.Join(filepath.Dir(fromFile), name)
			}
		} else {
			localCand = name
		}
	} else {
		b.WriteString("\n次の場所を探しました。")
		if dir, ok := requireLocalDir(fromFile); ok {
			localCand = filepath.Join(dir, name)
			b.WriteString("\n- 取り込み元ファイルと同じフォルダ")
		}
		for _, dir := range packageSearchDirs() {
			b.WriteString("\n- " + filepath.Join(dir, name))
		}
		b.WriteString("\n- 埋め込みパッケージ gonako-package/" + filepath.ToSlash(name))
	}
	if localCand != "" {
		b.WriteString(requireHints(localCand))
	}
	if explicit {
		trimmed := strings.TrimPrefix(strings.TrimPrefix(filepath.ToSlash(name), "./"), "../")
		if !filepath.IsAbs(name) && trimmed != "" && !strings.Contains(trimmed, "..") {
			if _, ok := findPackageRequire(trimmed); ok {
				fmt.Fprintf(&b, "\nヒント: パッケージ『%s』はあります。パッケージを取り込むには ./ を付けずに書いてください。", trimmed)
			}
		}
	}
	return b.String()
}

// requireHints は取り込み先の候補パスの近くにある、間違えやすいファイルを案内する。
func requireHints(cand string) string {
	var b strings.Builder
	dir, base := filepath.Split(cand)
	if dir == "" {
		dir = "."
	}
	alt := ""
	if strings.HasSuffix(base, ".nako3") {
		alt = strings.TrimSuffix(base, ".nako3") + ".nako"
	} else if strings.HasSuffix(base, ".nako") {
		alt = strings.TrimSuffix(base, ".nako") + ".nako3"
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || n == base {
			continue
		}
		switch {
		case strings.EqualFold(n, base):
			fmt.Fprintf(&b, "\nヒント: 大文字・小文字が違う『%s』があります。", n)
		case alt != "" && n == alt:
			fmt.Fprintf(&b, "\nヒント: 拡張子が違う『%s』があります。", n)
		}
	}
	return b.String()
}

const embeddedPrefix = "embed:"

// テストでも実行ファイルの配置と埋め込み内容を検証できるようにする。
var runtimeExecutable = os.Executable

// packageFiles のパスは `gonako-package/` を除いた相対パス（embeddedFile で変換する）。
var packageFiles fs.FS = nako3package.Files

// embeddedFile は取込パス `gonako-package/...` を埋め込みFS内のパスへ変換する。
func embeddedFile(candidate string) string {
	return strings.TrimPrefix(candidate, "gonako-package/")
}

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
	info, err := fs.Stat(packageFiles, embeddedFile(candidate))
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
		data, err = fs.ReadFile(packageFiles, embeddedFile(strings.TrimPrefix(filePath, embeddedPrefix)))
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
