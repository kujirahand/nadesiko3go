package main

// G01: .app を作り直す代表入力の時間と割当。trunkとheadで同じ入力を同じ入口から
// 実行し、置換のための追加コストだけが乗ることを確かめる。

import (
	"os"
	"path/filepath"
	"testing"
)

// 代表入力はHTML 1ページと小さな同梱データ。既存.appがある状態からの再ビルドを
// 測るので、退避と置換の経路も毎回通る。
func BenchmarkG01MacAppRebuild(b *testing.B) {
	dir := b.TempDir()
	runtimePath := filepath.Join(dir, "runtime")
	if err := os.WriteFile(runtimePath, []byte("RUNTIME-BYTES"), 0o755); err != nil {
		b.Fatal(err)
	}
	runtimePathForBuild = func() (string, error) { return runtimePath, nil }
	b.Cleanup(func() { runtimePathForBuild = os.Executable })

	folder := filepath.Join(dir, "src")
	for name, content := range map[string]string{
		"index.html":    "<h1>見本</h1>",
		"index.json":    `{"サイズ":[640,480]}`,
		"data/note.txt": "同梱データ",
	} {
		path := filepath.Join(folder, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	entry := filepath.Join(folder, "index.html")
	out := filepath.Join(dir, "App.app")
	if res := buildAppFromFolder(folder, entry, out, "App"); !res.OK {
		b.Fatalf("準備の変換に失敗した: %s", res.Error)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if res := buildAppFromFolder(folder, entry, out, "App"); !res.OK {
			b.Fatalf("変換に失敗した: %s", res.Error)
		}
	}
}
