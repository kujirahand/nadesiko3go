package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

func TestLoadWindowSettingsFromDir(t *testing.T) {
	dir := t.TempDir()
	if _, found, err := loadWindowSettingsFromDir(dir); err != nil || found {
		t.Fatalf("index.jsonがない場合の結果が違います: found=%v err=%v", found, err)
	}
	data := `{"サイズ":[640,480],"位置":"中央","タイトル":"見本"}`
	if err := os.WriteFile(filepath.Join(dir, windowConfigFile), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	settings, found, err := loadWindowSettingsFromDir(dir)
	if err != nil || !found {
		t.Fatalf("index.jsonを読めません: found=%v err=%v", found, err)
	}
	if settings.Width != 640 || settings.Height != 480 || !settings.Center || settings.Title != "見本" {
		t.Fatalf("設定が違います: %#v", settings)
	}
}

func TestMergeWindowSettings(t *testing.T) {
	base := defaultWindowSettings("既定", 960, 640)
	override := base
	override.HasSize = false
	override.HasState = false
	override.HasResizable = false
	override.HasTitle = true
	override.Title = "変更後"
	override.HasPosition = true
	override.Center = true
	got := mergeWindowSettings(base, override)
	if got.Title != "変更後" || got.Width != 960 || got.Height != 640 || !got.Center {
		t.Fatalf("設定の合成結果が違います: %#v", got)
	}
}

func TestMergeWindowSettingsTheme(t *testing.T) {
	base := defaultWindowSettings("既定", 960, 640)
	if !base.HasTheme || base.Theme != guilib.ThemeAuto {
		t.Fatalf("既定のテーマは自動のはずです: %#v", base)
	}
	if got := mergeWindowSettings(base, guilib.WindowSettings{}); got.Theme != guilib.ThemeAuto {
		t.Fatalf("テーマ省略時は既定値を保つはずです: %#v", got)
	}
	got := mergeWindowSettings(base, guilib.WindowSettings{HasTheme: true, Theme: guilib.ThemeLight})
	if got.Theme != guilib.ThemeLight {
		t.Fatalf("index.jsonのテーマが反映されていません: %#v", got)
	}
}
