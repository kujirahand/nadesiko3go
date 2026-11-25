package nodelib

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/safepath"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

func zipCommands(m map[string]command) {
	m["ZIP一覧取得"] = command{ // @ZIPファイルAのエントリ一覧を辞書(名前・サイズ・圧縮後サイズ・更新日時・フォルダ)の配列で返す // @ZIPいちらんしゅとく
		josi: [][]string{{"の", "を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			return zipList(str(a, 0))
		},
	}

	m["ZIPエントリ読"] = command{ // @ZIPファイルAからエントリ名Bの内容をバイト配列で読む // @ZIPえんとりよ
		josi: [][]string{{"の", "から"}, {"を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			data, err := zipReadEntry(str(a, 0), str(a, 1))
			if err != nil {
				return value.Undefined(), err
			}
			return bytesValue(data), nil
		},
	}

	m["ZIP追加"] = command{ // @ZIPファイルAへエントリ名BでデータC(文字列またはバイト配列)を追加する(ZIPが無ければ新規作成。同じZIPへの同時追加は不可) // @ZIPついか
		josi:       [][]string{{"へ", "に"}, {"で"}, {"を"}},
		returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			data, err := anyBytes(argAt(a, 2))
			if err != nil {
				return value.Undefined(), err
			}
			if err := zipAddEntry(str(a, 0), str(a, 1), data); err != nil {
				return value.Undefined(), err
			}
			return value.Undefined(), nil
		},
	}

	m["圧縮"] = command{ // @ファイルまたはフォルダAをZIP形式でBへ圧縮する(Go標準のarchive/zipで処理するため外部ツールは不要) // @あっしゅく
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

	m["解凍"] = command{ // @ZIPファイルAをフォルダBへ解凍する(Go標準のarchive/zipで処理するため外部ツールは不要) // @かいとう
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
// 初めて表面化する書き込み失敗 (容量不足など) があるため、close の戻り値を
// 捨てず、walk のエラーと合わせてすべて返す (#330)。
// close は defer で名前付き戻り値へ合成する。実行順は登録と逆順で
// zw.Close() → out.Close() になり、walk 中に panic が起きても
// (VM は panic を回収して言語エラーにする) 両方とも必ず1回ずつ呼ばれる。
// テストから close 失敗を注入できるよう、出力先は io.WriteCloser で受ける。
func writeZip(src string, srcInfo os.FileInfo, out io.WriteCloser) (err error) {
	zw := zip.NewWriter(out)
	defer func() {
		err = errors.Join(err, out.Close())
	}()
	defer func() {
		err = errors.Join(err, zw.Close())
	}()

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

		_, copyErr := io.Copy(w, file)
		closeErr := file.Close()
		return errors.Join(copyErr, closeErr)
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

// zipEntryName はZIPエントリ名を検証し、「/」区切りに正規化して返す。
// 絶対パス・ドライブ指定・「..」を含む名前は展開時にZip Slipの原因になるためエラーにする。
func zipEntryName(name string) (string, error) {
	n := strings.ReplaceAll(name, "\\", "/")
	if n == "" {
		return "", errors.New("ZIPのエントリ名が空です。")
	}
	if strings.HasPrefix(n, "/") || (len(n) >= 2 && n[1] == ':') {
		return "", fmt.Errorf("ZIPのエントリ名に絶対パスは使えません: %s", name)
	}
	for _, part := range strings.Split(n, "/") {
		if part == ".." {
			return "", fmt.Errorf("ZIPのエントリ名に「..」は使えません: %s", name)
		}
	}
	return n, nil
}

// zipList はZIPのエントリ一覧を辞書の配列で返す。
func zipList(path string) (value.Value, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return value.Undefined(), err
	}
	defer r.Close()
	items := make([]value.Value, 0, len(r.File))
	for _, f := range r.File {
		d := value.NewDict()
		d.Set("名前", value.String(f.Name))
		d.Set("サイズ", value.Number(float64(f.UncompressedSize64)))
		d.Set("圧縮後サイズ", value.Number(float64(f.CompressedSize64)))
		d.Set("更新日時", value.String(f.Modified.Format("2006-01-02 15:04:05")))
		d.Set("フォルダ", value.Bool(f.FileInfo().IsDir()))
		items = append(items, value.DictValue(d))
	}
	return value.ArrayValue(value.NewArray(items...)), nil
}

// zipEntryMaxBytes は『ZIPエントリ読』でメモリへ展開する1エントリの上限。
// 圧縮率の高いデータ(ZIP爆弾)でメモリを使い尽くさないための制限である。
const zipEntryMaxBytes = 512 << 20

// zipReadEntry はZIP内のエントリの内容を返す。
func zipReadEntry(path, name string) ([]byte, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	want := strings.ReplaceAll(name, "\\", "/")
	for _, f := range r.File {
		if f.Name != want {
			continue
		}
		if f.UncompressedSize64 > zipEntryMaxBytes {
			return nil, fmt.Errorf("エントリ『%s』の展開後サイズ(%dバイト)が上限(%dバイト)を超えています。", name, f.UncompressedSize64, zipEntryMaxBytes)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, zipEntryMaxBytes+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > zipEntryMaxBytes {
			return nil, fmt.Errorf("エントリ『%s』の展開後サイズが上限(%dバイト)を超えています。", name, zipEntryMaxBytes)
		}
		return data, nil
	}
	return nil, fmt.Errorf("ZIPにエントリ『%s』がありません。", name)
}

// zipAddEntry はZIPへエントリを追加する(ZIPが無ければ新規作成)。
// 既存エントリは再圧縮せずそのままコピーし、結果は一時ファイル経由で
// 原子的に置き換える。書き込みはストリーミングで行い、ZIP全体をメモリへ
// 保持しない。同じZIPへの同時追加は想定していない(後から置換した側が残る)。
func zipAddEntry(path, name string, data []byte) error {
	name, err := zipEntryName(name)
	if err != nil {
		return err
	}
	// 既存ZIPは書き込み中も読み続けるため、先に開いて重複を確認する。
	var r *zip.ReadCloser
	if rc, err := zip.OpenReader(path); err == nil {
		r = rc
		defer r.Close()
		for _, f := range r.File {
			if f.Name == name {
				return fmt.Errorf("ZIPに同名のエントリ『%s』が既にあります。", name)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeFileAtomicFunc(path, 0o644, func(w io.Writer) error {
		zw := zip.NewWriter(w)
		if r != nil {
			for _, f := range r.File {
				if err := zw.Copy(f); err != nil {
					return err
				}
			}
		}
		hw, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			return err
		}
		if _, err := hw.Write(data); err != nil {
			return err
		}
		return zw.Close()
	})
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
