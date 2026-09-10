// Package commanddoc は、なでしこの命令一覧(JSON)からの検索と、
// マニュアルサイトの検索を提供する。`gonako doc KEYWORD` の中身である。
//
// 命令一覧は `just gen-command-list` が生成した command-list.json を
// //go:embed でバイナリに埋め込むので、オフラインでも検索できる。
package commanddoc

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

//go:embed command-list.json
var commandListJSON []byte

// docBaseURL はなでしこ3マニュアルの場所。
const docBaseURL = "https://nadesi.com/v3/doc/"

// Command は命令1つ分の情報。JSONのキーは command-list.json に合わせてある。
type Command struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Josi     [][]string `json:"josi,omitempty"`
	Plugin   string     `json:"plugin,omitempty"`
	Category string     `json:"category,omitempty"`
	Desc     string     `json:"desc,omitempty"`
	Template string     `json:"template,omitempty"`
	Yomi     string     `json:"yomi,omitempty"`
	File     string     `json:"file,omitempty"`
	Line     int        `json:"line,omitempty"`
	// URL はGo版のソースコードの場所（GitHub）。
	URL string `json:"url,omitempty"`
	// DocURL はマニュアルの解説ページ。JSONには含まれないのでここで補う。
	DocURL string `json:"doc_url,omitempty"`
	// Score は検索の一致度。並び順の理由をAIにも見せるため出力に含める。
	Score int `json:"score,omitempty"`
}

// Commands は埋め込まれた命令一覧を返す。
func Commands() ([]Command, error) {
	var list []Command
	if err := json.Unmarshal(commandListJSON, &list); err != nil {
		return nil, fmt.Errorf("命令一覧JSONを読み込めません: %w", err)
	}
	for i := range list {
		list[i].DocURL = DocURL(list[i].Name)
	}
	return list, nil
}

// DocURL は命令名からマニュアルの解説ページのURLを作る。
func DocURL(name string) string {
	return docBaseURL + "index.php?" + url.QueryEscape("gonako/"+name)
}

// Search は命令一覧をキーワードで検索する。キーワードを複数与えた場合は、
// そのすべてに一致する命令だけを返す（AND検索）。
// 結果は一致度の高い順、同点なら命令名順に並ぶ。
func Search(list []Command, keywords []string) []Command {
	var found []Command
	for _, cmd := range list {
		total := 0
		matchedAll := true
		for _, keyword := range keywords {
			score := scoreCommand(cmd, keyword)
			if score == 0 {
				matchedAll = false
				break
			}
			total += score
		}
		if !matchedAll {
			continue
		}
		cmd.Score = total
		found = append(found, cmd)
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Score != found[j].Score {
			return found[i].Score > found[j].Score
		}
		return found[i].Name < found[j].Name
	})
	return found
}

// scoreCommand は命令とキーワードの一致度を返す。0なら一致しない。
// 命令名そのものへの一致を最優先し、読み・説明・分類の順に弱くする。
func scoreCommand(cmd Command, keyword string) int {
	key := fold(keyword)
	if key == "" {
		return 0
	}
	name := fold(cmd.Name)
	switch {
	case name == key:
		return 100
	case strings.HasPrefix(name, key):
		return 80
	case strings.Contains(name, key):
		return 60
	}
	if strings.Contains(fold(cmd.Yomi), key) {
		return 40
	}
	if strings.Contains(fold(cmd.Desc), key) {
		return 20
	}
	if strings.Contains(fold(cmd.Category), key) || strings.Contains(fold(cmd.Plugin), key) {
		return 10
	}
	return 0
}

// fold は比較用に文字列をそろえる。英字は大文字小文字を区別しない。
func fold(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
