#!/usr/bin/env bash
set -euo pipefail

# nadesiko3go (gonako) インストーラー (macOS / Linux 用)
# 使い方:
#   curl -fsSL https://raw.githubusercontent.com/kujirahand/nadesiko3go/main/scripts/install.sh | bash

REPO="kujirahand/nadesiko3go"
DEFAULT_VERSION="3.8.1"

# バージョンの決定
if [ -n "${GONAKO_VERSION:-}" ]; then
  VERSION="$GONAKO_VERSION"
else
  # GitHub API から最新リリースを取得（失敗した場合はデフォルト値）
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

BIN_NAME="gonako-${VERSION}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${BIN_NAME}"

# インストール先の決定
INSTALL_DIR="${GONAKO_INSTALL_DIR:-}"
if [ -z "$INSTALL_DIR" ]; then
  if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
fi

mkdir -p "$INSTALL_DIR"
TARGET="$INSTALL_DIR/gonako"

echo "===> なでしこ3 (gonako v${VERSION}) をダウンロード中..."
echo "  URL: $URL"
echo "  保存先: $TARGET"

TMP_FILE="$(mktemp)"
if ! curl -fSL "$URL" -o "$TMP_FILE"; then
  rm -f "$TMP_FILE"
  echo "エラー: ダウンロードに失敗しました ($URL)" >&2
  exit 1
fi

mv "$TMP_FILE" "$TARGET"
chmod 755 "$TARGET"

echo "===> インストールが完了しました: $TARGET"

# PATH の確認
if ! command -v gonako >/dev/null 2>&1; then
  echo ""
  echo "注意: $INSTALL_DIR に PATH が通っていません。"
  echo "シェルの設定ファイル (~/.zshrc や ~/.bashrc) に以下を追加してください:"
  echo ""
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
  echo ""
  export PATH="$INSTALL_DIR:$PATH"
fi

echo "===> 動作確認:"
"$TARGET" -e '「なでしこ3のインストールに成功しました！」と表示。' || true
