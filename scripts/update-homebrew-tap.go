//go:build ignore

// update-homebrew-tap は Homebrew Tap リポジトリ (kujirahand/homebrew-nadesiko3) の
// Formula/gonako.rb と Casks/gonako-gui.rb を、指定バージョンのリリース成果物に
// 合わせて自動生成する。SHA-256の算出・ファイル書き換え・コミット・プッシュまでを行う。
//
// 使い方:
//
//	go run ./scripts/update-homebrew-tap.go                 現在のバージョンでTapを更新（コミットまでしない）
//	go run ./scripts/update-homebrew-tap.go 3.8.4           バージョンを指定して更新
//	go run ./scripts/update-homebrew-tap.go -commit -push   更新してコミット＆プッシュまで行う
//	go run ./scripts/update-homebrew-tap.go -local          公開前に手元の release/ のZIPから算出する
//	go run ./scripts/update-homebrew-tap.go -check          書き換えず、Tapが最新かどうかだけ検査する（CI用）
//
// SHA-256は既定でGitHub Releasesにアップロード済みのZIPから算出する。実際に
// 配布されるファイルが唯一の正解であり、手元の release/ を再ビルドすると
// ハッシュがずれるためである。-local を付けたときだけ手元のZIPを優先する。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/version"
)

var semverRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// asset はFormula/Caskが参照する配布ZIPの1件を表す。
type asset struct {
	key  string // テンプレート内で参照する名前（darwinArm64 など）
	name string // 配布ファイル名
	sum  string // SHA-256（後から埋める）
}

type config struct {
	version    string
	tapDir     string
	tapRepo    string
	releaseDir string
	repoSlug   string
	local      bool
	wait       time.Duration
	commit     bool
	push       bool
	check      bool
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.version, "version", "", "対象バージョン（省略時は internal/version.Version）")
	flag.StringVar(&cfg.tapDir, "tap", "homebrew-nadesiko3", "Tapリポジトリの作業ディレクトリ")
	flag.StringVar(&cfg.tapRepo, "tap-repo", "https://github.com/kujirahand/homebrew-nadesiko3.git", "Tapリポジトリのclone元（作業ディレクトリが無い場合に使う）")
	flag.StringVar(&cfg.releaseDir, "release-dir", "release", "ローカルのリリース成果物ディレクトリ")
	flag.StringVar(&cfg.repoSlug, "repo", "kujirahand/nadesiko3go", "リリース成果物を配布しているGitHubリポジトリ")
	flag.BoolVar(&cfg.local, "local", false, "GitHub Releasesではなく、手元の release/ にあるZIPからSHA-256を算出する")
	flag.DurationVar(&cfg.wait, "wait", 0, "成果物がまだ公開されていないとき、この時間まで待って再取得する（例: 20m）")
	flag.BoolVar(&cfg.commit, "commit", false, "更新後にTapリポジトリでコミットする")
	flag.BoolVar(&cfg.push, "push", false, "コミット後にプッシュする（-commit も同時に有効化される）")
	flag.BoolVar(&cfg.check, "check", false, "書き換えず、Tapが生成結果と一致するかだけ検査する")
	flag.Parse()

	if cfg.version == "" && flag.NArg() > 0 {
		cfg.version = flag.Arg(0)
	}
	if cfg.version == "" {
		cfg.version = version.Version
	}
	cfg.version = strings.TrimPrefix(cfg.version, "v")
	if !semverRe.MatchString(cfg.version) {
		fmt.Fprintf(os.Stderr, "エラー: バージョン番号の形式が不正です: %q (例: 3.8.4)\n", cfg.version)
		os.Exit(1)
	}
	if cfg.push {
		cfg.commit = true
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	fmt.Printf("===> Homebrew Tap の更新 (バージョン: %s)\n", cfg.version)

	if !cfg.check {
		if err := ensureTapDir(cfg); err != nil {
			return err
		}
	} else if _, err := os.Stat(cfg.tapDir); err != nil {
		return fmt.Errorf("Tapディレクトリがありません: %s", cfg.tapDir)
	}

	assets := assetList(cfg.version)
	for i := range assets {
		sum, from, err := sha256OfAsset(cfg, assets[i].name)
		if err != nil {
			return err
		}
		assets[i].sum = sum
		fmt.Printf("  %s  %s (%s)\n", sum, assets[i].name, from)
	}

	sums := map[string]string{}
	for _, a := range assets {
		sums[a.key] = a.sum
	}

	formula, err := render(formulaTmpl, cfg, sums)
	if err != nil {
		return err
	}
	cask, err := render(caskTmpl, cfg, sums)
	if err != nil {
		return err
	}

	files := map[string]string{
		filepath.Join(cfg.tapDir, "Formula", "gonako.rb"):   formula,
		filepath.Join(cfg.tapDir, "Casks", "gonako-gui.rb"): cask,
	}

	if cfg.check {
		mismatched := false
		for path, want := range files {
			got, err := os.ReadFile(path)
			if err != nil || string(got) != want {
				fmt.Fprintf(os.Stderr, "  [NG] %s が v%s の内容と一致しません\n", path, cfg.version)
				mismatched = true
				continue
			}
			fmt.Printf("  [OK] %s\n", path)
		}
		if mismatched {
			return fmt.Errorf("Tapの内容が古いです。`just homebrew-update` を実行してください")
		}
		fmt.Println("===> Tapは最新です")
		return nil
	}

	changed := false
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		old, err := os.ReadFile(path)
		if err == nil && string(old) == body {
			fmt.Printf("  変更なし: %s\n", path)
			continue
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
		fmt.Printf("  書き込み: %s\n", path)
		changed = true
	}

	if !cfg.commit {
		fmt.Println("===> 完了（コミットは行いません。反映するには -commit -push を付けて実行）")
		return nil
	}
	if !changed {
		fmt.Println("===> 変更がないためコミットしません")
		return nil
	}
	if err := git(cfg.tapDir, "add", "Formula/gonako.rb", "Casks/gonako-gui.rb"); err != nil {
		return err
	}
	msg := fmt.Sprintf("Update gonako/gonako-gui to v%s", cfg.version)
	if err := git(cfg.tapDir, "commit", "-m", msg); err != nil {
		return err
	}
	if cfg.push {
		if err := git(cfg.tapDir, "push"); err != nil {
			return err
		}
		fmt.Println("===> Tapへプッシュしました")
		return nil
	}
	fmt.Println("===> コミットしました（プッシュは未実行）")
	return nil
}

// ensureTapDir はTapの作業ディレクトリを用意する。無ければcloneする。
func ensureTapDir(cfg config) error {
	if st, err := os.Stat(filepath.Join(cfg.tapDir, ".git")); err == nil && st.IsDir() {
		return nil
	}
	if _, err := os.Stat(cfg.tapDir); err == nil {
		// .git が無いただのディレクトリなら、そのまま書き込み先として使う
		return nil
	}
	fmt.Printf("  clone: %s -> %s\n", cfg.tapRepo, cfg.tapDir)
	cmd := exec.Command("git", "clone", cfg.tapRepo, cfg.tapDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// assetList はFormula/Caskが参照する配布ZIPの一覧を作る。
func assetList(ver string) []asset {
	return []asset{
		{key: "cliDarwinArm64", name: fmt.Sprintf("gonako-%s-darwin-arm64.zip", ver)},
		{key: "cliDarwinAmd64", name: fmt.Sprintf("gonako-%s-darwin-amd64.zip", ver)},
		{key: "cliLinuxArm64", name: fmt.Sprintf("gonako-%s-linux-arm64.zip", ver)},
		{key: "cliLinuxAmd64", name: fmt.Sprintf("gonako-%s-linux-amd64.zip", ver)},
		{key: "guiDarwinArm64", name: fmt.Sprintf("gonako-gui-%s-darwin-arm64.app.zip", ver)},
		{key: "guiDarwinAmd64", name: fmt.Sprintf("gonako-gui-%s-darwin-amd64.app.zip", ver)},
	}
}

// sha256OfAsset は成果物のSHA-256を返す。既定ではGitHub Releasesの公開ファイルを
// 使い、-local 指定時だけ手元の release/ 配下のZIPを使う。
func sha256OfAsset(cfg config, name string) (sum string, from string, err error) {
	if cfg.local {
		path := filepath.Join(cfg.releaseDir, name)
		f, err := os.Open(path)
		if err != nil {
			return "", "", fmt.Errorf("手元の成果物がありません: %s (`just release` を実行してください)", path)
		}
		defer f.Close()
		s, err := sha256Reader(f)
		if err != nil {
			return "", "", err
		}
		return s, path, nil
	}

	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", cfg.repoSlug, cfg.version, name)
	// リリース公開直後はアップロードが終わっていないことがあるので、-wait の間は待って再試行する。
	deadline := time.Now().Add(cfg.wait)
	for attempt := 1; ; attempt++ {
		s, status, err := fetchSHA256(url)
		if err == nil {
			return s, "GitHub Releases", nil
		}
		retriable := status == http.StatusNotFound || status == 0 || status >= 500
		if !retriable || time.Now().After(deadline) {
			return "", "", err
		}
		fmt.Printf("  待機中 (%d回目): %s がまだ取得できません\n", attempt, name)
		time.Sleep(30 * time.Second)
	}
}

// fetchSHA256 はURLの内容のSHA-256を返す。第2戻り値はHTTPステータス（通信失敗時は0）。
func fetchSHA256(url string) (string, int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", 0, fmt.Errorf("%s のダウンロードに失敗: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", resp.StatusCode, fmt.Errorf("%s の取得に失敗しました (HTTP %d)。リリースへのアップロードが済んでいるか確認してください", url, resp.StatusCode)
	}
	s, err := sha256Reader(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}
	return s, resp.StatusCode, nil
}

func sha256Reader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func render(tmpl string, cfg config, sums map[string]string) (string, error) {
	t, err := template.New("rb").Parse(tmpl)
	if err != nil {
		return "", err
	}
	data := map[string]any{
		"Version": cfg.version,
		"Repo":    cfg.repoSlug,
		"Sha":     sums,
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func git(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Formula（CLI版）のテンプレート。#{version} はRuby側の展開なので、
// Goのテンプレートでは {{"{{"}} を使わずそのまま書ける。
const formulaTmpl = `class Gonako < Formula
  desc "日本語プログラミング言語 なでしこ3 (Go言語版)"
  homepage "https://github.com/{{.Repo}}"
  version "{{.Version}}"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/{{.Repo}}/releases/download/#{version}/gonako-#{version}-darwin-arm64.zip"
      sha256 "{{.Sha.cliDarwinArm64}}"
    else
      url "https://github.com/{{.Repo}}/releases/download/#{version}/gonako-#{version}-darwin-amd64.zip"
      sha256 "{{.Sha.cliDarwinAmd64}}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/{{.Repo}}/releases/download/#{version}/gonako-#{version}-linux-arm64.zip"
      sha256 "{{.Sha.cliLinuxArm64}}"
    else
      url "https://github.com/{{.Repo}}/releases/download/#{version}/gonako-#{version}-linux-amd64.zip"
      sha256 "{{.Sha.cliLinuxAmd64}}"
    end
  end

  def install
    bin.install "gonako"
  end

  test do
    assert_match "こんにちは", shell_output("#{bin}/gonako -e '「こんにちは」と表示'")
  end
end
`

// Cask（GUI版）のテンプレート。
const caskTmpl = `cask "gonako-gui" do
  version "{{.Version}}"

  if Hardware::CPU.arm?
    url "https://github.com/{{.Repo}}/releases/download/#{version}/gonako-gui-#{version}-darwin-arm64.app.zip"
    sha256 "{{.Sha.guiDarwinArm64}}"
  else
    url "https://github.com/{{.Repo}}/releases/download/#{version}/gonako-gui-#{version}-darwin-amd64.app.zip"
    sha256 "{{.Sha.guiDarwinAmd64}}"
  end

  name "なでしこ3 (gonako-gui)"
  desc "日本語プログラミング言語 なでしこ3 GUIエディタ＆実行環境"
  homepage "https://github.com/{{.Repo}}"

  app "gonako-gui-#{version}-darwin-#{Hardware::CPU.arm? ? "arm64" : "amd64"}.app", target: "gonako-gui.app"

  postflight_steps do
    run "/usr/bin/xattr", args: ["-cr", "{{"{{"}}appdir}}/gonako-gui.app"]
  end

  zap trash: [
    "~/Library/Saved Application State/com.nadesiko3.gonako.gui.savedState",
  ]
end
`
