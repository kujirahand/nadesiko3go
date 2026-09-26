package gonakopackages

import "embed"

// PackageFiles はビルド時のなでしこパッケージを保持する。
//
//go:embed all:gonako-package
var PackageFiles embed.FS
