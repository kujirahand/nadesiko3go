package nodelib

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

func zipCommands(m map[string]command) {
	m["圧縮"] = command{
		josi: [][]string{{"を", "から"}, {"へ", "に", "で"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			src := str(a, 0)
			dst := str(a, 1)
			if dst == "" {
				dst = src + ".zip"
			}
			if err := createZip(src, dst); err != nil {
				return value.Bool(false), err
			}
			return value.Bool(true), nil
		},
	}

	m["解凍"] = command{
		josi: [][]string{{"を", "から"}, {"へ", "に", "で"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			src := str(a, 0)
			dst := str(a, 1)
			if dst == "" {
				dst = "."
			}
			if err := extractZip(src, dst); err != nil {
				return value.Bool(false), err
			}
			return value.Bool(true), nil
		},
	}

	m["圧縮解凍ツールパス変更"] = command{
		josi:       [][]string{{"に", "へ"}},
		returnNone: true,
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			ctx.SetSysVar("圧縮解凍ツールパス", argAt(a, 0))
			return value.Undefined(), nil
		},
	}

	m["圧縮時"] = command{
		josi: [][]string{{"で", "の"}, {"を", "から"}, {"に", "へ"}},
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			src := str(a, 1)
			dst := str(a, 2)
			if dst == "" {
				dst = src + ".zip"
			}
			if err := createZip(src, dst); err != nil {
				return value.Bool(false), err
			}
			ctx.SetSysVar("対象", value.Bool(true))
			if fn, ok := toFunc(ctx, argAt(a, 0)); ok {
				return ctx.CallFunc(fn, []value.Value{value.Bool(true)})
			}
			return value.Bool(true), nil
		},
	}

	m["解凍時"] = command{
		josi: [][]string{{"で", "の"}, {"を", "から"}, {"に", "へ"}},
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			src := str(a, 1)
			dst := str(a, 2)
			if dst == "" {
				dst = "."
			}
			if err := extractZip(src, dst); err != nil {
				return value.Bool(false), err
			}
			ctx.SetSysVar("対象", value.Bool(true))
			if fn, ok := toFunc(ctx, argAt(a, 0)); ok {
				return ctx.CallFunc(fn, []value.Value{value.Bool(true)})
			}
			return value.Bool(true), nil
		},
	}
}

func createZip(src, dst string) error {
	zipFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Dir(filepath.Clean(src))
	}

	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(fi)
		if err != nil {
			return err
		}

		if baseDir != "" {
			rel, err := filepath.Rel(baseDir, path)
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(rel)
		} else {
			header.Name = filepath.Base(path)
		}

		if fi.IsDir() {
			if header.Name == "." {
				return nil
			}
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		if fi.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(w, file)
		return err
	})
}

func extractZip(src, destDir string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	// 展開先を実体の絶対パスに直す。「.」指定でも包含判定が効くようにし、
	// 展開先自身がシンボリックリンクでも内側判定がずれないようにする (#194)
	absDest := resolveExisting(destDir)

	for _, f := range r.File {
		fpath := filepath.Join(absDest, filepath.FromSlash(f.Name))
		// Zip Slip対策: 展開先の外へ出るエントリは飛ばす
		rel, err := filepath.Rel(absDest, fpath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue
		}
		// 途中の既存シンボリックリンクを通ると展開先の外へ書き出すため拒否する (#185)
		if hasSymlinkComponent(absDest, rel) {
			continue
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(fpath, f.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// resolveExisting はpathを絶対パスにし、実在する最長の先祖までシンボリック
// リンクを解決してから未作成の末尾を連結した実体パスを返す。
func resolveExisting(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	var tail []string
	cur := abs
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
}

// hasSymlinkComponent はbaseからrelを辿り、途中に既存のシンボリックリンクが
// あれば真を返す。存在しない要素以降は新規作成されるので調べない。
func hasSymlinkComponent(base, rel string) bool {
	cur := base
	for _, elem := range strings.Split(rel, string(os.PathSeparator)) {
		cur = filepath.Join(cur, elem)
		fi, err := os.Lstat(cur)
		if err != nil {
			return false
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}
