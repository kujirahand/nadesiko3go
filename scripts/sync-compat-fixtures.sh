#!/bin/sh
set -eu

repo_dir=${1:-./nadesiko3}
source_dir="$repo_dir/core/test/fixtures/compat"
target_dir=./testdata/compat

if [ ! -d "$source_dir/cases" ] || [ ! -d "$source_dir/expected" ]; then
  echo "fixtureが見つかりません: $source_dir" >&2
  exit 1
fi

mkdir -p "$target_dir/cases" "$target_dir/expected"

# 本家で削除されたJSONをGo側に残さない。対象は生成コピー先のJSONだけに限定する。
for target_file in "$target_dir/cases/"*.json; do
  [ -e "$target_file" ] || break
  [ -f "$source_dir/cases/$(basename "$target_file")" ] || rm -f "$target_file"
done
for target_file in "$target_dir/expected/"*.json; do
  [ -e "$target_file" ] || break
  [ -f "$source_dir/expected/$(basename "$target_file")" ] || rm -f "$target_file"
done

cp "$source_dir"/cases/*.json "$target_dir/cases/"
cp "$source_dir"/expected/*.json "$target_dir/expected/"
cp "$source_dir/SPEC.md" "$source_dir/README.md" "$target_dir/"

commit=$(git -C "$repo_dir" rev-parse HEAD)
source_file="$target_dir/SOURCE.tmp"
{
  echo "repository=nadesiko3"
  echo "commit=$commit"
  echo "fixture_path=core/test/fixtures/compat"
} > "$source_file"
mv "$source_file" "$target_dir/SOURCE"

echo "compat fixtureを同期しました: $commit"
