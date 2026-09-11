// Package version は gonako のバージョン番号を一元管理する。
//
// 更新は必ず `just version-update X.Y.Z` で行うこと。手で書き換えると
// scripts/install.sh・scripts/install.ps1・README.md・docs/release-homebrew.md
// など他ファイルとズレる。
package version

// Version は gonako / gonako-cui / gonako-gui のリリース版番号。
// gitタグ・配布ファイル名（gonako-<Version>-darwin-arm64 など）と一致させる。
const Version = "3.8.4"

// Nadesiko は「ナデシコバージョン」「ナデシコ言語バージョン」定数が返す
// なでしこ言語仕様のバージョン。本家(TypeScript版)のpackage.jsonと合わせる。
// gonako独自のパッチリリースでは Version だけを進めてよい。
const Nadesiko = "3.8.2"
