package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kujirahand/nadesiko3go/internal/csvlib"
	"github.com/kujirahand/nadesiko3go/internal/guilib"
	"github.com/kujirahand/nadesiko3go/internal/imagelib"
	"github.com/kujirahand/nadesiko3go/internal/mathlib"
	"github.com/kujirahand/nadesiko3go/internal/nodelib"
	"github.com/kujirahand/nadesiko3go/internal/officelib"
	"github.com/kujirahand/nadesiko3go/internal/pdflib"
	"github.com/kujirahand/nadesiko3go/internal/sqlitelib"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
	"github.com/webview/webview_go"
)

//go:embed ui/*
var uiFS embed.FS

// appVersion is what the editor reports and what a converted macOS app
// records in its Info.plist.
const appVersion = "3.6.0"

// guiPlugins is the command set a GUI program can use. It goes both into the
// default runtime registry (so that a bundled program compiles and runs with
// the same commands) and into the editor's own registry.
func guiPlugins() []stdlib.Plugin {
	return []stdlib.Plugin{
		nodelib.New(), csvlib.New(), mathlib.New(), sqlitelib.New(),
		officelib.New(), pdflib.New(), imagelib.New(), guilib.New(),
	}
}

func init() {
	vm.RegisterPlugin(sqlitelib.New(), officelib.New(), pdflib.New(), imagelib.New(), guilib.New())
}

// RunResult is the JSON structure returned to JavaScript after execution.
type RunResult struct {
	OK     bool   `json:"ok"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// AppInfo contains metadata about the running gonako-gui application.
type AppInfo struct {
	Version    string `json:"version"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	HomeDir    string `json:"homeDir"`
	DesktopDir string `json:"desktopDir"`
}

// CommandItem describes a nadesiko command for the command palette/list.
type CommandItem struct {
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Josi       [][]string `json:"josi"`
	ReturnNone bool       `json:"returnNone,omitempty"`
	Category   string     `json:"category,omitempty"`
	Desc       string     `json:"desc,omitempty"`
	Template   string     `json:"template,omitempty"`
	Yomi       string     `json:"yomi,omitempty"`
}

// TemplateItem describes a sample nadesiko3 script file.
type TemplateItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Desc     string `json:"desc"`
	Code     string `json:"code"`
}

// FileItem represents a single file or directory in the file browser.
type FileItem struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

// DirListing is returned to JavaScript when browsing directories.
type DirListing struct {
	CurrentDir string     `json:"currentDir"`
	ParentDir  string     `json:"parentDir"`
	Items      []FileItem `json:"items"`
	Error      string     `json:"error,omitempty"`
}

const usage = `gonako-gui - なでしこ3 GUI (軽量WebView)

使い方:
  gonako-gui [オプション] [HTMLフォルダまたはファイル]

例:
  gonako-gui                        内蔵の標準エディタUIを起動
  gonako-gui ./my-app/              ./my-app フォルダ内の index.html を表示
  gonako-gui ./my-app/index.html    指定したHTMLファイルを表示
  gonako-gui -dir ./web/ -title "マイアプリ"

オプション:
  -dir string      配信・表示するHTMLフォルダのパス
  -url string      直接開くURL (http:// または https://)
  -title string    ウィンドウタイトル (既定: "なでしこ3 (gonako-gui)")
  -width int       ウィンドウの幅 (既定: 1080)
  -height int      ウィンドウの高さ (既定: 720)
  -debug           開発者ツール（デバッグモード）を有効化
  -help, --help    このヘルプを表示
`

func getCommandList() []CommandItem {
	data, err := fs.ReadFile(uiFS, "ui/command-list.json")
	if err == nil {
		var items []CommandItem
		if err := json.Unmarshal(data, &items); err == nil && len(items) > 0 {
			return items
		}
	}

	reg := stdlib.NewRegistry(
		nodelib.New(), csvlib.New(), mathlib.New(), sqlitelib.New(),
		officelib.New(), pdflib.New(), imagelib.New(), guilib.New(),
	)
	list := reg.FuncList()
	items := make([]CommandItem, 0, len(list))
	for name, item := range list {
		if item.Type != "func" {
			continue
		}
		items = append(items, CommandItem{
			Name:       name,
			Type:       item.Type,
			Josi:       item.Josi,
			ReturnNone: item.ReturnNone,
			Desc:       fmt.Sprintf("命令『%s』を実行します", name),
			Template:   name,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func getTemplateList() []TemplateItem {
	entries, err := fs.ReadDir(uiFS, "ui/samples")
	if err != nil {
		return nil
	}
	var list []TemplateItem
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".nako3") {
			continue
		}
		data, err := fs.ReadFile(uiFS, "ui/samples/"+e.Name())
		if err != nil {
			continue
		}
		code := string(data)
		base := strings.TrimSuffix(e.Name(), ".nako3")
		title := base
		if idx := strings.Index(base, "_"); idx >= 0 {
			title = base[idx+1:]
		}

		desc := title
		lines := strings.Split(code, "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "//") {
			desc = strings.TrimSpace(strings.TrimPrefix(lines[0], "//"))
		}

		category := "サンプル"
		switch {
		case strings.Contains(base, "こんにちは"):
			category = "基本"
		case strings.Contains(base, "FizzBuzz"):
			category = "制御構文"
		case strings.Contains(base, "計算"):
			category = "データ構造"
		case strings.Contains(base, "ハッシュ") || strings.Contains(base, "ファイル"):
			category = "システム"
		case strings.Contains(base, "CSV"):
			category = "ファイル処理"
		case strings.Contains(base, "Excel"):
			category = "オフィス"
		case strings.Contains(base, "PDF"):
			category = "オフィス"
		case strings.Contains(base, "画像"):
			category = "グラフィック"
		case strings.Contains(base, "ウィンドウ"):
			category = "GUI"
		}

		list = append(list, TemplateItem{
			ID:       base,
			Title:    title,
			Category: category,
			Desc:     desc,
			Code:     code,
		})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list
}

func getDesktopDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	desktop := filepath.Join(home, "Desktop")
	if fi, err := os.Stat(desktop); err == nil && fi.IsDir() {
		return desktop
	}
	return home
}

func listFiles(dirPath string) DirListing {
	if dirPath == "" || dirPath == "$DESKTOP" {
		dirPath = getDesktopDir()
	} else if dirPath == "~" || dirPath == "$HOME" {
		home, err := os.UserHomeDir()
		if err != nil {
			dirPath = "."
		} else {
			dirPath = home
		}
	}
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return DirListing{Error: err.Error()}
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return DirListing{CurrentDir: absPath, ParentDir: filepath.Dir(absPath), Error: err.Error()}
	}

	var items []FileItem
	for _, entry := range entries {
		name := entry.Name()
		// 隠しファイル（"."から始まるファイル）は除外
		if strings.HasPrefix(name, ".") {
			continue
		}
		fullPath := filepath.Join(absPath, name)
		info, err := entry.Info()
		var size int64
		var modTime string
		if err == nil {
			size = info.Size()
			modTime = info.ModTime().Format("2006-01-02 15:04")
		}
		items = append(items, FileItem{
			Name:    name,
			Path:    fullPath,
			IsDir:   entry.IsDir(),
			Size:    size,
			ModTime: modTime,
		})
	}

	// フォルダを先頭に、名前順でソート
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	parent := filepath.Dir(absPath)
	if parent == absPath {
		parent = ""
	}

	return DirListing{
		CurrentDir: absPath,
		ParentDir:  parent,
		Items:      items,
	}
}

func createNewFile(dirPath string) (string, string, error) {
	if dirPath == "" || dirPath == "$DESKTOP" {
		dirPath = getDesktopDir()
	} else if dirPath == "$HOME" {
		home, err := os.UserHomeDir()
		if err != nil {
			dirPath = "."
		} else {
			dirPath = home
		}
	}
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return "", "", err
	}

	dateStr := time.Now().Format("2006-01-02")
	for i := 1; i <= 999; i++ {
		filename := fmt.Sprintf("%s-新規-%d.nako3", dateStr, i)
		fullPath := filepath.Join(absDir, filename)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			content := fmt.Sprintf("// %s\n「こんにちは」と表示。\n", filename)
			if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
				return "", "", err
			}
			return fullPath, filename, nil
		}
	}
	return "", "", fmt.Errorf("新規ファイルの上限に達しました")
}

func revealInFinder(targetPath string) error {
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		absPath = targetPath
	}

	switch runtime.GOOS {
	case "darwin":
		fi, err := os.Stat(absPath)
		if err == nil && !fi.IsDir() {
			return exec.Command("open", "-R", absPath).Start()
		}
		return exec.Command("open", absPath).Start()
	case "windows":
		return exec.Command("explorer", "/select,", absPath).Start()
	default:
		fi, err := os.Stat(absPath)
		if err == nil && !fi.IsDir() {
			return exec.Command("xdg-open", filepath.Dir(absPath)).Start()
		}
		return exec.Command("xdg-open", absPath).Start()
	}
}

func main() {
	// 実行ファイルに変換されたアプリなら、エディタではなくそのアプリを動かす
	if runBundledApp() {
		return
	}

	flags := flag.NewFlagSet("gonako-gui", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Print(usage)
	}

	dirFlag := flags.String("dir", "", "配信するHTMLフォルダのパス")
	urlFlag := flags.String("url", "", "開くURL")
	titleFlag := flags.String("title", "なでしこ3 (gonako-gui)", "ウィンドウタイトル")
	widthFlag := flags.Int("width", 1080, "ウィンドウの幅")
	heightFlag := flags.Int("height", 720, "ウィンドウの高さ")
	debugFlag := flags.Bool("debug", os.Getenv("GONAKO_DEBUG") == "1", "デバッグモード有効化")

	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	targetDir := *dirFlag
	targetURL := *urlFlag
	startPage := "index.html"

	// 位置引数の処理
	if flags.NArg() > 0 && targetDir == "" && targetURL == "" {
		arg := flags.Arg(0)
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			targetURL = arg
		} else {
			info, err := os.Stat(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "指定されたパス『%s』が見つかりません: %v\n", arg, err)
				os.Exit(1)
			}
			if info.IsDir() {
				targetDir = arg
			} else {
				targetDir = filepath.Dir(arg)
				startPage = filepath.Base(arg)
			}
		}
	}

	var handler http.Handler
	if targetDir != "" {
		absDir, err := filepath.Abs(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "パスの解決に失敗しました: %v\n", err)
			os.Exit(1)
		}
		handler = http.FileServer(http.Dir(absDir))
	} else if targetURL == "" {
		// 組み込みUIファイルをHTTPサーバーで配信
		subFS, err := fs.Sub(uiFS, "ui")
		if err != nil {
			fmt.Fprintf(os.Stderr, "UIアセットの読み込みに失敗しました: %v\n", err)
			os.Exit(1)
		}
		handler = http.FileServer(http.FS(subFS))
	}

	var finalURL string
	if targetURL != "" {
		finalURL = targetURL
	} else {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fmt.Fprintf(os.Stderr, "ローカルサーバーの起動に失敗しました: %v\n", err)
			os.Exit(1)
		}
		defer listener.Close()

		port := listener.Addr().(*net.TCPAddr).Port
		server := &http.Server{Handler: handler}
		go func() {
			_ = server.Serve(listener)
		}()
		finalURL = fmt.Sprintf("http://127.0.0.1:%d/%s", port, startPage)
	}

	// WebViewウィンドウの初期化
	w := webview.New(*debugFlag)
	if w == nil {
		fmt.Fprintln(os.Stderr, "WebViewの初期化に失敗しました。OSのWebView2/WebKitGTK/Cocoa環境を確認してください。")
		os.Exit(1)
	}
	defer w.Destroy()

	w.SetTitle(*titleFlag)
	w.SetSize(*widthFlag, *heightFlag, webview.HintNone)

	guiRegistry := stdlib.NewRegistry(guiPlugins()...)

	// Go ↔ JavaScript バインディング: なでしこコードの実行
	_ = w.Bind("runNakoCode", func(code string) string {
		var outBuf strings.Builder
		host := vm.NewCUIHost(&outBuf, strings.NewReader(""), nil)

		runErr := vm.RunWithHostAndRegistry(code, "gui.nako3", guiRegistry, host)
		result := RunResult{
			OK:     runErr == nil,
			Output: outBuf.String(),
		}
		if runErr != nil {
			result.Error = runErr.Error()
		}
		b, _ := json.Marshal(result)
		return string(b)
	})

	// Go ↔ JavaScript バインディング: アプリケーション情報
	_ = w.Bind("getAppInfo", func() string {
		home, _ := os.UserHomeDir()
		info := AppInfo{
			Version:    appVersion,
			OS:         runtime.GOOS,
			Arch:       runtime.GOARCH,
			HomeDir:    home,
			DesktopDir: getDesktopDir(),
		}
		b, _ := json.Marshal(info)
		return string(b)
	})

	// Go ↔ JavaScript バインディング: 命令一覧取得
	_ = w.Bind("getCommandList", func() string {
		items := getCommandList()
		b, _ := json.Marshal(items)
		return string(b)
	})

	// Go ↔ JavaScript バインディング: ひな形一覧取得 (cmd/gonako-gui/ui/samples/*.nako3 から自動取得)
	_ = w.Bind("getTemplateList", func() string {
		templates := getTemplateList()
		b, _ := json.Marshal(templates)
		return string(b)
	})

	// Go ↔ JavaScript バインディング: ファイル一覧取得
	_ = w.Bind("listFiles", func(dirPath string) string {
		listing := listFiles(dirPath)
		b, _ := json.Marshal(listing)
		return string(b)
	})

	// Go ↔ JavaScript バインディング: ファイル読み込み
	_ = w.Bind("readFile", func(path string) string {
		data, err := os.ReadFile(path)
		res := struct {
			OK       bool   `json:"ok"`
			Content  string `json:"content,omitempty"`
			IsBinary bool   `json:"isBinary,omitempty"`
			Path     string `json:"path"`
			Error    string `json:"error,omitempty"`
		}{
			Path: path,
		}
		if err != nil {
			res.OK = false
			res.Error = err.Error()
		} else if !utf8.Valid(data) {
			// PNGなどのバイナリファイル。文字コード範囲外のバイトを含むので、
			// 中身は送らず「編集不可」の表示だけJS側に任せる。
			res.OK = true
			res.IsBinary = true
		} else {
			res.OK = true
			res.Content = string(data)
		}
		b, _ := json.Marshal(res)
		return string(b)
	})

	// Go ↔ JavaScript バインディング: ファイル保存
	_ = w.Bind("saveFile", func(path, content string) string {
		err := os.WriteFile(path, []byte(content), 0o644)
		res := struct {
			OK    bool   `json:"ok"`
			Path  string `json:"path"`
			Error string `json:"error,omitempty"`
		}{
			Path: path,
		}
		if err != nil {
			res.OK = false
			res.Error = err.Error()
		} else {
			res.OK = true
		}
		b, _ := json.Marshal(res)
		return string(b)
	})

	// Go ↔ JavaScript バインディング: 新規ファイル作成 (YYYY-MM-DD-新規-N.nako3)
	_ = w.Bind("createNewFile", func(dirPath string) string {
		fullPath, filename, err := createNewFile(dirPath)
		res := struct {
			OK    bool   `json:"ok"`
			Path  string `json:"path,omitempty"`
			Name  string `json:"name,omitempty"`
			Error string `json:"error,omitempty"`
		}{}
		if err != nil {
			res.OK = false
			res.Error = err.Error()
		} else {
			res.OK = true
			res.Path = fullPath
			res.Name = filename
		}
		b, _ := json.Marshal(res)
		return string(b)
	})

	// Go ↔ JavaScript バインディング: OSファイラーで表示 (Finder / Explorer)
	_ = w.Bind("revealInFinder", func(targetPath string) string {
		err := revealInFinder(targetPath)
		res := struct {
			OK    bool   `json:"ok"`
			Error string `json:"error,omitempty"`
		}{}
		if err != nil {
			res.OK = false
			res.Error = err.Error()
		} else {
			res.OK = true
		}
		b, _ := json.Marshal(res)
		return string(b)
	})

	// Go ↔ JavaScript バインディング: フォルダを実行ファイルに変換
	_ = w.Bind("buildAppFromFolder", func(folderPath, entryPath, outPath, title string) string {
		res := buildAppFromFolder(folderPath, entryPath, outPath, title)
		b, _ := json.Marshal(res)
		return string(b)
	})

	// Go ↔ JavaScript バインディング: Go言語でビルド (gogen)
	_ = w.Bind("buildWithGo", func(sourcePath, outPath string) string {
		res := buildWithGo(sourcePath, outPath)
		b, _ := json.Marshal(res)
		return string(b)
	})

	// WebViewを開く
	w.Navigate(finalURL)
	w.Run()
}
