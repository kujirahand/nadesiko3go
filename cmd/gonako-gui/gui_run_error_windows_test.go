//go:build windows

package main

import (
	"testing"

	"golang.org/x/sys/windows"
)

// classifyStderrVisibility が使う数値は Win32 の FILE_TYPE_* と同じであること。
func TestStderrFileTypeConstantsMatchWindows(t *testing.T) {
	pairs := []struct {
		name string
		got  uint32
		want uint32
	}{
		{"DISK", stderrFileTypeDisk, windows.FILE_TYPE_DISK},
		{"CHAR", stderrFileTypeChar, windows.FILE_TYPE_CHAR},
		{"PIPE", stderrFileTypePipe, windows.FILE_TYPE_PIPE},
		{"REMOTE", stderrFileTypeRemote, windows.FILE_TYPE_REMOTE},
		{"UNKNOWN", stderrFileTypeUnknown, windows.FILE_TYPE_UNKNOWN},
	}
	for _, pair := range pairs {
		if pair.got != pair.want {
			t.Errorf("FILE_TYPE_%s = %#x, want %#x", pair.name, pair.got, pair.want)
		}
	}
}
