package bundle_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
)

func TestBuildFailurePreservesExistingOutput(t *testing.T) {
	for _, unreadable := range []bool{false, true} {
		name := "missing-resource"
		if unreadable {
			name = "unreadable-resource"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "app")
			old := "PRE-EXISTING-BUILD-OUTPUT"
			if err := os.WriteFile(out, []byte(old), 0o755); err != nil {
				t.Fatal(err)
			}
			res := filepath.Join(dir, "res")
			if unreadable {
				if runtime.GOOS == "windows" {
					t.Skip("Windowsではchmodによる読取禁止を再現できません")
				}
				if err := os.Mkdir(res, 0o755); err != nil {
					t.Fatal(err)
				}
				file := filepath.Join(res, "private.txt")
				if err := os.WriteFile(file, []byte("resource"), 0o000); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(file, 0o600) })
				if _, err := os.ReadFile(file); err == nil {
					t.Skip("実行ユーザーが読取禁止のファイルを読めるため再現できません")
				}
			}
			err := bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{
				Kind: bundle.KindHTML, Entry: "index.html", ResourceDir: res,
			})
			if err == nil {
				t.Fatal("リソース読込失敗を期待しました")
			}
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != old {
				t.Fatalf("旧出力が失われました: %q", data)
			}
			assertNoBuildTemps(t, dir)
		})
	}
}

func TestBuildReplacesExistingOutputOnSuccess(t *testing.T) {
	dir := t.TempDir()
	rt := fakeRuntime(t, dir)
	out := filepath.Join(dir, "app")
	if err := os.WriteFile(out, []byte("PRE-EXISTING-BUILD-OUTPUT"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldInfo, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("完成したアプリ"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 出力先とリソースを同じフォルダにして、一時ファイルが梱包されないことも確認する。
	if err := bundle.BuildSpec(out, rt, bundle.Spec{
		Kind: bundle.KindHTML, Entry: "index.html", ResourceDir: dir, Flat: true,
		Skip: map[string]bool{out: true, rt: true},
	}); err != nil {
		t.Fatal(err)
	}
	app, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	resources := app.Resources()
	if len(resources) != 1 || resources[0] != "index.html" {
		t.Fatalf("梱包リソースが不正です: %v", resources)
	}
	if data, ok := app.ReadResource("index.html"); !ok || string(data) != "完成したアプリ" {
		t.Fatalf("リソースが読めません: %q, %v", data, ok)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != oldInfo.Mode().Perm() {
		t.Fatalf("実行権限が不正です: %v", info.Mode())
	}
	assertNoBuildTemps(t, dir)
}

func TestBuildReplacementFailureCleansTemp(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "app")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(out, "keep.txt")
	if err := os.WriteFile(marker, []byte("保持する内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{
		Kind: bundle.KindHTML, Entry: "index.html",
	}); err == nil {
		t.Fatal("既存フォルダへの置換失敗を期待しました")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "保持する内容" {
		t.Fatalf("既存フォルダが変更されました: %q, %v", data, err)
	}
	assertNoBuildTemps(t, dir)
}

func assertNoBuildTemps(t *testing.T, dir string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, ".gonako-build-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("一時ファイルが残っています: %v", files)
	}
}

func TestBuildPreservesExistingOutputMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("WindowsではUnixのアクセス権限を検証できません")
	}
	for _, mode := range []os.FileMode{0o700, 0o750, 0o644} {
		t.Run(mode.String(), func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "app")
			if err := os.WriteFile(out, []byte("旧成果物"), mode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(out, mode); err != nil {
				t.Fatal(err)
			}
			if err := bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{
				Kind: bundle.KindHTML, Entry: "index.html",
			}); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(out)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != mode {
				t.Fatalf("既存の権限が変わりました: 期待=%04o 実際=%04o", mode, info.Mode().Perm())
			}
		})
	}
}

func TestBuildNewOutputRespectsUmask(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("WindowsではUnixのアクセス権限を検証できません")
	}
	dir := t.TempDir()
	// プロセス全体のumaskを変更せず、通常のファイル作成で期待する権限を得る。
	reference := filepath.Join(dir, "reference")
	if err := os.WriteFile(reference, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	want, err := os.Stat(reference)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "app")
	if err := bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{
		Kind: bundle.KindHTML, Entry: "index.html",
	}); err != nil {
		t.Fatal(err)
	}
	got, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode().Perm() != want.Mode().Perm() {
		t.Fatalf("umaskが反映されていません: 期待=%04o 実際=%04o", want.Mode().Perm(), got.Mode().Perm())
	}
}

func TestBuildSkipsChainedLinksToMissingOutput(t *testing.T) {
	for _, absolute := range []bool{false, true} {
		name := "relative"
		if absolute {
			name = "absolute"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			rt := fakeRuntime(t, dir)
			out := filepath.Join(dir, "app")
			// alias1 → alias2 → alias3 → 未作成の出力先。
			for _, link := range [][2]string{{"alias1", "alias2"}, {"alias2", "alias3"}, {"alias3", "app"}} {
				target := link[1]
				if absolute {
					target = filepath.Join(dir, target)
				}
				if err := os.Symlink(target, filepath.Join(dir, link[0])); err != nil {
					t.Skipf("シンボリックリンクを作れません: %v", err)
				}
			}
			if err := bundle.BuildSpec(out, rt, bundle.Spec{
				Kind: bundle.KindHTML, Entry: "index.html", ResourceDir: dir, Flat: true,
				Skip: map[string]bool{out: true, rt: true},
			}); err != nil {
				t.Fatal(err)
			}
			app, err := bundle.Open(out)
			if err != nil {
				t.Fatal(err)
			}
			defer app.Close()
			if resources := app.Resources(); len(resources) != 0 {
				t.Fatalf("除外対象のリンクが混入しました: %v", resources)
			}
			assertNoBuildTemps(t, dir)
		})
	}
}

func TestBuildReplacesOutputSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "old-app")
	old := "リンク先の旧成果物"
	if err := os.WriteFile(target, []byte(old), 0o700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "app")
	if err := os.Symlink("old-app", out); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}
	if err := bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{Kind: bundle.KindHTML, Entry: "index.html"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(out)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("リンクが通常ファイルに置き換わっていません: %v", info.Mode())
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != old {
		t.Fatalf("リンク先が変更されました: %q, %v", data, err)
	}
	app, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	assertNoBuildTemps(t, dir)
}

func TestBuildSkipsLeftoverTempResources(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "res")
	if err := os.Mkdir(res, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{"index.html": "アプリ", ".gonako-build-abandoned": "未完成の梱包"} {
		if err := os.WriteFile(filepath.Join(res, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 出力先とリソースを同じフォルダにして、前回の残骸が混入しないか確認する。
	out := filepath.Join(res, "app")
	if err := bundle.BuildSpec(out, fakeRuntime(t, dir), bundle.Spec{
		Kind: bundle.KindHTML, Entry: "index.html", ResourceDir: res, Flat: true,
		Skip: map[string]bool{out: true},
	}); err != nil {
		t.Fatal(err)
	}
	app, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if files := app.Resources(); len(files) != 1 || files[0] != "index.html" {
		t.Fatalf("一時ファイルが混入しました: %v", files)
	}
	if data, err := os.ReadFile(filepath.Join(res, ".gonako-build-abandoned")); err != nil || string(data) != "未完成の梱包" {
		t.Fatalf("既存の残骸が変更されました: %q, %v", data, err)
	}
}
