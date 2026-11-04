package main

import (
	"reflect"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
)

func TestSplitDroppedPaths(t *testing.T) {
	got := splitDroppedPaths([]byte("/a/日本語.txt\x00/b/c d.png\x00"))
	want := []string{"/a/日本語.txt", "/b/c d.png"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if got := splitDroppedPaths(nil); len(got) != 0 {
		t.Fatalf("空入力: %#v", got)
	}
}

func TestWithDroppedFilesKeepsOtherEvents(t *testing.T) {
	payload := guilib.EventPayload{Values: map[string]guilib.EventField{"1": {Value: "x"}}}
	got := withDroppedFiles("click", payload)
	if !reflect.DeepEqual(got, payload) {
		t.Fatalf("click: %#v", got)
	}
	// ネイティブ側に記録が無ければ、JavaScriptが渡したファイル名のまま。
	drop := guilib.EventPayload{DropFiles: []string{"a.txt"}}
	got = withDroppedFiles("drop", drop)
	if !reflect.DeepEqual(got.DropFiles, []string{"a.txt"}) {
		t.Fatalf("drop: %#v", got)
	}
}
