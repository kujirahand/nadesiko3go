package guilib

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

type canvasImageContext struct {
	stdlib.Context
	resource []byte
	reads    int
	requests []canvasImageRequest
}

func (c *canvasImageContext) ReadResource(name string) ([]byte, bool) {
	c.reads++
	return c.resource, name == "梱包.png"
}
func (c *canvasImageContext) RequestGUI(kind, message string) (string, error) {
	var request canvasImageRequest
	if err := json.Unmarshal([]byte(message), &request); err != nil {
		return "", err
	}
	c.requests = append(c.requests, request)
	return "", nil
}

func TestCanvasImageResourceAndArgumentChecks(t *testing.T) {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	ctx := &canvasImageContext{resource: data.Bytes()}
	screen := NewScreen()
	p := NewWithScreen(screen)
	h := value.Number(float64(screen.create("canvas", "", "", "", 0)))
	args := func(ns ...float64) []value.Value {
		a := value.NewArray()
		for _, n := range ns {
			a.Set(a.Len(), value.Number(n))
		}
		return []value.Value{h, value.String("梱包.png"), value.ArrayValue(a)}
	}
	if _, err := p.Impls()["キャンバス線太設定"](nil, []value.Value{h, value.Number(3)}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Impls()["キャンバス線色設定"](nil, []value.Value{h, value.String("blue")}); err != nil {
		t.Fatal(err)
	}
	draw := p.Impls()["キャンバス画像描画"]
	if _, err := draw(ctx, args(1.5, 2)); err != nil {
		t.Fatal(err)
	}
	if ctx.requests[0].LineWidth != 3 || ctx.requests[0].StrokeColor != "blue" {
		t.Fatalf("画像の線設定=%#v", ctx.requests[0])
	}
	if len(ctx.requests) != 1 || ctx.requests[0].Action != "image" || ctx.requests[0].Coordinates[0] != 1.5 {
		t.Fatalf("要求=%#v", ctx.requests)
	}
	// 座標の数、非有限値、0や負の拡大縮小サイズを画面へ送らない。
	for _, ns := range [][]float64{{0}, {0, 0, 1}, {0, math.Inf(1)}, {0, math.NaN()}, {0, 0, 0, 1}, {0, 0, 1, -1}} {
		if _, err := draw(ctx, args(ns...)); err == nil {
			t.Fatalf("不正な座標=%v", ns)
		}
	}
	ctx.resource = []byte("画像ではない")
	if _, err := draw(ctx, args(0, 0)); err == nil {
		t.Fatal("壊れた画像を読み込みました")
	}
	if len(ctx.requests) != 1 {
		t.Fatal("不正な画像や引数を画面に送りました")
	}
	if _, err := p.Impls()["キャンバス画像保存"](ctx, []value.Value{h, value.String("画像.bmp")}); err == nil {
		t.Fatal("対応外の形式で保存しました")
	}
	if len(ctx.requests) != 1 {
		t.Fatal("対応外の形式で画面に要求しました")
	}
}

func TestCanvasImageValidatesTargetBeforeReading(t *testing.T) {
	screen := NewScreen()
	p := NewWithScreen(screen)
	label := screen.create("span", "", "", "", 0)
	for _, handle := range []int{0, 999, label} {
		ctx := &canvasImageContext{}
		args := []value.Value{value.Number(float64(handle)), value.String("存在しない.png"), value.ArrayValue(value.NewArray(value.Number(0), value.Number(0)))}
		_, err := p.cmdCanvasImageDraw(ctx, args)
		if err == nil || strings.Contains(err.Error(), "開けません") || ctx.reads != 0 {
			t.Fatalf("ハンドル=%d: %v, 読込回数=%d", handle, err, ctx.reads)
		}
	}
}

func TestCanvasImageJPEGSource(t *testing.T) {
	var data bytes.Buffer
	if err := jpeg.Encode(&data, image.NewRGBA(image.Rect(0, 0, 1, 1)), nil); err != nil {
		t.Fatal(err)
	}
	ctx := &canvasImageContext{resource: data.Bytes()}
	screen := NewScreen()
	p := NewWithScreen(screen)
	h := screen.create("canvas", "", "", "", 0)
	_, err := p.cmdCanvasImageDraw(ctx, []value.Value{value.Number(float64(h)), value.String("梱包.png"), value.ArrayValue(value.NewArray(value.Number(0), value.Number(0)))})
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.requests) != 1 || !strings.HasPrefix(ctx.requests[0].Source, "data:image/jpeg;base64,") {
		t.Fatalf("JPEG要求=%#v", ctx.requests)
	}
}
