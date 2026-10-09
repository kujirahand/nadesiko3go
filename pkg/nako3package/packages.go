// Package nako3package はランタイムへ埋め込むなでしこパッケージを持つ。
//
// このディレクトリに置いた `<package-name>.nako3` と `<package-name>/` 以下の
// ファイルが、ビルド時にそのまま埋め込まれる（→ docs/gonako-package.md）。
package nako3package

import "embed"

// Files は埋め込んだパッケージのファイルシステム。
// パスはこのディレクトリからの相対パス（例: `demo.nako3`, `demo/child.nako3`）。
// 取込ではパッケージ用の `.nako3` と関連ファイルだけを参照する。
//
//go:embed all:*
var Files embed.FS
