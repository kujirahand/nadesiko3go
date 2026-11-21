//go:build (linux || openbsd || freebsd || netbsd) && webkit2gtk_4_1

package webview

// WebKitGTK 4.1 向け。Ubuntu 24.04 と Debian 13 には 4.0 の開発パッケージが無い。
// scripts/webkit-gtk-tag.sh が pkg-config を見て、必要なときだけこのタグを付ける。

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
*/
import "C"
