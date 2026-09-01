package pdflib_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/pdflib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"golang.org/x/image/font/gofont/goregular"
)

func TestPDFLibBasic(t *testing.T) {
	p := pdflib.New()
	impls := p.Impls()

	// 1. PDF新規作成
	docVal, err := impls["PDF新規作成"](nil, nil)
	if err != nil {
		t.Fatalf("PDF新規作成 failed: %v", err)
	}
	id, ok := docVal.Number()
	if !ok || id != 1 {
		t.Fatalf("doc ID = %v, want 1", docVal)
	}

	// 2. タイトル設定
	_, err = impls["PDFタイトル設定"](nil, []value.Value{value.String("テスト文書")})
	if err != nil {
		t.Fatalf("PDFタイトル設定 failed: %v", err)
	}

	// 3. ページ追加
	_, err = impls["PDFページ追加"](nil, nil)
	if err != nil {
		t.Fatalf("PDFページ追加 failed: %v", err)
	}

	// 4. フォント設定 (標準フォント)
	_, err = impls["PDFフォント設定"](nil, []value.Value{value.String(""), value.Number(14)})
	if err != nil {
		t.Fatalf("PDFフォント設定 standard failed: %v", err)
	}

	// 5. 文字描画
	pos := value.ArrayValue(value.NewArray(value.Number(20), value.Number(30)))
	_, err = impls["PDF文字描画"](nil, []value.Value{value.String("Hello PDF"), pos})
	if err != nil {
		t.Fatalf("PDF文字描画 failed: %v", err)
	}

	// 6. 複数行文字描画
	rect := value.ArrayValue(value.NewArray(value.Number(20), value.Number(40), value.Number(100), value.Number(50)))
	_, err = impls["PDF複数行文字描画"](nil, []value.Value{value.String("Line 1\nLine 2\nLine 3"), rect})
	if err != nil {
		t.Fatalf("PDF複数行文字描画 failed: %v", err)
	}

	// 7. 線描画
	lineCoords := value.ArrayValue(value.NewArray(value.Number(10), value.Number(10), value.Number(100), value.Number(10)))
	_, err = impls["PDF線描画"](nil, []value.Value{lineCoords, value.String("青")})
	if err != nil {
		t.Fatalf("PDF線描画 failed: %v", err)
	}

	// 8. 矩形描画
	boxCoords := value.ArrayValue(value.NewArray(value.Number(20), value.Number(100), value.Number(50), value.Number(30)))
	_, err = impls["PDF矩形描画"](nil, []value.Value{boxCoords, value.String("#FFAA00")})
	if err != nil {
		t.Fatalf("PDF矩形描画 failed: %v", err)
	}

	// 9. 画像描画
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")
	createTestPNG(t, imgPath, 50, 50)

	imgRect := value.ArrayValue(value.NewArray(value.Number(80), value.Number(100), value.Number(40), value.Number(40)))
	_, err = impls["PDF画像描画"](nil, []value.Value{value.String(imgPath), imgRect})
	if err != nil {
		t.Fatalf("PDF画像描画 failed: %v", err)
	}

	// 10. TTFフォントの読み込みテスト
	fontPath := filepath.Join(tmpDir, "goregular.ttf")
	if err := os.WriteFile(fontPath, goregular.TTF, 0644); err != nil {
		t.Fatalf("write TTF failed: %v", err)
	}
	_, err = impls["PDFフォント設定"](nil, []value.Value{value.String(fontPath), value.Number(16)})
	if err != nil {
		t.Fatalf("PDFフォント設定 TTF failed: %v", err)
	}

	pos2 := value.ArrayValue(value.NewArray(value.Number(20), value.Number(160)))
	_, err = impls["PDF文字描画"](nil, []value.Value{value.String("UTF-8 Text with TTF"), pos2})
	if err != nil {
		t.Fatalf("PDF文字描画 with TTF failed: %v", err)
	}

	// 11. PDF保存
	pdfPath := filepath.Join(tmpDir, "output.pdf")
	_, err = impls["PDF保存"](nil, []value.Value{value.String(pdfPath)})
	if err != nil {
		t.Fatalf("PDF保存 failed: %v", err)
	}

	pdfData, err := os.ReadFile(pdfPath)
	if err != nil || len(pdfData) == 0 {
		t.Fatalf("saved PDF is empty or missing: %v", err)
	}
}

func TestPDFLibMultipleDocs(t *testing.T) {
	p := pdflib.New()
	impls := p.Impls()

	d1, err := impls["PDF新規作成"](nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = impls["PDFタイトル設定"](nil, []value.Value{value.String("Doc1")})

	d2, err := impls["PDF新規作成"](nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = impls["PDFタイトル設定"](nil, []value.Value{value.String("Doc2")})

	// 切替
	_, err = impls["PDF切替"](nil, []value.Value{d1})
	if err != nil {
		t.Fatalf("PDF切替 to d1 failed: %v", err)
	}
	_, err = impls["PDF閉"](nil, nil)
	if err != nil {
		t.Fatalf("PDF閉 d1 failed: %v", err)
	}

	_, err = impls["PDF切替"](nil, []value.Value{d2})
	if err != nil {
		t.Fatalf("PDF切替 to d2 failed: %v", err)
	}
	_, err = impls["PDF閉"](nil, nil)
	if err != nil {
		t.Fatalf("PDF閉 d2 failed: %v", err)
	}
}

func TestPDFLibErrors(t *testing.T) {
	p := pdflib.New()
	impls := p.Impls()

	// 未作成状態での操作
	_, err := impls["PDF文字描画"](nil, []value.Value{value.String("text"), value.ArrayValue(value.NewArray(value.Number(0), value.Number(0)))})
	if err == nil {
		t.Fatal("expected error without active PDF")
	}

	_, err = impls["PDF切替"](nil, []value.Value{value.Number(999)})
	if err == nil {
		t.Fatal("expected error with invalid doc id")
	}
}

func createTestPNG(t *testing.T, filename string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
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
