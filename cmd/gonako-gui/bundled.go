package main

// 実行ファイルに変換されたアプリの起動と、変換そのものを受け持つ (AGENTS.md §10)。
//
//	[ gonako-gui 本体 ][ ペイロード(zip) ][ フッタ ]
//
// gonako-gui は自分自身をランタイムとして使うので、変換する側にも
// 受け取る側にもGoツールチェインが要らない。

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	htmltemplate "html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/guilib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
	"github.com/webview/webview_go"
)

// BuildResult is the JSON structure returned to JavaScript after a build.
type BuildResult struct {
	OK    bool   `json:"ok"`
	Path  string `json:"path,omitempty"`
	Size  int64  `json:"size,omitempty"`
	Kind  string `json:"kind,omitempty"`
	Error string `json:"error,omitempty"`
	// Warning is set when the new app is already at Path but something could
	// not be cleaned up. That is a completed build, not Error.
	Warning string `json:"warning,omitempty"`
	// Leftover lists what the warning left behind, so the caller can show the
	// real paths instead of claiming the old app is fully recoverable.
	Leftover []string `json:"leftover,omitempty"`
}

// runBundledApp runs the application packed into this executable, if there is
// one. It reports whether it ran, so main can fall back to the editor.
func runBundledApp() bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}
	packed, err := bundle.Open(self)
	if err != nil {
		return false // 同梱されていない、ふつうの gonako-gui
	}
	defer packed.Close()

	if packed.Kind == bundle.KindHTML {
		runBundledHTML(packed)
	} else {
		runBundledProgram(packed)
	}
	return true
}

// runBundledHTML serves the packed folder to a WebView window.
func runBundledHTML(packed *bundle.Bundle) {
	resources, err := packed.ResourceFS()
	if err != nil {
		showMessageWindow(packed.Title, fmt.Sprintf("同梱ファイルを読めません: %v", err))
		return
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		showMessageWindow(packed.Title, fmt.Sprintf("ローカルサーバーを起動できません: %v", err))
		return
	}
	defer listener.Close()

	server := &http.Server{Handler: loopbackOnly(newSiteHandler(resources))}
	go func() { _ = server.Serve(listener) }()

	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d/%s", port, packed.Entry)

	settings, err := bundledWindowSettings(packed)
	if err != nil {
		showMessageWindow(packed.Title, err.Error())
		return
	}
	w := newAppWindow(settings)
	if w == nil {
		return
	}
	defer w.Destroy()

	// 梱包したHTMLからも window.gonako とwnako3が使えるようにする（#63）。
	session := &guiSession{window: newNativeWindowController(w)}
	bindAsyncRunAPI(w, session, os.Args[1:], packed)
	forceWNako3 := false
	if data, ok := packed.ReadResource(windowConfigFile); ok {
		forceWNako3 = wnako3ConfigFromIndexJSON(data)
	}
	installWNako3(w, newWNako3PageConfig(forceWNako3), session.window, packed)

	w.Navigate(url)
	w.Run()
}

// runBundledProgram runs the packed なでしこプログラム and shows what it
// printed. A program that opened its own window with 『ウィンドウ作成』 prints
// nothing, and then there is no second window to show.
func runBundledProgram(packed *bundle.Bundle) {
	settings, err := bundledWindowSettings(packed)
	if err != nil {
		showMessageWindow(packed.Title, err.Error())
		return
	}
	w := newAppWindow(settings)
	if w == nil {
		return
	}
	defer w.Destroy()
	session := &guiSession{window: newNativeWindowController(w)}
	_ = w.Bind("startNakoEvent", func(runID uint64, handle int, event string, values map[string]string) string {
		b, _ := json.Marshal(session.startEvent(runID, handle, event, values))
		return string(b)
	})
	_ = w.Bind("pollNakoRun", func(runID uint64) string {
		b, _ := json.Marshal(session.poll(runID))
		return string(b)
	})
	_ = w.Bind("resolveNakoDialog", func(runID, dialogID uint64, text string, accepted bool) bool {
		return session.resolveDialog(runID, dialogID, text, accepted)
	})
	_ = w.Bind("closeBundledWindow", func() {
		w.Terminate()
	})
	runID := session.startCompiled(packed.Program, os.Args[1:], packed)
	w.SetHtml(bundledAsyncProgramPage(runID))
	w.Run()
}

// 実行画面のひな形の置き場。//go:embed でこのバイナリの中に入っている。
const bundledAssetRoot = "ui/bundled"

// bundledPage は ui/bundled/app.html にCSSとJSを埋め込むためのテンプレート。
// WebViewのSetHtmlは単一のHTML文字列しか受け取れないので、外部ファイル参照
// ではなく、実行時にひとつのHTMLへまとめてから渡す（#42）。
var bundledPage = sync.OnceValues(func() (*bundledPageAssets, error) {
	shell, err := fs.ReadFile(uiFS, bundledAssetRoot+"/app.html")
	if err != nil {
		return nil, err
	}
	style, err := fs.ReadFile(uiFS, bundledAssetRoot+"/app.css")
	if err != nil {
		return nil, err
	}
	script, err := fs.ReadFile(uiFS, bundledAssetRoot+"/app.js")
	if err != nil {
		return nil, err
	}
	// CSSとJSは自前のアセットなのでエスケープせずに素通しする。
	tmpl, err := template.New("bundled-app").Parse(string(shell))
	if err != nil {
		return nil, err
	}
	return &bundledPageAssets{tmpl: tmpl, style: string(style), script: string(script)}, nil
})

// bundledPageAssets は組み立て済みの実行画面テンプレートと、そこへ差し込む
// CSS・JSの中身を持つ。読み込みはプロセスに一度きり。
type bundledPageAssets struct {
	tmpl   *template.Template
	style  string
	script string
}

// bundledAsyncProgramPage は実行ID runID のプログラムを表示する画面を組み立てる。
func bundledAsyncProgramPage(runID uint64) string {
	a, err := bundledPage()
	if err != nil {
		return textPage("画面テンプレートを読み込めません: " + err.Error())
	}
	var buf bytes.Buffer
	data := struct {
		Style  string
		Script string
		RunID  uint64
	}{a.style, a.script, runID}
	if err := a.tmpl.Execute(&buf, data); err != nil {
		return textPage("画面テンプレートを展開できません: " + err.Error())
	}
	return buf.String()
}

// bundledWindowSettings はmanifestのタイトルへ、梱包されたindex.jsonを重ねる。
func bundledWindowSettings(packed *bundle.Bundle) (guilib.WindowSettings, error) {
	settings := defaultWindowSettings(packed.Title, 960, 640)
	fromFile, found, err := loadBundledWindowSettings(packed)
	if err != nil {
		return guilib.WindowSettings{}, err
	}
	if found {
		settings = mergeWindowSettings(settings, fromFile)
	}
	return settings, nil
}

// newAppWindow は変換済みアプリを実行するウィンドウを開く。
func newAppWindow(settings guilib.WindowSettings) webview.WebView {
	w := webview.New(os.Getenv("GONAKO_DEBUG") == "1")
	if w == nil {
		fmt.Fprintln(os.Stderr, "WebViewの初期化に失敗しました。")
		return nil
	}
	bindFileDrop(w)
	if !settings.HasTitle || settings.Title == "" {
		settings.HasTitle = true
		settings.Title = "なでしこ3"
	}
	if err := applyWindowSettings(w, settings, false); err != nil {
		fmt.Fprintf(os.Stderr, "ウィンドウ設定を適用できません: %v\n", err)
		w.Destroy()
		return nil
	}
	return w
}

// showMessageWindow reports a startup failure where a GUI application can
// actually be read: in a window, since there is no terminal attached.
func showMessageWindow(title, message string) {
	w := newAppWindow(defaultWindowSettings(title, 520, 220))
	if w == nil {
		fmt.Fprintln(os.Stderr, message)
		return
	}
	defer w.Destroy()
	w.SetHtml(textPage(message))
	w.Run()
}

// textPage wraps plain text so a WebView shows it as it was printed.
func textPage(text string) string {
	tmpl, err := bundledTextTemplate()
	if err != nil {
		// テンプレートが壊れていても、伝えたい文面だけは見せる。
		return "<pre>" + html.EscapeString(text) + "</pre>"
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Text string }{text}); err != nil {
		return "<pre>" + html.EscapeString(text) + "</pre>"
	}
	return buf.String()
}

// bundledTextTemplate は ui/bundled/text.html を読む。表示するのは利用者の
// プログラムが出した文字列なので、html/template に任せてエスケープする。
var bundledTextTemplate = sync.OnceValues(func() (*htmltemplate.Template, error) {
	data, err := fs.ReadFile(uiFS, bundledAssetRoot+"/text.html")
	if err != nil {
		return nil, err
	}
	return htmltemplate.New("bundled-text").Parse(string(data))
})

// runtimePathForBuild names the executable a converted app is built on: this
// one, so that packaging needs no Go toolchain. It is a variable so that a
// test can build on a runtime it prepared itself.
var runtimePathForBuild = os.Executable

// buildAppFromFolder packs folderPath into a stand-alone executable that runs
// entryPath on startup. It needs no Go toolchain: the runtime it appends the
// payload to is this very executable.
func buildAppFromFolder(folderPath, entryPath, outPath, title string) BuildResult {
	folder, err := filepath.Abs(folderPath)
	if err != nil {
		return BuildResult{Error: fmt.Sprintf("フォルダの場所が分かりません: %v", err)}
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return BuildResult{Error: fmt.Sprintf("フォルダ『%s』が見つかりません", folderPath)}
	}

	entry, err := filepath.Abs(entryPath)
	if err != nil {
		return BuildResult{Error: fmt.Sprintf("ファイルの場所が分かりません: %v", err)}
	}
	rel, err := filepath.Rel(folder, entry)
	if err != nil || strings.HasPrefix(rel, "..") {
		return BuildResult{Error: "メインファイルは、変換するフォルダの中にある必要があります"}
	}

	out, err := filepath.Abs(outPath)
	if err != nil {
		return BuildResult{Error: fmt.Sprintf("出力先が分かりません: %v", err)}
	}

	self, err := runtimePathForBuild()
	if err != nil {
		return BuildResult{Error: fmt.Sprintf("ランタイムの場所が分かりません: %v", err)}
	}

	spec := bundle.Spec{
		Name:        filepath.Base(entry),
		Title:       title,
		ResourceDir: folder,
		Flat:        true,
		// 出力先がフォルダの中にあっても、自分自身を巻き込まないようにする
		Skip: map[string]bool{out: true},
	}

	switch strings.ToLower(filepath.Ext(entry)) {
	case ".nako3", ".nako":
		code, err := os.ReadFile(entry)
		if err != nil {
			return BuildResult{Error: fmt.Sprintf("ファイルを読み込めません: %v", err)}
		}
		prog, err := vm.CompileProgram(string(code), filepath.Base(entry))
		if err != nil {
			return BuildResult{Error: err.Error()}
		}
		spec.Kind = bundle.KindProgram
		spec.Program = prog
	case ".html", ".htm":
		spec.Kind = bundle.KindHTML
		spec.Entry = filepath.ToSlash(rel)
	default:
		return BuildResult{Error: "なでしこプログラム(.nako3)かHTMLファイル(.html)を開いてから実行してください"}
	}

	// macOSでは「1つの実行ファイル」ではなく .app フォルダにまとめる。
	// Dockに名前とアイコンが出て、Finderからそのまま開ける形になる。
	// 完成するまで最終パスには触らない。隣に隠しフォルダを作り、その中の
	// 最終basenameの.appを完成させてから置き換える。
	target := out
	var staging *macAppStaging
	if strings.EqualFold(filepath.Ext(out), appExt) {
		if err := checkMacAppOutput(out); err != nil {
			return BuildResult{Error: err.Error()}
		}
		staging, err = newMacAppStaging(out)
		if err != nil {
			return BuildResult{Error: err.Error()}
		}
		defer func() { _ = staging.discard() }()
		// 出力先がフォルダの中にあるときだけ、既存の.appが梱包対象に入る。
		// 完成まで残すので、その中身を明示的に外しておく。
		if outputInside(folder, out) {
			skipExistingTree(spec.Skip, out)
		}
		exe, err := prepareMacAppBundle(staging.app, title)
		if err != nil {
			return BuildResult{Error: err.Error()}
		}
		useFolderIcon(staging.app, folder)
		target = exe
		// .app の中身は変換の途中で書き換わるので、梱包対象から外す
		spec.Skip[exe] = true
	}

	if err := bundle.BuildSpec(target, self, spec); err != nil {
		return BuildResult{Error: err.Error()}
	}
	if err := chmodBuiltApp(target, 0o755); err != nil {
		return BuildResult{Error: fmt.Sprintf("実行権限を付けられません: %v", err)}
	}
	if err := makeWindowsGUI(target); err != nil {
		return BuildResult{Error: fmt.Sprintf("Windowsアプリの設定を書き換えられません: %v", err)}
	}
	result := BuildResult{OK: true, Path: out, Kind: spec.Kind}
	if staging != nil {
		installed, err := installMacApp(staging.app, out)
		if err != nil {
			return BuildResult{Error: err.Error()}
		}
		var warnings []string
		if installed.Leftover != "" {
			warnings = append(warnings, fmt.Sprintf(
				"新しいアプリは公開しましたが、古いアプリを片付けられませんでした。残ったbackup『%s』は完全でない可能性があります: %v",
				installed.Leftover, installed.CleanupErr))
			result.Leftover = append(result.Leftover, installed.Leftover)
		}
		if err := staging.discard(); err != nil {
			warnings = append(warnings, fmt.Sprintf("新しいアプリは公開しましたが、一時フォルダを片付けられませんでした: %v", err))
			result.Leftover = append(result.Leftover, staging.root)
		}
		result.Warning = strings.Join(warnings, "\n")
	}
	// サイズは公開後の最終成果物から測る。置換前のoutは存在しないか旧.app
	// なので、ここより前で測ると初回0・再ビルド旧サイズになる。
	result.Size = pathSize(out)
	return result
}

// Windowsのサブシステム。実行ファイルのヘッダに入っている。
const (
	peSubsystemGUI = 2 // コンソールを開かない
	peSubsystemCUI = 3 // 起動すると黒いコンソール窓が出る
)

// makeWindowsGUI marks a converted Windows executable as a GUI application, so
// that double-clicking it does not open a console window behind the WebView.
//
// 書き換えるのはPEヘッダのサブシステム欄1つだけ。Go製の実行ファイルは
// CheckSum欄が0で、Windowsもユーザーモードの実行ファイルでは検証しないので、
// 直す必要はない。末尾のペイロードもPEローダは見ないため影響しない。
//
// PEでないファイル(macOSやLinuxの実行ファイル)は、何もせずに返る。
func makeWindowsGUI(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	// MS-DOSヘッダ。末尾にPEヘッダの位置が入っている。
	head := make([]byte, 0x40)
	if _, err := f.ReadAt(head, 0); err != nil {
		return nil // 小さすぎる。PEではない
	}
	if head[0] != 'M' || head[1] != 'Z' {
		return nil
	}
	peOff := int64(binary.LittleEndian.Uint32(head[0x3C:]))
	if peOff <= 0 {
		return nil
	}

	sig := make([]byte, 4)
	if _, err := f.ReadAt(sig, peOff); err != nil {
		return nil
	}
	if string(sig) != "PE\x00\x00" {
		return nil
	}

	// 「PE\0\0」(4) + COFFヘッダ(20) の次がオプショナルヘッダ。
	// サブシステム欄はその68バイト目で、32bit(PE32)でも64bit(PE32+)でも同じ位置。
	optOff := peOff + 24
	magic := make([]byte, 2)
	if _, err := f.ReadAt(magic, optOff); err != nil {
		return nil
	}
	switch binary.LittleEndian.Uint16(magic) {
	case 0x10b, 0x20b: // PE32 / PE32+
	default:
		return nil
	}

	subOff := optOff + 68
	cur := make([]byte, 2)
	if _, err := f.ReadAt(cur, subOff); err != nil {
		return nil
	}
	if binary.LittleEndian.Uint16(cur) != peSubsystemCUI {
		return nil // すでにGUIとしてビルドされている
	}
	binary.LittleEndian.PutUint16(cur, peSubsystemGUI)
	_, err = f.WriteAt(cur, subOff)
	return err
}

// pathSize totals a file, or a folder such as an .app bundle.
func pathSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total
}

// macOSアプリの決まり。ひな形は cmd/gonako-gui/ui/macapp/ にあり、
// //go:embed でこのバイナリの中に入っている。
const (
	appExt          = ".app"
	macTemplateRoot = "ui/macapp"
	macPlistPath    = "Contents/Info.plist"
	macExecDir      = "Contents/MacOS"
	macIconName     = "AppIcon"
	// 変換したアプリが持ち込みアイコンとして使えるファイル名
	macUserIcon = "icon.icns"
)

// macAppFields is what the Info.plist template is filled in with.
type macAppFields struct {
	Name       string
	Executable string
	Identifier string
	Version    string
	Icon       string
}

// prepareMacAppBundle writes a fresh .app folder from the embedded template and
// reports where the executable must go.
//
// It writes where it is told and never replaces an existing bundle, so the
// caller finishes the app in a private folder and swaps it in only after
// packaging succeeded. 持ち込みの icon.icns がフォルダにあれば、それをアプリの
// アイコンにする。
func prepareMacAppBundle(appPath, title string) (execPath string, err error) {
	name := strings.TrimSuffix(filepath.Base(appPath), appExt)
	if name == "" {
		return "", fmt.Errorf("アプリ名が空です")
	}
	if title == "" {
		title = name
	}

	fields := macAppFields{
		Name:       title,
		Executable: name,
		Identifier: macBundleIdentifier(name),
		Version:    appVersion,
		Icon:       macIconName,
	}

	err = fs.WalkDir(uiFS, macTemplateRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(macTemplateRoot, p)
		if err != nil {
			return err
		}
		dest := filepath.Join(appPath, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, err := uiFS.ReadFile(p)
		if err != nil {
			return err
		}
		// Info.plist だけは、アプリごとの名前を埋めてから書き出す
		if rel == macPlistPath {
			data, err = fillMacPlist(data, fields)
			if err != nil {
				return err
			}
		}
		return os.WriteFile(dest, data, 0o644)
	})
	if err != nil {
		return "", fmt.Errorf("macOSアプリのひな形を書き出せません: %w", err)
	}

	execDir := filepath.Join(appPath, macExecDir)
	if err := os.MkdirAll(execDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(execDir, name), nil
}

// 変換の途中で失敗を注入できるように、テストが差し替える口を用意する。
var (
	chmodBuiltApp  = os.Chmod
	renameBuiltApp = os.Rename
)

// checkMacAppOutput refuses to replace anything that is not a macOS app, so a
// failed build can never be blamed for deleting an unrelated folder.
func checkMacAppOutput(appPath string) error {
	info, err := os.Stat(appPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("『%s』を確認できません: %w", appPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("『%s』はフォルダではありません", appPath)
	}
	if _, err := os.Stat(filepath.Join(appPath, macPlistPath)); err != nil {
		return fmt.Errorf("『%s』はmacOSアプリではないので上書きしません", appPath)
	}
	return nil
}

// macAppStaging is the private folder a .app is finished in. The bundle inside
// keeps the final basename so the executable name and the bundle identifier
// never inherit the random folder name.
type macAppStaging struct {
	root string
	app  string
}

// newMacAppStaging creates the private folder next to the final app. Keeping it
// a sibling means the later swap stays on one filesystem.
func newMacAppStaging(appPath string) (*macAppStaging, error) {
	root, err := os.MkdirTemp(filepath.Dir(appPath), "."+filepath.Base(appPath)+".staging-")
	if err != nil {
		return nil, fmt.Errorf("一時フォルダを作れません: %w", err)
	}
	return &macAppStaging{root: root, app: filepath.Join(root, filepath.Base(appPath))}, nil
}

// discard removes the private folder. It is safe to call after the bundle was
// moved out, and on every failure path.
func (s *macAppStaging) discard() error {
	return os.RemoveAll(s.root)
}

// macAppInstall reports how far the swap got. A non-nil error means the new
// app never took the final path. Leftover names an old backup that stayed
// behind after the new app was published, which is a warning, not a failure:
// the backup may already be partly deleted, so it must not be presented as a
// complete rescue copy.
type macAppInstall struct {
	Leftover   string
	CleanupErr error
}

// installMacApp swaps the finished staging bundle onto appPath. The previous
// app is moved aside first and put back when the swap fails, so a failed
// rebuild never leaves the user without a working app. Once the new app took
// the final path, a cleanup failure is reported separately instead of being
// rolled back or hidden.
func installMacApp(stagingApp, appPath string) (macAppInstall, error) {
	var result macAppInstall
	backup := ""
	if _, err := os.Lstat(appPath); err == nil {
		var err error
		backup, err = macAppBackupPath(appPath)
		if err != nil {
			return result, err
		}
		if err := renameBuiltApp(appPath, backup); err != nil {
			return result, fmt.Errorf("古いアプリを退避できません: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return result, fmt.Errorf("出力先『%s』を確認できません: %w", appPath, err)
	}

	if err := renameBuiltApp(stagingApp, appPath); err != nil {
		if backup == "" {
			return result, fmt.Errorf("新しいアプリを置けません: %w", err)
		}
		if restoreErr := renameBuiltApp(backup, appPath); restoreErr != nil {
			return result, fmt.Errorf(
				"新しいアプリを置けません: %w。古いアプリは『%s』に残っていますが、元の場所へ戻せません: %v",
				err, backup, restoreErr)
		}
		return result, fmt.Errorf("新しいアプリを置けません: %w", err)
	}
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil {
			result.Leftover = backup
			result.CleanupErr = err
		}
	}
	return result, nil
}

// macAppBackupPath picks an unused hidden name next to the app.
func macAppBackupPath(appPath string) (string, error) {
	dir := filepath.Dir(appPath)
	for range 10 {
		candidate := filepath.Join(dir, "."+filepath.Base(appPath)+".backup-"+rand.Text())
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("退避先の名前が重複しました")
}

// outputInside reports whether path is below folder. The bundle walker packs
// everything below the resource folder, so only then does an existing output
// need explicit skipping.
func outputInside(folder, path string) bool {
	rel, err := filepath.Rel(folder, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// skipExistingTree keeps an existing output folder out of the payload. The
// bundle walker skips files, not folders, so every file has to be named.
func skipExistingTree(skip map[string]bool, root string) {
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			skip[p] = true
		}
		return nil
	})
}

// useFolderIcon replaces the template icon with one the folder provides.
func useFolderIcon(appPath, folder string) {
	src := filepath.Join(folder, macUserIcon)
	data, err := os.ReadFile(src)
	if err != nil {
		return // 持ち込みアイコンなし。ひな形のアイコンをそのまま使う
	}
	dest := filepath.Join(appPath, "Contents", "Resources", macIconName+".icns")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err == nil {
		_ = os.WriteFile(dest, data, 0o644)
	}
}

// fillMacPlist fills the Info.plist template in, escaping the values so that a
// name with 『&』 or 『<』 in it cannot break the XML.
func fillMacPlist(tmplData []byte, fields macAppFields) ([]byte, error) {
	tmpl, err := template.New("plist").Funcs(template.FuncMap{}).Parse(string(tmplData))
	if err != nil {
		return nil, err
	}
	escaped := macAppFields{
		Name:       xmlEscape(fields.Name),
		Executable: xmlEscape(fields.Executable),
		Identifier: xmlEscape(fields.Identifier),
		Version:    xmlEscape(fields.Version),
		Icon:       xmlEscape(fields.Icon),
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, escaped); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// macBundleIdentifier builds a CFBundleIdentifier from the application name.
// 使える文字は英数字・ハイフン・ドットだけなので、日本語の名前は落として
// 「app」に寄せる。識別子は表示には使われない。
func macBundleIdentifier(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r == ' ' || r == '_' || r == '.':
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "app"
	}
	return "com.nadesiko3.gonako." + slug
}

// macOSでの署名について (AGENTS.md §10)。
//
// 末尾にペイロードを足しても、macOSの ad-hoc 署名は壊れない。署名が守る範囲は
// CodeDirectory の codeLimit までで、その後ろに足したバイトは対象外だからで、
// Apple Silicon でもそのまま起動できることを実機で確認している。
//
// むしろ codesign し直すと壊れる。追記済みのファイルは Mach-O の末尾に余分な
// データがある状態なので、codesign が「main executable failed strict
// validation」で拒否し、書き換えられた実行ファイルは SIGKILL される。
// したがって、ここでは署名に触らない。
