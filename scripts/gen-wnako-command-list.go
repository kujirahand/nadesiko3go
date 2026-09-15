//go:build ignore

// 本家ブラウザ版なでしこ(wnako3)の命令一覧を、gonakoの command-list.json と
// 同じ形へ変換するスクリプト（#101）。
//
//	入力: cmd/gonako-gui/ui/wnako3/command.json.js （just copy-nadesiko3 が取り込む）
//	出力: cmd/gonako-gui/ui/command-list-wnako.json  （GUIエディタの命令一覧が読む）
//	      internal/commanddoc/command-list-wnako.json（CUIの `gonako doc --wnako` が読む）
//
// command.json.js は次の形をしている。
//
//	module.exports = {"plugin_system":{"文字列処理":[["関数","置換","AのBをCに","説明","よみ"], ...]}}
//
// 配列の中身は [種別, 命令名, 引数の書式, 説明, よみ] である。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

const (
	srcPath = "cmd/gonako-gui/ui/wnako3/command.json.js"
)

var outPaths = []string{
	"cmd/gonako-gui/ui/command-list-wnako.json",
	"internal/commanddoc/command-list-wnako.json",
}

// CommandDoc は command-list.json と同じ形。
type CommandDoc struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Josi     [][]string `json:"josi,omitempty"`
	Plugin   string     `json:"plugin,omitempty"`
	Category string     `json:"category"`
	Desc     string     `json:"desc"`
	Template string     `json:"template"`
	Yomi     string     `json:"yomi,omitempty"`
}

// paramRe は「...A」「SRC」のような引数名と、それに続く助詞を取り出す。
// 引数名は半角英大文字と数字・アンダースコアで書かれている。
var paramRe = regexp.MustCompile(`(\.\.\.)?([A-Z][A-Z0-9_]*)([^A-Z]*)`)

// parseArgSpec は「AにBを/Aと」のような引数の書式を、助詞のグループと
// 挿入用テンプレートに変換する。'/' 区切りは呼び出し方全体の言い換えなので、
// 引数の位置ごとに助詞をまとめ直す。
func parseArgSpec(name, spec string) (josi [][]string, template string) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, name
	}
	var (
		groups  [][]string
		params  []string
		first   = true
		tmplBuf strings.Builder
	)
	for _, alt := range strings.Split(spec, "/") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		for i, m := range paramRe.FindAllStringSubmatch(alt, -1) {
			pName := m[1] + m[2]
			j := strings.TrimSpace(m[3])
			for len(groups) <= i {
				groups = append(groups, nil)
				params = append(params, "")
			}
			if params[i] == "" {
				params[i] = pName
			}
			if j != "" && !containsString(groups[i], j) {
				groups[i] = append(groups[i], j)
			}
			if first {
				fmt.Fprintf(&tmplBuf, "【%s】%s", pName, j)
			}
		}
		first = false
	}
	if len(groups) == 0 {
		return nil, name
	}
	// 助詞が1つも無い引数は、空文字のグループにして位置だけ残す。
	for i := range groups {
		if groups[i] == nil {
			groups[i] = []string{""}
		}
	}
	return groups, tmplBuf.String() + name
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// readCommandJSON は module.exports = {...} からJSON部分だけを取り出して読む。
func readCommandJSON(path string) (map[string]map[string][][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("%s にJSONらしき部分が見つかりません", path)
	}
	var parsed map[string]map[string][][]string
	if err := json.Unmarshal([]byte(text[start:end+1]), &parsed); err != nil {
		return nil, fmt.Errorf("%s を解析できません: %w", path, err)
	}
	return parsed, nil
}

func main() {
	parsed, err := readCommandJSON(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}

	var docs []CommandDoc
	seen := map[string]bool{}
	plugins := make([]string, 0, len(parsed))
	for plugin := range parsed {
		plugins = append(plugins, plugin)
	}
	sort.Strings(plugins)

	for _, plugin := range plugins {
		categories := make([]string, 0, len(parsed[plugin]))
		for category := range parsed[plugin] {
			categories = append(categories, category)
		}
		sort.Strings(categories)
		for _, category := range categories {
			for _, entry := range parsed[plugin][category] {
				if len(entry) < 2 {
					continue
				}
				kind, name := entry[0], entry[1]
				if name == "" || seen[name] {
					continue
				}
				seen[name] = true
				spec, desc, yomi := "", "", ""
				if len(entry) > 2 {
					spec = entry[2]
				}
				if len(entry) > 3 {
					desc = entry[3]
				}
				if len(entry) > 4 {
					yomi = entry[4]
				}
				doc := CommandDoc{
					Name:     name,
					Type:     "func",
					Plugin:   plugin,
					Category: category,
					Desc:     desc,
					Yomi:     yomi,
				}
				if kind == "定数" {
					// 定数は説明の代わりに値が入っているので、その旨を添える。
					doc.Type = "const"
					doc.Template = name
					if desc != "" {
						doc.Desc = fmt.Sprintf("定数 (値: %s)", desc)
					} else {
						doc.Desc = "定数"
					}
				} else {
					doc.Josi, doc.Template = parseArgSpec(name, spec)
				}
				docs = append(docs, doc)
			}
		}
	}

	sort.Slice(docs, func(i, j int) bool { return docs[i].Name < docs[j].Name })

	b, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSONエンコードエラー: %v\n", err)
		os.Exit(1)
	}
	for _, out := range outPaths {
		if err := os.WriteFile(out, b, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "ファイル出力エラー: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("[OK] wnakoの命令 %d 件を %s に生成しました。\n", len(docs), strings.Join(outPaths, ", "))
}
