//go:build ignore

// analyze-gocode は nadesiko3go リポジトリ内の Go 言語ソースコードの
// 行数・文字数・ファイル数・バイト数等を集計・分析するスクリプト。
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// FileStats は1ファイルの集計情報
type FileStats struct {
	Path        string
	Lines       int // 総行数
	CodeLines   int // コード行
	CommentLine int // コメント行
	BlankLines  int // 空行
	Runes       int // 文字数 (Unicode rune数)
	NonSpace    int // 空白・改行を除く文字数
	Bytes       int // バイト数
	IsTest      bool
	Category    string // "cmd", "internal", "pkg", "scripts", "benchmark", "other"
	Dir         string // 集計用ディレクトリ名
}

// GroupStats はディレクトリまたはカテゴリごとの集計
type GroupStats struct {
	Name        string
	Files       int
	Lines       int
	CodeLines   int
	CommentLine int
	BlankLines  int
	Runes       int
	NonSpace    int
	Bytes       int
}

func (g *GroupStats) Add(f FileStats) {
	g.Files++
	g.Lines += f.Lines
	g.CodeLines += f.CodeLines
	g.CommentLine += f.CommentLine
	g.BlankLines += f.BlankLines
	g.Runes += f.Runes
	g.NonSpace += f.NonSpace
	g.Bytes += f.Bytes
}

func main() {
	rootPath := flag.String("root", ".", "対象のルートディレクトリ")
	flag.Parse()

	var files []FileStats

	err := filepath.WalkDir(*rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(*rootPath, path)
		if err != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)

		// 除外ディレクトリ
		if d.IsDir() {
			base := d.Name()
			if strings.HasPrefix(base, ".") && base != "." {
				return filepath.SkipDir
			}
			if rel == "nadesiko3" || rel == "bin" || rel == "out" || rel == "benchmark/build" || rel == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Goファイルのみ対象
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		// 除外ファイルチェック（パスに含まれる場合など）
		if strings.HasPrefix(rel, "nadesiko3/") ||
			strings.HasPrefix(rel, "benchmark/build/") ||
			strings.HasPrefix(rel, "bin/") ||
			strings.HasPrefix(rel, "out/") {
			return nil
		}

		st, err := analyzeFile(path, rel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "警告: %s の読み込みに失敗しました: %v\n", path, err)
			return nil
		}
		files = append(files, st)
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}

	printReport(files)
}

func analyzeFile(fullPath, relPath string) (FileStats, error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return FileStats{}, err
	}

	isTest := strings.HasSuffix(relPath, "_test.go")

	category := "other"
	parts := strings.Split(relPath, "/")
	if len(parts) > 0 {
		switch parts[0] {
		case "cmd":
			category = "cmd"
		case "internal":
			category = "internal"
		case "pkg":
			category = "pkg"
		case "scripts":
			category = "scripts"
		case "benchmark":
			category = "benchmark"
		}
	}

	// ディレクトリ名
	dir := filepath.Dir(relPath)
	if dir == "." {
		dir = "(root)"
	}

	totalRunes := utf8.RuneCount(data)
	totalBytes := len(data)

	// 行単位の解析
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" && len(data) > 0 && data[len(data)-1] == '\n' {
		lines = lines[:len(lines)-1]
	}

	codeLines := 0
	commentLines := 0
	blankLines := 0
	inBlockComment := false
	nonSpaceRunes := 0

	for _, r := range string(data) {
		if !unicode.IsSpace(r) {
			nonSpaceRunes++
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			blankLines++
			continue
		}

		if inBlockComment {
			commentLines++
			if strings.Contains(trimmed, "*/") {
				inBlockComment = false
			}
			continue
		}

		if strings.HasPrefix(trimmed, "/*") {
			commentLines++
			if !strings.Contains(trimmed[2:], "*/") {
				inBlockComment = true
			}
			continue
		}

		if strings.HasPrefix(trimmed, "//") {
			commentLines++
			continue
		}

		codeLines++
	}

	return FileStats{
		Path:        relPath,
		Lines:       len(lines),
		CodeLines:   codeLines,
		CommentLine: commentLines,
		BlankLines:  blankLines,
		Runes:       totalRunes,
		NonSpace:    nonSpaceRunes,
		Bytes:       totalBytes,
		IsTest:      isTest,
		Category:    category,
		Dir:         dir,
	}, nil
}

func printReport(files []FileStats) {
	var (
		totalGroup  GroupStats
		prodGroup   GroupStats
		testGroup   GroupStats
		toolGroup   GroupStats
		dirStatsMap = make(map[string]*GroupStats)
	)

	totalGroup.Name = "総計 (全体)"
	prodGroup.Name = "本体実装 (cmd, internal, pkg)"
	testGroup.Name = "テストコード (*_test.go)"
	toolGroup.Name = "スクリプト・ベンチマーク"

	for _, f := range files {
		totalGroup.Add(f)

		if f.Category == "scripts" || f.Category == "benchmark" {
			toolGroup.Add(f)
		} else if f.IsTest {
			testGroup.Add(f)
		} else {
			prodGroup.Add(f)
		}

		dStat, exists := dirStatsMap[f.Dir]
		if !exists {
			dStat = &GroupStats{Name: f.Dir}
			dirStatsMap[f.Dir] = dStat
		}
		dStat.Add(f)
	}

	fmt.Println("===============================================================================================")
	fmt.Println("                            nadesiko3go Go言語コード規模 分析レポート")
	fmt.Println("===============================================================================================")
	fmt.Printf("%s %6s %8s %8s %8s %8s %12s %10s\n",
		padRight("区分", 34), "ファイル", "総行数", "コード行", "コメント", "空行", "文字数(rune)", "サイズ")
	fmt.Println("-----------------------------------------------------------------------------------------------")

	printRow(prodGroup)
	printRow(testGroup)
	printRow(toolGroup)
	fmt.Println("-----------------------------------------------------------------------------------------------")
	printRow(totalGroup)
	fmt.Println("===============================================================================================")
	fmt.Printf("※ 空白・改行を除く純粋文字数: %s 文字 (全体の約%.1f%%) / 総バイト数: %s\n\n",
		formatNum(totalGroup.NonSpace),
		float64(totalGroup.NonSpace)/float64(totalGroup.Runes)*100,
		formatBytes(totalGroup.Bytes))

	// ディレクトリ別内訳
	var dirs []*GroupStats
	for _, st := range dirStatsMap {
		dirs = append(dirs, st)
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Lines > dirs[j].Lines
	})

	fmt.Println("【 ディレクトリ別内訳 (行数順) 】")
	fmt.Println("-----------------------------------------------------------------------------------------------")
	fmt.Printf("%s %6s %8s %8s %8s %12s %10s\n",
		padRight("ディレクトリ", 34), "ファイル", "総行数", "コード行", "コメント", "文字数(rune)", "サイズ")
	fmt.Println("-----------------------------------------------------------------------------------------------")
	for _, d := range dirs {
		fmt.Printf("%s %6d %8s %8s %8s %12s %10s\n",
			padRight(truncate(d.Name, 34), 34),
			d.Files,
			formatNum(d.Lines),
			formatNum(d.CodeLines),
			formatNum(d.CommentLine),
			formatNum(d.Runes),
			formatBytes(d.Bytes),
		)
	}
	fmt.Println("-----------------------------------------------------------------------------------------------")
}

func printRow(g GroupStats) {
	fmt.Printf("%s %6d %8s %8s %8s %8s %12s %10s\n",
		padRight(g.Name, 34),
		g.Files,
		formatNum(g.Lines),
		formatNum(g.CodeLines),
		formatNum(g.CommentLine),
		formatNum(g.BlankLines),
		formatNum(g.Runes),
		formatBytes(g.Bytes),
	)
}

func strWidth(s string) int {
	w := 0
	for _, r := range s {
		if r > 0x7f {
			w += 2
		} else {
			w += 1
		}
	}
	return w
}

func padRight(s string, width int) string {
	w := strWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func formatNum(n int) string {
	in := fmt.Sprintf("%d", n)
	var out bytes.Buffer
	l := len(in)
	for i, c := range in {
		if i > 0 && (l-i)%3 == 0 {
			out.WriteRune(',')
		}
		out.WriteRune(c)
	}
	return out.String()
}

func formatBytes(b int) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	} else if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%.2f MB", float64(b)/(1024*1024))
}

func truncate(s string, maxLen int) string {
	if strWidth(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	var sb strings.Builder
	for _, r := range runes {
		if strWidth(sb.String()+string(r)) > maxLen-3 {
			break
		}
		sb.WriteRune(r)
	}
	sb.WriteString("...")
	return sb.String()
}
