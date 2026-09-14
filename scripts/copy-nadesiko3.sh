#!/bin/sh
# 本家なでしこ3のブラウザ版（wnako3.js とプラグイン）を gonako-gui に取り込む（#63）。
#
# 使い方: ./scripts/copy-nadesiko3.sh [本家リポジトリのパス]
#
# 1. ローカルの本家リポジトリ（既定は ./nadesiko3）に release/*.js があればそれを使う
# 2. なければ jsDelivr から nadesiko3@<Nadesiko> の release/*.js を取得する
#    その版がnpmに無ければ nadesiko3@latest を使う
#
# 取り込んだ版と取得元は VERSION に記録し、gonako-gui が getAppInfo で返す。
set -eu

repo_dir=${1:-./nadesiko3}
target_dir=./cmd/gonako-gui/ui/wnako3
go=${GO:-go}
cdn_base=https://cdn.jsdelivr.net/npm
api_base=https://data.jsdelivr.com/v1/packages/npm

# gonakoが名乗るなでしこ言語バージョン（internal/version.Nadesiko）
wanted=$("$go" run ./scripts/print-version.go -nadesiko)

work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT

if ls "$repo_dir"/release/*.js >/dev/null 2>&1; then
  source=local
  version=$(sed -n 's/^[[:space:]]*"version":[[:space:]]*"\([^"]*\)".*/\1/p' "$repo_dir/package.json" | head -n 1)
  cp "$repo_dir"/release/*.js "$work_dir/"
  [ -f "$repo_dir/LICENSE" ] && cp "$repo_dir/LICENSE" "$work_dir/LICENSE"
  if [ "$version" != "$wanted" ]; then
    echo "[警告] ローカルの本家は $version ですが、gonakoのなでしこ言語バージョンは $wanted です" >&2
  fi
else
  source=cdn
  version=$wanted
  if ! listing=$(curl -sfL "$api_base/nadesiko3@$version?structure=flat"); then
    latest=$(curl -sfL "$api_base/nadesiko3/resolved?specifier=latest" |
      sed -n 's/.*"version":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
    if [ -z "$latest" ]; then
      echo "nadesiko3の最新版を解決できません" >&2
      exit 1
    fi
    echo "[警告] nadesiko3@$version はnpmにありません。最新版 $latest を使います" >&2
    version=$latest
    listing=$(curl -sfL "$api_base/nadesiko3@$version?structure=flat")
  fi
  files=$(printf '%s\n' "$listing" | grep -o '"name":[[:space:]]*"/release/[^"/]*\.js"' |
    sed 's/.*"\/release\/\([^"]*\)"/\1/')
  if [ -z "$files" ]; then
    echo "nadesiko3@$version に release/*.js が見つかりません" >&2
    exit 1
  fi
  for file in $files; do
    echo "取得: $cdn_base/nadesiko3@$version/release/$file"
    curl -sfL -o "$work_dir/$file" "$cdn_base/nadesiko3@$version/release/$file"
  done
  curl -sfL -o "$work_dir/LICENSE" "$cdn_base/nadesiko3@$version/LICENSE" || true
fi

{
  echo "version=$version"
  echo "source=$source"
} > "$work_dir/VERSION"

# 本家で削除されたファイルを残さないよう、コピー先を丸ごと入れ替える
rm -rf "$target_dir"
mkdir -p "$target_dir"
cp "$work_dir"/* "$target_dir/"

echo "nadesiko3 $version ($source) を $target_dir に取り込みました"
