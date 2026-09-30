package main

import (
	"encoding/json"
	"reflect"
	"testing"
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
	values := map[string]string{"1": "x"}
	got := withDroppedFiles("click", values)
	if !reflect.DeepEqual(got, values) {
		t.Fatalf("click: %#v", got)
	}
	// ネイティブ側に記録が無ければ、JavaScriptが渡したファイル名のまま。
	drop := map[string]string{"__gonako_drop_files": `["a.txt"]`}
	got = withDroppedFiles("drop", drop)
	var names []string
	if err := json.Unmarshal([]byte(got["__gonako_drop_files"]), &names); err != nil || len(names) != 1 || names[0] != "a.txt" {
		t.Fatalf("drop: %#v %v", got, err)
	}
}
