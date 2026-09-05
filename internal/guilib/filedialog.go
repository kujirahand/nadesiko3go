package guilib

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

type fileDialogs struct {
	open   func(defaultDir, extension string) (string, error)
	save   func(defaultDir, defaultName, extension string) (string, error)
	folder func(defaultDir string) (string, error)
}

func nativeFileDialogs() fileDialogs {
	return fileDialogs{
		open:   openFileDialogPlatform,
		save:   saveFileDialogPlatform,
		folder: folderDialogPlatform,
	}
}

// OpenFileDialog shows the operating system's native open dialog.
func OpenFileDialog(defaultDir, extension string) (string, error) {
	return openFileDialogPlatform(normalizeDefaultDir(defaultDir), normalizeExtension(extension))
}

// SaveFileDialog shows the operating system's native save dialog.
func SaveFileDialog(defaultDir, defaultName, extension string) (string, error) {
	extension = normalizeExtension(extension)
	if defaultName == "" {
		defaultName = defaultFileName(extension)
	}
	path, err := saveFileDialogPlatform(normalizeDefaultDir(defaultDir), defaultName, extension)
	if err != nil {
		return "", err
	}
	return addDefaultExtension(path, extension), nil
}

// FolderDialog shows the operating system's native folder selection dialog.
func FolderDialog(defaultDir string) (string, error) {
	return folderDialogPlatform(normalizeDefaultDir(defaultDir))
}

func normalizeDefaultDir(path string) string {
	if path == "" {
		if home, err := os.UserHomeDir(); err == nil {
			path = home
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func normalizeExtension(extension string) string {
	extension = strings.TrimSpace(extension)
	if extension == "" || extension == "." || extension == "*" || extension == "*.*" {
		return ""
	}
	if strings.HasPrefix(extension, "*.") {
		extension = extension[1:]
	} else if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}
	if strings.ContainsAny(extension, `/\\*?;`) || len(extension) == 1 {
		return ""
	}
	return extension
}

func defaultFileName(extension string) string {
	if extension == "" {
		extension = ".nako3"
	}
	return "新規ファイル" + extension
}

func addDefaultExtension(path, extension string) string {
	if path == "" || filepath.Ext(path) != "" {
		return path
	}
	if extension == "" {
		extension = ".nako3"
	}
	return path + extension
}

func contextBaseDir(ctx stdlib.Context) string {
	if v := ctx.SysVar("母艦パス"); v.Kind() == value.KindString {
		if path := value.ToString(v); path != "" {
			return path
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}
