package main

import (
	"testing"
)

func TestEscapePS(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"simple", "simple"},
		{"it's a test", "it''s a test"},
		{"C:\\path\\with'quote'\\file.nako3", "C:\\path\\with''quote''\\file.nako3"},
	}

	for _, tt := range tests {
		got := escapePS(tt.input)
		if got != tt.want {
			t.Errorf("escapePS(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
