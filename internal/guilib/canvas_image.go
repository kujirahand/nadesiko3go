package guilib

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// canvasImageRequest は画像読み込みや画面のPNG取得の完了を待つ要求。
type canvasImageRequest struct {
	LineWidth   float64   `json:"lineWidth,omitempty"`
	StrokeColor string    `json:"strokeColor,omitempty"`
	Handle      int       `json:"handle"`
	Action      string    `json:"action"`
	Source      string    `json:"source,omitempty"`
	Coordinates []float64 `json:"coordinates,omitempty"`
}

func (p *Plugin) prepareCanvasRequest(name string, target value.Value, request canvasImageRequest) (canvasImageRequest, error) {
	h, err := handleValue(target)
	if err != nil {
		return request, err
	}
	p.screen.mu.Lock()
	node, err := p.screen.node(h, name)
	if err == nil && node.tag != "canvas" {
		err = fmt.Errorf("『%s』にはcanvasのハンドルを指定してください。", name)
	}
	if err == nil {
		request.LineWidth, request.StrokeColor = node.canvasStroke()
	}
	p.screen.mu.Unlock()
	if err != nil {
		return request, err
	}
	request.Handle = h
	return request, nil
}

func (p *Plugin) requestCanvas(ctx stdlib.Context, name string, target value.Value, request canvasImageRequest) (string, error) {
	request, err := p.prepareCanvasRequest(name, target, request)
	if err != nil {
		return "", err
	}
	gui, ok := ctx.(stdlib.GUIRequestContext)
	if !ok {
		return "", fmt.Errorf("『%s』はgonako-guiのウィンドウモードで実行してください。", name)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	answer, err := gui.RequestGUI("canvas", string(payload))
	if err != nil {
		return "", fmt.Errorf("『%s』: %w", name, err)
	}
	return answer, nil
}

func (p *Plugin) cmdCanvasImageDraw(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	request, err := p.prepareCanvasRequest("キャンバス画像描画", arg(args, 0), canvasImageRequest{Action: "image"})
	if err != nil {
		return value.Undefined(), err
	}
	coords, ok := arg(args, 2).Array()
	if !ok || coords == nil || (coords.Len() != 2 && coords.Len() != 4) {
		return value.Undefined(), fmt.Errorf("『キャンバス画像描画』には座標[X,Y]または[X,Y,幅,高さ]を指定してください。")
	}
	for i := 0; i < coords.Len(); i++ {
		n, ok := coords.Get(i).Number()
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) || (i >= 2 && n <= 0) {
			return value.Undefined(), fmt.Errorf("『キャンバス画像描画』の座標は有限の数値、幅・高さは0より大きい数値にしてください。")
		}
		request.Coordinates = append(request.Coordinates, n)
	}
	name := value.ToString(arg(args, 1))
	var data []byte
	if ctx != nil {
		data, ok = ctx.ReadResource(name)
	} else {
		ok = false
	}
	if !ok {
		var err error
		data, err = os.ReadFile(name)
		if err != nil {
			return value.Undefined(), fmt.Errorf("画像『%s』を開けません: %w", name, err)
		}
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return value.Undefined(), fmt.Errorf("画像『%s』を読めません: %w", name, err)
	}
	mime := "image/" + format
	request.Source = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	_, err = p.requestCanvas(ctx, "キャンバス画像描画", arg(args, 0), request)
	return value.Undefined(), err
}

func (p *Plugin) cmdCanvasImageSave(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	name := value.ToString(arg(args, 1))
	ext := strings.ToLower(filepath.Ext(name))
	if ext != "" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" {
		return value.Undefined(), fmt.Errorf("『キャンバス画像保存』はPNG・JPEG・GIF形式に対応しています。")
	}
	answer, err := p.requestCanvas(ctx, "キャンバス画像保存", arg(args, 0), canvasImageRequest{Action: "save"})
	if err != nil {
		return value.Undefined(), err
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(answer, prefix) {
		return value.Undefined(), fmt.Errorf("キャンバスからPNG画像を取得できませんでした。")
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(answer, prefix))
	if err != nil {
		return value.Undefined(), fmt.Errorf("キャンバス画像のデータが不正です: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return value.Undefined(), fmt.Errorf("キャンバス画像を読めません: %w", err)
	}
	var output bytes.Buffer
	switch ext {
	case ".jpg", ".jpeg":
		err = jpeg.Encode(&output, img, &jpeg.Options{Quality: 90})
	case ".gif":
		err = gif.Encode(&output, img, nil)
	default:
		err = png.Encode(&output, img)
	}
	if err != nil {
		return value.Undefined(), err
	}
	if err = os.WriteFile(name, output.Bytes(), 0644); err != nil {
		return value.Undefined(), fmt.Errorf("画像『%s』を保存できません: %w", name, err)
	}
	return value.Undefined(), nil
}
