package main

import (
	"os"
	"strings"
	"testing"
)

func readUIAsset(t *testing.T, name string) string {
	t.Helper()
	b, err := uiFS.ReadFile("ui/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEditorUsesClipboardBridgeWithoutBrowserClipboard(t *testing.T) {
	app := readUIAsset(t, "app.js")
	if strings.Contains(app, "navigator.clipboard.readText") {
		t.Fatal("app.js must not manually paste clipboard text in addition to the WebView default action")
	}
	for _, required := range []string{
		"isCmdOrCtrl && isInput",
		"window.readClipboardText()",
		"window.writeClipboardText(selectedText)",
		"activeEl.setRangeText",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing input clipboard bridge %q", required)
		}
	}
}

func TestHighlightScrollUsesUnclampedTransform(t *testing.T) {
	highlight := readUIAsset(t, "editor-highlight.js")
	if !strings.Contains(highlight, "layer.style.transform") {
		t.Fatal("editor-highlight.js must move the highlight layer by the textarea scroll offset")
	}
	for _, oldAssignment := range []string{"layer.scrollTop =", "layer.scrollLeft ="} {
		if strings.Contains(highlight, oldAssignment) {
			t.Fatalf("editor-highlight.js must not use clamped overlay scrolling: found %q", oldAssignment)
		}
	}

	style := readUIAsset(t, "style.css")
	if !strings.Contains(style, "overflow: visible;") {
		t.Fatal("the transformed highlight layer must remain visible inside the clipping editor host")
	}
}

func TestGUIEventBridgeIsWired(t *testing.T) {
	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"window.runNakoFile(code, currentFilePath || '', isWindowMode)",
		"window.startNakoEvent(",
		"collectGUIValues()",
		"applyGUIOperations(status.operations)",
		// イベント中のダイアログに応答できること（#59）。
		"window.resolveNakoDialog(eventRunID",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing GUI bridge contract %q", required)
		}
	}
}

func TestGUIOperationBridgeSupportsPartAttributesAndSelectReplacement(t *testing.T) {
	for _, asset := range []string{"app.js", "bundled/app.js"} {
		source := readUIAsset(t, asset)
		for _, required := range []string{".attributes || {}", ".styles || {}", "replaceChildren()"} {
			if !strings.Contains(source, required) {
				t.Fatalf("%s is missing GUI part operation support %q", asset, required)
			}
		}
	}
}

func TestFileDialogCommandsInsertStringLiterals(t *testing.T) {
	want := map[string]string{
		"ファイル選択":   "『S』のファイル選択",
		"保存ファイル選択": "『S』の保存ファイル選択",
		"フォルダ選択":   "『S』のフォルダ選択",
	}
	for _, command := range getCommandList() {
		expected, ok := want[command.Name]
		if !ok {
			continue
		}
		if command.Template != expected {
			t.Errorf("%s template = %q, want %q", command.Name, command.Template, expected)
		}
		delete(want, command.Name)
	}
	for name := range want {
		t.Errorf("command list does not contain %s", name)
	}
}

func TestCommandGroupsCollapseUnlessSearching(t *testing.T) {
	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"const expandedCmdGroups = new Set()",
		"const collapsed = !isSearching && !expandedCmdGroups.has(groupName)",
		"if (isSearching) return",
		"if (mode === 'group') expandedCmdGroups.clear()",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing grouped-search behavior %q", required)
		}
	}
}

func TestFileTabLoadsDirectoryOnFirstOpen(t *testing.T) {
	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"let hasLoadedFileList = false",
		"let isLoadingFileList = false",
		"if (!hasLoadedFileList && !isLoadingFileList)",
		"loadDirectory(desktopDirPath || homeDirPath || '$DESKTOP')",
		"hasLoadedFileList = true",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing first file-tab load behavior %q", required)
		}
	}
}

func TestFileTabNavigationControls(t *testing.T) {
	html := readUIAsset(t, "index.html")
	for _, required := range []string{
		`id="btn-file-refresh"`,
		`id="btn-file-up"`,
		`id="btn-open-folder"`,
		`id="btn-new-folder"`,
	} {
		if !strings.Contains(html, required) {
			t.Fatalf("index.html is missing file navigation control %q", required)
		}
	}
	for _, removed := range []string{`id="btn-file-home"`, `id="btn-new-file"`} {
		if strings.Contains(html, removed) {
			t.Fatalf("index.html still contains removed file control %q", removed)
		}
	}

	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"window.revealInFinder(folderPath)",
		"window.createNewFolder(",
		"await loadDirectory(pathDirName(data.path))",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing file navigation behavior %q", required)
		}
	}
	if strings.Contains(app, "window.showFolderDialog(") {
		t.Fatal("folder open button must reveal the current folder instead of showing a selection dialog")
	}
}

func TestNewButtonOffersFileOrProjectChoice(t *testing.T) {
	html := readUIAsset(t, "index.html")
	for _, required := range []string{
		`id="btn-new"`,
		`id="new-menu"`,
		`id="menu-item-new-file"`,
		`id="menu-item-new-project"`,
	} {
		if !strings.Contains(html, required) {
			t.Fatalf("index.html is missing new-menu control %q", required)
		}
	}

	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"toggleNewMenu()",
		"menuItemNewFile.addEventListener('click', newFile)",
		"menuItemNewProject.addEventListener('click', newProject)",
		"window.createNewFolder(",
		"window.createAIProject(data.path)",
		"window.createProjectMainFile(data.path)",
		"'main.nako3'",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing new-menu behavior %q", required)
		}
	}
	if strings.Contains(app, "btnNew.addEventListener('click', newFile)") {
		t.Fatal("btn-new must open the new-file/new-project picker instead of creating a file directly")
	}
}

func TestNewFolderAutomaticallyScaffoldsAIProject(t *testing.T) {
	app := readUIAsset(t, "app.js")
	newFolderStart := strings.Index(app, "btnNewFolder.addEventListener")
	if newFolderStart < 0 {
		t.Fatal("app.js is missing the new-folder button handler")
	}
	newFolderEnd := strings.Index(app[newFolderStart:], "\n  });")
	if newFolderEnd < 0 {
		t.Fatal("could not find the end of the new-folder button handler")
	}
	handler := app[newFolderStart : newFolderStart+newFolderEnd]

	for _, required := range []string{
		"window.createAIProject(data.path)",
	} {
		if !strings.Contains(handler, required) {
			t.Fatalf("new-folder handler is missing automatic AI project scaffolding %q", required)
		}
	}
}

func TestAIProjectTemplateMenuIsWired(t *testing.T) {
	html := readUIAsset(t, "index.html")
	for _, required := range []string{`id="menu-item-ai-project"`, "AI用の雛形を作成"} {
		if !strings.Contains(html, required) {
			t.Fatalf("index.html is missing AI project menu %q", required)
		}
	}

	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"(currentFilePath ? pathDirName(currentFilePath) : '') || currentDirPath",
		"showConfirmDialog(",
		"既存のファイルは上書きしません",
		"window.createAIProject(targetDir)",
		"activateTab(tabBtnFile, tabContentFile)",
		"await loadDirectory(data.path)",
		"AGENTS.md", "CLAUDE.md",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing AI project behavior %q", required)
		}
	}
}

func TestOutputPanelCanCloseAndReopensOnRun(t *testing.T) {
	html := readUIAsset(t, "index.html")
	if !strings.Contains(html, `id="btn-close-output"`) {
		t.Fatal("index.html is missing the output panel close button")
	}

	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"function setOutputPanelOpen(open)",
		"btnCloseOutput.addEventListener('click', () => setOutputPanelOpen(false))",
		"async function runCode() {\n    setOutputPanelOpen(true)",
		"splitterH.classList.toggle('is-closed', !open)",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing output panel behavior %q", required)
		}
	}
}

// ウィンドウモードでは『表示』は画面プレビューに描かれる。同じ文字を下の
// 出力欄にも流すと二重に見えるので、出力欄はコマンドラインのときだけ使う。
func TestWindowModeDoesNotDuplicateDisplayOutput(t *testing.T) {
	app := readUIAsset(t, "app.js")
	if !strings.Contains(app, "if (!isWindowMode) appendGUIOutput(status.output") {
		t.Fatal("ウィンドウモードでは『表示』の出力を出力欄へ流してはいけない（画面プレビューと二重になる）")
	}
	if strings.Contains(app, "appendGUIOutput(data.output") {
		t.Fatal("実行結果・イベント結果の出力を出力欄へ流すと、画面プレビューと二重になる")
	}
}

// バイナリは確認ダイアログを出さずに読み取り専用で開き、
// Shift_JIS/EUC-JPは確認なしでUTF-8へ変換して開く（#44）。
func TestBinaryFileOpensReadOnlyWithoutConfirmation(t *testing.T) {
	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"window.readFile(filePath)",
		"isBinaryFile = !!data.isBinary",
		"editor.readOnly = isBinaryFile",
		"data.encoding === 'Shift_JIS' || data.encoding === 'EUC-JP' ? data.encoding : 'UTF-8'",
		"window.saveFile(targetPath, editor.value, currentFileEncoding)",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing binary/encoding handling %q", required)
		}
	}
	for _, forbidden := range []string{
		"showBinaryOpenConfirmDialog",
		"window.readFile(filePath, true)",
	} {
		if strings.Contains(app, forbidden) {
			t.Fatalf("app.js must not ask for confirmation any more: %q", forbidden)
		}
	}
}

func TestFileItemsUseContextMenuWithoutMoreButton(t *testing.T) {
	app := readUIAsset(t, "app.js")
	if strings.Contains(app, "item-more-btn") {
		t.Fatal("file items must not include a more button")
	}
	if !strings.Contains(app, "item.addEventListener('contextmenu'") {
		t.Fatal("file items must retain the context menu")
	}

	style := readUIAsset(t, "style.css")
	if strings.Contains(style, ".item-more-btn") {
		t.Fatal("style.css still contains unused more button styles")
	}
}

func TestDOMHTMLAndFocusOperationsAreApplied(t *testing.T) {
	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"op.type === 'html'",
		"el.innerHTML = op.html || ''",
		"op.type === 'focus'",
		"el.focus()",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing DOM operation %q", required)
		}
	}
}

func TestDOMLifecycleOperationsAreApplied(t *testing.T) {
	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"const guiElements = new Map()",
		"guiElements.set(Number(op.handle), el)",
		"if (!op.detached)",
		"op.type === 'append'",
		"parent.appendChild(el)",
		"op.type === 'remove'",
		"guiElements.delete(handle)",
		"el.remove()",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing DOM lifecycle operation %q", required)
		}
	}
}

func TestPromptDialogIgnoresIMEEnter(t *testing.T) {
	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"dialogInput.addEventListener('compositionstart'",
		"dialogInput.addEventListener('compositionend'",
		"event.isComposing || dialogInputIMEComposing || event.keyCode === 229",
		"if (isDialogIMEKeyEvent(e)) return;",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing IME dialog guard %q", required)
		}
	}
	if got := strings.Count(app, "if (isDialogIMEKeyEvent(e)) return;"); got != 3 {
		t.Fatalf("IME dialog guard count = %d, want 3", got)
	}
}

// 命令一覧をgonako/wnakoで切り替えられること（#101）。
func TestCommandListSourceSwitch(t *testing.T) {
	html := readUIAsset(t, "index.html")
	for _, required := range []string{
		`id="cmd-source-gonako"`,
		`id="cmd-source-wnako"`,
	} {
		if !strings.Contains(html, required) {
			t.Fatalf("index.html に命令一覧の切り替えボタン %s がありません", required)
		}
	}

	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"const commandCache = { gonako: null, wnako: null }",
		"bind: 'getWNakoCommandList'",
		"json: 'command-list-wnako.json'",
		"localStorage.setItem('gonako-cmd-source', source)",
		"cmdSourceWnakoBtn.addEventListener('click', () => setCmdSource('wnako'))",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js に命令一覧の切り替え処理 %q がありません", required)
		}
	}
}

// wnakoの命令一覧が同梱され、マニュアルへのリンクが付くこと（#101）。
func TestWNakoCommandListIsEmbedded(t *testing.T) {
	items := getWNakoCommandList()
	if len(items) < 500 {
		t.Fatalf("wnakoの命令一覧が少なすぎます: %d件", len(items))
	}
	for _, item := range items {
		if item.Name != "表示" {
			continue
		}
		if item.DocURL == "" {
			t.Fatal("『表示』のマニュアルURLがありません")
		}
		if item.Template != "【S】を表示" {
			t.Fatalf("『表示』の書式が違います: %q", item.Template)
		}
		return
	}
	t.Fatal("wnakoの命令一覧に『表示』がありません")
}

// gonakoの命令一覧にもマニュアル(Web)へのリンクが付くこと。
func TestGonakoCommandListHasDocURL(t *testing.T) {
	items := getCommandList()
	if len(items) < 100 {
		t.Fatalf("gonakoの命令一覧が少なすぎます: %d件", len(items))
	}
	checked := 0
	for _, item := range items {
		if item.DocURL == "" {
			t.Fatalf("『%s』のマニュアルURLがありません", item.Name)
		}
		if !strings.HasPrefix(item.DocURL, "https://nadesi.com/v3/doc/index.php?") {
			t.Fatalf("『%s』のマニュアルURLが不正です: %s", item.Name, item.DocURL)
		}
		if item.Name == "表示" {
			if !strings.Contains(item.DocURL, "plugin_system%2F") {
				t.Fatalf("『表示』のマニュアルURLにplugin_systemがありません: %s", item.DocURL)
			}
			checked++
		}
		if item.Name == "画像新規作成" {
			if !strings.Contains(item.DocURL, "gonako%2F") {
				t.Fatalf("『画像新規作成』のマニュアルURLにgonakoがありません: %s", item.DocURL)
			}
			checked++
		}
	}
	if checked < 2 {
		t.Fatal("代表的な命令（表示、画像新規作成）が検証されませんでした")
	}

	// wnakoの命令一覧も{プラグイン名}%2F{命令名}形式のURLを持つこと
	wnakoItems := getWNakoCommandList()
	for _, item := range wnakoItems {
		if item.Name == "AJAX_JSON取得" {
			if !strings.Contains(item.DocURL, "plugin_browser%2F") {
				t.Fatalf("wnakoの『AJAX_JSON取得』のマニュアルURLが不正です: %s", item.DocURL)
			}
		}
	}

	app := readUIAsset(t, "app.js")

	// 命令クリック時はHTMLカードを即座に表示し、右上のリンクから外部ブラウザの
	// Webマニュアルへ誘導する。ソースURLは短いファイル名ラベルのリンクにする。
	for _, required := range []string{"function displayCommandHelp(", "help-card", "createHelpField("} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js にHTML形式の命令ヘルプ表示がありません: %q", required)
		}
	}
	if !strings.Contains(app, "→Webマニュアル") {
		t.Fatal("app.js に外部ブラウザへのマニュアルリンクがありません")
	}
	if !strings.Contains(app, "function openExternalLink(") {
		t.Fatal("app.js に外部ブラウザでリンクを開く openExternalLink がありません")
	}
	for _, required := range []string{"const sourceName = cmd.file", "createExternalHelpLink(cmd.url, sourceName, 'help-source-link')"} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js がソースURLを短いラベルのリンクとして表示していません: %q", required)
		}
	}
	if !strings.Contains(app, "window.openExternalURL") {
		t.Fatal("app.js が Go側の openExternalURL バインディングを呼んでいません")
	}
	for _, required := range []string{"JSON.parse(rawResult)", "result.ok !== true", "外部ブラウザを開けませんでした"} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js が外部ブラウザ起動失敗を処理していません: %q", required)
		}
	}

	html := readUIAsset(t, "index.html")
	if !strings.Contains(html, `<div id="output" class="output">`) {
		t.Fatal("index.html の出力領域がHTMLヘルプを表示できるコンテナではありません")
	}
	style := readUIAsset(t, "style.css")
	for _, required := range []string{".output.has-command-help", ".help-web-link", "position: sticky", ".help-card", ".help-source-link"} {
		if !strings.Contains(style, required) {
			t.Fatalf("style.css に命令ヘルプの表示スタイルがありません: %q", required)
		}
	}

	// Go側にも外部ブラウザでURLを開くバインディングがあること
	mainSrc, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainSrc), `w.Bind("openExternalURL"`) {
		t.Fatal("main.go に openExternalURL のBindがありません")
	}
	for _, required := range []string{"if err := cmd.Start(); err != nil", "_ = cmd.Wait()"} {
		if !strings.Contains(string(mainSrc), required) {
			t.Fatalf("main.go が外部ブラウザの子プロセスを正しく起動・回収していません: %q", required)
		}
	}
}

// ライトモード (#112): 配色はCSS変数に集約し、data-gonako-themeで切り替える。
func TestEditorSupportsLightTheme(t *testing.T) {
	style := readUIAsset(t, "style.css")
	if !strings.Contains(style, `:root[data-gonako-theme="light"]`) {
		t.Fatal("style.css must define the light theme palette")
	}
	for _, hardcoded := range []string{"#585b70", "#6c7086", "#7f849c", "#f5c2e7", "rgba(255, 255, 255, 0.08)"} {
		if strings.Count(style, hardcoded) != 1 {
			t.Fatalf("style.css should only use %q in the dark palette variables", hardcoded)
		}
	}
	html := readUIAsset(t, "index.html")
	app := readUIAsset(t, "app.js")
	if !strings.Contains(html, `id="menu-item-theme"`) {
		t.Fatal("index.html is missing the theme menu item")
	}
	for _, required := range []string{"window.getEditorTheme()", "window.setEditorTheme(next)"} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing theme bridge %q", required)
		}
	}
	for _, page := range []string{"bundled/app.css", "bundled/text.html", "wnako3run.html"} {
		if !strings.Contains(readUIAsset(t, page), "data-gonako-theme") {
			t.Fatalf("%s must follow data-gonako-theme", page)
		}
	}
}

func TestThemeScript(t *testing.T) {
	for theme, want := range map[string]string{"ライト": `("light")`, "ダーク": `("dark")`, "自動": `("auto")`} {
		script := themeScript(theme)
		if !strings.HasSuffix(script, want+";") || !strings.Contains(script, "data-gonako-theme") {
			t.Fatalf("themeScript(%s) is wrong: %s", theme, script)
		}
	}
}
