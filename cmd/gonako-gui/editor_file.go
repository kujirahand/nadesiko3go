package main

import (
	"bytes"
	"os"
	"unicode"
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
	editorEncodingEUCJP    = "EUC-JP"
	editorEncodingBinary   = "バイナリ"
)

// binarySignatures は先頭バイト列だけでバイナリと判断できる代表的な形式。
// PNG・JPEG・実行ファイルなどは、テキストとして開かせない。
var binarySignatures = [][]byte{
	{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, // PNG
	{0xFF, 0xD8, 0xFF},                            // JPEG
	{'G', 'I', 'F', '8'},                          // GIF
	{'B', 'M'},                                    // BMP
	{'%', 'P', 'D', 'F'},                          // PDF
	{'P', 'K', 0x03, 0x04},                        // ZIP (xlsx/docx等も含む)
	{0x1F, 0x8B},                                  // gzip
	{'R', 'a', 'r', '!'},                          // RAR
	{0xFD, '7', 'z', 'X', 'Z'},                    // xz
	{'7', 'z', 0xBC, 0xAF, 0x27, 0x1C},            // 7z
	{'M', 'Z'},                                    // Windows実行ファイル
	{0x7F, 'E', 'L', 'F'},                         // ELF
	{0xCF, 0xFA, 0xED, 0xFE},                      // Mach-O (64bit LE)
	{0xCE, 0xFA, 0xED, 0xFE},                      // Mach-O (32bit LE)
	{0xCA, 0xFE, 0xBA, 0xBE},                      // Mach-O fat / Java class
	{'R', 'I', 'F', 'F'},                          // WAV/AVI/WebP
	{'O', 'g', 'g', 'S'},                          // Ogg
	{'I', 'D', '3'},                               // MP3
	{0x00, 0x00, 0x01, 0x00},                      // ICO
	{'S', 'Q', 'L', 'i', 't', 'e', ' ', 'f'},      // SQLite
	{0xD0, 0xCF, 0x11, 0xE0},                      // 旧Office (doc/xls)
}

// binarySniffLen はバイナリ判定で調べる先頭バイト数。
const binarySniffLen = 8000

// looksBinary は、テキストとして開いてはいけないデータかどうかを判定する。
func looksBinary(data []byte) bool {
	for _, sig := range binarySignatures {
		if bytes.HasPrefix(data, sig) {
			return true
		}
	}
	head := data
	if len(head) > binarySniffLen {
		head = head[:binarySniffLen]
	}
	// NULバイトを含むものはテキストではないとみなす（UTF-16やPNGなど）
	if bytes.IndexByte(head, 0x00) >= 0 {
		return true
	}
	// 制御文字が多いものもバイナリ扱いにする
	control := 0
	for _, b := range head {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			control++
		}
	}
	return len(head) > 0 && control*100/len(head) >= 5
}

// readEditorFile はファイルを読み込み、扱い方を決める。
//   - バイナリ（PNG/JPEG/実行ファイルなど）… 内容を返さず読み取り専用にする
//   - Shift_JIS / EUC-JP … 確認なしでUTF-8へ変換し、文字コードを記録する
//   - UTF-8 … そのまま編集できる
func readEditorFile(path string) editorFileResult {
	result := editorFileResult{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.OK = true
	if looksBinary(data) {
		result.IsBinary = true
		result.Encoding = editorEncodingBinary
		return result
	}
	if utf8.Valid(data) {
		result.Content = string(data)
		result.Encoding = editorEncodingUTF8
		return result
	}
	if enc, text, ok := decodeJapanese(data); ok {
		result.Content = text
		result.Encoding = enc
		result.Converted = true
		return result
	}
	// どの文字コードでも読めないものはバイナリとして扱う
	result.IsBinary = true
	result.Encoding = editorEncodingBinary
	return result
}

// decodeJapanese はShift_JISとEUC-JPを試し、より日本語らしい方を選ぶ。
func decodeJapanese(data []byte) (encoding string, text string, ok bool) {
	type candidate struct {
		name  string
		text  string
		score int
	}
	var best *candidate
	if validShiftJIS(data) {
		if decoded, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), data); err == nil {
			c := candidate{name: editorEncodingShiftJIS, text: string(decoded), score: japaneseScore(string(decoded))}
			best = &c
		}
	}
	if validEUCJP(data) {
		if decoded, _, err := transform.Bytes(japanese.EUCJP.NewDecoder(), data); err == nil {
			score := japaneseScore(string(decoded))
			// 同点ならWindowsで多いShift_JISを優先する
			if best == nil || score > best.score {
				c := candidate{name: editorEncodingEUCJP, text: string(decoded), score: score}
				best = &c
			}
		}
	}
	if best == nil {
		return "", "", false
	}
	return best.name, best.text, true
}

// japaneseScore は日本語の文字が多いほど高い点数を返す。
func japaneseScore(s string) int {
	score := 0
	for _, r := range s {
		switch {
		case r == utf8.RuneError:
			score -= 10
		case r >= 0x3040 && r <= 0x30FF: // ひらがな・カタカナ
			score += 3
		case unicode.Is(unicode.Han, r):
			score += 2
		case r >= 0xFF01 && r <= 0xFF60: // 全角記号・英数
			score++
		case r >= 0xFF61 && r <= 0xFF9F: // 半角カタカナ
			score++
		case r >= 0xE000 && r <= 0xF8FF: // 私用領域は文字化けの疑い
			score -= 5
		}
	}
	return score
}

// validShiftJIS はバイト列がShift_JISとして矛盾しないかを調べる。
func validShiftJIS(data []byte) bool {
	for i := 0; i < len(data); {
		b := data[i]
		switch {
		case b <= 0x7F:
			i++
		case b >= 0xA1 && b <= 0xDF: // 半角カタカナ
			i++
		case (b >= 0x81 && b <= 0x9F) || (b >= 0xE0 && b <= 0xFC):
			if i+1 >= len(data) {
				return false
			}
			t := data[i+1]
			if t < 0x40 || t > 0xFC || t == 0x7F {
				return false
			}
			i += 2
		default:
			return false
		}
	}
	return true
}

// validEUCJP はバイト列がEUC-JPとして矛盾しないかを調べる。
func validEUCJP(data []byte) bool {
	for i := 0; i < len(data); {
		b := data[i]
		switch {
		case b <= 0x7F:
			i++
		case b == 0x8E: // 半角カタカナ
			if i+1 >= len(data) || data[i+1] < 0xA1 || data[i+1] > 0xDF {
				return false
			}
			i += 2
		case b == 0x8F: // 補助漢字
			if i+2 >= len(data) {
				return false
			}
			if data[i+1] < 0xA1 || data[i+1] > 0xFE || data[i+2] < 0xA1 || data[i+2] > 0xFE {
				return false
			}
			i += 3
		case b >= 0xA1 && b <= 0xFE:
			if i+1 >= len(data) || data[i+1] < 0xA1 || data[i+1] > 0xFE {
				return false
			}
			i += 2
		default:
			return false
		}
	}
	return true
}

// writeEditorFile はファイルを読んだときの文字コードを保って保存する。
func writeEditorFile(path, content, encoding string) error {
	data := []byte(content)
	switch encoding {
	case editorEncodingShiftJIS:
		encoded, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), data)
		if err != nil {
			return err
		}
		data = encoded
	case editorEncodingEUCJP:
		encoded, _, err := transform.Bytes(japanese.EUCJP.NewEncoder(), data)
		if err != nil {
			return err
		}
		data = encoded
	}
	return os.WriteFile(path, data, 0o644)
}
