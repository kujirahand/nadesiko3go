package main

import (
	"fmt"
	"io"
	"os"
	"testing"
)

func TestChooseGUIRunErrorDestination(t *testing.T) {
	tests := []struct {
		name          string
		stderrVisible bool
		parentConsole bool
		want          guiRunErrorDestination
	}{
		{"標準エラーが読める", true, false, guiRunErrorToStderr},
		{"親コンソールへ接続できる", false, true, guiRunErrorToStderr},
		{"両方ある", true, true, guiRunErrorToStderr},
		{"宛先が無い", false, false, guiRunErrorToDialog},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chooseGUIRunErrorDestination(tt.stderrVisible, tt.parentConsole)
			if got != tt.want {
				t.Fatalf("届け先 = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyStderrVisibility(t *testing.T) {
	const remote = stderrFileTypeRemote
	tests := []struct {
		name      string
		valid     bool
		fileType  uint32
		isConsole bool
		want      bool
	}{
		{"無効なハンドル", false, stderrFileTypePipe, true, false},
		{"パイプ", true, stderrFileTypePipe, false, true},
		{"ファイル", true, stderrFileTypeDisk, false, true},
		{"コンソール", true, stderrFileTypeChar, true, true},
		{"コンソールではない文字デバイス", true, stderrFileTypeChar, false, false},
		{"不明", true, stderrFileTypeUnknown, false, false},
		{"遠隔パイプ", true, stderrFileTypePipe | remote, false, true},
		{"遠隔コンソール", true, stderrFileTypeChar | remote, true, true},
		{"遠隔の非コンソール文字デバイス", true, stderrFileTypeChar | remote, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyStderrVisibility(tt.valid, tt.fileType, tt.isConsole)
			if got != tt.want {
				t.Fatalf("見える = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGUIRunErrorText(t *testing.T) {
	got := guiRunErrorText(fmt.Errorf("実行する .nako3 ファイルを1つ指定してください"))
	want := "実行する .nako3 ファイルを1つ指定してください\n使い方: gonako-gui run main.nako3"
	if got != want {
		t.Fatalf("文面 = %q, want %q", got, want)
	}
}

// 標準エラーへ書けるときはダイアログを出さず、エラーと使い方を1つの文面で書く。
func TestReportGUIRunErrorWritesUsageToStderr(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origShow := showGUIRunErrorDialog
	origErr := os.Stderr
	dialogText := ""
	showGUIRunErrorDialog = func(text string) { dialogText = text }
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origErr
		showGUIRunErrorDialog = origShow
		_ = r.Close()
		_ = w.Close()
	})

	reportGUIRunError(fmt.Errorf("指定されたパス『main.nako3』が見つかりません"))
	os.Stderr = origErr
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if dialogText != "" {
		t.Fatalf("標準エラーへ書けたのにダイアログを出した: %s", dialogText)
	}
	want := "指定されたパス『main.nako3』が見つかりません\n使い方: gonako-gui run main.nako3\n"
	if string(got) != want {
		t.Fatalf("標準エラー = %q, want %q", got, want)
	}
}
