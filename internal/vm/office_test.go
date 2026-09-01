package vm_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficeSpreadsheetE2E(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "sample.xlsx")
	csvPath := filepath.Join(tmpDir, "sample.csv")

	code := strings.Join([]string{
		`エクセル新規ブック`,
		`「A1」へ「果物」をエクセルセル設定`,
		`「B1」へ「個数」をエクセルセル設定`,
		`「A2」へ「りんご」をエクセル設定`,
		`「B2」へ10をエクセル設定`,
		`「A3」へ「みかん」をエクセル設定`,
		`「B3」へ20をエクセル設定`,
		`V1=「A2」のエクセルセル取得`,
		`V1を表示`,
		`V2=「B2」のエクセル取得`,
		`V2を表示`,
		`DATA=「A1」から「B3」までのエクセル一括取得`,
		`DATA[2][0]を表示`,
		`DATA[2][1]を表示`,
		`XLSX=` + nakoString(xlsxPath),
		`XLSXへエクセル保存`,
		`CSV=` + nakoString(csvPath),
		`CSVへエクセルCSV保存`,
		`エクセル閉`,
		`XLSXをエクセル開`,
		`RE_A3=「A3」のエクセルセル取得`,
		`RE_A3を表示`,
		`エクセル閉`,
	}, "\n")

	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatal(err)
	}

	want := "りんご\n10\nみかん\n20\nみかん"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestOfficeBatchSetAndSheetsE2E(t *testing.T) {
	code := strings.Join([]string{
		`エクセル新規ブック`,
		`ARR=[["氏名","点数"],["太郎",90],["花子",95]]`,
		`「A1」へARRをエクセル一括設定`,
		`「集計シート」のエクセル新規シート`,
		`「A1」へ「総合」をエクセル設定`,
		`L=エクセルシート列挙`,
		`L[0]を表示`,
		`L[1]を表示`,
		`「Sheet1」のエクセルシート注目`,
		`R=「B3」のエクセルセル取得`,
		`Rを表示`,
		`「A1:B1」を「赤」にエクセル背景色設定`,
		`「A1:B1」を「白」にエクセル文字色設定`,
		`1を30にエクセルセル幅設定`,
		`「集計シート」のエクセルシート削除`,
		`L2=エクセルシート列挙`,
		`L2の要素数を表示`,
		`エクセル閉`,
	}, "\n")

	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatal(err)
	}

	want := "Sheet1\n集計シート\n95\n1"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestOfficeMultipleBooksE2E(t *testing.T) {
	code := strings.Join([]string{
		`B1=エクセル新規ブック`,
		`「A1」へ「Book1のデータ」をエクセル設定`,
		`B2=エクセル新規ブック`,
		`「A1」へ「Book2のデータ」をエクセル設定`,
		`B1にエクセルブック切替`,
		`「A1」のエクセルセル取得を表示`,
		`エクセル閉`,
		`B2にエクセルブック切替`,
		`「A1」のエクセルセル取得を表示`,
		`エクセル閉`,
	}, "\n")

	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatal(err)
	}

	want := "Book1のデータ\nBook2のデータ"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
