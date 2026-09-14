//go:build windows

package deskutil

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// Dir はデスクトップフォルダの絶対パスを返す（Windows）。
// OneDriveのバックアップ機能などでデスクトップがユーザーフォルダ外へ
// 移動されている場合があるため、レジストリの User Shell Folders から
// 実際のパスを取得する。取得できない場合のみ既定パスへフォールバックする。
// ホームディレクトリも取得できない場合は空文字列を返す
// （カレントディレクトリ相対の "Desktop" を誤って有効なパスとして
// 扱わせないため）。
func Dir() string {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		if raw, _, err := k.GetStringValue("Desktop"); err == nil {
			if expanded, err := registry.ExpandString(raw); err == nil && expanded != "" {
				return expanded
			}
		}
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Desktop")
}
