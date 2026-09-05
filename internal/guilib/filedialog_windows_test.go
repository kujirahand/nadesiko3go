//go:build windows

package guilib

import "testing"

func TestWindowsDialogFilterPreservesSeparators(t *testing.T) {
	filter := windowsDialogFilter(".txt")
	nulCount := 0
	for _, c := range filter {
		if c == 0 {
			nulCount++
		}
	}
	if nulCount != 5 {
		t.Fatalf("Windows filter NUL count = %d, want 5", nulCount)
	}
}
