//go:build ignore

// version-update は internal/version/version.go を唯一の定義元として、
// Goコードから参照できない場所（インストーラースクリプト・READMEなど）の
// バージョン番号表記を同期させる。
//
// 使い方:
//
//	go run ./scripts/version-update.go 3.8.2        リリース版を3.8.2へ更新し、全ファイルを同期
//	go run ./scripts/version-update.go              現在の定義元の値へ全ファイルを再同期するだけ
//	go run ./scripts/version-update.go --check       書き換えず、ズレがあれば非ゼロ終了する（CI用）
//	go run ./scripts/version-update.go 3.8.2 --nadesiko 3.9.0  言語バージョンも合わせて変更
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"

	"github.com/kujirahand/nadesiko3go/internal/version"
)

var semverRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// singleOccurrence は「この正規表現にちょうど1箇所マッチし、その第1グループが
// バージョン番号」というファイルを表す。マッチしなければ対象追加漏れとして扱う。
type singleOccurrence struct {
	path    string
	pattern *regexp.Regexp
}

func main() {
	checkFlag := flag.Bool("check", false, "書き換えず、バージョン番号のズレを検査するだけ")
	nadesikoFlag := flag.String("nadesiko", "", "ナデシコ言語バージョンも合わせて変更する場合に指定")
	flag.Parse()

	newVersion := version.Version
	if flag.NArg() > 0 {
		newVersion = flag.Arg(0)
		if !semverRe.MatchString(newVersion) {
			fmt.Fprintf(os.Stderr, "エラー: バージョン番号の形式が不正です: %q (例: 3.8.2)\n", newVersion)
			os.Exit(1)
		}
	}
	newNadesiko := version.Nadesiko
	if *nadesikoFlag != "" {
		if !semverRe.MatchString(*nadesikoFlag) {
			fmt.Fprintf(os.Stderr, "エラー: --nadesiko の形式が不正です: %q (例: 3.8.2)\n", *nadesikoFlag)
			os.Exit(1)
		}
		newNadesiko = *nadesikoFlag
	}

	mismatched := false
	changed := false

	// 1. internal/version/version.go 自体（唯一の定義元）
	if newVersion != version.Version || newNadesiko != version.Nadesiko {
		if *checkFlag {
			fmt.Println("[ズレ] internal/version/version.go")
			mismatched = true
		} else {
			if err := updateVersionGoFile(newVersion, newNadesiko); err != nil {
				fmt.Fprintln(os.Stderr, "エラー:", err)
				os.Exit(1)
			}
			fmt.Println("[更新] internal/version/version.go")
			changed = true
		}
	}

	// 2. 1箇所だけバージョン番号を持つファイル群
	singles := []singleOccurrence{
		{"scripts/install.sh", regexp.MustCompile(`DEFAULT_VERSION="([0-9]+\.[0-9]+\.[0-9]+)"`)},
		{"scripts/install.ps1", regexp.MustCompile(`\$defaultVersion = "([0-9]+\.[0-9]+\.[0-9]+)"`)},
	}
	for _, s := range singles {
		ok, wasChanged, err := syncSingleOccurrence(s, newVersion, *checkFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "エラー:", err)
			os.Exit(1)
		}
		if !ok {
			mismatched = true
		}
		if wasChanged {
			changed = true
		}
	}

	// 3. 同じ数値が何度も出てくるドキュメント
	docFiles := []string{
		"docs/release-homebrew.md",
		"docs/release-scripts.md",
	}
	for _, doc := range docFiles {
		ok, wasChanged, err := syncAllOccurrences(doc, newVersion, *checkFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "エラー:", err)
			os.Exit(1)
		}
		if !ok {
			mismatched = true
		}
		if wasChanged {
			changed = true
		}
	}

	if *checkFlag {
		if mismatched {
			fmt.Fprintln(os.Stderr, "バージョン番号にズレがあります。`just version-update` を実行してください。")
			os.Exit(1)
		}
		fmt.Println("バージョン番号は一致しています:", newVersion)
		return
	}

	if changed {
		fmt.Println("バージョン番号を更新しました:", newVersion)
	} else {
		fmt.Println("変更はありません（すでに", newVersion, "です）")
	}
}

func updateVersionGoFile(newVersion, newNadesiko string) error {
	const path = "internal/version/version.go"
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(data)

	verPattern := regexp.MustCompile(`const Version = "[0-9]+\.[0-9]+\.[0-9]+"`)
	nakoPattern := regexp.MustCompile(`const Nadesiko = "[0-9]+\.[0-9]+\.[0-9]+"`)
	if !verPattern.MatchString(text) {
		return fmt.Errorf("%s に Version 定数が見つかりません", path)
	}
	if !nakoPattern.MatchString(text) {
		return fmt.Errorf("%s に Nadesiko 定数が見つかりません", path)
	}

	text = verPattern.ReplaceAllString(text, fmt.Sprintf(`const Version = "%s"`, newVersion))
	text = nakoPattern.ReplaceAllString(text, fmt.Sprintf(`const Nadesiko = "%s"`, newNadesiko))

	return os.WriteFile(path, []byte(text), 0o644)
}

// syncSingleOccurrence は pattern がちょうど1回マッチする前提で、そのキャプチャ
// グループを newVersion に揃える。マッチが0件・2件以上なら対象追加漏れ／曖昧化
// としてエラーにする。戻り値の ok は「事前に一致していたか（checkモード用）」。
func syncSingleOccurrence(s singleOccurrence, newVersion string, checkOnly bool) (ok bool, changed bool, err error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return false, false, err
	}
	text := string(data)

	matches := s.pattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) != 1 {
		return false, false, fmt.Errorf("%s: パターンが%d件マッチしました（1件である必要があります）", s.path, len(matches))
	}

	m := matches[0]
	current := text[m[2]:m[3]]
	if current == newVersion {
		return true, false, nil
	}
	if checkOnly {
		fmt.Printf("[ズレ] %s: %s -> %s\n", s.path, current, newVersion)
		return false, false, nil
	}

	updated := text[:m[2]] + newVersion + text[m[3]:]
	if err := os.WriteFile(s.path, []byte(updated), 0o644); err != nil {
		return false, false, err
	}
	fmt.Printf("[更新] %s: %s -> %s\n", s.path, current, newVersion)
	return true, true, nil
}

// semverTokenRe はファイル中に登場する「X.Y.Z」形式の数値をすべて拾う。
// docs/release-homebrew.md はバージョン番号の例示以外にこの形式の数値を
// 含まないことを確認済みなので、見つかった値はすべてリリース版番号とみなせる。
var semverTokenRe = regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+`)

// syncAllOccurrences はファイル中の全バージョン番号トークンを newVersion に
// 揃える。version.go を経由せず現在のファイル内容だけを見て判定するので、
// version.go の値を動かさずに `just version-update` を実行しても、手で
// ズレさせた箇所を検出・修復できる。
func syncAllOccurrences(path, newVersion string, checkOnly bool) (ok bool, changed bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false, err
	}
	text := string(data)

	staleCount := 0
	for _, tok := range semverTokenRe.FindAllString(text, -1) {
		if tok != newVersion {
			staleCount++
		}
	}
	if staleCount == 0 {
		return true, false, nil
	}
	if checkOnly {
		fmt.Printf("[ズレ] %s: 古いバージョン表記が%d件残っています (-> %s)\n", path, staleCount, newVersion)
		return false, false, nil
	}

	updated := semverTokenRe.ReplaceAllString(text, newVersion)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return false, false, err
	}
	fmt.Printf("[更新] %s: 古いバージョン表記%d件 -> %s\n", path, staleCount, newVersion)
	return true, true, nil
}
