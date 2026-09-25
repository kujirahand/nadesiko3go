# リリース作業手順書（配信スクリプトの使い方）

日本語プログラミング言語「なでしこ3」Go言語版（CLI: `gonako`, GUI: `gonako-gui`）の配布パッケージ作成から、GitHub Releases への公開、Homebrew Tap リポジトリ（`kujirahand/homebrew-nadesiko3`）の更新までの一連の手順書です。

---

## 1. 概要とリリース方針

### 1-1. バージョン番号の一元管理
- **バージョン番号の唯一の定義元は `internal/version/version.go` です。**
- リリース時は直接ファイルを編集せず、必ず `just version-update <VERSION>` コマンドを使用します。各ドキュメントの表記が一括で自動同期されます。
- インストーラー（`scripts/install.sh` / `scripts/install.ps1`）のフォールバック版は「公開済みの安定版」を表すため、`version-update` では変更せず、`just publish` がRelease公開後に切り替えます。公開前に切り替えると、最新版APIの取得に失敗したときに未公開版を取りに行って404になるためです。

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
① バージョン更新 & テスト  (just version-update 3.8.7 && just test)
           │
② 手元ビルド確認            (just release / release-<OS>)
           │
③ Gitコミット & PRマージ    (masterブランチを最新化)
           │
④ 一括配信 (アトミック公開)  (just publish)
```

### ステップ 1: バージョン番号の一括更新とテスト

バージョン番号を指定して、全関連ファイルを一括更新します。

```bash
# 例: 3.8.7 へ更新
just version-update 3.8.7

# ユニットテストとマニュアルテストの実行
just test
just doctest
```

### ステップ 2: 手元でのOS別ビルド確認

全OSを一度にビルドするとGUIのクロスコンパイルなどで途中失敗しやすいため（Issue #150）、
リリース成果物はOSごとに分けて作成します。`just release` は実行中のOSを自動判定し、
そのOS向けだけをビルドします（実行前に `release/` を空にします）。

```bash
# 実行中のOS向けだけをビルド（macOSならdarwinのみ）
just release
```

OSを明示するときは以下のレシピを使います。1台で複数OS分をまとめて作りたいときは、
これらを続けて実行すると `release/` に成果物が積み上がります。

```bash
# 出力先を空にしてから始める（任意。積み上げたいときは実行しない）
just release-clean
just release-darwin
just release-windows
just release-linux
```

| レシピ | 対象 | 生成物 |
|---|---|---|
| `just release-darwin` | macOS (arm64/amd64) | CLIバイナリ + GUI App Bundle |
| `just release-windows` | Windows (amd64) | CLIバイナリ + GUI exe |
| `just release-linux` | Linux (amd64/arm64) | CLIバイナリ + GUI実行ファイル |

`release/` ディレクトリに生成される主な成果物：

| ファイル名 | 対象環境 | 形式 |
|---|---|---|
| `gonako-3.8.7-darwin-arm64.zip` | macOS (Apple Silicon) | CLIバイナリ |
| `gonako-3.8.7-darwin-amd64.zip` | macOS (Intel) | CLIバイナリ |
| `gonako-3.8.7-linux-amd64.zip` | Linux (x86_64) | CLIバイナリ |
| `gonako-3.8.7-linux-arm64.zip` | Linux (aarch64) | CLIバイナリ |
| `gonako-3.8.7-windows-amd64.zip` | Windows (x86_64) | CLIバイナリ（`gonako.exe`） |
| `gonako-gui-3.8.7-darwin-arm64.app.zip` | macOS (Apple Silicon) | GUI App Bundle |
| `gonako-gui-3.8.7-darwin-amd64.app.zip` | macOS (Intel) | GUI App Bundle |
| `gonako-gui-3.8.7-windows-amd64.zip` | Windows (x86_64) | GUI exe（`gonako-gui.exe`） |
| `gonako-gui-3.8.7-linux-amd64.zip` | Linux (x86_64) | GUI実行ファイル |

> **Note**: 先に手元ビルドを通すことで、クロスコンパイルエラーや環境要因によるビルド失敗時の不要なロールバックを防ぎます。

### ステップ 2-2: 成果物をGitHub Releasesへアップロード（任意）

`just release-upload` は `release/*.zip` をGitHub Releasesへアップロードします。
リリースがまだ無ければドラフトとして自動作成し、同名ファイルは上書き（`--clobber`）します。
`release/` 内のZIPのファイル名にタグのバージョンが含まれない場合（別バージョンの残骸など）は、
アップロード前にエラーで止まります。バージョン文字列は形式も検証します。

```bash
just release-upload        # バージョンは internal/version/version.go から
just release-upload 3.8.7  # バージョンを明示
```

OSごとに別マシンでビルドする場合は、各マシンで `just release-<OS>` の後に
`just release-upload` を実行すると、全OSの成果物が1つのリリースに集まります。
（正式公開は `just publish`、または `gh release edit <VERSION> --draft=false --latest`）

### ステップ 3: コミットとプルリクエスト作成・マージ

バージョン更新の変更をコミットし、`master` ブランチへマージします。

```bash
git checkout -b release-v3.8.7
git add -A
git commit -m "chore: release v3.8.7"
git push -u origin release-v3.8.7

# PRを作成してマージ
gh pr create --title "chore: release v3.8.7" --fill
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

引数を省略した場合は `internal/version/version.go` の現在値が使われます（`just publish 3.8.7` のように明示指定も可能）。

> **Note**: `just publish` は `release/` にあるローカルZIPをアップロードし、そのZIPから
> HomebrewのSHA-256を算出します（`-local`）。そのため実行前に全OS分の成果物を `release/` に
> 集めておいてください（`just release-darwin` / `release-windows` / `release-linux` を実行）。
> OSごとに別マシンで作る場合は、各マシンで `just release-<OS>` → `just release-upload` を実行した
> うえで、`gh release edit <VERSION> --draft=false --latest` と
> `just homebrew-update "<VERSION> -push"`（`-local` なし）で公開・Tap更新します。

#### `just publish` 実行時の内部処理
0. **成果物の事前検証**: Homebrewが必要とする6つのZIP（CLIのdarwin/linux各arm64・amd64、GUIのdarwin各arm64・amd64）が `release/` に揃っているか確認。不足があれば何もアップロード・公開せずに終了（一部のOSだけの不完全なリリースを公開しないため）。
1. **ドラフトリリース作成**: `gh release create <VERSION> --draft` で未公開の下書きを作成。
2. **成果物のアップロード**: `release/upload-<VERSION>.sh` を実行し、全ZIPをアップロード。
3. **アトミック公開**: `gh release edit <VERSION> --draft=false --latest` で正式公開に切り替え。
4. **Homebrew Tap 更新**: `scripts/update-homebrew-tap.go -local -push` を実行し、手元のZIPから算出した SHA-256 を用いて `Formula/gonako.rb` および `Casks/gonako-gui.rb` を更新・コミット・プッシュ。
5. **インストーラーのフォールバック版切り替え**: `scripts/version-update.go --stable <VERSION>` で `scripts/install.sh` / `scripts/install.ps1` を公開済みの版へ更新。変更はコミットしてmasterへ反映してください。

---

## 4. 各スクリプトとタスクの詳細

### `just version-update [VERSION]` (`scripts/version-update.go`)
- `internal/version/version.go`（唯一の定義元）
- `docs/release-homebrew.md`, `docs/release-scripts.md`（ドキュメント内のバージョン表記）
のバージョンを一括同期します。`--check` を付けると、ズレがないか検査します（CI用）。
- インストーラーのフォールバック版（`install.sh` の `DEFAULT_VERSION` / `install.ps1` の `$defaultVersion`）は同期対象外です。`--stable <VERSION>` を付けたときだけ、この2箇所を切り替えます（`just publish` が公開後に実行）。

### `just release` (`scripts/build-release.go`)
- Linux向けGUIは、ホストとアーキテクチャが違う場合（amd64ホストでarm64をビルドする等）はクロスCコンパイラ（`aarch64-linux-gnu-gcc` 等）が必要です。無ければそのGUIはスキップされます。
- `just release` は実行中のOSを自動判定し、そのOS向けの CLI/GUI バイナリをクロスコンパイルして ZIP 圧縮します。実行前に `release/` を空にします。
- OSを明示する場合は `just release-darwin` / `just release-windows` / `just release-linux` を使います。
- あわせて `release/upload-<VERSION>.sh` および `release/upload-<VERSION>.bat` を出力します。
- アップロードは `just release-upload`（リリースが無ければドラフトを自動作成、同名ファイルは上書き）。
- オプション:
  - `just release-cli`: CLI版のみビルド
  - `just release-gui`: GUI版のみビルド
  - `just release-clean`: `release/` を空にする

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
just homebrew-update "3.8.7 -push"
```

### Q. 手動で成果物をアップロード・公開したい
何らかの理由でスクリプトを使わず手動で作業する場合は、以下の順序で実行してください：

```bash
VERSION="3.8.7"

# 1. ドラフトとして作成（一般ユーザーから見えないようにする）
gh release create "$VERSION" --draft --title "v$VERSION" --notes "Release $VERSION"

# 2. 成果物をアップロード
./release/upload-${VERSION}.sh

# 3. アップロード完了後にドラフトを解除して公開
gh release edit "$VERSION" --draft=false --latest

# 4. Homebrew Tap を更新
go run ./scripts/update-homebrew-tap.go -version "$VERSION" -local -push

# 5. インストーラーのフォールバック版を公開済みの版へ切り替えてコミット
go run ./scripts/version-update.go --stable "$VERSION"
```

### Q. ドラフトリリースが残ってしまった
アップロード途中で中断した場合など、ドラフトリリースを削除してやり直すには：

```bash
gh release delete 3.8.7 --yes
```

---

## 6. 関連ドキュメント

- [release-homebrew.md](release-homebrew.md): Homebrew Tap の構造と詳細解説
- [AGENTS.md](../AGENTS.md): プロジェクト設計指針とバージョン管理規則
