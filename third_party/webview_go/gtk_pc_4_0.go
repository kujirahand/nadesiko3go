//go:build (linux || openbsd || freebsd || netbsd) && !webkit2gtk_4_1

package webview

// WebKitGTK 4.0 向けの既定。ビルドタグ webkit2gtk_4_1 が無いときに使う。
// Ubuntu 22.04 や Debian 12 の libwebkit2gtk-4.0-dev がこの pkg-config 名を提供する。

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.0
*/
import "C"
