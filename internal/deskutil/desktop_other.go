//go:build !windows

package deskutil

import (
	"os"
	"path/filepath"
)

// Dir はデスクトップフォルダの絶対パスを返す（Windows以外）。
// ホームディレクトリが取得できない場合は空文字列を返す
// （カレントディレクトリ相対の "Desktop" を誤って有効なパスとして
// 扱わせないため）。
func Dir() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Desktop")
}
