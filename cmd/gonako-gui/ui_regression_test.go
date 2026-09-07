package main

import (
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
		"activeEl === editor",
		"window.readClipboardText()",
		"window.writeClipboardText(selectedText)",
		"editor.setRangeText",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing editor clipboard bridge %q", required)
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
		"window.dispatchNakoEvent(",
		"collectGUIValues()",
		"applyGUIOperations(data.operations",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing GUI bridge contract %q", required)
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

func TestBinaryFileRequiresConfirmationBeforeEditorLoad(t *testing.T) {
	app := readUIAsset(t, "app.js")
	for _, required := range []string{
		"showBinaryOpenConfirmDialog(fileName)",
		"window.readFile(filePath, false)",
		"window.readFile(filePath, true)",
		"currentFileEncoding = data.encoding === 'Shift_JIS' ? 'Shift_JIS' : 'UTF-8'",
		"window.saveFile(targetPath, editor.value, currentFileEncoding)",
		"保存時はShift_JIS形式を維持します",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("app.js is missing binary file confirmation behavior %q", required)
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
	if got := strings.Count(app, "if (isDialogIMEKeyEvent(e)) return;"); got != 2 {
		t.Fatalf("IME dialog guard count = %d, want 2", got)
	}
}
