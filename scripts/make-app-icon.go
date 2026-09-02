//go:build ignore

// なでしこ3アプリ(.app)のアイコンを作り直すためのスクリプト。
// scripts/gen-command-list.go と同じディレクトリにあるので、ビルド対象から外してある。
//
// 使い方:
//
//	go run scripts/make-app-icon.go icon.png     # 1024pxの元絵を描く
//
// macOS (.app のアイコン):
//
//	mkdir AppIcon.iconset
//	sips -z 16 16 icon.png --out AppIcon.iconset/icon_16x16.png  … 各サイズぶん
//	iconutil -c icns AppIcon.iconset -o cmd/gonako-gui/ui/macapp/Contents/Resources/AppIcon.icns
//
// Windows (.exe のアイコン):
//
//	# 各サイズのPNGを cmd/gonako-gui/app.ico にまとめ (ICOはPNGをそのまま入れられる)
//	go run github.com/akavel/rsrc@latest -ico cmd/gonako-gui/app.ico -arch amd64 \
//	    -o cmd/gonako-gui/rsrc_windows_amd64.syso
//	go run github.com/akavel/rsrc@latest -ico cmd/gonako-gui/app.ico -arch arm64 \
//	    -o cmd/gonako-gui/rsrc_windows_arm64.syso
//
// .syso はGoリンカがWindows向けビルドのときだけ取り込むので、
// 「フォルダを実行ファイルに変換」で作ったexeも自動的にこのアイコンになる。

package main

// なでしこ3アプリ用アイコン(🌸)を描く。標準ライブラリだけで完結させる。
// 2倍で描いてから縮小し、輪郭を滑らかにする。

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const (
	size  = 1024
	scale = 4 // スーパーサンプリング倍率
	big   = size * scale
)

var (
	bg     = color.NRGBA{0x1e, 0x1e, 0x2e, 0xff} // --bg-primary
	petal  = color.NRGBA{0xf3, 0x8b, 0xa8, 0xff} // --accent-pink
	petal2 = color.NRGBA{0xcb, 0xa6, 0xf7, 0xff} // --accent-mauve (下側の陰影)
	heart  = color.NRGBA{0xf9, 0xe2, 0xaf, 0xff} // --accent-yellow
)

func mix(a, b color.NRGBA, t float64) color.NRGBA {
	f := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t) }
	return color.NRGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 0xff}
}

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, big, big))
	c := float64(big) / 2
	radius := float64(big) * 0.22 // 角丸の半径

	inRounded := func(x, y float64) bool {
		w := float64(big)
		if x < radius && y < radius {
			return math.Hypot(x-radius, y-radius) <= radius
		}
		if x > w-radius && y < radius {
			return math.Hypot(x-(w-radius), y-radius) <= radius
		}
		if x < radius && y > w-radius {
			return math.Hypot(x-radius, y-(w-radius)) <= radius
		}
		if x > w-radius && y > w-radius {
			return math.Hypot(x-(w-radius), y-(w-radius)) <= radius
		}
		return true
	}

	petalDist := float64(big) * 0.205
	petalR := float64(big) * 0.185
	heartR := float64(big) * 0.115

	for y := 0; y < big; y++ {
		for x := 0; x < big; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			if !inRounded(fx, fy) {
				continue // 角の外は透明のまま
			}
			col := bg
			// 花びら5枚。真上から72度ずつ。
			for i := 0; i < 5; i++ {
				a := -math.Pi/2 + float64(i)*2*math.Pi/5
				px := c + petalDist*math.Cos(a)
				py := c + petalDist*math.Sin(a)
				if math.Hypot(fx-px, fy-py) <= petalR {
					// 下に行くほど紫を混ぜて立体感を出す
					col = mix(petal, petal2, (fy/float64(big))*0.45)
					break
				}
			}
			if math.Hypot(fx-c, fy-c) <= heartR {
				col = heart
			}
			img.SetNRGBA(x, y, col)
		}
	}

	// 縮小 (ボックスフィルタ)
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a int
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					p := img.NRGBAAt(x*scale+dx, y*scale+dy)
					r += int(p.R) * int(p.A) / 255
					g += int(p.G) * int(p.A) / 255
					b += int(p.B) * int(p.A) / 255
					a += int(p.A)
				}
			}
			n := scale * scale
			if a == 0 {
				continue
			}
			// 事前乗算を戻す
			aa := a / n
			out.SetNRGBA(x, y, color.NRGBA{
				uint8(r * 255 / a), uint8(g * 255 / a), uint8(b * 255 / a), uint8(aa),
			})
		}
	}

	f, err := os.Create(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, out); err != nil {
		panic(err)
	}
}
