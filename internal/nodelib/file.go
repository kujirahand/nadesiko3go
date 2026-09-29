package nodelib

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/deskutil"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// constants are the values nodelib defines, such as the path separator.
func constants() map[string]any {
	exe, _ := os.Executable()
	return map[string]any{
		"改行コード":          "\n",
		"パス区切":           string(filepath.Separator),
		"ナデシコランタイム":      "gonako",
		"ナデシコランタイムパス":    exe,
		"母艦パス":           "",
		"ファイルコピーデフォルト動作": "上書禁止",
		"AJAXオプション":      "",
		"圧縮解凍ツールパス":      "7z",
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
	m["追記"] = command{josi: [][]string{{"を"}, {"に", "へ"}}, returnNone: true, fn: appendFile} // @文字列Aをファイルパスの末尾に追記する // @ついき

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
	m["フォルダ作成"] = command{josi: [][]string{{"の", "を", "に", "へ"}}, returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			return value.Undefined(), os.MkdirAll(str(a, 0), 0o755)
		}}
	m["ファイル削除"] = command{josi: [][]string{{"の", "を"}}, returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			return value.Undefined(), os.Remove(str(a, 0))
		}}
	m["ショートカットファイル作成"] = command{josi: [][]string{{"を", "から"}, {"に", "へ"}}, returnNone: true, // @ファイルへのショートカットファイル(.lnk)を作成する // @しょーとかっとふぁいるさくせい
		fn: createShortcut}
	m["シンボリックリンク作成"] = command{josi: [][]string{{"を", "から"}, {"に", "へ"}}, returnNone: true, // @ファイルへのシンボリックリンクを作成する // @しんぼりっくりんくさくせい
		fn: createSymbolicLink}
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
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			return value.Undefined(), moveEntry(ctx, str(a, 0), str(a, 1), isOverwrite(ctx))
		}}
	m["ファイル上書移動"] = command{josi: [][]string{{"を", "から"}, {"に", "へ"}}, returnNone: true,
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			return value.Undefined(), moveEntry(ctx, str(a, 0), str(a, 1), true)
		}}
	m["ファイル移動時"] = command{josi: [][]string{{"で", "を", "の"}, {"から", "を"}, {"に", "へ"}},
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			if err := moveEntry(ctx, str(a, 1), str(a, 2), isOverwrite(ctx)); err != nil {
				return value.Undefined(), err
			}
			if fn, ok := toFunc(ctx, argAt(a, 0)); ok {
				return ctx.CallFunc(fn, nil)
			}
			return value.Undefined(), nil
		}}
	m["ファイルコピー"] = command{josi: [][]string{{"を", "から"}, {"に", "へ"}}, returnNone: true,
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			return value.Undefined(), copyMergeWithProgress(str(a, 0), str(a, 1), isOverwrite(ctx), ctx)
		}}
	m["ファイル上書コピー"] = command{josi: [][]string{{"を", "から"}, {"に", "へ"}}, returnNone: true,
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			return value.Undefined(), copyMergeWithProgress(str(a, 0), str(a, 1), true, ctx)
		}}
	m["ファイルコピー時"] = command{josi: [][]string{{"で", "を", "の"}, {"から", "を"}, {"に", "へ"}},
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			if err := copyMergeWithProgress(str(a, 1), str(a, 2), isOverwrite(ctx), ctx); err != nil {
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
	m["パス結合"] = command{josi: [][]string{{"と", "を"}}, variadic: true, // @複数のパス断片をOS標準の区切り文字で結合して返す // @ぱすけつごう
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
			dir := str(a, 0)
			return value.Undefined(), os.Chdir(filepath.Clean(dir))
		}}
	m["作業フォルダ変更"] = m["カレントディレクトリ変更"]

	m["テンポラリフォルダ"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		return value.String(os.TempDir()), nil
	}}
	m["一時フォルダ作成"] = command{josi: [][]string{{"に", "へ"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			// 省略・空白・文字列以外が指定されたときはOSのテンポラリフォルダを使う
			dir := ""
			if a0 := argAt(a, 0); a0.Kind() == value.KindString {
				dir = strings.TrimSpace(value.ToString(a0))
			}
			if dir == "" {
				dir = os.TempDir()
			}
			tmp, err := os.MkdirTemp(dir, "nako-")
			if err != nil {
				return value.Undefined(), fileError("作成でき", dir, err)
			}
			return value.String(tmp), nil
		}}
	m["ホームディレクトリ取得"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		dir, err := os.UserHomeDir()
		if err != nil {
			return value.String(""), nil
		}
		return value.String(dir), nil
	}}
	m["デスクトップ"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		return value.String(deskutil.Dir()), nil
	}}
	m["マイドキュメント"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		dir, _ := os.UserHomeDir()
		return value.String(filepath.Join(dir, "Documents")), nil
	}}
	m["母艦パス取得"] = command{fn: func(ctx stdlib.Context, _ []value.Value) (value.Value, error) {
		if v := ctx.SysVar("母艦パス"); v.Kind() == value.KindString && value.ToString(v) != "" {
			return v, nil
		}
		if cwd, err := os.Getwd(); err == nil {
			return value.String(cwd), nil
		}
		return value.String(""), nil
	}}

	osCommands(m)
	clipboardCommands(m)
	cryptoCommands(m)
	netCommands(m)
	zipCommands(m)
	encodingCommands(m)
	keyCommands(m)
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
	data, err := fileData(argAt(a, 0))
	if err != nil {
		return value.Undefined(), err
	}
	if err := os.WriteFile(str(a, 1), data, 0o644); err != nil {
		return value.Undefined(), fileError("保存でき", str(a, 1), err)
	}
	return value.Undefined(), nil
}

// fileData は文字列をUTF-8、数値配列をバイト列として返す。
func fileData(v value.Value) ([]byte, error) {
	if arr, ok := v.Array(); ok {
		data := make([]byte, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			n, ok := arr.Get(i).Number()
			if !ok || math.IsNaN(n) || math.IsInf(n, 0) || math.Trunc(n) != n || n < 0 || n > 255 {
				return nil, fmt.Errorf("保存する数値配列の%d番目は0〜255の整数で指定してください。", i+1)
			}
			data[i] = byte(n)
		}
		return data, nil
	}
	return []byte(value.ToString(v)), nil
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

func isOverwrite(ctx stdlib.Context) bool {
	mode := value.ToString(ctx.SysVar("ファイルコピーデフォルト動作"))
	return mode == "上書き" || mode == "上書" || mode == "overwrite"
}

type filePair struct {
	src string
	rel string
}

func listFilesRecursive(baseDir, curPath string) []filePair {
	info, err := os.Stat(curPath)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		rel, err := filepath.Rel(baseDir, curPath)
		if err != nil {
			rel = filepath.Base(curPath)
		}
		return []filePair{{src: curPath, rel: rel}}
	}
	var res []filePair
	entries, err := os.ReadDir(curPath)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		sub := listFilesRecursive(baseDir, filepath.Join(curPath, e.Name()))
		res = append(res, sub...)
	}
	return res
}

// moveEntry は『ファイル移動』系命令の共通処理。コピーしてから移動元を
// 削除する順序なので、移動先が移動元と同一実体またはその内側にあると、
// コピーしたばかりの内容まで削除に巻き込まれる。それを防ぐため、
// 実行前に checkMoveTarget で拒否する (#186, #195)。
func moveEntry(ctx stdlib.Context, src, dest string, overwrite bool) error {
	if err := checkMoveTarget(src, dest); err != nil {
		return err
	}
	if err := copyMergeWithProgress(src, dest, overwrite, ctx); err != nil {
		return err
	}
	if !value.ToBool(ctx.CommandState("__fileProcessStop")) {
		_ = os.RemoveAll(src)
	}
	return nil
}

// checkMoveTarget は、移動元 src と移動先 dest の関係を調べ、移動先が
// 移動元と同一実体またはその子孫にある場合はエラーを返す。
// 移動先から祖先を深い方へ順に辿り、移動元と同一の実体にぶつかったら
// 拒否する。os.Stat がシンボリックリンクを実体へ解決し、大小文字を
// 区別しないファイルシステムでは表記違いのパスも同一実体として返る
// ため、文字列表記の比較では捕捉できない経路も検出できる (#186, #195)。
func checkMoveTarget(src, dest string) error {
	if strings.TrimSpace(dest) == "" {
		return errors.New("ファイル移動先が指定されていません。")
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return nil // 読み取り可否の報告は copyMergeWithProgress が行う
	}
	// 相対パスのままでは祖先走査が「.」(作業フォルダ)で止まり、その上位の
	// 実祖先や「..」の先を検査できないため、必ず絶対パスにしてから辿る
	abs, err := filepath.Abs(dest)
	if err != nil {
		abs = filepath.Clean(dest)
	}
	paths := []string{abs}
	// 移動先自身がシンボリックリンクのとき、Stat はリンク先の実体を返す。
	// リンク先が移動元の内側にある場合を捕捉するため、解決後の実パスも調べる。
	if resolved, err := filepath.EvalSymlinks(abs); err == nil && resolved != abs {
		paths = append(paths, resolved)
	}
	for _, p := range paths {
		for cur, first := p, true; ; first = false {
			if info, err := os.Stat(cur); err == nil && os.SameFile(srcInfo, info) {
				if first {
					return fmt.Errorf("ファイル移動元と移動先が同じです: %s → %s", src, dest)
				}
				return fmt.Errorf("ファイル移動先は移動元の内側です: %s → %s", src, dest)
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
	}
	return nil
}

func copyMergeWithProgress(src, dest string, overwrite bool, ctx stdlib.Context) error {
	cbVal := ctx.CommandState("__fileProcessCallback")
	cbFn, hasCb := toFunc(ctx, cbVal)
	ctx.SetCommandState("__fileProcessStop", value.Bool(false))

	srcInfo, err := os.Stat(src)
	if err != nil {
		return fileError("読み込め", src, err)
	}

	if !overwrite {
		if _, err := os.Stat(dest); err == nil {
			return errors.New("ファイルコピー先に同名のファイルまたはフォルダが存在します: " + dest)
		}
	}

	if !srcInfo.IsDir() {
		data, err := os.ReadFile(src)
		if err != nil {
			return fileError("読み込め", src, err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fileError("作成でき", filepath.Dir(dest), err)
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fileError("書き込め", dest, err)
		}
		if hasCb {
			d := value.NewDict()
			d.Set("件数", value.Number(1))
			d.Set("現在", value.Number(1))
			progress := value.DictValue(d)
			ctx.SetSysVar("対象", progress)
			_, _ = ctx.CallFunc(cbFn, []value.Value{progress})
		}
		return nil
	}

	// ディレクトリのマージコピー
	files := listFilesRecursive(src, src)
	total := len(files)

	for i, f := range files {
		stopVal := ctx.CommandState("__fileProcessStop")
		if value.ToBool(stopVal) {
			break
		}
		destFile := filepath.Join(dest, f.rel)
		if err := os.MkdirAll(filepath.Dir(destFile), 0o755); err != nil {
			return fileError("作成でき", filepath.Dir(destFile), err)
		}
		data, err := os.ReadFile(f.src)
		if err != nil {
			return fileError("読み込め", f.src, err)
		}
		if err := os.WriteFile(destFile, data, 0o644); err != nil {
			return fileError("書き込め", destFile, err)
		}

		if hasCb {
			d := value.NewDict()
			d.Set("件数", value.Number(float64(total)))
			d.Set("現在", value.Number(float64(i+1)))
			progress := value.DictValue(d)
			ctx.SetSysVar("対象", progress)
			_, _ = ctx.CallFunc(cbFn, []value.Value{progress})
		}
	}
	return nil
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

// createShortcut は、Windowsでは.lnkショートカット、macOSではFinderエイリアス、
// それ以外ではシンボリックリンクを作成する。
func createShortcut(_ stdlib.Context, a []value.Value) (value.Value, error) {
	from := str(a, 0)
	to := str(a, 1)
	switch runtime.GOOS {
	case "windows":
		return value.Undefined(), createWindowsShortcut(from, to)
	case "darwin":
		return value.Undefined(), createMacOSAlias(from, to)
	default:
		return value.Undefined(), createSymlink(from, to)
	}
}

// createSymbolicLink はシンボリックリンクを作成する。Windowsでシンボリックリンクの
// 権限が無い場合は、命令が使えるように.lnkショートカットへフォールバックする。
func createSymbolicLink(_ stdlib.Context, a []value.Value) (value.Value, error) {
	from := str(a, 0)
	to := str(a, 1)
	if runtime.GOOS == "windows" {
		return value.Undefined(), createWindowsSymlinkOrShortcut(from, to)
	}
	return value.Undefined(), createSymlink(from, to)
}

// createSymlink は、fromを指すシンボリックリンクをtoに作成する。
// 相対パスで保存先フォルダが異なっても壊れないよう、元ファイルは絶対パスに変換する。
func createSymlink(from, to string) error {
	absFrom, err := filepath.Abs(from)
	if err != nil {
		return fmt.Errorf("シンボリックリンクの元ファイル『%s』の絶対パスを取得できません: %w", from, err)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return fmt.Errorf("シンボリックリンクの作成先フォルダ『%s』を作成できません: %w", filepath.Dir(to), err)
	}
	if err := os.Symlink(absFrom, to); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("シンボリックリンクの作成先『%s』に既にファイルまたはフォルダが存在します。", to)
		}
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("シンボリックリンク『%s』を作成できません。権限がありません。", to)
		}
		return fmt.Errorf("シンボリックリンク『%s』を作成できません: %w", to, err)
	}
	return nil
}

// createMacOSAlias は、AppleScript経由でFinderエイリアスを作成する。
func createMacOSAlias(from, to string) error {
	absFrom, err := filepath.Abs(from)
	if err != nil {
		return fmt.Errorf("エイリアスの元ファイル『%s』の絶対パスを取得できません: %w", from, err)
	}
	if _, err := os.Stat(absFrom); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("エイリアスの元ファイル『%s』が見つかりません。", absFrom)
		}
		return fmt.Errorf("エイリアスの元ファイル『%s』を確認できません: %w", absFrom, err)
	}
	absTo, err := filepath.Abs(to)
	if err != nil {
		return fmt.Errorf("エイリアスの作成先『%s』の絶対パスを取得できません: %w", to, err)
	}
	destDir := filepath.Dir(absTo)
	destName := filepath.Base(absTo)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("エイリアスの作成先フォルダ『%s』を作成できません: %w", destDir, err)
	}
	// 既存のファイルを誤って消さないよう、存在する場合はエラーにする。
	if _, err := os.Lstat(absTo); err == nil {
		return fmt.Errorf("エイリアスの作成先『%s』に既にファイルまたはフォルダが存在します。", absTo)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("エイリアスの作成先『%s』を確認できません: %w", absTo, err)
	}
	script := fmt.Sprintf(`tell application "Finder"
	set src to POSIX file %s
	set dstFolder to POSIX file %s
	make new alias file at dstFolder to src with properties {name:%s}
end tell`, appleScriptString(absFrom), appleScriptString(destDir), appleScriptString(destName))
	cmd := exec.Command("osascript", "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("エイリアス『%s』を作成できませんでした: %w\n%s", absTo, err, string(out))
	}
	return nil
}

// appleScriptString は、sをAppleScriptの文字列リテラルとして引用符で囲んで返す。
func appleScriptString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		if r == '\\' || r == '"' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// createWindowsShortcut は、PowerShell経由でWindows Script Hostを使い.lnkを作成する。
func createWindowsShortcut(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return fmt.Errorf("ショートカットファイルの作成先フォルダ『%s』を作成できません: %w", filepath.Dir(to), err)
	}
	// 既存のファイルを誤って上書きしないよう、存在する場合はエラーにする。
	if _, err := os.Lstat(to); err == nil {
		return fmt.Errorf("ショートカットファイルの作成先『%s』に既にファイルまたはフォルダが存在します。", to)
	}
	// ターゲットは絶対パスにして、ショートカットが動くようにする。
	absFrom, err := filepath.Abs(from)
	if err != nil {
		return fmt.Errorf("ショートカットの元ファイル『%s』の絶対パスを取得できません: %w", from, err)
	}
	script := fmt.Sprintf(
		"$ErrorActionPreference='Stop'; $s=(New-Object -ComObject WScript.Shell).CreateShortcut('%s'); $s.TargetPath='%s'; $s.Save()",
		escapePSSingleQuotes(to),
		escapePSSingleQuotes(absFrom),
	)
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ショートカットファイル『%s』を作成できませんでした: %w", to, err)
	}
	if _, err := os.Stat(to); err != nil {
		return fmt.Errorf("ショートカットファイル『%s』が作成後に見つかりません。", to)
	}
	return nil
}

// escapePSSingleQuotes は、PowerShellの単一引用符文字列用に単一引用符をエスケープする。
func escapePSSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// createWindowsSymlinkOrShortcut は、Windowsで本物のシンボリックリンクを作成する。
// 権限が無い場合は、一般ユーザーでも使えるよう「to.lnk」のショートカットにフォールバックする。
func createWindowsSymlinkOrShortcut(from, to string) error {
	absFrom, err := filepath.Abs(from)
	if err != nil {
		return fmt.Errorf("シンボリックリンクの元ファイル『%s』の絶対パスを取得できません: %w", from, err)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return fmt.Errorf("シンボリックリンクの作成先フォルダ『%s』を作成できません: %w", filepath.Dir(to), err)
	}
	if err := os.Symlink(absFrom, to); err != nil {
		if isWindowsSymlinkPrivilegeError(err) {
			if !strings.EqualFold(filepath.Ext(to), ".lnk") {
				to += ".lnk"
			}
			return createWindowsShortcut(from, to)
		}
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("シンボリックリンクの作成先『%s』に既にファイルまたはフォルダが存在します。", to)
		}
		return fmt.Errorf("シンボリックリンク『%s』を作成できません: %w", to, err)
	}
	return nil
}

// isWindowsSymlinkPrivilegeError は、errがWindowsの「クライアントは必要な特権を保有
// していません」エラーかどうかを返す。
func isWindowsSymlinkPrivilegeError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "privilege") || strings.Contains(msg, "特権")
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
