# リリース作業手順書（配信スクリプトの使い方）

日本語プログラミング言語「なでしこ3」Go言語版（CLI: `gonako`, GUI: `gonako-gui`）の配布パッケージ作成から、GitHub Releases への公開、Homebrew Tap リポジトリ（`kujirahand/homebrew-nadesiko3`）の更新までの一連の手順書です。

---

## 1. 概要とリリース方針

### 1-1. バージョン番号の一元管理
- **バージョン番号の唯一の定義元は `internal/version/version.go` です。**
- リリース時は直接ファイルを編集せず、必ず `just version-update <VERSION>` コマンドを使用します。インストーラーや各ドキュメントの表記が一括で自動同期されます。

### 1-2. コマンド間ラグの防止（アトミック公開）
- GitHub Releases に空のリリースを作った直後に成果物を順次アップロードすると、アップロード完了までの間、ワンライナーインストーラー（`curl ... | bash` / `irm ... | iex`）が 404 Not Found エラーになる問題（Issue #81）がありました。
- 本リポジトリの配信フロー（`just publish`）では、**未公開のドラフト（下書き）状態で全OSの成果物ZIPをアップロードし、すべて揃った瞬間にドラフトを解除して最新版（`latest`）としてアトミックに公開**します。

---

## 2. 前提環境

リリース作業を行う端末には以下が必要です：

- **Go**: 1.21以上（Go 1.26+ 推奨）
- **Just**: タスクランナー（`brew install just` 等）
- **GitHub CLI (`gh`)**: GitHub への認証が完了していること（`gh auth status` で確認）
- **リポジトリの権限**:
  - 本リポジトリ（`kujirahand/nadesiko3go`）への push / Release 作成権限
  - Homebrew Tap リポジトリ（[`kujirahand/homebrew-nadesiko3`](https://github.com/kujirahand/homebrew-nadesiko3)）への push 権限

---

## 3. 標準リリース手順（4ステップ）

手元で以下の順序でコマンドを実行します。

```text
① バージョン更新 & テスト  (just version-update 3.8.4 && just test)
           │
② 手元ビルド確認            (just release)
           │
③ Gitコミット & PRマージ    (masterブランチを最新化)
           │
④ 一括配信 (アトミック公開)  (just publish)
```

### ステップ 1: バージョン番号の一括更新とテスト

バージョン番号を指定して、全関連ファイルを一括更新します。

```bash
# 例: 3.8.4 へ更新
just version-update 3.8.4

# ユニットテストとマニュアルテストの実行
just test
just doctest
```

### ステップ 2: 手元での全プラットフォームビルド確認

全OS・アーキテクチャ向けの配布ZIPが手元で正常にビルドできることを確認します。

```bash
just release
```

`release/` ディレクトリに以下の9ファイルが生成されます：

| ファイル名 | 対象環境 | 形式 |
|---|---|---|
| `gonako-3.8.4-darwin-arm64.zip` | macOS (Apple Silicon) | CLIバイナリ |
| `gonako-3.8.4-darwin-amd64.zip` | macOS (Intel) | CLIバイナリ |
| `gonako-3.8.4-linux-amd64.zip` | Linux (x86_64) | CLIバイナリ |
| `gonako-3.8.4-linux-arm64.zip` | Linux (aarch64) | CLIバイナリ |
| `gonako-3.8.4-windows-amd64.zip` | Windows (x86_64) | CLIバイナリ（`gonako.exe`） |
| `gonako-gui-3.8.4-darwin-arm64.app.zip` | macOS (Apple Silicon) | GUI App Bundle |
| `gonako-gui-3.8.4-darwin-amd64.app.zip` | macOS (Intel) | GUI App Bundle |
| `gonako-gui-3.8.4-windows-amd64.zip` | Windows (x86_64) | GUI exe（`gonako-gui.exe`） |
| `gonako-gui-3.8.4-linux-amd64.zip` | Linux (x86_64) | GUI実行ファイル |

> **Note**: 先に手元ビルドを通すことで、クロスコンパイルエラーや環境要因によるビルド失敗時の不要なロールバックを防ぎます。

### ステップ 3: コミットとプルリクエスト作成・マージ

バージョン更新の変更をコミットし、`master` ブランチへマージします。

```bash
git checkout -b release-v3.8.4
git add -A
git commit -m "chore: release v3.8.4"
git push -u origin release-v3.8.4

# PRを作成してマージ
gh pr create --title "chore: release v3.8.4" --fill
# （GitHub Webまたはgh pr mergeでmasterにマージ）

# ローカルのmasterを最新に更新
git checkout master
git pull origin master
```

### ステップ 4: GitHub Releases作成とHomebrew Tap更新（一括配信）

手元の成果物を一括で配信します。

```bash
just publish
```

引数を省略した場合は `internal/version/version.go` の現在値が使われます（`just publish 3.8.4` のように明示指定も可能）。

#### `just publish` 実行時の内部処理
1. **ドラフトリリース作成**: `gh release create <VERSION> --draft` で未公開の下書きを作成。
2. **成果物のアップロード**: `release/upload-<VERSION>.sh` を実行し、全ZIPをアップロード。
3. **アトミック公開**: `gh release edit <VERSION> --draft=false --latest` で正式公開に切り替え。
4. **Homebrew Tap 更新**: `scripts/update-homebrew-tap.go -local -push` を実行し、手元のZIPから算出した SHA-256 を用いて `Formula/gonako.rb` および `Casks/gonako-gui.rb` を更新・コミット・プッシュ。

---

## 4. 各スクリプトとタスクの詳細

### `just version-update [VERSION]` (`scripts/version-update.go`)
- `internal/version/version.go`（唯一の定義元）
- `scripts/install.sh`（macOS/Linux インストーラーの `DEFAULT_VERSION`）
- `scripts/install.ps1`（Windows インストーラーの `$defaultVersion`）
- `docs/release-homebrew.md`, `docs/release-scripts.md`（ドキュメント内のバージョン表記）
のバージョンを一括同期します。`--check` を付けると、ズレがないか検査します（CI用）。

### `just release` (`scripts/build-release.go`)
- 全プラットフォーム向けの CLI/GUI バイナリをクロスコンパイルして ZIP 圧縮します。
- あわせて `release/upload-<VERSION>.sh` および `release/upload-<VERSION>.bat` を出力します。
- オプション:
  - `just release-cli`: CLI版のみビルド
  - `just release-gui`: GUI版のみビルド

### `just publish` (`scripts/publish-release.sh`)
- GitHub Release のドラフト作成、成果物アップロード、ドラフト解除（アトミック公開）、Homebrew Tap 更新までを一貫して実行します。

### `just homebrew-update` (`scripts/update-homebrew-tap.go`)
- Homebrew Tap のみ再更新したい場合に使用します。
- 例:
  ```bash
  just homebrew-update              # Formula/Caskの生成確認（コミットしない）
  just homebrew-update "-push"      # 生成してコミット＆プッシュ
  just homebrew-check               # Tapが現在のバージョンに追従しているか検査
  ```

---

## 5. トラブルシューティング

### Q. GitHub Release は作成されたが、Homebrew Tap の更新に失敗した
すでに GitHub Release が公開済みであれば、Tap の更新だけをやり直すことができます：

```bash
just homebrew-update "3.8.4 -push"
```

### Q. 手動で成果物をアップロード・公開したい
何らかの理由でスクリプトを使わず手動で作業する場合は、以下の順序で実行してください：

```bash
VERSION="3.8.4"

# 1. ドラフトとして作成（一般ユーザーから見えないようにする）
gh release create "$VERSION" --draft --title "v$VERSION" --notes "Release $VERSION"

# 2. 成果物をアップロード
./release/upload-${VERSION}.sh

# 3. アップロード完了後にドラフトを解除して公開
gh release edit "$VERSION" --draft=false --latest

# 4. Homebrew Tap を更新
go run ./scripts/update-homebrew-tap.go -version "$VERSION" -local -push
```

### Q. ドラフトリリースが残ってしまった
アップロード途中で中断した場合など、ドラフトリリースを削除してやり直すには：

```bash
gh release delete 3.8.4 --yes
```

---

## 6. 関連ドキュメント

- [release-homebrew.md](release-homebrew.md): Homebrew Tap の構造と詳細解説
- [AGENTS.md](../AGENTS.md): プロジェクト設計指針とバージョン管理規則
