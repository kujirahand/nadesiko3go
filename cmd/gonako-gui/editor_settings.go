package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

const (
	editorSettingsDirName  = "gonako-gui"
	editorSettingsFileName = "settings.json"
)

// EditorSettings はエディタの利用者設定である。
type EditorSettings struct {
	Theme string `json:"theme"`
}

func defaultEditorSettings() EditorSettings {
	return EditorSettings{Theme: guilib.ThemeAuto}
}

// editorSettingsPath は、分かりやすさを優先してどのOSでも
// ~/.config/gonako-gui/settings.json を使う。XDG_CONFIG_HOMEがあればそちらを使う。
func editorSettingsPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("ホームフォルダが分かりません: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, editorSettingsDirName, editorSettingsFileName), nil
}

// loadEditorSettings は設定ファイルを読む。ファイルがなければ既定値を返す。
// 不正な値は既定値に置き換え、エディタの起動を妨げない。
func loadEditorSettings() (EditorSettings, error) {
	settings := defaultEditorSettings()
	path, err := editorSettingsPath()
	if err != nil {
		return settings, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, fmt.Errorf("%sを読めません: %w", path, err)
	}
	var loaded EditorSettings
	if err := json.Unmarshal(data, &loaded); err != nil {
		return settings, fmt.Errorf("%sをJSONとして読めません: %w", path, err)
	}
	if theme, err := guilib.NormalizeWindowTheme(loaded.Theme); err == nil {
		settings.Theme = theme
	}
	return settings, nil
}

// saveEditorTheme はテーマだけを書き換える。ほかの項目は残す。
func saveEditorTheme(theme string) error {
	path, err := editorSettingsPath()
	if err != nil {
		return err
	}
	raw := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &raw)
		if raw == nil {
			raw = map[string]any{}
		}
	}
	raw["theme"] = theme
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("設定フォルダを作れません: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("%sへ書き込めません: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%sへ書き込めません: %w", path, err)
	}
	return nil
}
