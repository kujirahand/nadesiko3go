package main

// 実行ファイルに変換されたアプリの起動と、変換そのものを受け持つ (AGENTS.md §10)。
//
//	[ gonako-gui 本体 ][ ペイロード(zip) ][ フッタ ]
//
// gonako-gui は自分自身をランタイムとして使うので、変換する側にも
// 受け取る側にもGoツールチェインが要らない。

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"html"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/vm"
	"github.com/webview/webview_go"
)

// htmlLikeRE decides whether a program's output should be shown as HTML.
// エディタの「ウィンドウ」表示と同じ判定にしてある。
var htmlLikeRE = regexp.MustCompile(`(?is)<[a-z][\s\S]*>`)

// BuildResult is the JSON structure returned to JavaScript after a build.
type BuildResult struct {
	OK    bool   `json:"ok"`
	Path  string `json:"path,omitempty"`
	Size  int64  `json:"size,omitempty"`
	Kind  string `json:"kind,omitempty"`
	Error string `json:"error,omitempty"`
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

	server := &http.Server{Handler: http.FileServer(http.FS(resources))}
	go func() { _ = server.Serve(listener) }()

	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d/%s", port, packed.Entry)

	w := newAppWindow(packed.Title)
	if w == nil {
		return
	}
	defer w.Destroy()
	w.Navigate(url)
	w.Run()
}

// runBundledProgram runs the packed なでしこプログラム and shows what it
// printed. A program that opened its own window with 『ウィンドウ作成』 prints
// nothing, and then there is no second window to show.
func runBundledProgram(packed *bundle.Bundle) {
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), os.Args[1:])
	host.Bundle = packed

	runErr := vm.RunCompiled(packed.Program, host)
	text := out.String()

	if runErr != nil {
		showMessageWindow(packed.Title, text+"\n"+runErr.Error())
		return
	}
	if strings.TrimSpace(text) == "" {
		return
	}
	w := newAppWindow(packed.Title)
	if w == nil {
		return
	}
	defer w.Destroy()
	// 出力がHTMLならそのまま描き、そうでなければ文字のまま読めるように包む
	if htmlLikeRE.MatchString(text) {
		w.SetHtml(text)
	} else {
		w.SetHtml(textPage(text))
	}
	w.Run()
}

// newAppWindow opens the window a converted application runs in.
func newAppWindow(title string) webview.WebView {
	w := webview.New(os.Getenv("GONAKO_DEBUG") == "1")
	if w == nil {
		fmt.Fprintln(os.Stderr, "WebViewの初期化に失敗しました。")
		return nil
	}
	if title == "" {
		title = "なでしこ3"
	}
	w.SetTitle(title)
	w.SetSize(960, 640, webview.HintNone)
	return w
}

// showMessageWindow reports a startup failure where a GUI application can
// actually be read: in a window, since there is no terminal attached.
func showMessageWindow(title, message string) {
	w := newAppWindow(title)
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
	return `<!DOCTYPE html><html lang="ja"><head><meta charset="UTF-8">` +
		`<style>body{margin:0;padding:16px;background:#1e1e2e;color:#cdd6f4;` +
		`font-family:"SF Mono","Consolas","Menlo",monospace;font-size:14px;line-height:1.6;}` +
		`pre{margin:0;white-space:pre-wrap;word-break:break-word;}</style></head>` +
		`<body><pre>` + html.EscapeString(text) + `</pre></body></html>`
}

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
	target := out
	if strings.EqualFold(filepath.Ext(out), appExt) {
		exe, err := prepareMacAppBundle(out, title)
		if err != nil {
			return BuildResult{Error: err.Error()}
		}
		useFolderIcon(out, folder)
		target = exe
		// .app の中身は変換の途中で書き換わるので、梱包対象から外す
		spec.Skip[exe] = true
	}

	if err := bundle.BuildSpec(target, self, spec); err != nil {
		return BuildResult{Error: err.Error()}
	}
	if err := os.Chmod(target, 0o755); err != nil {
		return BuildResult{Error: fmt.Sprintf("実行権限を付けられません: %v", err)}
	}
	if err := makeWindowsGUI(target); err != nil {
		return BuildResult{Error: fmt.Sprintf("Windowsアプリの設定を書き換えられません: %v", err)}
	}

	return BuildResult{OK: true, Path: out, Size: pathSize(out), Kind: spec.Kind}
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

// prepareMacAppBundle writes the .app folder from the embedded template and
// reports where the executable must go.
//
// 持ち込みの icon.icns がフォルダにあれば、それをアプリのアイコンにする。
func prepareMacAppBundle(appPath, title string) (execPath string, err error) {
	name := strings.TrimSuffix(filepath.Base(appPath), appExt)
	if name == "" {
		return "", fmt.Errorf("アプリ名が空です")
	}
	if title == "" {
		title = name
	}

	// 作り直せるように、前の .app は消してから書く。間違って別のフォルダを
	// 消さないよう、.app の形をしているものだけを対象にする。
	if info, statErr := os.Stat(appPath); statErr == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("『%s』はフォルダではありません", appPath)
		}
		if _, plistErr := os.Stat(filepath.Join(appPath, macPlistPath)); plistErr != nil {
			return "", fmt.Errorf("『%s』はmacOSアプリではないので上書きしません", appPath)
		}
		if err := os.RemoveAll(appPath); err != nil {
			return "", fmt.Errorf("古いアプリを消せません: %w", err)
		}
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
