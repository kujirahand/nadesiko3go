#!/usr/bin/env bash
# gonako-gui を、Linuxでは入っている WebKitGTK（4.0 または 4.1）に合わせてビルド・テスト・実行する。
#
# 使い方:
#   scripts/go-gui.sh build -o bin/gonako-gui ./cmd/gonako-gui
#   scripts/go-gui.sh test ./cmd/gonako-gui ./internal/guilib
#   scripts/go-gui.sh run ./cmd/gonako-gui
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

if [ "$#" -lt 1 ]; then
  echo "使い方: scripts/go-gui.sh <build|test|run> [go の引数...]" >&2
  exit 2
fi

sub="$1"
shift
case "$sub" in
  build|test|run) ;;
  *)
    echo "未対応のサブコマンドです: $sub（build / test / run）" >&2
    exit 2
    ;;
esac

GO="${GO:-go}"
tags="$("$root/scripts/webkit-gtk-tag.sh")"
args=("$sub")
if [ -n "$tags" ]; then
  args+=(-tags "$tags")
fi
if [ "$#" -gt 0 ]; then
  args+=("$@")
fi
exec "$GO" "${args[@]}"
