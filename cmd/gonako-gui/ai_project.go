package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed ai-template/AGENTS.md
var aiDevelopmentGuide string

var aiProjectFiles = map[string]string{
	"AGENTS.md": aiDevelopmentGuide,
	"CLAUDE.md": aiDevelopmentGuide,
}

// createAIProject は新しいプロジェクトフォルダとAI向けの指示ファイルを作る。
// 既存のフォルダやファイルを上書きしないため、フォルダ作成に失敗した時点で終了する。
func createAIProject(baseDir, name string) (string, []string, error) {
	projectPath, err := createNewFolder(baseDir, name)
	if err != nil {
		return "", nil, err
	}

	created := make([]string, 0, len(aiProjectFiles))
	for _, fileName := range []string{"AGENTS.md", "CLAUDE.md"} {
		filePath := filepath.Join(projectPath, fileName)
		if err := os.WriteFile(filePath, []byte(aiProjectFiles[fileName]), 0o644); err != nil {
			for _, createdPath := range created {
				_ = os.Remove(createdPath)
			}
			_ = os.Remove(projectPath)
			return "", nil, fmt.Errorf("%sを書き込めません: %w", fileName, err)
		}
		created = append(created, filePath)
	}
	return projectPath, created, nil
}

func aiProjectFileNames(paths []string) []string {
	names := make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, filepath.Base(path))
	}
	return names
}
