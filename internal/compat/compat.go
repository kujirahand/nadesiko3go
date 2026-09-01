// Package compat reads language-independent compatibility fixtures and writes
// results that can be consumed by nadesiko3's compat:check command.
package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

type Case struct {
	Name            string            `json:"name"`
	Code            *string           `json:"code"`
	Vars            []string          `json:"vars,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	Unsupported     map[string]string `json:"unsupported,omitempty"`
	IntentionalDiff map[string]string `json:"intentionalDiff,omitempty"`
}

type Group struct {
	Group       string `json:"group"`
	Description string `json:"description,omitempty"`
	Cases       []Case `json:"cases"`
}

type ErrorResult struct {
	Type    string `json:"type"`
	Line    *int   `json:"line"`
	Message string `json:"message"`
}

type Result struct {
	Name   string                 `json:"name"`
	Status string                 `json:"status"`
	Log    string                 `json:"log"`
	Vars   map[string]interface{} `json:"vars,omitempty"`
	Error  *ErrorResult           `json:"error,omitempty"`
}

type OutputGroup struct {
	Group       string            `json:"group"`
	Description string            `json:"description,omitempty"`
	GeneratedBy string            `json:"generatedBy"`
	Results     map[string]Result `json:"results"`
}

type Summary struct {
	Groups int
	Cases  int
}

func Load(casesDir string) ([]Group, error) {
	entries, err := os.ReadDir(casesDir)
	if err != nil {
		return nil, fmt.Errorf("ケースディレクトリを読めません: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, errors.New("ケースJSONがありません")
	}

	groups := make([]Group, 0, len(files))
	for _, name := range files {
		filePath := filepath.Join(casesDir, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("%sを読めません: %w", name, err)
		}
		var group Group
		if err := json.Unmarshal(data, &group); err != nil {
			return nil, fmt.Errorf("%sのJSONが不正です: %w", name, err)
		}
		if err := validateGroup(group, strings.TrimSuffix(name, filepath.Ext(name))); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func validateGroup(group Group, filename string) error {
	if group.Group == "" {
		return errors.New("groupがありません")
	}
	if group.Group != filename {
		return fmt.Errorf("group %q がファイル名 %q と一致しません", group.Group, filename)
	}
	if group.Cases == nil {
		return errors.New("casesがありません")
	}
	names := make(map[string]struct{}, len(group.Cases))
	for i, testCase := range group.Cases {
		if testCase.Name == "" {
			return fmt.Errorf("cases[%d]にnameがありません", i)
		}
		if _, exists := names[testCase.Name]; exists {
			return fmt.Errorf("ケース名が重複しています: %s", testCase.Name)
		}
		if testCase.Code == nil {
			return fmt.Errorf("ケース%sにcodeがありません", testCase.Name)
		}
		names[testCase.Name] = struct{}{}
	}
	return nil
}

func Run(casesDir, outDir string) (Summary, error) {
	groups, err := Load(casesDir)
	if err != nil {
		return Summary{}, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Summary{}, fmt.Errorf("出力ディレクトリを作れません: %w", err)
	}

	summary := Summary{Groups: len(groups)}
	for _, group := range groups {
		output := OutputGroup{
			Group:       group.Group,
			Description: group.Description,
			GeneratedBy: "gonako (Go)",
			Results:     make(map[string]Result, len(group.Cases)),
		}
		for _, testCase := range group.Cases {
			result := Result{
				Name:   testCase.Name,
				Status: "error",
				Log:    "",
				Error: &ErrorResult{
					Type:    "UnsupportedError",
					Line:    nil,
					Message: "未実装",
				},
			}
			if _, parseErr := parser.ParseSource(*testCase.Code, "main.nako3", stdlib.ParserFuncList()); parseErr != nil {
				var nakoErr *errs.NakoError
				if errors.As(parseErr, &nakoErr) {
					line := nakoErr.Line
					result.Error = &ErrorResult{
						Type:    nakoErr.CompatType(),
						Line:    &line,
						Message: nakoErr.Error(),
					}
				} else {
					return Summary{}, fmt.Errorf("%s/%sの構文解析に失敗しました: %w", group.Group, testCase.Name, parseErr)
				}
			}
			output.Results[testCase.Name] = result
			summary.Cases++
		}
		if err := writeGroup(filepath.Join(outDir, group.Group+".json"), output); err != nil {
			return Summary{}, err
		}
	}
	return summary, nil
}

func writeGroup(filePath string, group OutputGroup) error {
	data, err := json.MarshalIndent(group, "", "  ")
	if err != nil {
		return fmt.Errorf("結果JSONを作れません: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("%sを書けません: %w", filePath, err)
	}
	return nil
}
