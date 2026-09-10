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

// createAIProject は既存のプロジェクトフォルダへAI向けの指示ファイルを作る。
// どちらかのファイルが既にある場合は、片方だけ作ることも上書きもせずに終了する。
func createAIProject(projectDir string) (string, []string, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", nil, fmt.Errorf("フォルダの場所が分かりません: %w", err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return "", nil, fmt.Errorf("フォルダを確認できません: %w", err)
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("フォルダではありません: %s", absDir)
	}

	for _, fileName := range []string{"AGENTS.md", "CLAUDE.md"} {
		filePath := filepath.Join(absDir, fileName)
		if _, err := os.Lstat(filePath); err == nil {
			return "", nil, fmt.Errorf("%sは既に存在します。上書きしません", filePath)
		} else if !os.IsNotExist(err) {
			return "", nil, fmt.Errorf("%sを確認できません: %w", fileName, err)
		}
	}

	created := make([]string, 0, len(aiProjectFiles))
	for _, fileName := range []string{"AGENTS.md", "CLAUDE.md"} {
		filePath := filepath.Join(absDir, fileName)
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		createdCurrent := err == nil
		if err == nil {
			_, err = file.WriteString(aiProjectFiles[fileName])
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
		}
		if err != nil {
			if createdCurrent {
				_ = os.Remove(filePath)
			}
			for _, createdPath := range created {
				_ = os.Remove(createdPath)
			}
			return "", nil, fmt.Errorf("%sを書き込めません: %w", fileName, err)
		}
		created = append(created, filePath)
	}
	return absDir, created, nil
}

func aiProjectFileNames(paths []string) []string {
	names := make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, filepath.Base(path))
	}
	return names
}
