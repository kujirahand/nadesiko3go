package nodelib

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/safepath"
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
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	// 出力が入力と同じ実体だと、os.Createの切り詰めで元データが消える (#189)
	if dstInfo, err := os.Stat(dst); err == nil && os.SameFile(srcInfo, dstInfo) {
		return fmt.Errorf("圧縮先が圧縮元と同一です: %s", dst)
	}
	// 出力が入力フォルダの内側だと、作成中のZIP自身を梱包してしまうため拒否する
	if srcInfo.IsDir() && pathInside(src, dst) {
		return fmt.Errorf("圧縮先が圧縮元フォルダと同一またはその内側です: %s", dst)
	}

	zipFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	var baseDir string
	if srcInfo.IsDir() {
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

	result, err := safepath.Extract(&r.Reader, destDir, safepath.ExtractOptions{})
	if err != nil {
		return err
	}

	// 拒否されたエントリを分類して報告する（黙って飛ばすと利用者が気付けない）。
	if n := result.Rejected(); n > 0 {
		var parts []string
		if len(result.Escaped) > 0 {
			parts = append(parts, fmt.Sprintf("展開先の外へ出る%d件（%s）", len(result.Escaped), summarizeNames(result.Escaped)))
		}
		if len(result.ViaSymlink) > 0 {
			parts = append(parts, fmt.Sprintf("シンボリックリンク経由の%d件（%s）", len(result.ViaSymlink), summarizeNames(result.ViaSymlink)))
		}
		return fmt.Errorf("安全でないパスのエントリを%d件拒否しました: %s", n, strings.Join(parts, "および"))
	}
	return nil
}

// summarizeNames はエラー文言に載せる名前を先頭5件に絞り、
// 残りは件数だけ示す。悪意あるZIPが数千件を並べても文言が巨大化しないようにする。
func summarizeNames(names []string) string {
	const maxShow = 5
	if len(names) <= maxShow {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:maxShow], ", ") + fmt.Sprintf(", 他%d件", len(names)-maxShow)
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

// pathInside はtargetがbaseフォルダの内側（自身を含む）にあるかを返す。
// 実体パスで比較するため、シンボリックリンク越しの表記差も拾う。
func pathInside(base, target string) bool {
	rel, err := filepath.Rel(resolveExisting(base), resolveExisting(target))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}


