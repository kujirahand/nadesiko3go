// Package safepath は os.Root（Go 1.24+）を使った安全なZIP展開ヘルパを提供する。
//
// ZIP展開では2種類の脱出経路がある:
//  1. パスに ".." を含むなどして展開先の外へ出る（Zip Slip）
//  2. 既存のシンボリックリンクを経由して展開先の外へ書き込む
//
// os.Root はカーネルレベルでパス解決を行うため、検査と書き込みの間の
// TOCTOU（Time-of-check to time-of-use）脆弱性がなく、また展開先内側の
// 無害なシンボリックリンクも透過的に辿れる。
//
// filepath.Localize は末尾スラッシュを持つパス（ZIPのディレクトリエントリ）
// を "invalid path" で拒否するため、事前にトリムが必要。
package safepath

import (
	"archive/zip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ExtractOptions は Extract の挙動を制御する。
type ExtractOptions struct {
	// StripTopDir は各エントリ名の先頭コンポーネントを取り除く。
	// GitHub の zipball は "repo-branch/" を先頭に持つため、
	// そのまま展開すると1段余分にネストしてしまう。
	StripTopDir bool
	// DirPerm は作成するディレクトリのパーミッション。0 の場合は 0o755。
	DirPerm fs.FileMode
	// FilePermDefault はZIPエントリのモードが0の場合のファイル既定値。
	// 0 の場合は 0o644。
	FilePermDefault fs.FileMode
}

// ExtractResult は展開中に拒否されたエントリの分類結果。
type ExtractResult struct {
	// Escaped は filepath.Localize が拒否したエントリ名（Zip Slip 等）。
	Escaped []string
	// ViaSymlink は os.Root が拒否したエントリ名（シンボリックリンク経由の脱出）。
	ViaSymlink []string
}

// Rejected は拒否されたエントリの総数を返す。
func (r *ExtractResult) Rejected() int {
	if r == nil {
		return 0
	}
	return len(r.Escaped) + len(r.ViaSymlink)
}

// Extract はZIPの全エントリを destDir へ安全に展開する。
// os.Root を使って閉じ込めを行うため、Zip Slip やシンボリックリンク経由の
// 脱出を防げる。
//
// 拒否されたエントリは ExtractResult に分類して返す:
//   - Escaped: filepath.Localize が拒否（".." や絶対パス）
//   - ViaSymlink: 既存シンボリックリンク経由で root が脱出した
//
// シンボリックリンク脱出以外での失敗（ENOTDIR、EACCES 等）は通常の
// エラーとして即座に返す。黙って飛ばすと利用者が気付けないため。
//
// 拒否されたエントリがあっても展開自体は続き、最後にまとめて結果を返す。
func Extract(r *zip.Reader, destDir string, opts ExtractOptions) (*ExtractResult, error) {
	if opts.DirPerm == 0 {
		opts.DirPerm = 0o755
	}
	if opts.FilePermDefault == 0 {
		opts.FilePermDefault = 0o644
	}

	if err := os.MkdirAll(destDir, opts.DirPerm); err != nil {
		return nil, err
	}

	root, err := os.OpenRoot(destDir)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	result := &ExtractResult{}

	for _, f := range r.File {
		name := f.Name
		if opts.StripTopDir {
			name = stripTopDir(name)
			if name == "" {
				continue
			}
		}

		isDir := f.FileInfo().IsDir()
		// filepath.Localize は末尾スラッシュを "invalid path" で拒否する。
		// ZIPのディレクトリエントリはほぼ確実に末尾スラッシュを持つため、
		// 分類前にトリムする（Issue #261 の注意点）。
		localName := strings.TrimSuffix(name, "/")
		if localName == "" {
			// トップディレクトリを除去した結果が空（元が "topdir/" だけ）
			continue
		}

		// 分類: filepath.Localize でパスの妥当性を判定する。
		// ".." を含む、または絶対パスの場合はここで拒否される。
		localPath, err := filepath.Localize(localName)
		if err != nil {
			result.Escaped = append(result.Escaped, f.Name)
			continue
		}

		if isDir {
			if err := root.MkdirAll(localPath, opts.DirPerm); err != nil {
				if pathHasEscapingSymlink(root, localPath) {
					result.ViaSymlink = append(result.ViaSymlink, f.Name)
					continue
				}
				return nil, err
			}
			continue
		}

		// ファイルの親ディレクトリを確保する
		parent := filepath.Dir(localPath)
		if parent != "." {
			if err := root.MkdirAll(parent, opts.DirPerm); err != nil {
				if pathHasEscapingSymlink(root, parent) {
					result.ViaSymlink = append(result.ViaSymlink, f.Name)
					continue
				}
				return nil, err
			}
		}

		// f.Mode().Perm() でファイル種別ビット（setuid/sticky等）を落とす（#261 項目3）
		perm := f.Mode().Perm()
		if perm == 0 {
			perm = opts.FilePermDefault
		}

		outFile, err := root.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
		if err != nil {
			if pathHasEscapingSymlink(root, localPath) {
				result.ViaSymlink = append(result.ViaSymlink, f.Name)
				continue
			}
			return nil, err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return nil, err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// pathHasEscapingSymlink は root 内の localPath に、root の外を指す
// シンボリックリンクが含まれているかを調べる。
//
// os.Root の MkdirAll/OpenFile はリンク脱出以外（ENOTDIR、EACCES 等）でも
// 失敗するため、エラー原因が実際にシンボリックリンク脱出かどうかを
// 判定するために使う。root.Lstat はリンクを辿らず成功し、root.Readlink
// はリンク先が root 外でもターゲット文字列を返す（PR #272 レビュー指摘）。
func pathHasEscapingSymlink(root *os.Root, localPath string) bool {
	dir := filepath.Dir(localPath)
	elems := strings.Split(dir, string(filepath.Separator))
	if len(elems) == 1 && elems[0] == "." {
		elems = nil
	}
	// localPath 自体の最終コンポーネントも調べる（ファイル自身が脱出リンクの場合）
	if base := filepath.Base(localPath); base != "." && base != "" {
		elems = append(elems, base)
	}

	cur := ""
	for _, elem := range elems {
		if elem == "." || elem == "" {
			continue
		}
		if cur == "" {
			cur = elem
		} else {
			cur = cur + string(filepath.Separator) + elem
		}

		fi, err := root.Lstat(cur)
		if err != nil {
			// 存在しない = これ以降のコンポーネントも未作成なので脱出しない
			return false
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			// 通常ファイル/ディレクトリ = root が解決するので安全
			continue
		}
		// シンボリックリンクを見つけた。ターゲットを読む。
		// root.Readlink はターゲットが root 外でも成功する。
		target, err := root.Readlink(cur)
		if err != nil {
			return false
		}
		if symlinkTargetEscapes(cur, target) {
			return true
		}
	}
	return false
}

// symlinkTargetEscapes はシンボリックリンクのターゲットがリンクの位置から
// 見て root の外に出るかどうかを判定する。絶対パス、または解決後に root から
// 出る相対パス（".." が先頭に残る）を脱出とみなす。
//
// linkPath はリンク自身のパス（root からの相対パス）。
// target はリンクのターゲット文字列。
//
// 例:
//   - linkPath="sub/link", target="../inside" → sub/../inside = inside（安全）
//   - linkPath="sub/link", target="../../outside" → sub/../../outside = ../outside（脱出）
func symlinkTargetEscapes(linkPath, target string) bool {
	if filepath.IsAbs(target) {
		return true
	}
	// リンクの親ディレクトリを基準にターゲットを解決
	linkDir := filepath.Dir(linkPath)
	resolved := filepath.Join(linkDir, target)
	// クリーンにして、.. が先頭に残るかで判定
	cleaned := filepath.Clean(resolved)
	return cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator))
}

// stripTopDir はスラッシュ区切りのパスから先頭コンポーネントを取り除く。
// "a/b/c" → "b/c"、"a/" → ""、"a" → ""
func stripTopDir(name string) string {
	_, rest, found := strings.Cut(name, "/")
	if !found {
		return ""
	}
	return rest
}
