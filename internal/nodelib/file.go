package nodelib

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// constants are the values nodelib defines, such as the path separator.
func constants() map[string]any {
	exe, _ := os.Executable()
	bokan := filepath.Dir(exe)
	return map[string]any{
		"改行コード":          "\n",
		"パス区切":           string(filepath.Separator),
		"ナデシコランタイム":       "gonako",
		"ナデシコランタイムパス":     exe,
		"母艦パス":           bokan,
		"ファイルコピーデフォルト動作": "overwrite",
		"AJAXオプション":       "",
		"圧縮解凍ツールパス":      "zip",
	}
}

// commands lists every nodelib command.
func commands() map[string]command {
	m := map[string]command{}

	// --- ファイルの読み書き ---

	m["開"] = command{josi: [][]string{{"を", "から"}}, fn: readFile}
	m["読"] = m["開"]
	m["バイナリ読"] = command{josi: [][]string{{"を", "から"}}, fn: readBinaryFile}
	m["保存"] = command{josi: [][]string{{"を"}, {"に", "へ"}}, returnNone: true, fn: writeFile}
	m["追記"] = command{josi: [][]string{{"を"}, {"に", "へ"}}, returnNone: true, fn: appendFile}

	m["存在"] = command{josi: [][]string{{"が", "の"}}, fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
		// 同梱したリソースも「存在する」ものとして数える
		if _, ok := ctx.ReadResource(str(a, 0)); ok {
			return value.Bool(true), nil
		}
		info, err := os.Stat(str(a, 0))
		return value.Bool(err == nil && !info.IsDir()), nil
	}}
	m["フォルダ存在"] = command{josi: [][]string{{"が", "の"}}, fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
		info, err := os.Stat(str(a, 0))
		return value.Bool(err == nil && info.IsDir()), nil
	}}
	m["フォルダ作成"] = command{josi: [][]string{{"に", "へ", "の"}}, returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			return value.Undefined(), os.MkdirAll(str(a, 0), 0o755)
		}}
	m["ファイル削除"] = command{josi: [][]string{{"の", "を"}}, returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			return value.Undefined(), os.Remove(str(a, 0))
		}}
	m["ファイル削除時"] = command{josi: [][]string{{"で", "を", "の"}, {"の", "を"}},
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			if err := os.RemoveAll(str(a, 1)); err != nil {
				return value.Undefined(), fileError("削除でき", str(a, 1), err)
			}
			if fn, ok := toFunc(ctx, argAt(a, 0)); ok {
				return ctx.CallFunc(fn, nil)
			}
			return value.Undefined(), nil
		}}
	m["ファイル移動"] = command{josi: [][]string{{"を", "から"}, {"に", "へ"}}, returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			return value.Undefined(), os.Rename(str(a, 0), str(a, 1))
		}}
	m["ファイル上書移動"] = m["ファイル移動"]
	m["ファイル移動時"] = command{josi: [][]string{{"で", "を", "の"}, {"から", "を"}, {"に", "へ"}},
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			if err := os.Rename(str(a, 1), str(a, 2)); err != nil {
				return value.Undefined(), fileError("移動でき", str(a, 1), err)
			}
			if fn, ok := toFunc(ctx, argAt(a, 0)); ok {
				return ctx.CallFunc(fn, nil)
			}
			return value.Undefined(), nil
		}}
	m["ファイルコピー"] = command{josi: [][]string{{"を", "から"}, {"に", "へ"}}, returnNone: true, fn: copyFile}
	m["ファイル上書コピー"] = m["ファイルコピー"]
	m["ファイルコピー時"] = command{josi: [][]string{{"で", "を", "の"}, {"から", "を"}, {"に", "へ"}},
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			if _, err := copyFile(ctx, a[1:]); err != nil {
				return value.Undefined(), err
			}
			if fn, ok := toFunc(ctx, argAt(a, 0)); ok {
				return ctx.CallFunc(fn, nil)
			}
			return value.Undefined(), nil
		}}
	m["ファイル処理時"] = command{josi: [][]string{{"を", "で", "の"}}, returnNone: true,
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			ctx.SetCommandState("__fileProcessCallback", argAt(a, 0))
			ctx.SetCommandState("__fileProcessStop", value.Bool(false))
			return value.Undefined(), nil
		}}
	m["ファイル処理強制停止"] = command{returnNone: true,
		fn: func(ctx stdlib.Context, _ []value.Value) (value.Value, error) {
			ctx.SetCommandState("__fileProcessStop", value.Bool(true))
			return value.Undefined(), nil
		}}

	m["ファイルサイズ取得"] = command{josi: [][]string{{"の", "を", "から"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			info, err := os.Stat(str(a, 0))
			if err != nil {
				return value.Number(-1), nil
			}
			return value.Number(float64(info.Size())), nil
		}}

	m["ファイル情報取得"] = command{josi: [][]string{{"の", "を", "から"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			info, err := os.Stat(str(a, 0))
			if err != nil {
				return value.Undefined(), fileError("情報取得でき", str(a, 0), err)
			}
			d := value.NewDict()
			d.Set("サイズ", value.Number(float64(info.Size())))
			d.Set("size", value.Number(float64(info.Size())))
			d.Set("ディレクトリ", value.Bool(info.IsDir()))
			d.Set("isDirectory", value.Bool(info.IsDir()))
			modStr := info.ModTime().Format("2006-01-02 15:04:05")
			d.Set("更新日時", value.String(modStr))
			d.Set("mtime", value.String(modStr))
			return value.DictValue(d), nil
		}}

	m["ファイル列挙"] = command{josi: [][]string{{"の", "を", "で"}}, fn: listFiles}
	m["全ファイル列挙"] = command{josi: [][]string{{"の", "を", "で"}}, fn: listAllFiles}

	// --- パス操作 ---

	m["ファイル名抽出"] = pathCommand(filepath.Base)
	m["パス抽出"] = pathCommand(filepath.Dir)
	m["拡張子抽出"] = pathCommand(filepath.Ext)
	m["絶対パス変換"] = command{josi: [][]string{{"を", "の"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			abs, err := filepath.Abs(str(a, 0))
			if err != nil {
				return value.Undefined(), err
			}
			return value.String(abs), nil
		}}
	m["相対パス展開"] = command{josi: [][]string{{"を"}, {"で"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			base := str(a, 0)
			rel := str(a, 1)
			abs, err := filepath.Abs(filepath.Join(base, rel))
			if err != nil {
				return value.Undefined(), err
			}
			return value.String(abs), nil
		}}
	m["パス結合"] = command{josi: [][]string{{"と", "を"}}, variadic: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			parts := make([]string, 0, len(a))
			for i := range a {
				parts = append(parts, str(a, i))
			}
			return value.String(filepath.Join(parts...)), nil
		}}

	// --- 作業フォルダ ---

	m["カレントディレクトリ取得"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		dir, err := os.Getwd()
		if err != nil {
			return value.Undefined(), err
		}
		return value.String(dir), nil
	}}
	m["作業フォルダ取得"] = m["カレントディレクトリ取得"]

	m["カレントディレクトリ変更"] = command{josi: [][]string{{"に", "へ"}}, returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			return value.Undefined(), os.Chdir(str(a, 0))
		}}
	m["作業フォルダ変更"] = m["カレントディレクトリ変更"]

	m["テンポラリフォルダ"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		return value.String(os.TempDir()), nil
	}}
	m["一時フォルダ作成"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		dir, err := os.MkdirTemp("", "nako3_*")
		if err != nil {
			return value.Undefined(), err
		}
		return value.String(dir), nil
	}}
	m["ホームディレクトリ取得"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		dir, err := os.UserHomeDir()
		if err != nil {
			return value.String(""), nil
		}
		return value.String(dir), nil
	}}
	m["デスクトップ"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		dir, _ := os.UserHomeDir()
		return value.String(filepath.Join(dir, "Desktop")), nil
	}}
	m["マイドキュメント"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		dir, _ := os.UserHomeDir()
		return value.String(filepath.Join(dir, "Documents")), nil
	}}
	m["母艦パス取得"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		exe, err := os.Executable()
		if err != nil {
			return value.String(""), nil
		}
		return value.String(filepath.Dir(exe)), nil
	}}

	osCommands(m)
	cryptoCommands(m)
	netCommands(m)
	zipCommands(m)
	encodingCommands(m)
	return m
}

// pathCommand builds the commands that just transform a path string.
func pathCommand(f func(string) string) command {
	return command{josi: [][]string{{"の", "を", "から"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			return value.String(f(str(a, 0))), nil
		}}
}

// readFile reads a file, looking in the bundled resources first.
func readFile(ctx stdlib.Context, a []value.Value) (value.Value, error) {
	name := str(a, 0)
	if data, ok := ctx.ReadResource(name); ok {
		return value.String(string(data)), nil
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return value.Undefined(), fileError("読み込め", name, err)
	}
	return value.String(string(data)), nil
}

func readBinaryFile(ctx stdlib.Context, a []value.Value) (value.Value, error) {
	name := str(a, 0)
	var data []byte
	var ok bool
	if data, ok = ctx.ReadResource(name); !ok {
		var err error
		data, err = os.ReadFile(name)
		if err != nil {
			return value.Undefined(), fileError("読み込め", name, err)
		}
	}
	items := make([]value.Value, len(data))
	for i, b := range data {
		items[i] = value.Number(float64(b))
	}
	return value.ArrayValue(value.NewArray(items...)), nil
}

func writeFile(_ stdlib.Context, a []value.Value) (value.Value, error) {
	if err := os.WriteFile(str(a, 1), []byte(str(a, 0)), 0o644); err != nil {
		return value.Undefined(), fileError("保存でき", str(a, 1), err)
	}
	return value.Undefined(), nil
}

func appendFile(_ stdlib.Context, a []value.Value) (value.Value, error) {
	f, err := os.OpenFile(str(a, 1), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return value.Undefined(), fileError("追記でき", str(a, 1), err)
	}
	defer f.Close()
	if _, err := f.WriteString(str(a, 0)); err != nil {
		return value.Undefined(), fileError("追記でき", str(a, 1), err)
	}
	return value.Undefined(), nil
}

func copyFile(_ stdlib.Context, a []value.Value) (value.Value, error) {
	data, err := os.ReadFile(str(a, 0))
	if err != nil {
		return value.Undefined(), fileError("読み込め", str(a, 0), err)
	}
	if err := os.WriteFile(str(a, 1), data, 0o644); err != nil {
		return value.Undefined(), fileError("書き込め", str(a, 1), err)
	}
	return value.Undefined(), nil
}

func listFiles(_ stdlib.Context, a []value.Value) (value.Value, error) {
	pattern := str(a, 0)
	dir, mask := pattern, ""
	if strings.ContainsAny(filepath.Base(pattern), "*?") {
		dir, mask = filepath.Dir(pattern), filepath.Base(pattern)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return value.Undefined(), fileError("読み込め", dir, err)
	}
	var names []string
	for _, e := range entries {
		if mask != "" {
			if ok, _ := filepath.Match(mask, e.Name()); !ok {
				continue
			}
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	items := make([]value.Value, len(names))
	for i, n := range names {
		items[i] = value.String(n)
	}
	return value.ArrayValue(value.NewArray(items...)), nil
}

func listAllFiles(_ stdlib.Context, a []value.Value) (value.Value, error) {
	pattern := str(a, 0)
	basepath := pattern
	var matchRE *regexp.Regexp

	if strings.Contains(pattern, "*") {
		basepath = filepath.Dir(pattern)
		mask := filepath.Base(pattern)
		// Convert wildcards like *.jpg;*.png into regex
		maskPatterns := strings.Split(mask, ";")
		var reParts []string
		for _, mp := range maskPatterns {
			p := regexp.QuoteMeta(strings.TrimSpace(mp))
			p = strings.ReplaceAll(p, `\*`, `.*`)
			p = strings.ReplaceAll(p, `\?`, `.`)
			reParts = append(reParts, p)
		}
		reStr := "(?i)^(" + strings.Join(reParts, "|") + ")$"
		var err error
		matchRE, err = regexp.Compile(reStr)
		if err != nil {
			matchRE = nil
		}
	}

	var results []string
	err := filepath.Walk(basepath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		if matchRE != nil {
			if !matchRE.MatchString(fi.Name()) {
				return nil
			}
		}
		results = append(results, path)
		return nil
	})
	if err != nil {
		return value.Undefined(), fileError("列挙でき", basepath, err)
	}
	sort.Strings(results)

	items := make([]value.Value, len(results))
	for i, r := range results {
		items[i] = value.String(r)
	}
	return value.ArrayValue(value.NewArray(items...)), nil
}

// fileError wraps an OS error in a message that names the file.
func fileError(what, path string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("ファイル『" + path + "』が見つかりません。")
	}
	if errors.Is(err, os.ErrPermission) {
		return errors.New("ファイル『" + path + "』を" + what + "ません。権限がありません。")
	}
	return errors.New("ファイル『" + path + "』を" + what + "ません。" + err.Error())
}

// str reads an argument as a string.
func str(args []value.Value, i int) string {
	if i < 0 || i >= len(args) {
		return ""
	}
	return value.ToString(args[i])
}

func toFunc(ctx stdlib.Context, v value.Value) (*value.Func, bool) {
	if fn, ok := v.Func(); ok {
		return fn, true
	}
	if v.Kind() == value.KindString {
		fn := ctx.FindFunc(value.ToString(v))
		return fn, fn != nil
	}
	return nil, false
}
