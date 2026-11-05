package nodelib

import (
	"archive/zip"
	"errors"
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

	return writeZip(src, srcInfo, zipFile)
}

// writeZip は src を ZIP として out へ書き出す。
// zip.Writer の Close (中央ディレクトリと EOCD の書き込み) と out の Close で
// 初めて表面化する書き込み失敗 (容量不足など) があるため、defer で戻り値を
// 捨てず、walk のエラーと合わせてすべて返す (#330)。
// テストから close 失敗を注入できるよう、出力先は io.WriteCloser で受ける。
func writeZip(src string, srcInfo os.FileInfo, out io.WriteCloser) error {
	zw := zip.NewWriter(out)

	var baseDir string
	if srcInfo.IsDir() {
		baseDir = filepath.Dir(filepath.Clean(src))
	}

	walkErr := filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
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

		_, copyErr := io.Copy(w, file)
		closeErr := file.Close()
		return errors.Join(copyErr, closeErr)
	})

	zipCloseErr := zw.Close()
	outCloseErr := out.Close()
	return errors.Join(walkErr, zipCloseErr, outCloseErr)
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
//
// 「..」は文字列として先に畳まず、コンポーネントごとに解決しながら親へ戻す。
// filepath.Abs は Clean で「..」を字句的に畳むため、alias が src/inner を
// 指すときに alias/../new を「作業フォルダ直下の new」と誤認し、実体である
// src/new（移動元の内側）を見逃す。OS は「..」をシンボリックリンク解決後の
// 親に対して適用するので、ここでも同じ順序で辿る (#328)。
//
// 分割は os.PathSeparator で行うため、先に filepath.FromSlash で区切りを
// 正規化する。Windows のなでしこプログラムは「/」区切りでパスを書くことが
// 多く、正規化しないと「C:/work/alias/new」が1コンポーネント扱いになり、
// 未作成の末尾で EvalSymlinks が失敗して alias を解決しないまま cleaned
// path を返してしまう (#365)。Unix では no-op。
func resolveExisting(path string) string {
	path = filepath.FromSlash(path)
	abs := path
	if !filepath.IsAbs(abs) {
		wd, err := os.Getwd()
		if err != nil {
			return filepath.Clean(path)
		}
		// ドライブを持たない rooted path（\foo）は IsAbs が false になるが、
		// 作業フォルダからの相対ではなく現在のドライブのルートを指す。
		// 作業フォルダのボリュームで解決する。Unix では「/foo」が IsAbs に
		// なるためこの分岐には入らない。
		if vol := filepath.VolumeName(wd); vol != "" && isRootedPath(abs) {
			abs = vol + abs
		} else {
			abs = wd + string(os.PathSeparator) + abs
		}
	}
	vol := filepath.VolumeName(abs)
	resolved := vol + string(os.PathSeparator)
	parts := strings.Split(abs[len(vol):], string(os.PathSeparator))
	for i, comp := range parts {
		switch comp {
		case "", ".":
			continue
		case "..":
			// 解決済みの親へ戻す。シンボリックリンクの指す先が基準になる。
			resolved = filepath.Dir(resolved)
			continue
		}
		candidate := filepath.Join(resolved, comp)
		next, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			// 未作成または壊れたリンク。これ以降は解決できないため、
			// 残りをそのまま連結する（従来の契約を維持する）。
			return filepath.Join(append([]string{resolved}, parts[i:]...)...)
		}
		resolved = next
	}
	return resolved
}

// isRootedPath は区切り文字で始まるパスかを返す。Windows の「\foo」は
// ドライブを持たないため filepath.IsAbs が false になる。Unix では
// 区切り文字で始まるパスは常に絶対パスなので、この判定は使われない。
func isRootedPath(path string) bool {
	return len(path) > 0 && os.IsPathSeparator(path[0])
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
