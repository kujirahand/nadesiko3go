package officelib_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/officelib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

func TestOfficeLibBasic(t *testing.T) {
	p := officelib.New()
	impls := p.Impls()

	// 1. 新規ブック作成
	v, err := impls["エクセル新規ブック"](nil, nil)
	if err != nil {
		t.Fatalf("エクセル新規ブック failed: %v", err)
	}
	id, ok := v.Number()
	if !ok || id != 1 {
		t.Fatalf("unexpected book id: %v", v)
	}

	// 2. セル設定
	_, err = impls["エクセルセル設定"](nil, []value.Value{value.String("A1"), value.String("こんにちは")})
	if err != nil {
		t.Fatalf("エクセルセル設定 failed: %v", err)
	}
	_, err = impls["エクセルセル設定"](nil, []value.Value{value.String("B1"), value.Number(123.45)})
	if err != nil {
		t.Fatalf("エクセルセル設定 failed: %v", err)
	}

	// 3. セル取得
	gotA1, err := impls["エクセルセル取得"](nil, []value.Value{value.String("A1")})
	if err != nil {
		t.Fatalf("エクセルセル取得 A1 failed: %v", err)
	}
	if value.ToString(gotA1) != "こんにちは" {
		t.Fatalf("A1 value = %v, want こんにちは", gotA1)
	}

	gotB1, err := impls["エクセルセル取得"](nil, []value.Value{value.String("B1")})
	if err != nil {
		t.Fatalf("エクセルセル取得 B1 failed: %v", err)
	}
	if n, ok := gotB1.Number(); !ok || n != 123.45 {
		t.Fatalf("B1 value = %v, want 123.45", gotB1)
	}

	// 4. 一括設定と一括取得
	rangeData := value.NewArray(
		value.ArrayValue(value.NewArray(value.String("X"), value.String("Y"))),
		value.ArrayValue(value.NewArray(value.Number(10), value.Number(20))),
	)
	_, err = impls["エクセル一括設定"](nil, []value.Value{value.String("C1"), value.ArrayValue(rangeData)})
	if err != nil {
		t.Fatalf("エクセル一括設定 failed: %v", err)
	}

	gotRange, err := impls["エクセル一括取得"](nil, []value.Value{value.String("C1"), value.String("D2")})
	if err != nil {
		t.Fatalf("エクセル一括取得 failed: %v", err)
	}
	arr, ok := gotRange.Array()
	if !ok || arr.Len() != 2 {
		t.Fatalf("gotRange invalid length: %v", gotRange)
	}
	row0, _ := arr.Get(0).Array()
	row1, _ := arr.Get(1).Array()
	if value.ToString(row0.Get(0)) != "X" || value.ToString(row0.Get(1)) != "Y" {
		t.Fatalf("row0 = %v, want [X, Y]", row0)
	}
	n1, _ := row1.Get(0).Number()
	n2, _ := row1.Get(1).Number()
	if n1 != 10 || n2 != 20 {
		t.Fatalf("row1 = %v, want [10, 20]", row1)
	}

	// 5. 保存と開く
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "test.xlsx")
	csvPath := filepath.Join(tmpDir, "test.csv")

	_, err = impls["エクセル保存"](nil, []value.Value{value.String(xlsxPath)})
	if err != nil {
		t.Fatalf("エクセル保存 failed: %v", err)
	}

	_, err = impls["エクセルCSV保存"](nil, []value.Value{value.String(csvPath)})
	if err != nil {
		t.Fatalf("エクセルCSV保存 failed: %v", err)
	}
	csvBytes, err := os.ReadFile(csvPath)
	if err != nil || len(csvBytes) == 0 {
		t.Fatalf("csv file read failed: %v", err)
	}

	_, err = impls["エクセル閉"](nil, nil)
	if err != nil {
		t.Fatalf("エクセル閉 failed: %v", err)
	}

	// 6. 再オープン
	_, err = impls["エクセル開"](nil, []value.Value{value.String(xlsxPath)})
	if err != nil {
		t.Fatalf("エクセル開 failed: %v", err)
	}
	gotReA1, err := impls["エクセルセル取得"](nil, []value.Value{value.String("A1")})
	if err != nil || value.ToString(gotReA1) != "こんにちは" {
		t.Fatalf("reopened A1 = %v, want こんにちは", gotReA1)
	}

	// 7. シート操作
	_, err = impls["エクセル新規シート"](nil, []value.Value{value.String("テストシート")})
	if err != nil {
		t.Fatalf("エクセル新規シート failed: %v", err)
	}
	sheetListVal, err := impls["エクセルシート列挙"](nil, nil)
	if err != nil {
		t.Fatalf("エクセルシート列挙 failed: %v", err)
	}
	sheetListArr, ok := sheetListVal.Array()
	if !ok || sheetListArr.Len() < 2 {
		t.Fatalf("sheet list count = %d, want >= 2", sheetListArr.Len())
	}

	_, err = impls["エクセルシート注目"](nil, []value.Value{value.String("Sheet1")})
	if err != nil {
		t.Fatalf("エクセルシート注目 failed: %v", err)
	}

	_, err = impls["エクセルセル幅設定"](nil, []value.Value{value.String("A"), value.Number(25.0)})
	if err != nil {
		t.Fatalf("エクセルセル幅設定 failed: %v", err)
	}

	_, err = impls["エクセル背景色設定"](nil, []value.Value{value.String("A1"), value.String("赤")})
	if err != nil {
		t.Fatalf("エクセル背景色設定 failed: %v", err)
	}

	_, err = impls["エクセル文字色設定"](nil, []value.Value{value.String("A1"), value.String("白")})
	if err != nil {
		t.Fatalf("エクセル文字色設定 failed: %v", err)
	}

	_, err = impls["エクセルシート削除"](nil, []value.Value{value.String("テストシート")})
	if err != nil {
		t.Fatalf("エクセルシート削除 failed: %v", err)
	}

	_, _ = impls["エクセル閉"](nil, nil)
}

func TestOfficeLibMultipleBooks(t *testing.T) {
	p := officelib.New()
	impls := p.Impls()

	b1, err := impls["エクセル新規ブック"](nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = impls["エクセルセル設定"](nil, []value.Value{value.String("A1"), value.String("Book1")})

	b2, err := impls["エクセル新規ブック"](nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = impls["エクセルセル設定"](nil, []value.Value{value.String("A1"), value.String("Book2")})

	// b1に切替
	_, err = impls["エクセルブック切替"](nil, []value.Value{b1})
	if err != nil {
		t.Fatal(err)
	}
	v1, _ := impls["エクセルセル取得"](nil, []value.Value{value.String("A1")})
	if value.ToString(v1) != "Book1" {
		t.Fatalf("b1 A1 = %v, want Book1", v1)
	}

	// b2に切替
	_, err = impls["エクセルブック切替"](nil, []value.Value{b2})
	if err != nil {
		t.Fatal(err)
	}
	v2, _ := impls["エクセルセル取得"](nil, []value.Value{value.String("A1")})
	if value.ToString(v2) != "Book2" {
		t.Fatalf("b2 A1 = %v, want Book2", v2)
	}
}

func TestOfficeLibErrors(t *testing.T) {
	p := officelib.New()
	impls := p.Impls()

	// ブックが開かれていない状態での操作
	_, err := impls["エクセルセル設定"](nil, []value.Value{value.String("A1"), value.String("test")})
	if err == nil {
		t.Fatal("expected error when no book is open")
	}

	_, err = impls["エクセル開"](nil, []value.Value{value.String("non_existent_file.xlsx")})
	if err == nil {
		t.Fatal("expected error when opening non-existent file")
	}
}
