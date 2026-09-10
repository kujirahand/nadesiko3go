//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
	"github.com/kujirahand/nadesiko3go/internal/imagelib"
	"github.com/kujirahand/nadesiko3go/internal/nodelib"
	"github.com/kujirahand/nadesiko3go/internal/officelib"
	"github.com/kujirahand/nadesiko3go/internal/pdflib"
	"github.com/kujirahand/nadesiko3go/internal/sqlitelib"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

// GitHub上でこのリポジトリのソースを参照する際のブランチ名。
const sourceBranch = "master"

type CommandDoc struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Josi     [][]string `json:"josi"`
	Plugin   string     `json:"plugin,omitempty"`
	Category string     `json:"category"`
	Desc     string     `json:"desc"`
	Template string     `json:"template"`
	Yomi     string     `json:"yomi,omitempty"`
	File     string     `json:"file,omitempty"`
	Line     int        `json:"line,omitempty"`
	URL      string     `json:"url,omitempty"`
}

// pluginForFile は定義ファイルのパスから、その命令が属するプラグインを返す。
// stdlib/nodelib/csvlib/mathlibは本家のプラグイン名と対応させ、それ以外の
// （本家に存在しない）gonako独自のライブラリはまとめて「gonako」とする。
func pluginForFile(file string) string {
	switch {
	case strings.HasPrefix(file, "internal/stdlib/"):
		return "plugin_system"
	case strings.HasPrefix(file, "internal/nodelib/"):
		return "plugin_node"
	case strings.HasPrefix(file, "internal/csvlib/"):
		return "plugin_csv"
	case strings.HasPrefix(file, "internal/mathlib/"):
		return "plugin_math"
	case strings.HasPrefix(file, "internal/sqlitelib/"),
		strings.HasPrefix(file, "internal/officelib/"),
		strings.HasPrefix(file, "internal/pdflib/"),
		strings.HasPrefix(file, "internal/imagelib/"),
		strings.HasPrefix(file, "internal/guilib/"):
		return "gonako"
	default:
		return ""
	}
}

// goOnlyGroup は、本家(TS)に対応する命令が無いGo独自命令について、
// 定義ファイルごとのグループ名を明示的に指定する。
var goOnlyGroup = map[string]string{
	"internal/sqlitelib/sqlitelib.go": "SQLite",
	"internal/officelib/officelib.go": "Excel",
	"internal/pdflib/pdflib.go":       "PDF",
	"internal/imagelib/imagelib.go":   "画像",
	"internal/guilib/guilib.go":       "GUI",
	"internal/nodelib/clipboard.go":   "クリップボード",
	"internal/nodelib/os.go":          "Nodeプロセス",
	"internal/nodelib/file.go":        "ファイル入出力",
	"internal/stdlib/string.go":       "文字列処理",
}

// groupDirFallback は goOnlyGroup に個別指定が無いファイルのための、
// パッケージ単位のデフォルトグループ名。
var groupDirFallback = map[string]string{
	"sqlitelib": "SQLite",
	"officelib": "Excel",
	"pdflib":    "PDF",
	"imagelib":  "画像",
	"guilib":    "GUI",
	"nodelib":   "Node",
	"stdlib":    "システム",
	"csvlib":    "CSV",
	"mathlib":   "数学",
}

// groupForFile は本家に対応する命令が無いGo独自命令について、
// 定義ファイルからグループ名を決める。
func groupForFile(file string) string {
	if g, ok := goOnlyGroup[file]; ok {
		return g
	}
	base := filepath.Base(filepath.Dir(file))
	if g, ok := groupDirFallback[base]; ok {
		return g
	}
	return "Go拡張"
}

var (
	// '命令名': { // @説明 // @読み  （説明・読みの中に '/' が含まれてもよいように、
	// "// @" 以降を丸ごと拾ってから Go 側で分割する）
	cmdHeaderRe = regexp.MustCompile(`['"]([^'"]+)['"]\s*:\s*\{\s*//\s*@(.+)$`)
	// // @カテゴリー名
	catRe = regexp.MustCompile(`^\s*//\s*@([^@\n\r/]+)$`)
	// josi: [...]
	josiRe = regexp.MustCompile(`josi\s*:\s*(\[[^;{}]*\])`)
)

// splitDescYomi は "文字列Aを...返す // @よみ" のような "// @" 以降の残りを
// 説明文と読みがなに分割する。読みがなが無ければ空文字を返す。
func splitDescYomi(rest string) (desc, yomi string) {
	parts := strings.SplitN(rest, "// @", 2)
	desc = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		yomi = strings.TrimSpace(parts[1])
	}
	return desc, yomi
}

func parseJosi(raw string) [][]string {
	raw = strings.TrimSpace(raw)
	if raw == "[]" || raw == "" {
		return nil
	}
	// Parse nested array JSON-like string: [['a', 'b'], ['c']]
	// Replace single quotes with double quotes
	jsonStr := strings.ReplaceAll(raw, "'", "\"")
	var result [][]string
	if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
		return result
	}

	// Fallback regex parsing
	innerRe := regexp.MustCompile(`\[([^\[\]]*)\]`)
	matches := innerRe.FindAllStringSubmatch(raw, -1)
	for _, m := range matches {
		var group []string
		items := strings.Split(m[1], ",")
		for _, item := range items {
			clean := strings.Trim(strings.TrimSpace(item), "'\"[]")
			if clean != "" {
				group = append(group, clean)
			}
		}
		if len(group) > 0 {
			result = append(result, group)
		}
	}
	return result
}

func parseTSPlugins() map[string]CommandDoc {
	docs := make(map[string]CommandDoc)
	files, _ := filepath.Glob("nadesiko3/core/src/plugin_*.mts")
	nodeFiles, _ := filepath.Glob("nadesiko3/src/plugin_*.mts")
	files = append(files, nodeFiles...)

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		currentCategory := "基本"

		for i, line := range lines {
			lineTrimmed := strings.TrimSpace(line)
			if m := catRe.FindStringSubmatch(lineTrimmed); len(m) > 1 {
				cat := strings.TrimSpace(m[1])
				if !strings.HasPrefix(cat, "fileOverview") && len(cat) < 30 {
					currentCategory = cat
				}
				continue
			}

			if m := cmdHeaderRe.FindStringSubmatch(line); len(m) > 2 {
				name := m[1]
				desc, yomi := splitDescYomi(m[2])

				var josi [][]string
				for j := i; j < i+12 && j < len(lines); j++ {
					if j > i {
						trimmed := strings.TrimSpace(lines[j])
						if cmdHeaderRe.MatchString(lines[j]) || trimmed == "}," || trimmed == "}" {
							break
						}
					}
					if jm := josiRe.FindStringSubmatch(lines[j]); len(jm) > 1 {
						josi = parseJosi(jm[1])
						break
					}
				}

				docs[name] = CommandDoc{
					Name:     name,
					Type:     "func",
					Josi:     josi,
					Category: currentCategory,
					Desc:     desc,
					Template: makeTemplate(name, josi),
					Yomi:     yomi,
				}
			}
		}
	}
	return docs
}

func makeTemplate(name string, josi [][]string) string {
	if len(josi) == 0 {
		return name
	}
	paramNames := []string{"A", "B", "C", "D", "E", "F"}
	if len(josi) == 1 {
		j := ""
		if len(josi[0]) > 0 {
			j = josi[0][0]
		}
		if j == "" {
			return fmt.Sprintf("【S】%s", name)
		}
		return fmt.Sprintf("【S】%s%s", j, name)
	}

	var parts []string
	for idx, group := range josi {
		pName := "A"
		if idx < len(paramNames) {
			pName = paramNames[idx]
		}
		j := ""
		if len(group) > 0 {
			j = group[0]
		}
		parts = append(parts, fmt.Sprintf("【%s】%s", pName, j))
	}
	return strings.Join(parts, "") + name
}

var insertionTemplateOverrides = map[string]string{
	"ファイル選択":   "『S』のファイル選択",
	"保存ファイル選択": "『S』の保存ファイル選択",
	"フォルダ選択":   "『S』のフォルダ選択",
}

// --- Go実装側のソース走査 ---
//
// なでしこの命令はGo側でいくつかの書き方で定義される。どの書き方でも、
// 命令名の定義行に本家と同じ `// @説明 // @よみ` 形式のコメントを書けば、
// ここでそれを拾ってCommandDocに反映する。加えて、命令ごとに「一番確からしい
// 定義行」をGitHubへのソースリンクとして埋め込む（Issue #54）。
//
// 複数の書き方がヒットしうるので、優先度(priority)が最大のものを採用する：
//
//	100: m["名前"] = ...                      （stdlib/nodelibの実装本体）
//	 90: "名前": { ... }                      （*lib の commands() マップリテラル）
//	 80: "名前": 識別子,                       （Impls() の対応表）
//	 60: addFunc("名前", ...)                 （registry.goの命令表）
//	 55: add("名前", ...)                     （mathlibの命令表）
//	 40: list["名前"] = &lexer.FuncItem{...}  （FuncList内の個別代入）
//	 20: range []string{"名前", ...}          （複数命令の一括登録）
type srcMatch struct {
	file     string
	line     int
	priority int
	desc     string
	yomi     string
	hasDoc   bool
}

var (
	reMAssign    = regexp.MustCompile(`^\s*m\["([^"]+)"\]\s*=`)
	reMapKey     = regexp.MustCompile(`^\s*"([^"]+)"\s*:\s*\{`)
	reImplMap    = regexp.MustCompile(`^\s*"([^"]+)"\s*:\s*[A-Za-z0-9_.]+\s*,`)
	reAddFunc    = regexp.MustCompile(`\baddFunc\("([^"]+)"`)
	reAddPlain   = regexp.MustCompile(`\badd\("([^"]+)"`)
	reListAssign = regexp.MustCompile(`^\s*list\["([^"]+)"\]\s*=`)
	reRangeList  = regexp.MustCompile(`range\s*\[\]string\{(.+)\}`)
	reDocComment = regexp.MustCompile(`//\s*@(.+)$`)
)

// scanGoSources は internal/ 以下の *.go（_test.go を除く）を走査し、
// 命令名ごとの定義位置の候補一覧を作る。
func scanGoSources(root string) map[string][]srcMatch {
	result := map[string][]srcMatch{}
	add := func(name, file string, line, priority int, docRest string) {
		desc, yomi := "", ""
		hasDoc := false
		if docRest != "" {
			desc, yomi = splitDescYomi(docRest)
			hasDoc = desc != ""
		}
		result[name] = append(result[name], srcMatch{
			file: file, line: line, priority: priority,
			desc: desc, yomi: yomi, hasDoc: hasDoc,
		})
	}
	docRestOf := func(line string) string {
		if m := reDocComment.FindStringSubmatch(line); len(m) > 1 {
			return m[1]
		}
		return ""
	}

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel := filepath.ToSlash(path)
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			ln := i + 1
			if m := reMAssign.FindStringSubmatch(line); len(m) > 1 {
				add(m[1], rel, ln, 100, docRestOf(line))
			}
			if m := reMapKey.FindStringSubmatch(line); len(m) > 1 {
				add(m[1], rel, ln, 90, docRestOf(line))
			}
			if m := reImplMap.FindStringSubmatch(line); len(m) > 1 {
				add(m[1], rel, ln, 80, docRestOf(line))
			}
			if m := reAddFunc.FindStringSubmatch(line); len(m) > 1 {
				add(m[1], rel, ln, 60, docRestOf(line))
			}
			if m := reAddPlain.FindStringSubmatch(line); len(m) > 1 {
				add(m[1], rel, ln, 55, docRestOf(line))
			}
			if m := reListAssign.FindStringSubmatch(line); len(m) > 1 {
				add(m[1], rel, ln, 40, docRestOf(line))
			}
			if m := reRangeList.FindStringSubmatch(line); len(m) > 1 {
				for _, part := range strings.Split(m[1], ",") {
					part = strings.Trim(strings.TrimSpace(part), `"`)
					if part != "" {
						add(part, rel, ln, 20, "")
					}
				}
			}
		}
		return nil
	})
	return result
}

// bestLocation は命令名ごとに、最も優先度の高い定義位置を選ぶ。
func bestLocation(matches []srcMatch) (file string, line, priority int) {
	priority = -1
	for _, m := range matches {
		if m.priority > priority {
			file, line, priority = m.file, m.line, m.priority
		}
	}
	return
}

// bestDoc は命令名ごとに、`// @説明 // @よみ` コメントが付いている定義の中で
// 最も優先度の高いものを選ぶ。無ければ ok=false。
func bestDoc(matches []srcMatch) (desc, yomi string, ok bool) {
	priority := -1
	for _, m := range matches {
		if m.hasDoc && m.priority > priority {
			desc, yomi, priority = m.desc, m.yomi, m.priority
			ok = true
		}
	}
	return
}

func main() {
	tsDocs := parseTSPlugins()
	goSrc := scanGoSources("internal")

	reg := stdlib.NewRegistry(
		nodelib.New(), sqlitelib.New(),
		officelib.New(), pdflib.New(), imagelib.New(), guilib.New(),
	)
	list := reg.FuncList()

	var allDocs []CommandDoc

	for name, item := range list {
		if item.Type != "func" {
			continue
		}

		doc, tsOK := tsDocs[name]
		if !tsOK {
			doc = CommandDoc{
				Name:     name,
				Type:     item.Type,
				Josi:     item.Josi,
				Category: "命令",
				Desc:     fmt.Sprintf("命令『%s』を実行します", name),
				Template: makeTemplate(name, item.Josi),
			}
		}

		// Go側のソースコメントは、本家(TS)や汎用フォールバックより優先する
		// （Go独自命令の説明・読みはGoのソースコードが正典）。
		if desc, yomi, ok := bestDoc(goSrc[name]); ok {
			doc.Desc = desc
			doc.Yomi = yomi
		}

		// Ensure Josi from Go runtime list takes precedence if defined
		if len(item.Josi) > 0 {
			doc.Josi = item.Josi
			doc.Template = makeTemplate(name, item.Josi)
		} else if len(doc.Josi) == 0 {
			doc.Josi = nil
			doc.Template = name
		} else if doc.Template == "" {
			doc.Template = makeTemplate(name, doc.Josi)
		}
		if template, ok := insertionTemplateOverrides[name]; ok {
			doc.Template = template
		}

		if file, line, priority := bestLocation(goSrc[name]); priority >= 0 {
			doc.File = file
			doc.Line = line
			doc.URL = fmt.Sprintf(
				"https://github.com/kujirahand/nadesiko3go/blob/%s/%s#L%d",
				sourceBranch, file, line,
			)
		}

		doc.Plugin = pluginForFile(doc.File)
		// 本家(TS)に対応する命令が無いもの（≒Go独自命令）は、「命令」という
		// 意味のないグループに丸めず、定義ファイルに応じた具体的なグループ名を付ける。
		if !tsOK && doc.File != "" {
			doc.Category = groupForFile(doc.File)
		}

		allDocs = append(allDocs, doc)
	}

	sort.Slice(allDocs, func(i, j int) bool {
		return allDocs[i].Name < allDocs[j].Name
	})

	outPath := "cmd/gonako-gui/ui/command-list.json"
	b, err := json.MarshalIndent(allDocs, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSONエンコードエラー: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outPath, b, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "ファイル出力エラー: %v\n", err)
		os.Exit(1)
	}

	withYomi := 0
	withLoc := 0
	for _, d := range allDocs {
		if d.Yomi != "" {
			withYomi++
		}
		if d.URL != "" {
			withLoc++
		}
	}
	fmt.Printf("[OK] %d 件の命令情報を %s に生成しました。(フリガナ %d件 / ソース位置 %d件)\n",
		len(allDocs), outPath, withYomi, withLoc)
}
