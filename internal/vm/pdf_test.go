package vm_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func TestPDFBasicE2E(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "output.pdf")
	imgPath := filepath.Join(tmpDir, "test.png")
	fontPath := filepath.Join(tmpDir, "goregular.ttf")

	// テスト用画像とフォントの書き出し
	createTestPNGFile(t, imgPath, 64, 64)
	if err := os.WriteFile(fontPath, goregular.TTF, 0644); err != nil {
		t.Fatalf("write TTF failed: %v", err)
	}

	code := strings.Join([]string{
		`PDF新規作成`,
		`「テスト文書」にPDFタイトル設定`,
		`[10, 10, 100, 10]を「青」でPDF線描画`,
		`[10, 20, 50, 30]を「赤」でPDF矩形描画`,
		`IMG=` + nakoString(imgPath),
		`IMGの[70, 20, 30, 30]へPDF画像描画`,
		`""を12でPDFフォント設定`,
		`「Hello Nadesiko PDF」を[10, 60]へPDF文字描画`,
		`「Multi Line\nSecond Line」を[10, 70, 100, 40]へPDF複数行文字描画`,
		`PDFページ追加`,
		`FONT=` + nakoString(fontPath),
		`FONTを14にPDFフォント設定`,
		`「Page 2 with TTF Font」を[20, 30]へPDF文字描画`,
		`OUT=` + nakoString(pdfPath),
		`OUTへPDF保存`,
		`「PDF作成完了」と表示`,
	}, "\n")

	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatal(err)
	}

	if got != "PDF作成完了" {
		t.Fatalf("output = %q, want PDF作成完了", got)
	}

	fi, err := os.Stat(pdfPath)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("output PDF was not created or empty: %v", err)
	}
}

func TestPDFMultipleDocsE2E(t *testing.T) {
	code := strings.Join([]string{
		`D1=PDF新規作成`,
		`「Doc 1」にPDFタイトル設定`,
		`D2=PDF新規作成`,
		`「Doc 2」にPDFタイトル設定`,
		`D1にPDF切替`,
		`PDF閉`,
		`D2にPDF切替`,
		`PDF閉`,
		`「完了」と表示`,
	}, "\n")

	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatal(err)
	}

	if got != "完了" {
		t.Fatalf("output = %q, want 完了", got)
	}
}

func createTestPNGFile(t *testing.T, filename string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 0, G: 128, B: 255, A: 255})
		}
	}
	f, err := os.Create(filename)
	if err != nil {
		t.Fatalf("create test PNG failed: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode test PNG failed: %v", err)
	}
}
