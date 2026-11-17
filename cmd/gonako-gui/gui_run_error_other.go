//go:build !windows

package main

import (
	"fmt"
	"os"
)

// deliverGUIRunError は起動前エラーを標準エラーへ書く。
// リリースビルドが windowsgui になるのは Windows だけなので、
// それ以外は標準エラーが起動元へ届く。
func deliverGUIRunError(text string) bool {
	fmt.Fprintln(os.Stderr, text)
	return true
}

// platformShowGUIRunErrorDialog はダイアログを出さない環境向けの予備である。
// deliverGUIRunError が真を返すため、通常は呼ばれない。
func platformShowGUIRunErrorDialog(text string) {
	fmt.Fprintln(os.Stderr, text)
}
