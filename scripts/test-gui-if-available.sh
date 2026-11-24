#!/usr/bin/env bash
# GUIの開発環境が揃っている場合だけ、標準テストからGUIテストを実行する。
set -euo pipefail

GO="${GO:-go}"

skip() {
  echo "GUIテストをスキップ: $1"
  exit 0
}

if [ "$("$GO" env CGO_ENABLED)" != 1 ]; then
  skip 'cgo が無効です'
fi

case "$("$GO" env GOOS)" in
  darwin|windows) ;;
  linux)
    # CC/CXXにはclangやラッパー、オプション付きの指定も使える。
    for name in CC CXX; do
      compiler="$("$GO" env "$name")"
      read -r -a words <<< "$compiler"
      if [ "${#words[@]}" -eq 0 ] || ! command -v "${words[0]}" >/dev/null 2>&1; then
        skip "$name のコンパイラが見つかりません: $compiler"
      fi
    done
    if ! command -v pkg-config >/dev/null 2>&1 ||
       ! pkg-config --exists gtk+-3.0 ||
       ! { pkg-config --exists webkit2gtk-4.0 || pkg-config --exists webkit2gtk-4.1; }; then
      skip 'GTK・WebKitGTK の開発環境がありません'
    fi
    ;;
  *) skip '対応する GUI 開発環境がありません' ;;
esac

root="$(cd "$(dirname "$0")/.." && pwd)"
exec env GO="$GO" "$root/scripts/go-gui.sh" test ./cmd/gonako-gui ./internal/guilib
