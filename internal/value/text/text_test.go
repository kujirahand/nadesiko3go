package text

import "testing"

func TestRuneHelpers(t *testing.T) {
	const input = "𩸽あ"
	if got := RuneLen(input); got != 2 {
		t.Fatalf("RuneLen=%d, want 2", got)
	}
	if got := RuneAt(input, 0); got != "𩸽" {
		t.Fatalf("RuneAt=%q", got)
	}
	if got := RuneSlice(input, 1, 2); got != "あ" {
		t.Fatalf("RuneSlice=%q", got)
	}
}

func TestRuneHelpersClampIndexes(t *testing.T) {
	if got := RuneSlice("abc", -1, 9); got != "abc" {
		t.Fatalf("RuneSlice=%q", got)
	}
	if got := RuneAt("abc", 3); got != "" {
		t.Fatalf("out-of-range RuneAt=%q", got)
	}
}
