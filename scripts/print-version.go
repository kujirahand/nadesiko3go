//go:build ignore

// print-version は internal/version/version.go のリリース版番号を標準出力へ出す。
// シェルスクリプトからバージョン番号を参照するために使う。
// -nadesiko を付けると、なでしこ言語バージョン（version.Nadesiko）を出す。
package main

import (
	"flag"
	"fmt"

	"github.com/kujirahand/nadesiko3go/internal/version"
)

func main() {
	nadesiko := flag.Bool("nadesiko", false, "なでしこ言語バージョンを出力する")
	flag.Parse()
	if *nadesiko {
		fmt.Println(version.Nadesiko)
		return
	}
	fmt.Println(version.Version)
}
