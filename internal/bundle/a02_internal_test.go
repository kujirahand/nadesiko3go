package bundle

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// TestOpenRuntimeIgnoresOutOfRangeFooterLength は、openRuntime がフッタの
// ペイロード長を int64 へ直す前に uint64 のまま実ファイルの残りと比べる
// ことを固定する。0x8000000000000000 のような長さでも負数にせず、
// ランタイム全体をそのまま扱う (A02 のレビュー指摘)。
func TestOpenRuntimeIgnoresOutOfRangeFooterLength(t *testing.T) {
	for _, length := range []uint64{1 << 63, 1<<63 + 1, ^uint64(0)} {
		path := filepath.Join(t.TempDir(), "runtime")
		body := []byte("ランタイム本体")
		footer := make([]byte, footerSize)
		copy(footer, magic)
		binary.BigEndian.PutUint64(footer[len(magic):], length)
		if err := os.WriteFile(path, append(body, footer...), 0o755); err != nil {
			t.Fatal(err)
		}

		f, size, err := openRuntime(path)
		if err != nil {
			t.Fatalf("%#x: %v", length, err)
		}
		f.Close()
		if want := int64(len(body) + footerSize); size != want {
			t.Errorf("%#x: ランタイム長 = %d, want %d", length, size, want)
		}
	}
}

// TestOpenRuntimeStripsValidFooterPayload は、範囲内のフッタは今までどおり
// 切り詰めることを、上のテストと対にして固定する。
func TestOpenRuntimeStripsValidFooterPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime")
	body := []byte("ランタイム本体")
	payload := []byte("既存のペイロード")
	footer := make([]byte, footerSize)
	copy(footer, magic)
	binary.BigEndian.PutUint64(footer[len(magic):], uint64(len(payload)))
	data := append(append(append([]byte{}, body...), payload...), footer...)
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}

	f, size, err := openRuntime(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if want := int64(len(body)); size != want {
		t.Errorf("ランタイム長 = %d, want %d", size, want)
	}
}
