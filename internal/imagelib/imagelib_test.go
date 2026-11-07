package imagelib_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/imagelib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"golang.org/x/image/font/gofont/goregular"
)

func TestImageLibBasic(t *testing.T) {
	p := imagelib.New()
	impls := p.Impls()

	// 1. 画像新規作成 (100x80)
	size := value.ArrayValue(value.NewArray(value.Number(100), value.Number(80)))
	imgVal, err := impls["画像新規作成"](nil, []value.Value{size})
	if err != nil {
		t.Fatalf("画像新規作成 failed: %v", err)
	}
	id, ok := imgVal.Number()
	if !ok || id != 1 {
		t.Fatalf("img ID = %v, want 1", imgVal)
	}

	// 2. 幅・高さ取得
	wVal, err := impls["画像幅取得"](nil, nil)
	if err != nil || value.ToNumber(wVal) != 100 {
		t.Fatalf("width = %v, want 100", wVal)
	}
	hVal, err := impls["画像高さ取得"](nil, nil)
	if err != nil || value.ToNumber(hVal) != 80 {
		t.Fatalf("height = %v, want 80", hVal)
	}

	// 3. 背景色設定 (白)
	_, err = impls["画像背景色設定"](nil, []value.Value{value.String("白")})
	if err != nil {
		t.Fatalf("画像背景色設定 failed: %v", err)
	}

	// 点取得で白 (#FFFFFFFF) になっているか確認
	pt := value.ArrayValue(value.NewArray(value.Number(10), value.Number(10)))
	colVal, err := impls["画像点取得"](nil, []value.Value{pt})
	if err != nil {
		t.Fatalf("画像点取得 failed: %v", err)
	}
	if value.ToString(colVal) != "#FFFFFFFF" {
		t.Fatalf("pixel at (10,10) = %s, want #FFFFFFFF", value.ToString(colVal))
	}

	// 4. 点設定 (赤)
	_, err = impls["画像点設定"](nil, []value.Value{pt, value.String("赤")})
	if err != nil {
		t.Fatalf("画像点設定 failed: %v", err)
	}
	colVal, _ = impls["画像点取得"](nil, []value.Value{pt})
	if value.ToString(colVal) != "#FF0000FF" {
		t.Fatalf("pixel at (10,10) after set = %s, want #FF0000FF", value.ToString(colVal))
	}

	// 5. 矩形描画 (青)
	rect := value.ArrayValue(value.NewArray(value.Number(20), value.Number(20), value.Number(30), value.Number(30)))
	_, err = impls["画像矩形描画"](nil, []value.Value{rect, value.String("青")})
	if err != nil {
		t.Fatalf("画像矩形描画 failed: %v", err)
	}
	ptInRect := value.ArrayValue(value.NewArray(value.Number(25), value.Number(25)))
	colVal, _ = impls["画像点取得"](nil, []value.Value{ptInRect})
	if value.ToString(colVal) != "#0000FFFF" {
		t.Fatalf("pixel in rect = %s, want #0000FFFF", value.ToString(colVal))
	}

	// 6. 線描画 (緑)
	line := value.ArrayValue(value.NewArray(value.Number(0), value.Number(0), value.Number(50), value.Number(50)))
	_, err = impls["画像線描画"](nil, []value.Value{line, value.String("緑")})
	if err != nil {
		t.Fatalf("画像線描画 failed: %v", err)
	}

	// 7. 円描画 (黒)
	circle := value.ArrayValue(value.NewArray(value.Number(70), value.Number(40), value.Number(10)))
	_, err = impls["画像円描画"](nil, []value.Value{circle, value.String("黒")})
	if err != nil {
		t.Fatalf("画像円描画 failed: %v", err)
	}
	ptCenter := value.ArrayValue(value.NewArray(value.Number(70), value.Number(40)))
	colVal, _ = impls["画像点取得"](nil, []value.Value{ptCenter})
	if value.ToString(colVal) != "#000000FF" {
		t.Fatalf("circle center pixel = %s, want #000000FF", value.ToString(colVal))
	}

	// 8. 文字描画 (基本フォント)
	textOpt := value.ArrayValue(value.NewArray(value.Number(10), value.Number(70), value.Number(12), value.String("黒")))
	_, err = impls["画像文字描画"](nil, []value.Value{value.String("Hi"), textOpt})
	if err != nil {
		t.Fatalf("画像文字描画 basic font failed: %v", err)
	}

	// 9. 文字描画 (TTFフォント)
	tmpDir := t.TempDir()
	fontPath := filepath.Join(tmpDir, "goregular.ttf")
	if err := os.WriteFile(fontPath, goregular.TTF, 0644); err != nil {
		t.Fatalf("write TTF failed: %v", err)
	}
	textOptTTF := value.ArrayValue(value.NewArray(value.Number(10), value.Number(60), value.Number(14), value.String("赤"), value.String(fontPath)))
	_, err = impls["画像文字描画"](nil, []value.Value{value.String("Hello TTF"), textOptTTF})
	if err != nil {
		t.Fatalf("画像文字描画 TTF failed: %v", err)
	}

	// 10. 画像保存 (PNG, JPEG, GIF)
	pngPath := filepath.Join(tmpDir, "out.png")
	jpegPath := filepath.Join(tmpDir, "out.jpg")
	gifPath := filepath.Join(tmpDir, "out.gif")

	for _, pth := range []string{pngPath, jpegPath, gifPath} {
		_, err = impls["画像保存"](nil, []value.Value{value.String(pth)})
		if err != nil {
			t.Fatalf("画像保存 to %s failed: %v", pth, err)
		}
		fi, err := os.Stat(pth)
		if err != nil || fi.Size() == 0 {
			t.Fatalf("saved image %s is empty or missing", pth)
		}
	}

	// 11. リサイズ
	newSize := value.ArrayValue(value.NewArray(value.Number(50), value.Number(40)))
	_, err = impls["画像リサイズ"](nil, []value.Value{newSize})
	if err != nil {
		t.Fatalf("画像リサイズ failed: %v", err)
	}
	wVal, _ = impls["画像幅取得"](nil, nil)
	hVal, _ = impls["画像高さ取得"](nil, nil)
	if value.ToNumber(wVal) != 50 || value.ToNumber(hVal) != 40 {
		t.Fatalf("resized dimensions = (%v, %v), want (50, 40)", wVal, hVal)
	}

	_, _ = impls["画像閉"](nil, nil)

	// 12. 画像開く
	openVal, err := impls["画像開"](nil, []value.Value{value.String(pngPath)})
	if err != nil {
		t.Fatalf("画像開 failed: %v", err)
	}
	if value.ToNumber(openVal) != 2 {
		t.Fatalf("opened img id = %v, want 2", openVal)
	}
	wVal, _ = impls["画像幅取得"](nil, nil)
	hVal, _ = impls["画像高さ取得"](nil, nil)
	if value.ToNumber(wVal) != 100 || value.ToNumber(hVal) != 80 {
		t.Fatalf("opened PNG dimensions = (%v, %v), want (100, 80)", wVal, hVal)
	}
	_, _ = impls["画像閉"](nil, nil)
}

func TestImageLibMultipleImages(t *testing.T) {
	p := imagelib.New()
	impls := p.Impls()

	s1 := value.ArrayValue(value.NewArray(value.Number(10), value.Number(10)))
	i1, err := impls["画像新規作成"](nil, []value.Value{s1})
	if err != nil {
		t.Fatal(err)
	}

	s2 := value.ArrayValue(value.NewArray(value.Number(20), value.Number(20)))
	i2, err := impls["画像新規作成"](nil, []value.Value{s2})
	if err != nil {
		t.Fatal(err)
	}

	// 切替 i1
	_, err = impls["画像切替"](nil, []value.Value{i1})
	if err != nil {
		t.Fatal(err)
	}
	w1, _ := impls["画像幅取得"](nil, nil)
	if value.ToNumber(w1) != 10 {
		t.Fatalf("i1 width = %v, want 10", w1)
	}

	// 切替 i2
	_, err = impls["画像切替"](nil, []value.Value{i2})
	if err != nil {
		t.Fatal(err)
	}
	w2, _ := impls["画像幅取得"](nil, nil)
	if value.ToNumber(w2) != 20 {
		t.Fatalf("i2 width = %v, want 20", w2)
	}
}

func TestImageLibErrors(t *testing.T) {
	p := imagelib.New()
	impls := p.Impls()

	// 未作成状態
	_, err := impls["画像幅取得"](nil, nil)
	if err == nil {
		t.Fatal("expected error without open image")
	}

	// 無効なサイズ
	badSize := value.ArrayValue(value.NewArray(value.Number(-10), value.Number(0)))
	_, err = impls["画像新規作成"](nil, []value.Value{badSize})
	if err == nil {
		t.Fatal("expected error for negative size")
	}

	// 存在しないファイルを開く
	_, err = impls["画像開"](nil, []value.Value{value.String("no_such_file.png")})
	if err == nil {
		t.Fatal("expected error opening missing file")
	}

	// 範囲外の点取得
	size := value.ArrayValue(value.NewArray(value.Number(10), value.Number(10)))
	_, _ = impls["画像新規作成"](nil, []value.Value{size})
	outPt := value.ArrayValue(value.NewArray(value.Number(100), value.Number(100)))
	_, err = impls["画像点取得"](nil, []value.Value{outPt})
	if err == nil || !strings.Contains(err.Error(), "画像の外") {
		t.Fatalf("expected out-of-bounds error, got %v", err)
	}
}

// TestImageColorReplace は Issue #448 の『画像色置換』のテスト。
// 完全一致したピクセルだけが置換され、置換件数が返ることを確認する。
func TestImageColorReplace(t *testing.T) {
	p := imagelib.New()
	impls := p.Impls()
	replace, ok := impls["画像色置換"]
	if !ok {
		t.Fatal("『画像色置換』が登録されていません")
	}
	get := func(x, y int) string {
		t.Helper()
		pt := value.ArrayValue(value.NewArray(value.Number(float64(x)), value.Number(float64(y))))
		v, err := impls["画像点取得"](nil, []value.Value{pt})
		if err != nil {
			t.Fatalf("画像点取得(%d,%d) failed: %v", x, y, err)
		}
		return value.ToString(v)
	}
	set := func(x, y int, col string) {
		t.Helper()
		pt := value.ArrayValue(value.NewArray(value.Number(float64(x)), value.Number(float64(y))))
		if _, err := impls["画像点設定"](nil, []value.Value{pt, value.String(col)}); err != nil {
			t.Fatalf("画像点設定(%d,%d,%s) failed: %v", x, y, col, err)
		}
	}

	// 4x2 を赤で塗り、1点だけ青、1点だけアルファ違いの赤にする
	size := value.ArrayValue(value.NewArray(value.Number(4), value.Number(2)))
	if _, err := impls["画像新規作成"](nil, []value.Value{size}); err != nil {
		t.Fatalf("画像新規作成 failed: %v", err)
	}
	if _, err := impls["画像背景色設定"](nil, []value.Value{value.String("赤")}); err != nil {
		t.Fatalf("画像背景色設定 failed: %v", err)
	}
	set(0, 0, "青")
	set(1, 0, "#FF000000") // RGBは赤と同じだがアルファが違うので置換されないはず

	// Issue の用例: 赤 (#FF0000) を透明 (#00000000) に置換する。残りの赤は 8-1-1=6 点
	n, err := replace(nil, []value.Value{value.String("#FF0000"), value.String("#00000000")})
	if err != nil {
		t.Fatalf("画像色置換 failed: %v", err)
	}
	if value.ToNumber(n) != 6 {
		t.Fatalf("置換件数 = %v, want 6", n)
	}
	if got := get(2, 0); got != "#00000000" {
		t.Fatalf("置換後の点(2,0) = %s, want #00000000", got)
	}
	if got := get(3, 1); got != "#00000000" {
		t.Fatalf("置換後の点(3,1) = %s, want #00000000", got)
	}
	if got := get(0, 0); got != "#0000FFFF" {
		t.Fatalf("青い点(0,0) = %s, want #0000FFFF (変わらないはず)", got)
	}
	if got := get(1, 0); got != "#FF000000" {
		t.Fatalf("アルファ違いの点(1,0) = %s, want #FF000000 (変わらないはず)", got)
	}

	// 色名でも置換できる (青 → 黒)。#RRGGBB と #RRGGBBAA の等価もここで確認する
	n, err = replace(nil, []value.Value{value.String("青"), value.String("#000000")})
	if err != nil {
		t.Fatalf("画像色置換 (色名) failed: %v", err)
	}
	if value.ToNumber(n) != 1 {
		t.Fatalf("色名での置換件数 = %v, want 1", n)
	}
	if got := get(0, 0); got != "#000000FF" {
		t.Fatalf("置換後の点(0,0) = %s, want #000000FF", got)
	}

	// 一致するピクセルが無ければ 0 件で、画像は変わらない
	n, err = replace(nil, []value.Value{value.String("緑"), value.String("白")})
	if err != nil {
		t.Fatalf("画像色置換 (一致なし) failed: %v", err)
	}
	if value.ToNumber(n) != 0 {
		t.Fatalf("一致なしの置換件数 = %v, want 0", n)
	}

	// 置換元と置換先が同じ色なら 0 件で、画像は変わらない
	n, err = replace(nil, []value.Value{value.String("#00000000"), value.String("透明")})
	if err != nil {
		t.Fatalf("画像色置換 (同色) failed: %v", err)
	}
	if value.ToNumber(n) != 0 {
		t.Fatalf("同色の置換件数 = %v, want 0", n)
	}
	if got := get(2, 0); got != "#00000000" {
		t.Fatalf("同色置換後の点(2,0) = %s, want #00000000", got)
	}

	// 不正な色はエラーになる
	if _, err := replace(nil, []value.Value{value.String("存在しない色"), value.String("白")}); err == nil {
		t.Fatal("不正な置換元の色でエラーになりませんでした")
	}
	if _, err := replace(nil, []value.Value{value.String("白"), value.String("#XYZ")}); err == nil {
		t.Fatal("不正な置換先の色でエラーになりませんでした")
	}
}

// TestImageColorReplaceWithoutImage は、画像を開いていない状態の
// 『画像色置換』が他の画像命令と同じエラーを返すことを確認する。
func TestImageColorReplaceWithoutImage(t *testing.T) {
	p := imagelib.New()
	impls := p.Impls()
	_, err := impls["画像色置換"](nil, []value.Value{value.String("赤"), value.String("黒")})
	if err == nil || !strings.Contains(err.Error(), "画像新規作成") {
		t.Fatalf("画像なしのエラー = %v, want 『画像新規作成』への案内", err)
	}
}

// TestImageSaveUnsupportedFormatPreservesExistingFile は Issue #190 の回帰テスト。
// 未対応の拡張子で画像保存しようとしたとき、既存ファイルの内容が保持されることを確認する。
func TestImageSaveUnsupportedFormatPreservesExistingFile(t *testing.T) {
	p := imagelib.New()
	impls := p.Impls()

	size := value.ArrayValue(value.NewArray(value.Number(100), value.Number(100)))
	if _, err := impls["画像新規作成"](nil, []value.Value{size}); err != nil {
		t.Fatalf("画像新規作成に失敗しました: %v", err)
	}

	unsupportedPath := filepath.Join(t.TempDir(), "out.txt")
	want := []byte("IMPORTANT-EXISTING-CONTENT")
	if err := os.WriteFile(unsupportedPath, want, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := impls["画像保存"](nil, []value.Value{value.String(unsupportedPath)})
	if err == nil || !strings.Contains(err.Error(), "画像保存はPNG、JPEG、GIF形式に対応しています。") {
		t.Fatalf("未対応形式のエラーがありません: %v", err)
	}

	got, err := os.ReadFile(unsupportedPath)
	if err != nil {
		t.Fatalf("既存ファイルを読めません: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("既存ファイルの内容が変わりました: got %q, want %q", got, want)
	}
}
