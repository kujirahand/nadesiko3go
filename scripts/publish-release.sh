#!/bin/sh
# なでしこ3(gonako)のリリース配信をまとめて行う。
#
#   1. GitHub Releases のタグを作成（既にあればそのまま使う）
#   2. release/ 配下のZIPをアップロード
#   3. Homebrew Tap の Formula / Cask を更新してプッシュ
#
# 使い方: ./scripts/publish-release.sh [バージョン]
# バージョンを省略すると internal/version/version.go の値を使う。
# 事前に `just version-update X.Y.Z` と `just release` を済ませておくこと。
set -eu

cd "$(dirname "$0")/.."

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  VERSION=$(go run ./scripts/print-version.go)
fi
VERSION="${VERSION#v}"

echo "===> なでしこ3 v${VERSION} を配信します"

if ! command -v gh >/dev/null 2>&1; then
  echo "エラー: GitHub CLI (gh) が必要です" >&2
  exit 1
fi

# 1. リリース（タグ）の作成。タグ名は install.sh のURLに合わせて v を付けない。
if gh release view "$VERSION" >/dev/null 2>&1; then
  echo "--- [1/3] リリース ${VERSION} は既にあります"
else
  echo "--- [1/3] リリース ${VERSION} を作成します"
  gh release create "$VERSION" --title "v${VERSION}" --notes "Release ${VERSION}"
fi

# 2. 成果物のアップロード（just release が生成したスクリプトを使う）
UPLOAD="release/upload-${VERSION}.sh"
if [ ! -f "$UPLOAD" ]; then
  echo "エラー: ${UPLOAD} がありません。先に \`just release\` を実行してください" >&2
  exit 1
fi
echo "--- [2/3] 成果物をアップロードします"
sh "$UPLOAD"

# 3. Homebrew Tap の更新（公開済みZIPのSHA-256から生成してプッシュ）
echo "--- [3/3] Homebrew Tap を更新します"
go run ./scripts/update-homebrew-tap.go -version "$VERSION" -push

echo "===> 配信完了: https://github.com/kujirahand/nadesiko3go/releases/tag/${VERSION}"
