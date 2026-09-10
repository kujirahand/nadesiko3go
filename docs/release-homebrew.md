# Homebrew Tap へのリリース・登録手順

日本語プログラミング言語「なでしこ3」Go言語版（CLI: `gonako`, GUI: `gonako-gui`）を、公式Tapリポジトリ `kujirahand/homebrew-nadesiko3` に登録・更新する手順です。

**Homebrewへの配信は自動化されています。**通常は次の3コマンドだけで済みます（詳細は「0. 自動化された配信」）。
手動でやる場合の内訳は1章以降に残してあります。

```bash
just version-update 3.8.3   # バージョン番号を一括更新
just release                # 全プラットフォームの成果物をビルド
just publish                # GitHubリリース作成 → 成果物アップロード → Homebrew Tap更新
```

---

## 0. 自動化された配信

### 0-1. `just publish`（手元から一括で配信する）

`scripts/publish-release.sh` が次の3つを順に行います。

1. `gh release create <VERSION>`（既にタグがあればそのまま使う）
2. `release/upload-<VERSION>.sh` で成果物をアップロード
3. `scripts/update-homebrew-tap.go` でTapの `Formula/gonako.rb` と `Casks/gonako-gui.rb` を
   更新し、コミット＆プッシュ

バージョン番号を省略すると `internal/version/version.go` の値が使われます。
明示するときは `just publish 3.8.3` のように渡します。

### 0-2. `just homebrew-update`（Tapだけ更新する）

すでにGitHub Releasesへアップロード済みで、Tapの更新だけやり直したいときに使います。

```bash
just homebrew-update              # Formula/Caskを生成するだけ（コミットしない）
just homebrew-update "-push"      # 生成してコミット＆プッシュまで行う
just homebrew-update "3.8.3 -push"
just homebrew-check               # Tapが現在のバージョンに追随しているか検査する
```

SHA-256は**GitHub Releasesにアップロード済みのZIPから算出**します。実際に配布される
ファイルだけが正解であり、手元の `release/` を再ビルドするとハッシュがずれるためです。
公開前に手元のZIPから算出したいときだけ `-local` を付けます。

Tapの作業ディレクトリは既定で `./homebrew-nadesiko3`（`.gitignore` 対象）です。
無ければ自動的にcloneします。`-tap <dir>` で変更できます。

### 0-3. GitHub Actions（リリース公開で自動更新）

`.github/workflows/homebrew.yml` が `release: published` で起動し、同じスクリプトで
Tapを更新してプッシュします。手動実行（workflow_dispatch）ではバージョンの指定と、
プッシュせず差分だけ見る `dry_run` が選べます。

- 別リポジトリへプッシュするため、`nadesiko3go` の Secrets に
  **`HOMEBREW_TAP_TOKEN`**（`kujirahand/homebrew-nadesiko3` に対して contents:write を持つPAT）
  を登録しておく必要があります。
- リリース公開直後は成果物のアップロードが終わっていないことがあるため、
  スクリプトは `-wait 20m` で成果物が揃うまで待ってから算出します。

---

## 1. 前提環境

- **Go** 1.21以上（Go 1.26+ 推奨）
- **GitHub CLI** (`gh`)：GitHub へのログイン認証済みであること (`gh auth status`)
- **Tap用リポジトリ**：[`kujirahand/homebrew-nadesiko3`](https://github.com/kujirahand/homebrew-nadesiko3)

---

## 2. リリースバイナリのビルド

まず `just version-update` でバージョン番号を一箇所（`internal/version/version.go`）から
更新します。インストーラースクリプトやREADME、このドキュメントの表記も自動的に揃います。

```bash
just version-update 3.8.3
```

続けて `just release` を実行します（`VERSION` を省略すると更新後の値が使われます）。
CLI版とGUI版の各プラットフォーム用成果物（すべてZIP形式）が `release/` 配下に
一括生成され、あわせて `release/upload-${VERSION}.sh` および `release/upload-${VERSION}.bat`（GitHub Releasesへ
アップロードするスクリプト・バッチ）も生成されます。

```bash
just release
```

### 生成される主な成果物 (`release/`)

| ファイル名 | 対象 | 形式 |
|---|---|---|
| `gonako-3.8.3-darwin-arm64.zip` | macOS (Apple Silicon) | CLIバイナリzip |
| `gonako-3.8.3-darwin-amd64.zip` | macOS (Intel) | CLIバイナリzip |
| `gonako-3.8.3-linux-amd64.zip` | Linux (x86_64) | CLIバイナリzip |
| `gonako-3.8.3-linux-arm64.zip` | Linux (aarch64) | CLIバイナリzip |
| `gonako-3.8.3-windows-amd64.zip` | Windows (x86_64) | CLIバイナリzip（中身は`gonako.exe`） |
| `gonako-gui-3.8.3-darwin-arm64.app.zip` | macOS (Apple Silicon) | GUI App Bundle zip |
| `gonako-gui-3.8.3-darwin-amd64.app.zip` | macOS (Intel) | GUI App Bundle zip |
| `gonako-gui-3.8.3-windows-amd64.zip` | Windows (x86_64) | GUI exe zip |
| `gonako-gui-3.8.3-linux-amd64.zip` | Linux (x86_64) | GUI実行ファイルzip |
| `upload-3.8.3.sh` | - | GitHub Releasesアップロード用スクリプト（macOS/Linux用） |
| `upload-3.8.3.bat` | - | GitHub Releasesアップロード用バッチ（Windows用） |

---

## 3. GitHub Releases へのアップロード

生成した成果物を GitHub Releases の該当タグ（例: `3.8.3`）にアップロードします。
タグ名は `install.sh` / `install.ps1` のダウンロードURLに合わせて
`v` を付けないバージョン番号そのものにします。

```bash
VERSION=3.8.3

# リリースがまだない場合は作成
gh release create "$VERSION" --title "v$VERSION" --notes "Release $VERSION" 2>/dev/null || true

# 成果物をアップロード（just release が生成したスクリプトを使う）
# macOS / Linux:
./release/upload-${VERSION}.sh

# Windows (コマンドプロンプトまたはPowerShell):
.\release\upload-3.8.3.bat
```

---

## 4. SHA-256 チェックサムの確認

Formula および Cask に設定するための SHA-256 ハッシュ値を算出します。

```bash
VERSION=3.8.3
shasum -a 256 \
  "release/gonako-${VERSION}-darwin-arm64.zip" \
  "release/gonako-${VERSION}-darwin-amd64.zip" \
  "release/gonako-${VERSION}-linux-arm64.zip" \
  "release/gonako-${VERSION}-linux-amd64.zip" \
  "release/gonako-gui-${VERSION}-darwin-arm64.app.zip" \
  "release/gonako-gui-${VERSION}-darwin-amd64.app.zip"
```

---

## 5. Homebrew Tap リポジトリの更新（手動でやる場合）

> 通常は `just homebrew-update "-push"` で足ります。以下は中身の説明です。
> Formula / Cask の内容は `scripts/update-homebrew-tap.go` のテンプレートが定義元なので、
> 書式を変えるときはスクリプト側を直してください。

### 5-1. Tap リポジトリを作業ディレクトリに準備

```bash
TAP_DIR=$(mktemp -d)
git clone https://github.com/kujirahand/homebrew-nadesiko3.git "$TAP_DIR"
cd "$TAP_DIR"

mkdir -p Formula Casks
```

### 5-2. CLI版: `Formula/gonako.rb` の作成

```ruby
class Gonako < Formula
  desc "日本語プログラミング言語 なでしこ3 (Go言語版)"
  homepage "https://github.com/kujirahand/nadesiko3go"
  version "3.8.3"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kujirahand/nadesiko3go/releases/download/#{version}/gonako-#{version}-darwin-arm64.zip"
      sha256 "<darwin-arm64.zipのSHA-256>"
    else
      url "https://github.com/kujirahand/nadesiko3go/releases/download/#{version}/gonako-#{version}-darwin-amd64.zip"
      sha256 "<darwin-amd64.zipのSHA-256>"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/kujirahand/nadesiko3go/releases/download/#{version}/gonako-#{version}-linux-arm64.zip"
      sha256 "<linux-arm64.zipのSHA-256>"
    else
      url "https://github.com/kujirahand/nadesiko3go/releases/download/#{version}/gonako-#{version}-linux-amd64.zip"
      sha256 "<linux-amd64.zipのSHA-256>"
    end
  end

  # url が .zip なので Homebrew が自動展開する。展開後の実行ファイル名は "gonako"。
  def install
    bin.install "gonako"
  end

  test do
    assert_match "こんにちは", shell_output("#{bin}/gonako -e '「こんにちは」と表示'")
  end
end
```

### 5-3. GUI版: `Casks/gonako-gui.rb` の作成

```ruby
cask "gonako-gui" do
  version "3.8.3"

  if Hardware::CPU.arm?
    url "https://github.com/kujirahand/nadesiko3go/releases/download/#{version}/gonako-gui-#{version}-darwin-arm64.app.zip"
    sha256 "<darwin-arm64.app.zipのSHA-256>"
  else
    url "https://github.com/kujirahand/nadesiko3go/releases/download/#{version}/gonako-gui-#{version}-darwin-amd64.app.zip"
    sha256 "<darwin-amd64.app.zipのSHA-256>"
  end

  name "なでしこ3 (gonako-gui)"
  desc "日本語プログラミング言語 なでしこ3 GUIエディタ＆実行環境"
  homepage "https://github.com/kujirahand/nadesiko3go"

  app "gonako-gui-#{version}-darwin-#{Hardware::CPU.arm? ? "arm64" : "amd64"}.app", target: "gonako-gui.app"

  # Gatekeeper の隔離属性を自動解除
  postflight_steps do
    run "/usr/bin/xattr", args: ["-cr", "{{appdir}}/gonako-gui.app"]
  end

  zap trash: [
    "~/Library/Saved Application State/com.nadesiko3.gonako.gui.savedState",
  ]
end
```

### 5-4. コミット & プッシュ

```bash
git add Formula/gonako.rb Casks/gonako-gui.rb
git commit -m "Update gonako/gonako-gui to v${VERSION}"
git push origin main
```

---

## 6. インストール・動作確認

Tap リポジトリにプッシュ完了後、端末から以下のコマンドでインストールを検証できます。

```bash
# Tap を登録
brew tap kujirahand/nadesiko3

# CLI版のインストールと確認
brew install gonako
gonako -e '「こんにちは」と表示'

# GUI版のインストール（/Applications/gonako-gui.app に配置されます）
brew install --cask gonako-gui
open /Applications/gonako-gui.app
```

### 更新確認

```bash
brew update
brew upgrade gonako
brew upgrade --cask gonako-gui
```
