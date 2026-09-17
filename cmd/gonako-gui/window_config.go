package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

const windowConfigFile = "index.json"

func loadWindowSettingsFromDir(dir string) (guilib.WindowSettings, bool, error) {
	if dir == "" {
		return guilib.WindowSettings{}, false, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, windowConfigFile))
	if errors.Is(err, os.ErrNotExist) {
		return guilib.WindowSettings{}, false, nil
	}
	if err != nil {
		return guilib.WindowSettings{}, false, fmt.Errorf("%sを読めません: %w", windowConfigFile, err)
	}
	settings, err := guilib.DecodeWindowSettings(data)
	return settings, true, err
}

func loadBundledWindowSettings(packed *bundle.Bundle) (guilib.WindowSettings, bool, error) {
	data, ok := packed.ReadResource(windowConfigFile)
	if !ok {
		return guilib.WindowSettings{}, false, nil
	}
	settings, err := guilib.DecodeWindowSettings(data)
	return settings, true, err
}

func defaultWindowSettings(title string, width, height int) guilib.WindowSettings {
	return guilib.WindowSettings{
		HasTitle: true, Title: title,
		HasSize: true, Width: width, Height: height,
		HasResizable: true, Resizable: true,
		HasState: true, State: "通常",
		HasTheme: true, Theme: guilib.ThemeAuto,
	}
}

// mergeWindowSettings はbaseへoverrideで明示された項目だけを重ねる。
func mergeWindowSettings(base, override guilib.WindowSettings) guilib.WindowSettings {
	if override.HasSize {
		base.HasSize, base.Width, base.Height = true, override.Width, override.Height
	}
	if override.HasPosition {
		base.HasPosition, base.Center, base.X, base.Y = true, override.Center, override.X, override.Y
	}
	if override.HasState {
		base.HasState, base.State = true, override.State
	}
	if override.HasTitle {
		base.HasTitle, base.Title = true, override.Title
	}
	if override.HasResizable {
		base.HasResizable, base.Resizable = true, override.Resizable
	}
	if override.HasTheme {
		base.HasTheme, base.Theme = true, override.Theme
	}
	return base
}
