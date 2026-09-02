// Package commanddiff compares plugin_system command names with gonako's
// parser command table.
package commanddiff

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
)

type sourceCommand struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Plugin string `json:"plugin"`
}

// Report is the name-level difference between the official plugin_system and
// gonako. Constants and commands belonging to other plugins are not included.
type Report struct {
	OfficialCount int
	GoCount       int
	Missing       []string
	GoOnly        []string
}

// Compare reads nadesiko3/doc/command_list.json and compares its plugin_system
// functions with the functions registered by gonako.
func Compare(r io.Reader, funcs lexer.FuncList) (Report, error) {
	var source []sourceCommand
	if err := json.NewDecoder(r).Decode(&source); err != nil {
		return Report{}, fmt.Errorf("命令一覧JSONを読み込めません: %w", err)
	}

	official := map[string]struct{}{}
	for _, cmd := range source {
		if cmd.Plugin != "plugin_system" || cmd.Type != "関数" {
			continue
		}
		if cmd.Name == "" {
			return Report{}, fmt.Errorf("plugin_systemに名前のない命令があります")
		}
		official[cmd.Name] = struct{}{}
	}
	if len(official) == 0 {
		return Report{}, fmt.Errorf("plugin_systemの命令がありません")
	}

	goCommands := map[string]struct{}{}
	for name, item := range funcs {
		if item != nil && item.Type == "func" {
			goCommands[name] = struct{}{}
		}
	}

	report := Report{OfficialCount: len(official), GoCount: len(goCommands)}
	for name := range official {
		if _, ok := goCommands[name]; !ok {
			report.Missing = append(report.Missing, name)
		}
	}
	for name := range goCommands {
		if _, ok := official[name]; !ok {
			report.GoOnly = append(report.GoOnly, name)
		}
	}
	sort.Strings(report.Missing)
	sort.Strings(report.GoOnly)
	return report, nil
}

// WriteText writes a stable, human-readable report.
func WriteText(w io.Writer, source, sourceRef string, report Report) {
	fmt.Fprintln(w, "plugin_system 命令差分")
	fmt.Fprintf(w, "比較元: %s\n", source)
	if sourceRef != "" {
		fmt.Fprintf(w, "比較元コミット: %s\n", sourceRef)
	}
	fmt.Fprintf(w, "本家: %d命令 / Go版: %d命令\n", report.OfficialCount, report.GoCount)
	writeNames(w, "未実装", report.Missing)
	writeNames(w, "Go版のみ", report.GoOnly)
}

func writeNames(w io.Writer, heading string, names []string) {
	fmt.Fprintf(w, "\n%s (%d):\n", heading, len(names))
	if len(names) == 0 {
		fmt.Fprintln(w, "  なし")
		return
	}
	for _, name := range names {
		fmt.Fprintf(w, "  %s\n", name)
	}
}
