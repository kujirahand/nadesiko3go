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
//   - ViaSymlink: os.Root が拒否（既存シンボリックリンク経由の脱出）
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
				// os.Root が拒否 = シンボリックリンク経由の脱出
				result.ViaSymlink = append(result.ViaSymlink, f.Name)
			}
			continue
		}

		// ファイルの親ディレクトリを確保する
		parent := filepath.Dir(localPath)
		if parent != "." {
			if err := root.MkdirAll(parent, opts.DirPerm); err != nil {
				result.ViaSymlink = append(result.ViaSymlink, f.Name)
				continue
			}
		}

		// f.Mode().Perm() でファイル種別ビット（setuid/sticky等）を落とす（#261 項目3）
		perm := f.Mode().Perm()
		if perm == 0 {
			perm = opts.FilePermDefault
		}

		outFile, err := root.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
		if err != nil {
			result.ViaSymlink = append(result.ViaSymlink, f.Name)
			continue
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

// stripTopDir はスラッシュ区切りのパスから先頭コンポーネントを取り除く。
// "a/b/c" → "b/c"、"a/" → ""、"a" → ""
func stripTopDir(name string) string {
	_, rest, found := strings.Cut(name, "/")
	if !found {
		return ""
	}
	return rest
}
