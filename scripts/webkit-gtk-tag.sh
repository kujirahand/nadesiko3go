#!/bin/sh
# Linux で gonako-gui をビルドするときの Go ビルドタグを標準出力へ出す。
# webkit2gtk-4.0 がある、または Linux 以外なら空。4.1 だけあるときは webkit2gtk_4_1。
# どちらも無いときは終了コード 1。
set -eu

os=$(uname -s)
if [ -n "${GOOS:-}" ]; then
  case "$GOOS" in
    linux) os=Linux ;;
    *) os=$GOOS ;;
  esac
fi

if [ "$os" != "Linux" ]; then
  exit 0
fi

if ! command -v pkg-config >/dev/null 2>&1; then
  echo "pkg-config がありません。gcc / g++ / pkg-config と GTK・WebKitGTK の開発パッケージを入れてください。" >&2
  exit 1
fi

if pkg-config --exists webkit2gtk-4.0; then
  exit 0
fi

if pkg-config --exists webkit2gtk-4.1; then
  printf '%s\n' webkit2gtk_4_1
  exit 0
fi

cat >&2 <<'EOF'
WebKitGTK の開発ファイルが見つかりません。pkg-config で webkit2gtk-4.0 または webkit2gtk-4.1 が見える必要があります。

  Ubuntu 24.04 / Debian 13:
    sudo apt-get install -y gcc g++ pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
  Ubuntu 22.04 / Debian 12（4.0 でも 4.1 でも可）:
    sudo apt-get install -y gcc g++ pkg-config libgtk-3-dev libwebkit2gtk-4.0-dev
  Fedora:
    sudo dnf install gcc-c++ pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel
  Arch Linux:
    sudo pacman -S --needed gcc pkgconf gtk3 webkit2gtk-4.1

入れたあとは scripts/go-gui.sh または just gui でビルドしてください。
EOF
exit 1
