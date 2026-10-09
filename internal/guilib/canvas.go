package guilib

import (
	"fmt"
	"math"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// CanvasDrawing はWebView上のcanvasへ送る描画操作。画像ライブラリの画像とは独立する。
type CanvasDrawing struct {
	StrokeColor string    `json:"strokeColor,omitempty"`
	LineWidth   float64   `json:"lineWidth,omitempty"`
	Action      string    `json:"action"`
	Coordinates []float64 `json:"coordinates,omitempty"`
	FillColor   string    `json:"fillColor,omitempty"`
}

func (p *Plugin) canvasDraw(name, action string, count int) stdlib.Impl {
	return func(_ stdlib.Context, args []value.Value) (value.Value, error) {
		h, err := handleValue(arg(args, 0))
		if err != nil {
			return value.Undefined(), err
		}
		drawing := &CanvasDrawing{Action: action}
		if count > 0 {
			coords, ok := arg(args, 1).Array()
			if !ok || coords == nil || coords.Len() != count {
				return value.Undefined(), fmt.Errorf("『%s』の座標には%d個の数値の配列を指定してください。", name, count)
			}
			for i := 0; i < count; i++ {
				n, ok := coords.Get(i).Number()
				if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
					return value.Undefined(), fmt.Errorf("『%s』の座標には有限の数値を指定してください。", name)
				}
				if (action == "rect" && i >= 2 || action == "circle" && i == 2) && n < 0 {
					return value.Undefined(), fmt.Errorf("『%s』の幅・高さ・半径には0以上を指定してください。", name)
				}
				drawing.Coordinates = append(drawing.Coordinates, n)
			}
		}
		p.screen.mu.Lock()
		defer p.screen.mu.Unlock()
		node, err := p.screen.node(h, name)
		if err != nil {
			return value.Undefined(), err
		}
		if node.tag != "canvas" {
			return value.Undefined(), fmt.Errorf("『%s』には『キャンバス作成』などで作ったcanvasのハンドルを指定してください。", name)
		}
		drawing.LineWidth, drawing.StrokeColor = node.canvasStroke()
		drawing.FillColor = node.fillColor
		if drawing.FillColor == "" {
			drawing.FillColor = "#000000"
		}

		p.screen.operations = append(p.screen.operations, Operation{Type: "canvas", Handle: h, Canvas: drawing})
		return value.Undefined(), nil
	}
}

// canvasSetStyle はキャンバスごとに描画の線設定と塗色を保持する。描画時の操作へ値を載せる。
func (p *Plugin) canvasSetStyle(name, style string) stdlib.Impl {
	return func(_ stdlib.Context, args []value.Value) (value.Value, error) {
		h, err := handleValue(arg(args, 0))
		if err != nil {
			return value.Undefined(), err
		}
		var size float64
		if style == "width" {
			var ok bool
			size, ok = arg(args, 1).Number()
			if !ok || math.IsNaN(size) || math.IsInf(size, 0) || size <= 0 {
				return value.Undefined(), fmt.Errorf("『%s』の線の太さには0より大きい有限の数値を指定してください。", name)
			}
		}
		p.screen.mu.Lock()
		defer p.screen.mu.Unlock()
		node, err := p.screen.node(h, name)
		if err != nil {
			return value.Undefined(), err
		}
		if node.tag != "canvas" {
			return value.Undefined(), fmt.Errorf("『%s』にはcanvasのハンドルを指定してください。", name)
		}
		if style == "width" {
			node.strokeWidth = size
		} else if style == "fill" {
			node.fillColor = value.ToString(arg(args, 1))
		} else {
			node.strokeColor = value.ToString(arg(args, 1))
		}
		return value.Undefined(), nil
	}
}

// canvasStroke は全ての描画命令へ渡す線設定を返す。
func (node *screenNode) canvasStroke() (float64, string) {
	width, color := node.strokeWidth, node.strokeColor
	if width == 0 {
		width = 1
	}
	if color == "" {
		color = "#000000"
	}
	return width, color
}
