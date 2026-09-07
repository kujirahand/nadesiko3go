//go:build ignore

package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/kujirahand/nadesiko3go/internal/version"
)

type config struct {
	version   string
	platforms []string
	outDir    string
	skipCLI   bool
	skipGUI   bool
}

func main() {
	// バージョン番号の既定値: VERSION環境変数 > internal/version.Version の順で決める。
	// internal/version がリリース版番号の定義元であり、`just version-update` で更新する。
	defaultVersion := os.Getenv("VERSION")
	if defaultVersion == "" {
		defaultVersion = version.Version
	}

	defaultPlatforms := os.Getenv("PLATFORMS")
	if defaultPlatforms == "" {
		defaultPlatforms = "darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64"
	}

	verFlag := flag.String("version", defaultVersion, "Release version")
	platFlag := flag.String("platforms", defaultPlatforms, "Space-separated list of target platforms (os/arch)")
	outDirFlag := flag.String("outdir", "release", "Output directory for release archives")
	skipCLIFlag := flag.Bool("skip-cli", false, "Skip CLI (gonako) build")
	skipGUIFlag := flag.Bool("skip-gui", false, "Skip GUI (gonako-gui) build")
	flag.Parse()

	// justfile側が空文字を明示的に渡した場合（VERSION未設定時）もdefaultVersionへ戻す。
	if *verFlag == "" {
		*verFlag = defaultVersion
	}

	cfg := config{
		version:   *verFlag,
		platforms: strings.Fields(*platFlag),
		outDir:    *outDirFlag,
		skipCLI:   *skipCLIFlag,
		skipGUI:   *skipGUIFlag,
	}

	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "出力ディレクトリの作成に失敗しました: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("===> なでしこ3 リリースビルド (バージョン: %s)\n", cfg.version)

	var hasError bool

	// 1. gonako (CLI) ビルド
	if !cfg.skipCLI {
		fmt.Println("\n--- [1/2] CLI版 (gonako) のビルド ---")
		for _, p := range cfg.platforms {
			parts := strings.SplitN(p, "/", 2)
			if len(parts) != 2 {
				continue
			}
			goos, goarch := parts[0], parts[1]
			if err := buildCLI(cfg, goos, goarch); err != nil {
				fmt.Fprintf(os.Stderr, "  [ERROR] gonako (%s/%s) のビルドに失敗: %v\n", goos, goarch, err)
				hasError = true
			}
		}
	}

	// 2. gonako-gui (GUI) ビルド
	if !cfg.skipGUI {
		fmt.Println("\n--- [2/2] GUI版 (gonako-gui) のビルド ---")
		for _, p := range cfg.platforms {
			parts := strings.SplitN(p, "/", 2)
			if len(parts) != 2 {
				continue
			}
			goos, goarch := parts[0], parts[1]
			if err := buildGUI(cfg, goos, goarch); err != nil {
				fmt.Fprintf(os.Stderr, "  [ERROR] gonako-gui (%s/%s) のビルドに失敗: %v\n", goos, goarch, err)
				hasError = true
			}
		}
	}

	// 3. アップロード用スクリプトの生成（gh コマンドでZIPをリリースへ追加する）
	if err := writeUploadScript(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "  [ERROR] アップロードスクリプトの生成に失敗: %v\n", err)
		hasError = true
	}

	fmt.Println("\n===> リリースビルド完了")
	if hasError {
		os.Exit(1)
	}
}

// writeUploadScript は release/*.zip を gh コマンドで既存のGitHubリリースへ
// アップロードするスクリプト release/upload-{version}.sh を生成する。
func writeUploadScript(cfg config) error {
	zips, err := filepath.Glob(filepath.Join(cfg.outDir, "*.zip"))
	if err != nil {
		return err
	}

	scriptName := fmt.Sprintf("upload-%s.sh", cfg.version)
	scriptPath := filepath.Join(cfg.outDir, scriptName)

	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	sb.WriteString(fmt.Sprintf("# なでしこ3 (gonako) v%s のZIPをGitHubリリースへアップロードする\n", cfg.version))
	sb.WriteString("# 事前に `gh release create " + cfg.version + "` 等でリリース自体を作成しておくこと\n")
	sb.WriteString("# タグ名はダウンロードURL (install.sh/install.ps1) と合わせて v なしのバージョン番号そのもの\n")
	sb.WriteString("set -eu\n\n")
	sb.WriteString(fmt.Sprintf("TAG=\"%s\"\n\n", cfg.version))
	sb.WriteString("gh release upload \"$TAG\" \\\n")
	for _, z := range zips {
		name := filepath.Base(z)
		sb.WriteString(fmt.Sprintf("  \"$(dirname \"$0\")/%s\" \\\n", name))
	}
	sb.WriteString("  --clobber\n")

	return os.WriteFile(scriptPath, []byte(sb.String()), 0o755)
}

// buildCLI builds the pure Go gonako CLI executable and packages it as a zip.
func buildCLI(cfg config, goos, goarch string) error {
	binName := "gonako"
	if goos == "windows" {
		binName += ".exe"
	}
	outName := fmt.Sprintf("gonako-%s-%s-%s", cfg.version, goos, goarch)
	if goos == "windows" {
		outName += ".exe"
	}
	outPath := filepath.Join(cfg.outDir, outName)

	fmt.Printf("  -> ビルド中: %s (%s/%s)\n", outName, goos, goarch)

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", outPath, "./cmd/gonako")
	cmd.Env = append(os.Environ(),
		"GOOS="+goos,
		"GOARCH="+goarch,
		"CGO_ENABLED=0",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	defer os.Remove(outPath)

	zipName := fmt.Sprintf("gonako-%s-%s-%s.zip", cfg.version, goos, goarch)
	zipPath := filepath.Join(cfg.outDir, zipName)
	fmt.Printf("  -> zip アーカイブ作成中: %s\n", zipName)
	return zipSingleFile(outPath, zipPath, binName)
}

// buildGUI builds the webview-based gonako-gui application.
func buildGUI(cfg config, goos, goarch string) error {
	switch goos {
	case "darwin":
		return buildGUIDarwin(cfg, goarch)
	case "windows":
		return buildGUIWindows(cfg, goarch)
	case "linux":
		return buildGUILinux(cfg, goarch)
	default:
		fmt.Printf("  [SKIP] gonako-gui: 未対応のOSです (%s/%s)\n", goos, goarch)
		return nil
	}
}

func buildGUIDarwin(cfg config, goarch string) error {
	if runtime.GOOS != "darwin" {
		fmt.Printf("  [SKIP] gonako-gui (darwin/%s): macOS以外のホストからのクロスビルドには対応していません\n", goarch)
		return nil
	}

	binName := fmt.Sprintf("gonako-gui-%s-darwin-%s", cfg.version, goarch)
	binPath := filepath.Join(cfg.outDir, binName)

	fmt.Printf("  -> ビルド中: %s (darwin/%s)\n", binName, goarch)

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", binPath, "./cmd/gonako-gui")
	cmd.Env = append(os.Environ(),
		"GOOS=darwin",
		"GOARCH="+goarch,
		"CGO_ENABLED=1",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// macOS App Bundle (.app) の作成
	appName := fmt.Sprintf("gonako-gui-%s-darwin-%s.app", cfg.version, goarch)
	appPath := filepath.Join(cfg.outDir, appName)
	fmt.Printf("  -> macOS アプリバンドル作成中: %s\n", appName)

	_ = os.RemoveAll(appPath)
	if err := createMacAppBundle(appPath, binPath, cfg.version); err != nil {
		return fmt.Errorf(".app バンドルの作成に失敗しました: %w", err)
	}

	// .app.zip アーカイブの作成
	zipName := fmt.Sprintf("gonako-gui-%s-darwin-%s.app.zip", cfg.version, goarch)
	zipPath := filepath.Join(cfg.outDir, zipName)
	fmt.Printf("  -> zip アーカイブ作成中: %s\n", zipName)
	if err := zipDirectory(appPath, zipPath, appName); err != nil {
		return fmt.Errorf("zip の作成に失敗しました: %w", err)
	}

	_ = os.Remove(binPath)
	_ = os.RemoveAll(appPath)

	return nil
}

func createMacAppBundle(appPath, binPath, version string) error {
	contentsDir := filepath.Join(appPath, "Contents")
	macosDir := filepath.Join(contentsDir, "MacOS")
	resDir := filepath.Join(contentsDir, "Resources")

	if err := os.MkdirAll(macosDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		return err
	}

	// 実行ファイルをコピー
	destBin := filepath.Join(macosDir, "gonako-gui")
	if err := copyFile(binPath, destBin, 0o755); err != nil {
		return err
	}

	// PkgInfo を作成
	if err := os.WriteFile(filepath.Join(contentsDir, "PkgInfo"), []byte("APPL????"), 0o644); err != nil {
		return err
	}

	// AppIcon.icns をコピー
	srcIcon := filepath.Join("cmd", "gonako-gui", "ui", "macapp", "Contents", "Resources", "AppIcon.icns")
	if _, err := os.Stat(srcIcon); err == nil {
		destIcon := filepath.Join(resDir, "AppIcon.icns")
		if err := copyFile(srcIcon, destIcon, 0o644); err != nil {
			return err
		}
	}

	// Info.plist をテンプレートから生成
	tmplPath := filepath.Join("cmd", "gonako-gui", "ui", "macapp", "Contents", "Info.plist")
	tmplData, err := os.ReadFile(tmplPath)
	if err != nil {
		return err
	}

	type plistFields struct {
		Executable string
		Icon       string
		Identifier string
		Name       string
		Version    string
	}
	fields := plistFields{
		Executable: "gonako-gui",
		Icon:       "AppIcon",
		Identifier: "com.nadesiko3.gonako.gui",
		Name:       "なでしこ3",
		Version:    version,
	}

	tmpl, err := template.New("plist").Parse(string(tmplData))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, fields); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(contentsDir, "Info.plist"), buf.Bytes(), 0o644); err != nil {
		return err
	}

	// ad-hoc 署名を付与してバンドルの整合性を整える
	signCmd := exec.Command("codesign", "--force", "--deep", "-s", "-", appPath)
	_ = signCmd.Run()

	return nil
}


func buildGUIWindows(cfg config, goarch string) error {
	binName := fmt.Sprintf("gonako-gui-%s-windows-%s.exe", cfg.version, goarch)
	binPath := filepath.Join(cfg.outDir, binName)

	var envs []string
	envs = append(envs, os.Environ()...)
	envs = append(envs,
		"GOOS=windows",
		"GOARCH="+goarch,
		"CGO_ENABLED=1",
	)

	if runtime.GOOS != "windows" {
		var cc, cxx string
		if goarch == "amd64" {
			cc = "x86_64-w64-mingw32-gcc"
			cxx = "x86_64-w64-mingw32-g++"
		} else if goarch == "arm64" {
			cc = "aarch64-w64-mingw32-gcc"
			cxx = "aarch64-w64-mingw32-g++"
		}

		if cc == "" || !hasCommand(cc) {
			fmt.Printf("  [SKIP] gonako-gui (windows/%s): クロスコンパイラ %s が見つかりません\n", goarch, cc)
			return nil
		}

		envs = append(envs, "CC="+cc, "CXX="+cxx)
	}

	fmt.Printf("  -> ビルド中: %s (windows/%s)\n", binName, goarch)

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w -H=windowsgui", "-o", binPath, "./cmd/gonako-gui")
	cmd.Env = envs
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// zip アーカイブの作成
	zipName := fmt.Sprintf("gonako-gui-%s-windows-%s.zip", cfg.version, goarch)
	zipPath := filepath.Join(cfg.outDir, zipName)
	fmt.Printf("  -> zip アーカイブ作成中: %s\n", zipName)
	if err := zipSingleFile(binPath, zipPath, "gonako-gui.exe"); err != nil {
		return fmt.Errorf("zip の作成に失敗しました: %w", err)
	}

	_ = os.Remove(binPath)

	return nil
}

func buildGUILinux(cfg config, goarch string) error {
	binName := fmt.Sprintf("gonako-gui-%s-linux-%s", cfg.version, goarch)
	binPath := filepath.Join(cfg.outDir, binName)

	var envs []string
	envs = append(envs, os.Environ()...)
	envs = append(envs,
		"GOOS=linux",
		"GOARCH="+goarch,
		"CGO_ENABLED=1",
	)

	if runtime.GOOS != "linux" {
		var cc, cxx string
		if goarch == "amd64" {
			cc = "x86_64-linux-gnu-gcc"
			cxx = "x86_64-linux-gnu-g++"
		} else if goarch == "arm64" {
			cc = "aarch64-linux-gnu-gcc"
			cxx = "aarch64-linux-gnu-g++"
		}

		if cc == "" || !hasCommand(cc) {
			fmt.Printf("  [SKIP] gonako-gui (linux/%s): Linux用Cコンパイラ %s が見つからないためスキップします\n", goarch, cc)
			return nil
		}

		envs = append(envs, "CC="+cc, "CXX="+cxx)
	}

	fmt.Printf("  -> ビルド中: %s (linux/%s)\n", binName, goarch)

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", binPath, "./cmd/gonako-gui")
	cmd.Env = envs
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// zip アーカイブの作成
	zipName := fmt.Sprintf("gonako-gui-%s-linux-%s.zip", cfg.version, goarch)
	zipPath := filepath.Join(cfg.outDir, zipName)
	fmt.Printf("  -> zip アーカイブ作成中: %s\n", zipName)
	if err := zipSingleFile(binPath, zipPath, "gonako-gui"); err != nil {
		return fmt.Errorf("zip の作成に失敗しました: %w", err)
	}

	_ = os.Remove(binPath)

	return nil
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(mode)
}

func zipDirectory(srcDir, zipPath, prefixInZip string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		nameInZip := filepath.ToSlash(filepath.Join(prefixInZip, rel))
		if info.IsDir() {
			if rel == "." {
				return nil
			}
			nameInZip += "/"
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = nameInZip
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			if _, err := io.Copy(writer, file); err != nil {
				return err
			}
		}

		return nil
	})
}

func zipSingleFile(srcFile, zipPath, nameInZip string) error {
	info, err := os.Stat(srcFile)
	if err != nil {
		return err
	}

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = nameInZip
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	file, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(writer, file)
	return err
}
