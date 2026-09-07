go := env_var_or_default("GO", "go")
# 空のままなら internal/version.Version（唯一の定義元）を build-release.go 側で使う
version := env_var_or_default("VERSION", "")
platforms := env_var_or_default("PLATFORMS", "darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64")

# 既定タスク: 一覧を表示
default:
    @just --list

# gonako / gonako-gui をすべてビルド
build: cmd gui

# gonako本体をビルド
cmd:
    {{go}} build -o bin/gonako ./cmd/gonako

# 軽量CUI版をビルド(用途が限られる割にそこそこサイズがあるので配布しない)
cui:
    {{go}} build -o bin/gonako-cui ./cmd/gonako-cui

# GUI版をビルド
# macOSはwebview_goのWebKitフレームワークとcgoランタイムがどちらも-lobjcをリンクするため、
# Xcode 15以降のldが重複リンク警告を出す。実害はないが -no_warn_duplicate_libraries で抑制する。
gui:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ "{{os()}}" = "macos" ]; then
        export CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries"
    fi
    set -x
    {{go}} build -o bin/gonako-gui ./cmd/gonako-gui

# gonako / gonako-cui を $GOPATH/bin にインストール
install:
    {{go}} install ./cmd/gonako
    {{go}} install ./cmd/gonako-cui

# 配布用に各プラットフォーム向けのバイナリ・ツール（CLI・GUI）を作る
release:
    {{go}} run ./scripts/build-release.go -version "{{version}}" -platforms "{{platforms}}"

# CLI版のみ配布用バイナリを作る
release-cli:
    {{go}} run ./scripts/build-release.go -version "{{version}}" -platforms "{{platforms}}" -skip-gui

# GUI版のみ配布用バイナリを作る
release-gui:
    {{go}} run ./scripts/build-release.go -version "{{version}}" -platforms "{{platforms}}" -skip-cli

# GUI版をビルドせずに実行
run-gui:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ "{{os()}}" = "macos" ]; then
        export CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries"
    fi
    set -x
    {{go}} run ./cmd/gonako-gui

# ビルド成果物を削除
clean:
    rm -rf bin out benchmark/build

# ベンチマークを実行
benchmark: cmd
    {{go}} run ./benchmark/runner.go

# テストを実行
test:
    {{go}} test ./...

# マニュアルと固定サンプルを実行して、書かれている表示結果と一致するか確かめる
# 対象を絞るときは: just doctest testdata/doctest/core/plugin_system.txt
doctest *args:
    {{go}} run ./cmd/gonako doctest {{args}}

# 本家(nadesiko3)から差分fixtureをGo側へ同期する
sync-compat:
    ./scripts/sync-compat-fixtures.sh

# 差分fixtureの全ケースを実行してout/へ出力する
compat-run:
    {{go}} run ./cmd/gonako compat run --cases ./testdata/compat/cases --out ./out

# 本家のoracleと照合して通過率を出す
compat-check:
    cd nadesiko3 && npm run compat:check -- ../out

# command-list.jsonを生成する（GUIエディタの色分け用）
gen-command-list:
    {{go}} run ./scripts/gen-command-list.go

# バージョン番号を一括更新する（例: just version-update 3.8.2）
# 引数なしなら internal/version/version.go の現在値へ他ファイルを再同期する
version-update *args:
    {{go}} run ./scripts/version-update.go {{args}}
