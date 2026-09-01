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

type CommandDoc struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Josi     [][]string `json:"josi"`
	Category string     `json:"category"`
	Desc     string     `json:"desc"`
	Template string     `json:"template"`
	Yomi     string     `json:"yomi,omitempty"`
}

var (
	// '命令名': { // @説明 // @読み
	cmdHeaderRe = regexp.MustCompile(`['"]([^'"]+)['"]\s*:\s*\{\s*//\s*@([^\n/]+)(?://\s*@([^\n]+))?`)
	// // @カテゴリー名
	catRe = regexp.MustCompile(`^\s*//\s*@([^@\n\r/]+)$`)
	// josi: [...]
	josiRe = regexp.MustCompile(`josi\s*:\s*(\[[^;{}]+\])`)
)

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
				desc := strings.TrimSpace(m[2])
				yomi := ""
				if len(m) > 3 {
					yomi = strings.TrimSpace(m[3])
				}

				var josi [][]string
				for j := i; j < i+12 && j < len(lines); j++ {
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

func goSpecificDocs() map[string]CommandDoc {
	return map[string]CommandDoc{
		"ウィンドウ作成": {
			Name:     "ウィンドウ作成",
			Type:     "func",
			Josi:     [][]string{{"から", "で", "を"}},
			Category: "GUI",
			Desc:     "指定したURLまたはHTMLコードからWebViewウィンドウを新規作成して表示する",
			Template: "【URLまたはHTML】でウィンドウ作成",
		},
		"ウィンドウ設定": {
			Name:     "ウィンドウ設定",
			Type:     "func",
			Josi:     [][]string{{"の", "で", "を", "に"}},
			Category: "GUI",
			Desc:     "辞書オブジェクトでウィンドウのタイトルやサイズ（幅・高さ）を設定する",
			Template: "【{タイトル, サイズ: [幅, 高さ]}】のウィンドウ設定",
		},
		"エクセルブック作成": {
			Name:     "エクセルブック作成",
			Type:     "func",
			Josi:     nil,
			Category: "オフィス",
			Desc:     "新規Excelブックオブジェクトを作成してハンドルを返す",
			Template: "エクセルブック作成",
		},
		"エクセル開": {
			Name:     "エクセル開",
			Type:     "func",
			Josi:     [][]string{{"を", "から"}},
			Category: "オフィス",
			Desc:     "指定パスのExcelブックを開いてハンドルを返す",
			Template: "【ファイル名】を開く",
		},
		"エクセル保存": {
			Name:     "エクセル保存",
			Type:     "func",
			Josi:     [][]string{{"へ", "に"}},
			Category: "オフィス",
			Desc:     "現在のアクティブExcelブックを指定パスへ保存する",
			Template: "【ファイル名】へエクセル保存",
		},
		"エクセルセル設定": {
			Name:     "エクセルセル設定",
			Type:     "func",
			Josi:     [][]string{{"の"}, {"に", "へ"}, {"を"}},
			Category: "オフィス",
			Desc:     "指定シートのセル位置（例: 'A1'）に値を書き込む",
			Template: "【シート名】の【セル名】に【値】をエクセルセル設定",
		},
		"エクセルセル取得": {
			Name:     "エクセルセル取得",
			Type:     "func",
			Josi:     [][]string{{"から", "を", "の"}},
			Category: "オフィス",
			Desc:     "指定セル位置（例: 'A1'）の値を取得して返す",
			Template: "【セル名】のエクセルセル取得",
		},
		"エクセル一括取得": {
			Name:     "エクセル一括取得",
			Type:     "func",
			Josi:     [][]string{{"から"}, {"までの", "まで", "の"}},
			Category: "オフィス",
			Desc:     "指定範囲（例: 'A1' から 'C10'）の値を2次元配列として一括取得する",
			Template: "【開始セル】から【終了セル】までのエクセル一括取得",
		},
		"エクセルシート列挙": {
			Name:     "エクセルシート列挙",
			Type:     "func",
			Josi:     nil,
			Category: "オフィス",
			Desc:     "Excelブック内のシート名一覧を配列で返す",
			Template: "エクセルシート列挙",
		},
		"PDF新規作成": {
			Name:     "PDF新規作成",
			Type:     "func",
			Josi:     nil,
			Category: "オフィス",
			Desc:     "新規PDFドキュメントを作成してハンドルを返す",
			Template: "PDF新規作成",
		},
		"ページ追加": {
			Name:     "ページ追加",
			Type:     "func",
			Josi:     nil,
			Category: "オフィス",
			Desc:     "PDFドキュメントに新しいページを追加する",
			Template: "ページ追加",
		},
		"テキスト描画": {
			Name:     "テキスト描画",
			Type:     "func",
			Josi:     [][]string{{"の", "を"}},
			Category: "オフィス",
			Desc:     "PDFドキュメントの現在位置にテキストを描画する",
			Template: "【文字列】をテキスト描画",
		},
		"PDF保存": {
			Name:     "PDF保存",
			Type:     "func",
			Josi:     [][]string{{"へ", "に"}},
			Category: "オフィス",
			Desc:     "PDFドキュメントを指定ファイルパスへ出力保存する",
			Template: "【ファイル名】へPDF保存",
		},
		"画像新規作成": {
			Name:     "画像新規作成",
			Type:     "func",
			Josi:     [][]string{{"の", "で"}},
			Category: "グラフィック",
			Desc:     "指定サイズ [幅, 高さ] の新しいRGBA画像キャンバスを作成する",
			Template: "【[幅, 高さ]】の画像新規作成",
		},
		"画像開": {
			Name:     "画像開",
			Type:     "func",
			Josi:     [][]string{{"を", "の", "から"}},
			Category: "グラフィック",
			Desc:     "画像ファイル (PNG/JPEG/GIF) を読み込んでキャンバスを作成する",
			Template: "【ファイル名】を画像開く",
		},
		"画像保存": {
			Name:     "画像保存",
			Type:     "func",
			Josi:     [][]string{{"へ", "に"}},
			Category: "グラフィック",
			Desc:     "現在の画像をPNG/JPEGファイルへ保存する",
			Template: "【ファイル名】へ画像保存",
		},
		"画像矩形描画": {
			Name:     "画像矩形描画",
			Type:     "func",
			Josi:     [][]string{{"を"}, {"で"}},
			Category: "グラフィック",
			Desc:     "画像上の指定矩形 [X, Y, 幅, 高さ] を指定色で塗りつぶす",
			Template: "【[X, Y, 幅, 高さ]】を【色】で画像矩形描画",
		},
		"画像文字描画": {
			Name:     "画像文字描画",
			Type:     "func",
			Josi:     [][]string{{"を"}, {"で"}},
			Category: "グラフィック",
			Desc:     "画像上の指定位置にテキストを描画する",
			Template: "【文字列】を【[X, Y]】で画像文字描画",
		},
	}
}

func main() {
	tsDocs := parseTSPlugins()
	goDocs := goSpecificDocs()

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

		doc, ok := goDocs[name]
		if !ok {
			doc, ok = tsDocs[name]
		}

		if !ok {
			doc = CommandDoc{
				Name:     name,
				Type:     item.Type,
				Josi:     item.Josi,
				Category: "命令",
				Desc:     fmt.Sprintf("命令『%s』を実行します", name),
				Template: makeTemplate(name, item.Josi),
			}
		}

		// Ensure Josi from Go runtime list takes precedence if defined
		if len(item.Josi) > 0 {
			doc.Josi = item.Josi
			doc.Template = makeTemplate(name, item.Josi)
		} else if doc.Template == "" {
			doc.Template = makeTemplate(name, doc.Josi)
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

	fmt.Printf("[OK] %d 件の命令情報を %s に生成しました。\n", len(allDocs), outPath)
}
