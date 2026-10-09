package guilib

import (
	"math"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

func TestCanvasCommands(t *testing.T) {
	screen := NewScreen()
	reg := stdlib.NewRegistry(NewWithScreen(screen))
	code := `絵=[320,200]のキャンバス作成
小さい絵=[100,100]のキャンバス作成
絵に「red」をキャンバス線色設定
絵の[1.5,2,30,40]へキャンバス線描画
小さい絵に「#008000」をキャンバス塗色設定
小さい絵の[0,0,20,30]へキャンバス矩形描画
絵に「orange」をキャンバス塗色設定
絵の[100,80,10]にキャンバス円描画
小さい絵をキャンバス消去`
	if err := vm.RunWithHostAndRegistry(code, "canvas.nako3", reg, vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)); err != nil {
		t.Fatal(err)
	}
	var drawings []Operation
	for _, op := range screen.DrainOperations() {
		if op.Type == "canvas" {
			drawings = append(drawings, op)
		}
	}
	if len(drawings) != 4 {
		t.Fatalf("描画操作 = %#v", drawings)
	}
	for i, action := range []string{"line", "rect", "circle", "clear"} {
		op := drawings[i]
		if op.Canvas.Action != action || op.Handle != []int{1, 2, 1, 2}[i] {
			t.Fatalf("描画操作%d = %#v", i, op)
		}
	}
	if drawings[0].Canvas.Coordinates[0] != 1.5 || drawings[0].Canvas.StrokeColor != "red" {
		t.Fatalf("直線 = %#v", drawings[0].Canvas)
	}
}

func TestCanvasRejectsInvalidArguments(t *testing.T) {
	screen := NewScreen()
	p := NewWithScreen(screen)
	canvas := screen.create("canvas", "", "", "", 0)
	label := screen.create("span", "", "", "", 0)
	removed := screen.create("canvas", "", "", "", 0)
	if _, err := screen.remove(removed, "DOM部品削除"); err != nil {
		t.Fatal(err)
	}
	coords := func(ns ...float64) value.Value {
		a := value.NewArray()
		for _, n := range ns {
			a.Set(a.Len(), value.Number(n))
		}
		return value.ArrayValue(a)
	}
	for _, tc := range []struct {
		name   string
		handle int
		points value.Value
	}{
		{"キャンバス線描画", canvas, coords(0, 0, 1)},
		{"キャンバス線描画", canvas, coords(0, 0, 1, math.Inf(1))},
		{"キャンバス線描画", canvas, coords(0, 0, 1, math.NaN())},
		{"キャンバス矩形描画", canvas, coords(0, 0, -1, 10)},
		{"キャンバス円描画", canvas, coords(0, 0, -1)},
		{"キャンバス消去", label, value.Undefined()},
		{"キャンバス消去", removed, value.Undefined()},
		{"キャンバス消去", 999, value.Undefined()},
		{"キャンバス消去", 0, value.Undefined()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			screen.DrainOperations()
			_, err := p.Impls()[tc.name](nil, []value.Value{value.Number(float64(tc.handle)), tc.points})
			if err == nil {
				t.Fatal("不正な引数で成功しました")
			}
			if len(screen.DrainOperations()) != 0 {
				t.Fatal("エラー時に描画操作を送っています")
			}
		})
	}
}

func TestCanvasStrokeSettings(t *testing.T) {
	screen := NewScreen()
	reg := stdlib.NewRegistry(NewWithScreen(screen))
	code := `絵=[100,100]のキャンバス作成
小さい絵=[50,50]のキャンバス作成
絵に2.5をキャンバス線太設定
絵に「red」をキャンバス線色設定
絵に「white」をキャンバス塗色設定
絵の[20,20,30,30]へキャンバス矩形描画
絵に「orange」をキャンバス塗色設定
絵の[50,50,10]へキャンバス円描画
絵の[0,0,10,10]へキャンバス線描画
小さい絵の[0,0,10,10]へキャンバス線描画
絵に「blue」をキャンバス線色設定
絵の[0,0,10,10]へキャンバス線描画
絵をキャンバス消去
絵の[0,0,10,10]へキャンバス線描画
絵に空をキャンバス線色設定
絵の[0,0,10,10]へキャンバス線描画`
	if err := vm.RunWithHostAndRegistry(code, "stroke.nako3", reg, vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)); err != nil {
		t.Fatal(err)
	}
	var lines []*CanvasDrawing
	for _, op := range screen.DrainOperations() {
		if op.Canvas != nil && (op.Canvas.Action == "rect" || op.Canvas.Action == "circle") {
			if op.Canvas.LineWidth != 2.5 || op.Canvas.StrokeColor != "red" {
				t.Fatalf("図形の線設定=%#v", op.Canvas)
			}
		}
		if op.Canvas != nil && op.Canvas.Action == "line" {
			lines = append(lines, op.Canvas)
		}
	}
	if len(lines) != 5 {
		t.Fatalf("直線=%#v", lines)
	}
	for i, want := range []struct {
		width float64
		color string
	}{{2.5, "red"}, {1, "#000000"}, {2.5, "blue"}, {2.5, "blue"}, {2.5, "#000000"}} {
		if lines[i].LineWidth != want.width || lines[i].StrokeColor != want.color {
			t.Fatalf("直線%d=%#v", i, lines[i])
		}
	}
}

func TestCanvasStrokeRejectsInvalidSettings(t *testing.T) {
	screen := NewScreen()
	p := NewWithScreen(screen)
	h := value.Number(float64(screen.create("canvas", "", "", "", 0)))
	label := value.Number(float64(screen.create("span", "", "", "", 0)))
	for _, width := range []value.Value{value.Number(0), value.Number(-1), value.Number(math.NaN()), value.Number(math.Inf(1)), value.String("5")} {
		if _, err := p.Impls()["キャンバス線太設定"](nil, []value.Value{h, width}); err == nil {
			t.Fatalf("不正な線太=%#v", width)
		}
	}
	for _, name := range []string{"キャンバス線太設定", "キャンバス線色設定", "キャンバス塗色設定"} {
		for _, handle := range []value.Value{label, value.Number(999), value.Number(0)} {
			if _, err := p.Impls()[name](nil, []value.Value{handle, value.Number(5)}); err == nil {
				t.Fatalf("不正な対象=%#v", handle)
			}
		}
	}
}

func TestCanvasFillSettings(t *testing.T) {
	screen := NewScreen()
	reg := stdlib.NewRegistry(NewWithScreen(screen))
	code := `絵=[100,100]のキャンバス作成
別絵=[100,100]のキャンバス作成
絵に「red」をキャンバス線色設定
絵に「blue」をキャンバス塗色設定
絵の[10,10,20,20]へキャンバス矩形描画
別絵の[10,10,20,20]にキャンバス矩形描画
絵をキャンバス消去
絵の[50,50,10]にキャンバス円描画
絵の[0,0,10,10]へキャンバス線描画
絵に空をキャンバス塗色設定
絵の[10,10,20,20]へキャンバス矩形描画`
	if err := vm.RunWithHostAndRegistry(code, "fill.nako3", reg, vm.NewCUIHost(&strings.Builder{}, strings.NewReader(""), nil)); err != nil {
		t.Fatal(err)
	}
	var drawings []*CanvasDrawing
	for _, op := range screen.DrainOperations() {
		if op.Canvas != nil && op.Canvas.Action != "clear" {
			drawings = append(drawings, op.Canvas)
		}
	}
	if len(drawings) != 5 {
		t.Fatalf("描画=%#v", drawings)
	}
	for i, want := range []struct{ fill, stroke string }{{"blue", "red"}, {"#000000", "#000000"}, {"blue", "red"}, {"blue", "red"}, {"#000000", "red"}} {
		if drawings[i].FillColor != want.fill || drawings[i].StrokeColor != want.stroke {
			t.Fatalf("描画%d=%#v", i, drawings[i])
		}
	}
	for _, name := range []string{"キャンバス線描画", "キャンバス矩形描画", "キャンバス円描画"} {
		if len(reg.FuncList()[name].Josi) != 2 {
			t.Fatalf("%sの引数数=%d", name, len(reg.FuncList()[name].Josi))
		}
	}
}
