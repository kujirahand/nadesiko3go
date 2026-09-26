// Package nakopackage は埋め込まれたなでしこパッケージを内部で共有する。
package nakopackage

import "io/fs"

// Files はルートの埋め込み登録処理が初期化するファイルシステム。
var Files fs.FS
