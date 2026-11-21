package main

import (
	"strings"
	"testing"
)

// #523: キー押下のスクロール抑止はキャンバス向け。入力欄に同じイベントを
// 付けると、矢印キーとスペースまで止められて文字が打てなくなる。
func TestKeyDownPreventDefaultSkipsTextEntry(t *testing.T) {
	for _, name := range []string{"app.js", "bundled/app.js"} {
		src := readUIAsset(t, name)
		if !strings.Contains(src, "isContentEditable") {
			t.Fatalf("%s が入力中のキー押下を区別していません", name)
		}
		if !strings.Contains(src, "tag === 'input' || tag === 'textarea'") {
			t.Fatalf("%s が input / textarea をスクロール抑止の対象外にしていません", name)
		}
	}
}
