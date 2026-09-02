#!/usr/bin/env bash
set -euo pipefail

# nadesiko3go (gonako & gonako-gui) インストーラー (macOS / Linux 用)
# 使い方:
#   curl -fsSL https://raw.githubusercontent.com/kujirahand/nadesiko3go/master/scripts/install.sh | bash

REPO="kujirahand/nadesiko3go"
DEFAULT_VERSION="3.8.1"

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
BIN_NAME="gonako-${VERSION}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${BIN_NAME}"
TARGET="$INSTALL_DIR/gonako"

echo "===> [1/2] なでしこ3 CLI版 (gonako v${VERSION}) をインストール中..."
TMP_FILE="$(mktemp)"
if curl -fSL "$URL" -o "$TMP_FILE"; then
  mv "$TMP_FILE" "$TARGET"
  chmod 755 "$TARGET"
  echo "  -> インストール完了: $TARGET"
else
  rm -f "$TMP_FILE"
  echo "  [警告] CLI版のダウンロードに失敗しました ($URL)" >&2
fi

# ----------------------------------------------------
# 2. GUI版 (gonako-gui / なでしこ3.app) のインストール
# ----------------------------------------------------
if [ "$OS" = "darwin" ]; then
  echo "===> [2/2] なでしこ3 GUI版 (gonako-gui v${VERSION}) をインストール中..."
  GUI_ZIP="gonako-gui-${VERSION}-darwin-${ARCH}.app.zip"
  GUI_URL="https://github.com/${REPO}/releases/download/${VERSION}/${GUI_ZIP}"
  TMP_ZIP="$(mktemp).zip"
  TMP_DIR="$(mktemp -d)"

  if curl -fSL "$GUI_URL" -o "$TMP_ZIP"; then
    unzip -q -o "$TMP_ZIP" -d "$TMP_DIR"
    APP_SRC=$(find "$TMP_DIR" -name "*.app" -maxdepth 2 | head -n 1)

    if [ -n "$APP_SRC" ] && [ -d "$APP_SRC" ]; then
      APP_DEST="/Applications/なでしこ3.app"
      if [ ! -w "/Applications" ]; then
        mkdir -p "$HOME/Applications"
        APP_DEST="$HOME/Applications/なでしこ3.app"
      fi

      rm -rf "$APP_DEST"
      cp -R "$APP_SRC" "$APP_DEST"

      # Gatekeeper の隔離属性（quarantine）を解除
      xattr -cr "$APP_DEST" 2>/dev/null || true

      # コマンドラインからも呼び出せるようにシンボリックリンクを作成
      if [ -f "$APP_DEST/Contents/MacOS/gonako-gui" ]; then
        ln -sf "$APP_DEST/Contents/MacOS/gonako-gui" "$INSTALL_DIR/gonako-gui"
      fi

      echo "  -> アプリケーションを配置しました: $APP_DEST"
      echo "  -> コマンドラインリンクを作成: $INSTALL_DIR/gonako-gui"
    fi
  else
    echo "  [スキップ] GUI版のダウンロードに失敗しました ($GUI_URL)" >&2
  fi

  rm -rf "$TMP_ZIP" "$TMP_DIR"
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
