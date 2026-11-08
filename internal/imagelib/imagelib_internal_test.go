package imagelib

import (
	"image"
	"image/color"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// TestReplaceColorSubImage は、Bounds.Min が (0,0) でなく Stride が
// 幅×4 と一致しない画像 (SubImage) でも『画像色置換』が対象範囲だけを
// 正しく置換することを確認する (Issue #448、レビュー指摘への対応)。
// 通常の命令経路では画像は常に Min=(0,0) で作られるため、内部テストで検証する。
func TestReplaceColorSubImage(t *testing.T) {
	// 親画像 4x2 を赤で塗り、x=1..3 の部分画像を対象にする (Stride=16, 幅=3)
	parent := image.NewRGBA(image.Rect(0, 0, 4, 2))
	red := color.RGBA{R: 0xFF, A: 0xFF}
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			parent.SetRGBA(x, y, red)
		}
	}
	sub, ok := parent.SubImage(image.Rect(1, 0, 4, 2)).(*image.RGBA)
	if !ok {
		t.Fatal("SubImage が *image.RGBA ではありません")
	}
	if sub.Bounds().Min.X != 1 || sub.Stride == sub.Bounds().Dx()*4 {
		t.Fatalf("前提が崩れています: Min=%v Stride=%d 幅=%d", sub.Bounds().Min, sub.Stride, sub.Bounds().Dx())
	}

	p := New()
	p.images[1] = &canvas{img: sub}
	p.active = 1

	n, err := p.replaceColor(nil, []value.Value{value.String("赤"), value.String("透明")})
	if err != nil {
		t.Fatalf("画像色置換 failed: %v", err)
	}
	if value.ToNumber(n) != 6 {
		t.Fatalf("置換件数 = %v, want 6 (3x2)", n)
	}
	// 範囲外 (x=0 の列) は親画像でも変わらないはず
	if got := parent.RGBAAt(0, 0); got != red {
		t.Fatalf("範囲外の点(0,0) = %v, want %v (変わらないはず)", got, red)
	}
	if got := parent.RGBAAt(0, 1); got != red {
		t.Fatalf("範囲外の点(0,1) = %v, want %v (変わらないはず)", got, red)
	}
	// 範囲内は透明になっているはず
	transparent := color.RGBA{}
	if got := parent.RGBAAt(1, 0); got != transparent {
		t.Fatalf("範囲内の点(1,0) = %v, want %v", got, transparent)
	}
	if got := parent.RGBAAt(3, 1); got != transparent {
		t.Fatalf("範囲内の点(3,1) = %v, want %v", got, transparent)
	}
}

// TestReplaceColorsSubImage は、『画像色一括置換』でも SubImage の対象範囲
// だけを置換することを確認する (TestReplaceColorSubImage と同じ前提)。
func TestReplaceColorsSubImage(t *testing.T) {
	parent := image.NewRGBA(image.Rect(0, 0, 4, 2))
	red := color.RGBA{R: 0xFF, A: 0xFF}
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			parent.SetRGBA(x, y, red)
		}
	}
	sub, ok := parent.SubImage(image.Rect(1, 0, 4, 2)).(*image.RGBA)
	if !ok {
		t.Fatal("SubImage が *image.RGBA ではありません")
	}
	p := New()
	p.images[1] = &canvas{img: sub}
	p.active = 1

	d := value.NewDict()
	d.Set("赤", value.String("透明"))
	n, err := p.replaceColors(nil, []value.Value{value.DictValue(d)})
	if err != nil {
		t.Fatalf("画像色一括置換 failed: %v", err)
	}
	if value.ToNumber(n) != 6 {
		t.Fatalf("置換件数 = %v, want 6 (3x2)", n)
	}
	if got := parent.RGBAAt(0, 0); got != red {
		t.Fatalf("範囲外の点(0,0) = %v, want %v (変わらないはず)", got, red)
	}
	transparent := color.RGBA{}
	if got := parent.RGBAAt(3, 1); got != transparent {
		t.Fatalf("範囲内の点(3,1) = %v, want %v", got, transparent)
	}
}
