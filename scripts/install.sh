#!/usr/bin/env bash
set -euo pipefail

# nadesiko3go (gonako & gonako-gui) インストーラー (macOS / Linux 用)
# 使い方:
#   curl -fsSL https://nadesi.com/install/gonako | bash
# または:
#   curl -fsSL https://raw.githubusercontent.com/kujirahand/nadesiko3go/master/scripts/install.sh | bash

REPO="kujirahand/nadesiko3go"
DEFAULT_VERSION="3.8.4"

# バージョンの決定
if [ -n "${GONAKO_VERSION:-}" ]; then
  VERSION="$GONAKO_VERSION"
else
  LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/' || true)
  if [ -n "$LATEST" ]; then
    VERSION="${LATEST#v}"
  else
    VERSION="$DEFAULT_VERSION"
  fi
fi

# OS の判定
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  darwin|linux) ;;
  *)
    echo "エラー: 未対応のOSです ($OS)" >&2
    exit 1
    ;;
esac

# CPU アーキテクチャの判定
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "エラー: 未対応のCPUアーキテクチャです ($ARCH)" >&2
    exit 1
    ;;
esac

# インストール先の決定 (CLI)
INSTALL_DIR="${GONAKO_INSTALL_DIR:-}"
if [ -z "$INSTALL_DIR" ]; then
  if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
fi
mkdir -p "$INSTALL_DIR"

# ----------------------------------------------------
# 1. CLI版 (gonako) のインストール
# ----------------------------------------------------
TARGET="$INSTALL_DIR/gonako"

download_cli() {
  local ver="$1"
  local zip_name="gonako-${ver}-${OS}-${ARCH}.zip"
  local url="https://github.com/${REPO}/releases/download/${ver}/${zip_name}"
  local tmp_zip="$(mktemp).zip"
  local tmp_dir="$(mktemp -d)"

  echo "===> [1/2] なでしこ3 CLI版 (gonako v${ver}) をダウンロード中..."
  if curl -fSL "$url" -o "$tmp_zip" 2>/dev/null; then
    unzip -q -o "$tmp_zip" -d "$tmp_dir"
    mv "$tmp_dir/gonako" "$TARGET"
    chmod 755 "$TARGET"
    rm -rf "$tmp_zip" "$tmp_dir"
    echo "  -> インストール完了: $TARGET"
    return 0
  fi
  rm -rf "$tmp_zip" "$tmp_dir"
  return 1
}

if ! download_cli "$VERSION"; then
  # 最新版の取得に失敗し、かつデフォルトバージョンと異なる場合は、安定版（DEFAULT_VERSION）で再試行する
  if [ "$VERSION" != "$DEFAULT_VERSION" ]; then
    echo "  [再試行] v${VERSION} のダウンロードに失敗したため、安定版 v${DEFAULT_VERSION} を試みます..."
    if download_cli "$DEFAULT_VERSION"; then
      VERSION="$DEFAULT_VERSION"
    else
      echo "エラー: CLI版のダウンロードに失敗しました" >&2
      exit 1
    fi
  else
    echo "エラー: CLI版のダウンロードに失敗しました" >&2
    exit 1
  fi
fi

# ----------------------------------------------------
# 2. GUI版 (gonako-gui / なでしこ3.app) のインストール
# ----------------------------------------------------
APP_DEST=""
download_gui() {
  local ver="$1"
  local gui_zip="gonako-gui-${ver}-darwin-${ARCH}.app.zip"
  local gui_url="https://github.com/${REPO}/releases/download/${ver}/${gui_zip}"
  local tmp_zip="$(mktemp).zip"
  local tmp_dir="$(mktemp -d)"

  if curl -fSL "$gui_url" -o "$tmp_zip" 2>/dev/null; then
    unzip -q -o "$tmp_zip" -d "$tmp_dir"
    local app_src
    app_src=$(find "$tmp_dir" -name "*.app" -maxdepth 2 | head -n 1)

    if [ -n "$app_src" ] && [ -d "$app_src" ]; then
      APP_DEST="/Applications/gonako-gui.app"
      if [ ! -w "/Applications" ]; then
        mkdir -p "$HOME/Applications"
        APP_DEST="$HOME/Applications/gonako-gui.app"
      fi

      rm -rf "$APP_DEST"
      cp -R "$app_src" "$APP_DEST"

      # Gatekeeper の隔離属性（quarantine）を解除
      xattr -cr "$APP_DEST" 2>/dev/null || true

      # コマンドラインからも呼び出せるようにシンボリックリンクを作成
      if [ -f "$APP_DEST/Contents/MacOS/gonako-gui" ]; then
        ln -sf "$APP_DEST/Contents/MacOS/gonako-gui" "$INSTALL_DIR/gonako-gui"
      fi

      echo "  -> アプリケーションを配置しました: $APP_DEST"
      echo "  -> コマンドラインリンクを作成: $INSTALL_DIR/gonako-gui"
      rm -rf "$tmp_zip" "$tmp_dir"
      return 0
    fi
  fi
  rm -rf "$tmp_zip" "$tmp_dir"
  return 1
}

if [ "$OS" = "darwin" ]; then
  echo "===> [2/2] なでしこ3 GUI版 (gonako-gui v${VERSION}) をインストール中..."
  if ! download_gui "$VERSION"; then
    if [ "$VERSION" != "$DEFAULT_VERSION" ]; then
      echo "  [再試行] GUI版 v${VERSION} のダウンロードに失敗したため、安定版 v${DEFAULT_VERSION} を試みます..."
      download_gui "$DEFAULT_VERSION" || echo "  [スキップ] GUI版のダウンロードに失敗しました" >&2
    else
      echo "  [スキップ] GUI版のダウンロードに失敗しました" >&2
    fi
  fi
fi

# ----------------------------------------------------
# PATH の確認と案内
# ----------------------------------------------------
echo ""
if ! command -v gonako >/dev/null 2>&1; then
  echo "注意: $INSTALL_DIR に PATH が通っていません。"
  echo "シェルの設定ファイル (~/.zshrc や ~/.bashrc) に以下を追加してください:"
  echo ""
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
  echo ""
  export PATH="$INSTALL_DIR:$PATH"
fi

echo "===> なでしこ3 のインストールが完了しました！"
echo ""
if [ -f "$TARGET" ]; then
  "$TARGET" -e '「CLI版 (gonako): こんにちは！」と表示。' || true
fi
if [ "$OS" = "darwin" ] && [ -d "${APP_DEST:-}" ]; then
  echo "GUI版は「${APP_DEST}」またはターミナルから「gonako-gui」で起動できます。"
fi
