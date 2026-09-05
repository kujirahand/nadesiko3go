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

func TestEditorUsesNativePaste(t *testing.T) {
	app := readUIAsset(t, "app.js")
	if strings.Contains(app, "navigator.clipboard.readText") {
		t.Fatal("app.js must not manually paste clipboard text in addition to the WebView default action")
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
