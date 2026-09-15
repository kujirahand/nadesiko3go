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
//	go run ./scripts/version-update.go --stable 3.8.2  Release公開後、インストーラーのフォールバック版だけを切り替える
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

// installers は「最新版APIの取得に失敗したときに使う公開済み安定版」を持つ
// インストーラー。開発中の版番号ではなく、Release公開後に --stable で更新する。
var installers = []singleOccurrence{
	{"scripts/install.sh", regexp.MustCompile(`DEFAULT_VERSION="([0-9]+\.[0-9]+\.[0-9]+)"`)},
	{"scripts/install.ps1", regexp.MustCompile(`\$defaultVersion = "([0-9]+\.[0-9]+\.[0-9]+)"`)},
}

// checkInstallersAgree は各インストーラーのフォールバック値がちょうど1箇所ずつ
// 見つかり、互いに一致していることを確かめる。
func checkInstallersAgree() error {
	first := ""
	for _, s := range installers {
		data, err := os.ReadFile(s.path)
		if err != nil {
			return err
		}
		matches := s.pattern.FindAllStringSubmatch(string(data), -1)
		if len(matches) != 1 {
			return fmt.Errorf("%s: パターンが%d件マッチしました（1件である必要があります）", s.path, len(matches))
		}
		if first == "" {
			first = matches[0][1]
		} else if matches[0][1] != first {
			return fmt.Errorf("インストーラー間でフォールバック版が一致しません: %s は %s（%s は %s）", s.path, matches[0][1], installers[0].path, first)
		}
	}
	return nil
}

// switchInstallers は全インストーラーのフォールバック版を newVersion へ切り替える。
// 片方だけ書き換わった中間状態を残さないよう、先に全ファイルを読んで検証し、
// 更新後の内容をメモリ上で作ってから書き込む。書き込みが途中で失敗したら、
// 書き込み済みのファイルを元の内容へ戻す。
func switchInstallers(newVersion string) error {
	type pending struct {
		path, current, original, updated string
		mode                             os.FileMode
	}
	var plans []pending
	for _, s := range installers {
		info, err := os.Stat(s.path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(s.path)
		if err != nil {
			return err
		}
		text := string(data)
		matches := s.pattern.FindAllStringSubmatchIndex(text, -1)
		if len(matches) != 1 {
			return fmt.Errorf("%s: パターンが%d件マッチしました（1件である必要があります）", s.path, len(matches))
		}
		m := matches[0]
		if text[m[2]:m[3]] == newVersion {
			continue
		}
		plans = append(plans, pending{
			path:     s.path,
			current:  text[m[2]:m[3]],
			original: text,
			updated:  text[:m[2]] + newVersion + text[m[3]:],
			mode:     info.Mode().Perm(),
		})
	}

	for i, p := range plans {
		if err := os.WriteFile(p.path, []byte(p.updated), p.mode); err != nil {
			// 書き込み済みのファイルを元へ戻す（戻せなければその旨も伝える）
			for _, done := range plans[:i] {
				if rerr := os.WriteFile(done.path, []byte(done.original), done.mode); rerr != nil {
					return fmt.Errorf("%s の書き込みに失敗し（%v）、%s を元に戻せませんでした: %v", p.path, err, done.path, rerr)
				}
			}
			return fmt.Errorf("%s の書き込みに失敗したため、変更を取り消しました: %v", p.path, err)
		}
	}
	for _, p := range plans {
		fmt.Printf("[更新] %s: %s -> %s\n", p.path, p.current, newVersion)
	}
	return nil
}

func main() {
	checkFlag := flag.Bool("check", false, "書き換えず、バージョン番号のズレを検査するだけ")
	nadesikoFlag := flag.String("nadesiko", "", "ナデシコ言語バージョンも合わせて変更する場合に指定")
	stableFlag := flag.String("stable", "", "インストーラーのフォールバック（公開済み安定版）だけをこの版へ切り替える")
	flag.Parse()

	// --stable はRelease公開後に publish-release.sh から呼ぶ。定義元や
	// release/ 配下（Homebrew Tap更新に使う成果物）には触れない。
	if *stableFlag != "" {
		if !semverRe.MatchString(*stableFlag) {
			fmt.Fprintf(os.Stderr, "エラー: --stable の形式が不正です: %q (例: 3.8.2)\n", *stableFlag)
			os.Exit(1)
		}
		if err := switchInstallers(*stableFlag); err != nil {
			fmt.Fprintln(os.Stderr, "エラー:", err)
			os.Exit(1)
		}
		return
	}

	newVersion := version.Version
	if flag.NArg() > 0 {
		newVersion = flag.Arg(0)
		if !semverRe.MatchString(newVersion) {
			fmt.Fprintf(os.Stderr, "エラー: バージョン番号の形式が不正です: %q (例: 3.8.2)\n", newVersion)
			os.Exit(1)
		}
	}

	// バージョンを更新するときは、古いバージョンのビルド成果物が release/ に
	// 混ざったまま残らないよう、実行のたびに中身を空にする（--check時は不変）。
	if !*checkFlag {
		if err := clearReleaseDir(); err != nil {
			fmt.Fprintln(os.Stderr, "エラー:", err)
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

	// 2. インストーラーのフォールバック値は「公開済みの安定版」を表すので、
	// ここでは開発中の版番号へ揃えない（Release公開前にマージされると、
	// 最新版APIの取得失敗時に未公開版を取りに行って404になるため）。
	// 公開後に publish-release.sh から --stable で切り替える。
	// 検査では、2つのインストーラーの値が一致しているかだけを見る。
	if err := checkInstallersAgree(); err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
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

// clearReleaseDir は release/ ディレクトリの中身を空にする。ディレクトリ自体が
// 無ければ何もしない（`just release` 未実行の環境で余計なディレクトリを
// 作らないため）。
func clearReleaseDir() error {
	const dir = "release"
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(dir + "/" + e.Name()); err != nil {
			return err
		}
		fmt.Printf("[削除] %s/%s\n", dir, e.Name())
	}
	return nil
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
