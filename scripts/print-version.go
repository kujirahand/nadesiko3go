//go:build ignore

// print-version は internal/version/version.go のリリース版番号を標準出力へ出す。
// シェルスクリプトからバージョン番号を参照するために使う。
package main

import (
	"fmt"

	"github.com/kujirahand/nadesiko3go/internal/version"
)

func main() {
	fmt.Println(version.Version)
}
