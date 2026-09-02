# Homebrew Tap へのリリース・登録手順

日本語プログラミング言語「なでしこ3」Go言語版（CLI: `gonako`, GUI: `gonako-gui`）を、公式Tapリポジトリ `kujirahand/homebrew-nadesiko3` に登録・更新する手順です。

---

## 1. 前提環境

- **Go** 1.21以上（Go 1.26+ 推奨）
- **GitHub CLI** (`gh`)：GitHub へのログイン認証済みであること (`gh auth status`)
- **Tap用リポジトリ**：[`kujirahand/homebrew-nadesiko3`](https://github.com/kujirahand/homebrew-nadesiko3)

---

## 2. リリースバイナリのビルド

バージョン番号（例: `3.8.1`）を指定して `make release` を実行します。CLI版とGUI版の各プラットフォーム用成果物が `bin/` 配下に一括生成されます。

```bash
make release VERSION=3.8.1
```

### 生成される主な成果物 (`bin/`)

| ファイル名 | 対象 | 形式 |
|---|---|---|
| `gonako-3.8.1-darwin-arm64` | macOS (Apple Silicon) | CLIバイナリ |
| `gonako-3.8.1-darwin-amd64` | macOS (Intel) | CLIバイナリ |
| `gonako-3.8.1-linux-amd64` | Linux (x86_64) | CLIバイナリ |
| `gonako-3.8.1-linux-arm64` | Linux (aarch64) | CLIバイナリ |
| `gonako-3.8.1-windows-amd64.exe` | Windows (x86_64) | CLIバイナリ |
| `gonako-gui-3.8.1-darwin-arm64.app.zip` | macOS (Apple Silicon) | GUI App Bundle zip |
| `gonako-gui-3.8.1-darwin-amd64.app.zip` | macOS (Intel) | GUI App Bundle zip |
| `gonako-gui-3.8.1-windows-amd64.zip` | Windows (x86_64) | GUI exe zip |

---

## 3. GitHub Releases へのアップロード

生成した成果物を GitHub Releases の該当タグ（例: `3.8.1`）にアップロードします。

```bash
VERSION=3.8.1

# リリースがまだない場合は作成
gh release create "$VERSION" --title "v$VERSION" --notes "Release $VERSION" 2>/dev/null || true

# 成果物をアップロード
gh release upload "$VERSION" \
  "bin/gonako-${VERSION}-darwin-arm64" \
  "bin/gonako-${VERSION}-darwin-amd64" \
  "bin/gonako-${VERSION}-linux-amd64" \
  "bin/gonako-${VERSION}-linux-arm64" \
  "bin/gonako-${VERSION}-windows-amd64.exe" \
  "bin/gonako-gui-${VERSION}-darwin-arm64.app.zip" \
  "bin/gonako-gui-${VERSION}-darwin-amd64.app.zip" \
  "bin/gonako-gui-${VERSION}-windows-amd64.zip" \
  --clobber
```

---

## 4. SHA-256 チェックサムの確認

Formula および Cask に設定するための SHA-256 ハッシュ値を算出します。

```bash
VERSION=3.8.1
shasum -a 256 \
  "bin/gonako-${VERSION}-darwin-arm64" \
  "bin/gonako-${VERSION}-darwin-amd64" \
  "bin/gonako-${VERSION}-linux-arm64" \
  "bin/gonako-${VERSION}-linux-amd64" \
  "bin/gonako-gui-${VERSION}-darwin-arm64.app.zip" \
  "bin/gonako-gui-${VERSION}-darwin-amd64.app.zip"
```

---

## 5. Homebrew Tap リポジトリの更新

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
  version "3.8.1"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kujirahand/nadesiko3go/releases/download/#{version}/gonako-#{version}-darwin-arm64"
      sha256 "<darwin-arm64のSHA-256>"
    else
      url "https://github.com/kujirahand/nadesiko3go/releases/download/#{version}/gonako-#{version}-darwin-amd64"
      sha256 "<darwin-amd64のSHA-256>"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/kujirahand/nadesiko3go/releases/download/#{version}/gonako-#{version}-linux-arm64"
      sha256 "<linux-arm64のSHA-256>"
    else
      url "https://github.com/kujirahand/nadesiko3go/releases/download/#{version}/gonako-#{version}-linux-amd64"
      sha256 "<linux-amd64のSHA-256>"
    end
  end

  def install
    cpu = Hardware::CPU.arm? ? "arm64" : "amd64"
    os = OS.mac? ? "darwin" : "linux"
    bin.install "gonako-#{version}-#{os}-#{cpu}" => "gonako"
  end

  test do
    assert_match "こんにちは", shell_output("#{bin}/gonako -e '「こんにちは」と表示'")
  end
end
```

### 5-3. GUI版: `Casks/gonako-gui.rb` の作成

```ruby
cask "gonako-gui" do
  version "3.8.1"

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

  app "gonako-gui-#{version}-darwin-#{Hardware::CPU.arm? ? "arm64" : "amd64"}.app", target: "なでしこ3.app"

  # Gatekeeper の隔離属性を自動解除
  postflight do
    system_command "/usr/bin/xattr",
                   args: ["-cr", "#{appdir}/なでしこ3.app"]
  end

  zap trash: [
    "~/Library/Saved Application State/com.nadesiko3.gonako.gui.savedState",
  ]
end
```

### 5-4. コミット & プッシュ

```bash
git add Formula/gonako.rb Casks/gonako-gui.rb
git commit -m "Release gonako and gonako-gui v${VERSION}"
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

# GUI版のインストール（/Applications/なでしこ3.app に配置されます）
brew install --cask gonako-gui
open /Applications/なでしこ3.app
```

### 更新確認

```bash
brew update
brew upgrade gonako
brew upgrade --cask gonako-gui
```
