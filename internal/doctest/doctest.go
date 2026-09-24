// Package doctest runs sample code from the manual and fixed test data, then
// checks that it prints the documented result.
//
// The format is the one nadesiko3/doc/doctest.md defines: a 『{{{#nako3』 block
// whose body contains a 『### 表示結果:』 line. A block without one is prose, not
// a test. 『### GO表示結果:』 is the same, but marks a sample for a gonako専用
// (独自) command — it runs under CNako exactly like 『### 表示結果:』.
// 任意の接頭辞（『### L表示結果:』など）も同じ形式として抽出できる。
//
//	{{{#nako3
//	「こんにちは」と表示。
//	### 表示結果: こんにちは
//	}}}
package doctest

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Runtime says which engine a test needs.
type Runtime string

const (
	// CNako is the command-line engine, which this package can run.
	CNako Runtime = "cnako"
	// WNako is the browser engine. Its tests are collected but not run here.
	WNako Runtime = "wnako"
)

// マニュアルに書く表示結果ラベル。
const (
	LabelCNako  = "表示結果"
	LabelWNako  = "WEB表示結果"
	LabelGOnako = "GO表示結果"
)

// DefaultLabels は --label を省略したときの対象。現行の CNako 相当。
var DefaultLabels = []string{LabelCNako, LabelGOnako}

// Test is one sample block taken from a DocTest text file.
type Test struct {
	File string
	// Line is where 『{{{#nako3』 was, counting from 1.
	Line    int
	Code    string
	Expect  string
	Runtime Runtime
	// Label は 『表示結果』『GO表示結果』『L表示結果』など、期待出力の見出し。
	Label string
}

var (
	// expectHead matches the line that starts the expected output.
	// 接頭辞は英数字とアンダースコア（空でもよい）。WEB / GO / L など。
	expectHead = regexp.MustCompile(`^###[ \t]*([A-Za-z0-9_]*表示結果)[ \t]*[:：]?[ \t]?(.*)$`)
	// expectTail matches the second and later lines of it.
	expectTail = regexp.MustCompile(`^###[ \t]?(.*)$`)
	// trailingSpace matches the whitespace both sides trim before comparing.
	trailingSpace = regexp.MustCompile(`\s+$`)
)

// Extract pulls the sample blocks out of one DocTest file. A block with no
// expected output is skipped.
func Extract(text, file string) []Test {
	lines := splitLines(text)
	var tests []Test

	inBlock := false
	blockLine := 0
	var block []string
	for i, line := range lines {
		if !inBlock {
			if strings.HasPrefix(strings.TrimSpace(line), "{{{#nako3") {
				inBlock, blockLine, block = true, i+1, nil
			}
			continue
		}
		if strings.TrimSpace(line) == "}}}" {
			inBlock = false
			if test, ok := parseBlock(block, file, blockLine); ok {
				tests = append(tests, test)
			}
			continue
		}
		block = append(block, line)
	}
	return tests
}

// parseBlock splits a block into the code to run and the output to expect.
//
// Code may appear on both sides of the expected output, so the two halves are
// joined back together.
func parseBlock(block []string, file string, line int) (Test, bool) {
	head := -1
	var headMatch []string
	for i, s := range block {
		if m := expectHead.FindStringSubmatch(strings.TrimRight(s, " \t")); m != nil {
			head, headMatch = i, m
			break
		}
	}
	if head < 0 {
		return Test{}, false
	}

	label := headMatch[1]
	expects := []string{headMatch[2]}

	i := head + 1
	for ; i < len(block); i++ {
		m := expectTail.FindStringSubmatch(strings.TrimRight(block[i], " \t"))
		if m == nil {
			break
		}
		expects = append(expects, m[1])
	}

	code := append(append([]string{}, block[:head]...), block[i:]...)
	return Test{
		File:    file,
		Line:    line,
		Code:    strings.Join(code, "\n"),
		Expect:  trimTrailing(strings.Join(expects, "\n")),
		Runtime: runtimeOf(label),
		Label:   label,
	}, true
}

func runtimeOf(label string) Runtime {
	if label == LabelWNako {
		return WNako
	}
	return CNako
}

// Collect gathers the tests under the given files or folders, keeping only the
// ones for the given runtime. CNako は DefaultLabels（表示結果とGO表示結果）を対象にする。
func Collect(targets []string, runtime Runtime) ([]Test, error) {
	if runtime == WNako {
		return CollectByLabel(targets, LabelWNako)
	}
	return CollectByLabel(targets, DefaultLabels...)
}

// CollectByLabel は指定した表示結果ラベルのサンプルだけを集める。
// ラベルを省略すると DefaultLabels を使う。
func CollectByLabel(targets []string, labels ...string) ([]Test, error) {
	if len(labels) == 0 {
		labels = DefaultLabels
	}
	allowed := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		if n := NormalizeLabel(label); n != "" {
			allowed[n] = struct{}{}
		}
	}

	var tests []Test
	for _, target := range targets {
		files, err := findFiles(target)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			text := string(data)
			if !hasExpectation(text) {
				continue
			}
			for _, test := range Extract(text, file) {
				if _, ok := allowed[test.Label]; ok {
					tests = append(tests, test)
				}
			}
		}
	}
	return tests, nil
}

// NormalizeLabel は CLI や見出しからラベル名だけを取り出す。
// 『### L表示結果:』や『L表示結果』はどちらも 『L表示結果』になる。
func NormalizeLabel(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "###")
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ":：")
	return strings.TrimSpace(s)
}

// hasExpectation is a quick check that skips a page with no samples at all.
func hasExpectation(text string) bool {
	for _, line := range splitLines(text) {
		if expectHead.MatchString(strings.TrimRight(line, " \t")) {
			return true
		}
	}
	return false
}

// findFiles lists the .txt files under a path, which may itself be a file.
// The order is stable so that a run reports the same thing twice.
func findFiles(target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if strings.HasSuffix(target, ".txt") {
			return []string{target}, nil
		}
		return nil, nil
	}

	var files []string
	err = filepath.WalkDir(target, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".txt") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

// trimTrailing removes the trailing whitespace both the expected and the
// actual output have removed before they are compared.
func trimTrailing(s string) string { return trailingSpace.ReplaceAllString(s, "") }
