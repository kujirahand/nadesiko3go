package nodelib

import (
	"archive/zip"
	"fmt"
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

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	// 展開先を実体の絶対パスに直す。「.」指定でも包含判定が効くようにし、
	// 展開先自身がシンボリックリンクでも内側判定がずれないようにする (#194)
	absDest := resolveExisting(destDir)

	// 安全なエントリは全て展開し、拒否したエントリは最後にまとめて
	// エラーとして報告する（黙って飛ばすと利用者が気付けない）。
	// 理由は「展開先の外へ出る」と「シンボリックリンク経由」に分類し、
	// 実態と合わない文面にならないようにする
	var escaped, viaLink []string
	for _, f := range r.File {
		fpath := filepath.Join(absDest, filepath.FromSlash(f.Name))
		// Zip Slip対策: 展開先の外へ出るエントリは拒否する
		rel, err := filepath.Rel(absDest, fpath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			escaped = append(escaped, f.Name)
			continue
		}
		// 途中の既存シンボリックリンクを通ると展開先の外へ書き出すため拒否する (#185)
		if hasSymlinkComponent(absDest, rel) {
			viaLink = append(viaLink, f.Name)
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
	if n := len(escaped) + len(viaLink); n > 0 {
		var parts []string
		if len(escaped) > 0 {
			parts = append(parts, fmt.Sprintf("展開先の外へ出る%d件（%s）", len(escaped), summarizeNames(escaped)))
		}
		if len(viaLink) > 0 {
			parts = append(parts, fmt.Sprintf("シンボリックリンク経由の%d件（%s）", len(viaLink), summarizeNames(viaLink)))
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

// hasSymlinkComponent はbaseからrelを辿り、途中に既存のシンボリックリンクが
// あれば真を返す。存在しない要素は将来新規作成されるためそれ以深を調べないが、
// EACCES などのその他のエラーは判定続行不能として安全側（真=拒否）に倒す。
// なお、展開先の**内側**を指す無害なリンクまで拒否するのは現時点の仕様である。
// 透過的に辿れるようにする根本改善は os.Root での閉じ込めとして Issue #261 に切り出し。
// 検査と実際の作成・書き込みの間に他プロセスがリンクを差し替える
// TOCTOUの余地は残るが、CLI用途では現実的な脅威が小さいため許容とする。
func hasSymlinkComponent(base, rel string) bool {
	cur := base
	for _, elem := range strings.Split(rel, string(os.PathSeparator)) {
		cur = filepath.Join(cur, elem)
		fi, err := os.Lstat(cur)
		if err != nil {
			// 存在しないだけなら安全、それ以外のエラーは拒否に倒す
			return !os.IsNotExist(err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}
