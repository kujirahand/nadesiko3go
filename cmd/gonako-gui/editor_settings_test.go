package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

func TestEditorSettingsPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	got, err := editorSettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "gonako-gui", "settings.json")
	if got != want {
		t.Fatalf("設定ファイルの場所が違います: got=%s want=%s", got, want)
	}

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	got, _ = editorSettingsPath()
	if got != filepath.Join(xdg, "gonako-gui", "settings.json") {
		t.Fatalf("XDG_CONFIG_HOMEが使われていません: %s", got)
	}
}

func TestEditorSettingsLoadAndSave(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	settings, err := loadEditorSettings()
	if err != nil || settings.Theme != guilib.ThemeAuto {
		t.Fatalf("設定ファイルがないときは自動のはずです: %#v %v", settings, err)
	}

	path := filepath.Join(dir, "gonako-gui", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"theme":"auto","other":123,"cursor":9007199254740993}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := saveEditorTheme(guilib.ThemeLight); err != nil {
		t.Fatalf("保存に失敗しました: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"other": 123`) || !strings.Contains(string(data), `"cursor": 9007199254740993`) {
		t.Fatalf("ほかの設定項目が消えたか変わっています: %s", data)
	}
	settings, err = loadEditorSettings()
	if err != nil || settings.Theme != guilib.ThemeLight {
		t.Fatalf("保存したテーマを読めません: %#v %v", settings, err)
	}

	if err := os.WriteFile(path, []byte(`{"theme":"青"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if settings, err := loadEditorSettings(); err != nil || settings.Theme != guilib.ThemeAuto {
		t.Fatalf("不正なテーマは自動になるはずです: %#v %v", settings, err)
	}
	if err := os.WriteFile(path, []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	if settings, err := loadEditorSettings(); err == nil || settings.Theme != guilib.ThemeAuto {
		t.Fatalf("壊れたJSONはエラーと既定値を返すはずです: %#v %v", settings, err)
	}
	if err := saveEditorTheme(guilib.ThemeDark); err == nil {
		t.Fatal("壊れたJSONの設定ファイルを上書きしてはいけません")
	}
	if data, _ := os.ReadFile(path); string(data) != `{` {
		t.Fatalf("壊れた設定ファイルが書き換えられています: %s", data)
	}
}
