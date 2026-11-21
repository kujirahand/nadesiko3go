#!/usr/bin/env bash
# Debian / Ubuntu で gonako-gui のビルドに必要なパッケージを入れる。
# WebKitGTK は 4.1 を優先し、リポジトリに無ければ 4.0 を入れる。
set -euo pipefail

if ! command -v apt-get >/dev/null 2>&1; then
  echo "apt-get がありません。Fedora や Arch のパッケージ名は docs/build-gonako-gui.md を見てください。" >&2
  exit 1
fi

sudo apt-get update
pkgs=(gcc g++ pkg-config libgtk-3-dev)
if apt-cache show libwebkit2gtk-4.1-dev >/dev/null 2>&1; then
  pkgs+=(libwebkit2gtk-4.1-dev)
elif apt-cache show libwebkit2gtk-4.0-dev >/dev/null 2>&1; then
  pkgs+=(libwebkit2gtk-4.0-dev)
else
  echo "libwebkit2gtk-4.1-dev も libwebkit2gtk-4.0-dev もパッケージ一覧にありません。" >&2
  exit 1
fi

sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "${pkgs[@]}"
echo "インストールしました: ${pkgs[*]}"
echo "次: scripts/go-gui.sh build -o bin/gonako-gui ./cmd/gonako-gui"
