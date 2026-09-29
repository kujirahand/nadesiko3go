//go:build ignore

// gen-manual.go は、命令一覧(command-list.json)から次の2つを更新する。
//
//  1. manual/gonako/<命令>.txt  --- マニュアルが無い命令の雛形（既存ファイルは上書きしない）
//  2. manual/gonako-commands.db --- nadesiko3doc用の命令一覧DB（毎回ゼロから作り直す）
//
// 先に `just gen-command-list` で command-list.json を更新しておくこと。
// （`just gen-manual` は両方をまとめて実行する）
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	listPath  = "internal/commanddoc/command-list.json"
	manualDir = "manual/gonako"
	dbPath    = "manual/gonako-commands.db"
	pluginTag = "gonako"
)

type commandDoc struct {
	Name     string     `json:"name"`
	Josi     [][]string `json:"josi"`
	Plugin   string     `json:"plugin"`
	Category string     `json:"category"`
	Desc     string     `json:"desc"`
	Yomi     string     `json:"yomi"`
	URL      string     `json:"url"`
}

// skeletonPlugins は、マニュアルの雛形を自動生成する対象のプラグイン。
// plugin_math/plugin_csv/plugin_toml は本家nadesiko3docに同名のページがあるため対象外。
var skeletonPlugins = map[string]bool{
	"gonako":      true,
	"plugin_node": true,
}

// makeArgs は助詞リストから nadesiko3doc 形式の引数表記を作る。
// 例: 置換 → "AのBをCに|AでBからCへ"、足 → "AにBを|Aと"
func makeArgs(josi [][]string) string {
	if len(josi) == 0 {
		return ""
	}
	names := "ABCDEFGH"
	maxLen := 0
	for _, g := range josi {
		if len(g) > maxLen {
			maxLen = len(g)
		}
	}
	var variants []string
	for k := 0; k < maxLen; k++ {
		var sb strings.Builder
		for i, g := range josi {
			if k >= len(g) {
				continue
			}
			n := "A"
			if i < len(names) {
				n = string(names[i])
			}
			sb.WriteString(n + g[k])
		}
		if sb.Len() > 0 {
			variants = append(variants, sb.String())
		}
	}
	return strings.Join(variants, "|")
}

// skeleton はマニュアルの雛形を返す。利用例は実行結果を検証できないので書かない。
func skeleton(c commandDoc) string {
	desc := c.Desc
	if desc == "" {
		desc = fmt.Sprintf("命令『%s』を実行します", c.Name)
	}
	return fmt.Sprintf("●説明\n\n%s\n\n●参照\n\n", desc)
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

func main() {
	data, err := os.ReadFile(listPath)
	if err != nil {
		fatal("命令一覧を読めません(先に just gen-command-list を実行): %v", err)
	}
	var cmds []commandDoc
	if err := json.Unmarshal(data, &cmds); err != nil {
		fatal("命令一覧のJSONエラー: %v", err)
	}
	if err := os.MkdirAll(manualDir, 0o755); err != nil {
		fatal("フォルダ作成エラー: %v", err)
	}

	// 1. マニュアルの雛形を生成（既存は上書きしない）
	created := 0
	for _, c := range cmds {
		path := filepath.Join(manualDir, c.Name+".txt")
		if _, err := os.Stat(path); err == nil || !skeletonPlugins[c.Plugin] {
			continue
		}
		if err := os.WriteFile(path, []byte(skeleton(c)), 0o644); err != nil {
			fatal("マニュアル出力エラー: %v", err)
		}
		fmt.Println("[新規] " + path)
		created++
	}

	// 2. DBを作り直す
	_ = os.Remove(dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fatal("DBを開けません: %v", err)
	}
	defer db.Close()
	schema := []string{
		`CREATE TABLE commands (
  command_id INTEGER PRIMARY KEY,
  pagename TEXT NOT NULL,
  plugin TEXT,
  genre TEXT,
  name TEXT,
  type TEXT,
  kana TEXT,
  args TEXT,
  desc TEXT,
  src_url TEXT DEFAULT '',
  ctime INTEGER,
  mtime INTEGER
)`,
		`CREATE UNIQUE INDEX commands_index ON commands (pagename)`,
		`CREATE TABLE plugins (
  name TEXT NOT NULL,
  desc TEXT DEFAULT '',
  nakotype TEXT DEFAULT ''
)`,
		`CREATE UNIQUE INDEX plugins_index ON plugins (name)`,
	}
	for _, s := range schema {
		if _, err := db.Exec(s); err != nil {
			fatal("スキーマ作成エラー: %v", err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		fatal("トランザクション開始エラー: %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO plugins (name, desc, nakotype) VALUES (?, ?, ?)`,
		pluginTag, "", "拡張プラグイン,gonako"); err != nil {
		fatal("plugins登録エラー: %v", err)
	}
	count := 0
	for _, c := range cmds {
		// マニュアルのページがある命令だけを登録する
		info, err := os.Stat(filepath.Join(manualDir, c.Name+".txt"))
		if err != nil {
			continue
		}
		t := info.ModTime().Unix()
		count++
		_, err = tx.Exec(`INSERT INTO commands
  (command_id, pagename, plugin, genre, name, type, kana, args, desc, src_url, ctime, mtime)
  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			count, pluginTag+"/"+c.Name, pluginTag, c.Category, c.Name, "関数",
			c.Yomi, makeArgs(c.Josi), c.Desc, c.URL, t, t)
		if err != nil {
			fatal("commands登録エラー(%s): %v", c.Name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		fatal("コミットエラー: %v", err)
	}
	fmt.Printf("[OK] マニュアル雛形 %d 件を作成、%s に %d 件を登録しました。\n", created, dbPath, count)
}
