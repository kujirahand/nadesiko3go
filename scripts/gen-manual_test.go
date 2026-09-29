//go:build ignore

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateManualPages(t *testing.T) {
	root := t.TempDir()
	for _, plugin := range []string{"plugin_system", "plugin_node", "plugin_csv", "plugin_math", "plugin_toml"} {
		if err := os.Mkdir(filepath.Join(root, plugin), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, plugin, plugin+"命令.txt"), []byte("本家の説明\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "gonako"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(root, "gonako", "既存.txt")
	if err := os.WriteFile(existing, []byte("独自の説明\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds := []commandDoc{{Name: "既存", Plugin: "plugin_system"}, {Name: "独自", Plugin: "gonako", Desc: "独自の命令"}, {Name: "参照先なし", Plugin: "plugin_node", Desc: "Go独自の命令"}}
	for _, plugin := range []string{"plugin_system", "plugin_node", "plugin_csv", "plugin_math", "plugin_toml"} {
		cmds = append(cmds, commandDoc{Name: plugin + "命令", Plugin: plugin})
	}
	created, err := createManualPages(cmds, root)
	if err != nil {
		t.Fatal(err)
	}
	if created != len(cmds)-1 {
		t.Fatalf("作成数 = %d, 期待値 = %d", created, len(cmds)-1)
	}
	check := func(name, want string) {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, "gonako", name+".txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != want {
			t.Errorf("%s: %q, 期待値 %q", name, b, want)
		}
	}
	check("既存", "独自の説明\n")
	for _, plugin := range []string{"plugin_system", "plugin_node", "plugin_csv", "plugin_math", "plugin_toml"} {
		check(plugin+"命令", "#include("+plugin+"/"+plugin+"命令)\n")
	}
	for _, name := range []string{"独自", "参照先なし"} {
		b, err := os.ReadFile(filepath.Join(root, "gonako", name+".txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(b), "●説明\n") {
			t.Errorf("%s: 雛形になっていません: %q", name, b)
		}
	}
	created, err = createManualPages(cmds, root)
	if err != nil || created != 0 {
		t.Fatalf("再実行: 作成数 = %d, エラー = %v", created, err)
	}
}
