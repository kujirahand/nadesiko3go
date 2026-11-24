package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGUIVersionOptions(t *testing.T) {
	if os.Getenv("GONAKO_VERSION_TEST") == "1" {
		os.Args = []string{"gonako-gui", os.Getenv("GONAKO_VERSION_ARG")}
		main()
		return
	}
	for _, arg := range []string{"-version", "--version"} {
		t.Run(arg, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestGUIVersionOptions$")
			cmd.Env = append(os.Environ(), "GONAKO_VERSION_TEST=1", "GONAKO_VERSION_ARG="+arg)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("実行失敗: %v: %s", err, out)
			}
			if !strings.Contains(string(out), guiVersionText()) {
				t.Fatalf("バージョンがありません: %q", out)
			}
		})
	}
}
