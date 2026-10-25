package main

import (
	"fmt"
	"reflect"
	"testing"
)

// lookPathFound は指定した名前だけが見つかる lookPath を返します。
func lookPathFound(names ...string) func(string) (string, error) {
	return func(name string) (string, error) {
		for _, n := range names {
			if n == name {
				return "/usr/bin/" + name, nil
			}
		}
		return "", fmt.Errorf("not found: %s", name)
	}
}

func TestTerminalSpecForDarwin(t *testing.T) {
	spec, err := terminalSpecFor("darwin", "/tmp/work", lookPathFound())
	if err != nil {
		t.Fatal(err)
	}
	want := terminalSpec{Name: "open", Args: []string{"-a", "Terminal", "/tmp/work"}}
	if !reflect.DeepEqual(spec, want) {
		t.Fatalf("spec = %+v, want %+v", spec, want)
	}
}

func TestTerminalSpecForWindowsPrefersWindowsTerminal(t *testing.T) {
	spec, err := terminalSpecFor("windows", `C:\work`, lookPathFound("wt.exe"))
	if err != nil {
		t.Fatal(err)
	}
	want := terminalSpec{Name: "wt.exe", Args: []string{"-d", `C:\work`}}
	if !reflect.DeepEqual(spec, want) {
		t.Fatalf("spec = %+v, want %+v", spec, want)
	}
}

func TestTerminalSpecForWindowsFallsBackToPowerShell(t *testing.T) {
	spec, err := terminalSpecFor("windows", `C:\work`, lookPathFound())
	if err != nil {
		t.Fatal(err)
	}
	// パスはコマンド文字列へ埋め込まず、作業フォルダとして渡すこと
	want := terminalSpec{Name: "powershell.exe", Args: []string{"-NoExit"}, Dir: `C:\work`}
	if !reflect.DeepEqual(spec, want) {
		t.Fatalf("spec = %+v, want %+v", spec, want)
	}
}

func TestTerminalSpecForLinux(t *testing.T) {
	spec, err := terminalSpecFor("linux", "/tmp/work", lookPathFound("konsole"))
	if err != nil {
		t.Fatal(err)
	}
	want := terminalSpec{Name: "konsole", Args: []string{"--workdir", "/tmp/work"}}
	if !reflect.DeepEqual(spec, want) {
		t.Fatalf("spec = %+v, want %+v", spec, want)
	}

	if _, err := terminalSpecFor("linux", "/tmp/work", lookPathFound()); err == nil {
		t.Fatal("ターミナルが無い場合はエラーになるべきです")
	}
}

func TestTerminalSpecForUnsupportedOS(t *testing.T) {
	if _, err := terminalSpecFor("plan9", "/tmp/work", lookPathFound("xterm")); err == nil {
		t.Fatal("未対応OSではエラーになるべきです")
	}
}

func TestOpenTerminalRejectsMissingDir(t *testing.T) {
	if err := openTerminal(t.TempDir() + "/no-such-dir"); err == nil {
		t.Fatal("存在しないフォルダではエラーになるべきです")
	}
}
