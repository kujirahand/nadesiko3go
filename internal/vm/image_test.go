package vm_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func TestImageBasicE2E(t *testing.T) {
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "image_out.png")
	fontPath := filepath.Join(tmpDir, "goregular.ttf")

	if err := os.WriteFile(fontPath, goregular.TTF, 0644); err != nil {
		t.Fatalf("write TTF failed: %v", err)
	}

	code := strings.Join([]string{
		`[200, 150]の画像新規作成`,
		`画像幅取得を表示`,
		`画像高取得を表示`,
		`「白」に画像背景色設定`,
		`[10, 10]の画像点取得を表示`,
		`[10, 10]を「赤」に画像点設定`,
		`[10, 10]の画像点取得を表示`,
		`[0, 0, 100, 100]を「緑」で画像線描画`,
		`[20, 20, 40, 40]を「青」で画像矩形描画`,
		`[100, 100, 20]を「黄」で画像円描画`,
		`FONT=` + nakoString(fontPath),
		`「Hello Image」を[10, 130, 16, "黒", FONT]で画像文字描画`,
		`OUT=` + nakoString(pngPath),
		`OUTへ画像保存`,
		`[100, 75]に画像リサイズ`,
		`画像幅取得を表示`,
		`画像高取得を表示`,
		`画像閉`,
		`OUTを画像開`,
		`画像幅取得を表示`,
		`画像高取得を表示`,
		`画像閉`,
	}, "\n")

	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatal(err)
	}

	want := "200\n150\n#FFFFFFFF\n#FF0000FF\n100\n75\n200\n150"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}

	fi, err := os.Stat(pngPath)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("saved PNG was not created or empty: %v", err)
	}
}

func TestImageMultipleImagesE2E(t *testing.T) {
	code := strings.Join([]string{
		`I1=[50, 50]の画像新規作成`,
		`I2=[100, 100]の画像新規作成`,
		`I1に画像切替`,
		`画像幅取得を表示`,
		`画像閉`,
		`I2に画像切替`,
		`画像幅取得を表示`,
		`画像閉`,
	}, "\n")

	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatal(err)
	}

	want := "50\n100"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
