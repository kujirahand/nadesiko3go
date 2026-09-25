go := env_var_or_default("GO", "go")
# macOSだけWebKitの重複ライブラリ警告を抑制する
set export
CGO_LDFLAGS := if os() == "macos" { "-Wl,-no_warn_duplicate_libraries" } else { "" }
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
    {{go}} build -o bin/gonako-gui ./cmd/gonako-gui

# ブラウザ向けWebAssembly版（コア機能のみ）を bin/wasm にビルド
# gonako.wasm と、Go付属の wasm_exec.js、サンプルページ index.html を出力する
wasm:
    mkdir -p bin/wasm
    GOOS=js GOARCH=wasm {{go}} build -trimpath -ldflags="-s -w" -o bin/wasm/gonako.wasm ./cmd/gonako-wasm
    cp "$({{go}} env GOROOT)/lib/wasm/wasm_exec.js" bin/wasm/
    cp cmd/gonako-wasm/web/index.html bin/wasm/
    gzip -9 -k -f bin/wasm/gonako.wasm
    command -v brotli >/dev/null 2>&1 && brotli -q 11 -f bin/wasm/gonako.wasm -o bin/wasm/gonako.wasm.br || true

# WebAssembly版をNode.jsで動かして確かめる
wasm-test: wasm
    node scripts/wasm-smoke.mjs bin/wasm

# WebAssembly版のサンプルページをローカルで開く（http://localhost:8080/）
wasm-serve: wasm
    python3 -m http.server 8080 -d bin/wasm

# gonako / gonako-cui を $GOPATH/bin にインストール
install:
    {{go}} install ./cmd/gonako
    {{go}} install ./cmd/gonako-cui

# release/ を空にする（リリース作成の最初に実行する）
release-clean:
    rm -rf release
    mkdir -p release

# 配布用に現在のOS向けのバイナリ・ツール（CLI・GUI）を作る
# 実行中のOSを自動判定し、release-darwin / release-windows / release-linux の
# 該当するものだけを実行する。先に release/ を空にしてから作り直す。
release version=version: release-clean
    #!/usr/bin/env bash
    set -euo pipefail
    # 子の just 呼び出しにバージョン指定を引き継ぐ
    export VERSION="{{version}}"
    case "{{os()}}" in
      macos) target=release-darwin ;;
      linux) target=release-linux ;;
      windows) target=release-windows ;;
      *) echo "未対応のOSです: {{os()}}" >&2; exit 1 ;;
    esac
    echo "===> $target を実行します"
    just "$target"

# macOS向けの配布用バイナリ（gonako・gonako-gui）を作る
release-darwin:
    {{go}} run ./scripts/build-release.go -version "{{version}}" -platforms "darwin/arm64 darwin/amd64"

# Windows向けの配布用バイナリ（gonako・gonako-gui）を作る
release-windows:
    {{go}} run ./scripts/build-release.go -version "{{version}}" -platforms "windows/amd64"

# Linux向けの配布用バイナリ（gonako・gonako-gui）を作る
release-linux:
    {{go}} run ./scripts/build-release.go -version "{{version}}" -platforms "linux/amd64 linux/arm64"

# release/ にある成果物をGitHubリリースへアップロードする
# リリースが無ければドラフトとして自動作成し、同名ファイルは上書きする
release-upload version=version:
    #!/usr/bin/env bash
    set -euo pipefail
    version="{{version}}"
    if [ -z "$version" ]; then
      version=$({{go}} run ./scripts/print-version.go)
    fi
    version="${version#v}"
    if ! command -v gh >/dev/null 2>&1; then
      echo "エラー: GitHub CLI (gh) が必要です" >&2
      exit 1
    fi
    shopt -s nullglob
    files=(release/*.zip)
    if [ "${#files[@]}" -eq 0 ]; then
      echo "エラー: release/*.zip がありません。先に just release-darwin などを実行してください" >&2
      exit 1
    fi
    if ! gh release view "$version" >/dev/null 2>&1; then
      echo "--- リリース ${version} をドラフトとして作成します"
      gh release create "$version" --draft --title "v${version}" --notes "Release ${version}"
    fi
    echo "--- ${#files[@]} 件の成果物をアップロードします"
    gh release upload "$version" "${files[@]}" --clobber
    echo "===> アップロード完了: v${version}（正式公開は gh release edit ${version} --draft=false --latest）"

# CLI版のみ配布用バイナリを作る
release-cli:
    {{go}} run ./scripts/build-release.go -version "{{version}}" -platforms "{{platforms}}" -skip-gui

# GUI版のみ配布用バイナリを作る
release-gui:
    {{go}} run ./scripts/build-release.go -version "{{version}}" -platforms "{{platforms}}" -skip-cli

# GUI版をビルドせずに実行
run-gui:
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

# Go言語コードの行数・文字数・ファイル数を集計・分析する
analyze-gocode *args:
    {{go}} run ./scripts/analyze-gocode.go {{args}}

# マニュアルと固定サンプルを実行して、書かれている表示結果と一致するか確かめる
# 対象を絞るときは: just doctest testdata/doctest/core/plugin_system.txt
doctest *args:
    @if [ ! -d manual ]; then echo '[警告] manualフォルダ(nadesiko3doc/data)を./manualとしてシンボリックリンクを作成してください'; fi
    {{go}} run ./cmd/gonako doctest {{args}}

# 本家(nadesiko3)から差分fixtureをGo側へ同期する
sync-compat:
    ./scripts/sync-compat-fixtures.sh

# 本家のブラウザ版(wnako3.jsとプラグイン)をgonako-guiへ取り込む
# ./nadesiko3/release が無ければjsDelivrから取得する
copy-nadesiko3 *args:
    ./scripts/copy-nadesiko3.sh {{args}}

# 差分fixtureの全ケースを実行してout/へ出力する
compat-run:
    {{go}} run ./cmd/gonako compat run --cases ./testdata/compat/cases --out ./out

# 本家のoracleと照合して通過率を出す
compat-check:
    cd nadesiko3 && npm run compat:check -- ../out

# command-list.jsonを生成する（GUIエディタの色分け用）
gen-command-list:
    {{go}} run ./scripts/gen-command-list.go

# wnako(本家ブラウザ版)の命令一覧を生成する（GUIエディタの[wnako]表示用 / #101）
# 入力は just copy-nadesiko3 が取り込む ui/wnako3/command.json.js
gen-wnako-command-list:
    {{go}} run ./scripts/gen-wnako-command-list.go

# バージョン番号を一括更新する（例: just version-update 3.8.2）
# 引数なしなら internal/version/version.go の現在値へ他ファイルを再同期する
version-update *args:
    {{go}} run ./scripts/version-update.go {{args}}

# Homebrew Tapを更新する（公開済みリリースのSHA-256からFormula/Caskを生成）
# 例: just homebrew-update 3.8.4 / just homebrew-update "3.8.4 -push"
homebrew-update *args:
    {{go}} run ./scripts/update-homebrew-tap.go {{args}}

# Homebrew Tapが現在のバージョンに追随しているか検査する
homebrew-check:
    {{go}} run ./scripts/update-homebrew-tap.go -check

# GitHubリリースの作成・成果物のアップロード・Homebrew Tapの更新まで一括で行う
# 事前に `just version-update X.Y.Z` と `just release` を済ませておくこと
publish version=version:
    ./scripts/publish-release.sh "{{version}}"
