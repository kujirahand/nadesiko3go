package imagelib

import (
	"image"
	"image/draw"
	"image/png"
	"os"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// screenshot は「スクリーンショット撮影」の実体。撮影自体はOS別の
// captureScreen（screenshot_*.go）に委ね、結果をp.addで既存の画像
// ハンドルと同じ形（『画像保存』などと連携できる形）にして返す。
//
// targetが「全体」や空文字なら画面全体、それ以外はタイトルにtargetを
// 含む最初のウィンドウを撮影する。該当するウィンドウが見つからない
// 場合は画面全体と同じ意味になる。
func (p *Plugin) screenshot(_ stdlib.Context, args []value.Value) (value.Value, error) {
	target := value.ToString(arg(args, 0))
	img, err := captureScreen(target)
	if err != nil {
		return value.Undefined(), err
	}
	return p.add(img), nil
}

// decodePNGFile はcaptureScreenの各OS実装が書き出した一時PNGファイルを
// 読み込み、他の画像命令と同じ*image.RGBAへ変換する共通ヘルパー。
func decodePNGFile(filename string) (*image.RGBA, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	rgba := image.NewRGBA(image.Rect(0, 0, decoded.Bounds().Dx(), decoded.Bounds().Dy()))
	draw.Draw(rgba, rgba.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	return rgba, nil
}
