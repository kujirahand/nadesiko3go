package guilib

import "testing"

func TestNormalizeExtension(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{".txt", ".txt"},
		{"txt", ".txt"},
		{"*.nako3", ".nako3"},
		{"*.*", ""},
		{"../txt", ""},
	}
	for _, test := range tests {
		if got := normalizeExtension(test.input); got != test.want {
			t.Errorf("normalizeExtension(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestAddDefaultExtension(t *testing.T) {
	tests := []struct {
		path      string
		extension string
		want      string
	}{
		{"", ".txt", ""},
		{"program", ".txt", "program.txt"},
		{"program.nako", ".txt", "program.nako"},
		{"program", "", "program.nako3"},
	}
	for _, test := range tests {
		if got := addDefaultExtension(test.path, test.extension); got != test.want {
			t.Errorf("addDefaultExtension(%q, %q) = %q, want %q", test.path, test.extension, got, test.want)
		}
	}
}
