// Package gonakopackages は直下のgonako-packageを埋め込み登録する。公開APIは持たない。
package gonakopackages

import (
	"embed"
	"github.com/kujirahand/nadesiko3go/internal/nakopackage"
)

// go:embedは親ディレクトリを参照できないため、この宣言だけをルートに置く。
//
//go:embed all:gonako-package
var packageFiles embed.FS

func init() {
	nakopackage.Files = packageFiles
}
