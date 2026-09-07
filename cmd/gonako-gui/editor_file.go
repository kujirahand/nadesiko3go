package main

import (
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type editorFileResult struct {
	OK        bool   `json:"ok"`
	Content   string `json:"content,omitempty"`
	IsBinary  bool   `json:"isBinary,omitempty"`
	Converted bool   `json:"converted,omitempty"`
	Encoding  string `json:"encoding,omitempty"`
	Path      string `json:"path"`
	Error     string `json:"error,omitempty"`
}

const (
	editorEncodingUTF8     = "UTF-8"
	editorEncodingShiftJIS = "Shift_JIS"
)

// readEditorFile first reports non-UTF-8 data without returning it. When the
// user explicitly allows reading it, Shift_JIS text (a common CSV encoding) is
// converted to UTF-8 so that it can be safely displayed in the HTML editor.
func readEditorFile(path string, allowNonUTF8 bool) editorFileResult {
	result := editorFileResult{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.OK = true
	if utf8.Valid(data) {
		result.Content = string(data)
		result.Encoding = editorEncodingUTF8
		return result
	}
	if !allowNonUTF8 {
		result.IsBinary = true
		return result
	}

	decoded, _, decodeErr := transform.Bytes(japanese.ShiftJIS.NewDecoder(), data)
	if decodeErr != nil {
		decoded = []byte(strings.ToValidUTF8(string(data), "�"))
		result.Encoding = "不明"
	} else {
		result.Encoding = editorEncodingShiftJIS
	}
	result.Content = string(decoded)
	result.Converted = true
	return result
}

// writeEditorFile preserves the encoding remembered when the file was read.
func writeEditorFile(path, content, encoding string) error {
	data := []byte(content)
	if encoding == editorEncodingShiftJIS {
		encoded, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), data)
		if err != nil {
			return err
		}
		data = encoded
	}
	return os.WriteFile(path, data, 0o644)
}
