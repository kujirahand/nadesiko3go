// Package imagelib implements deterministic raster image creation commands.
// Images are kept behind numeric handles and can be loaded from or saved to
// PNG and JPEG files.
package imagelib

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type canvas struct{ img *image.RGBA }

// Plugin owns images for one runtime.
type Plugin struct {
	images map[int64]*canvas
	active int64
	next   int64
}

func New() *Plugin { return &Plugin{images: map[int64]*canvas{}} }

type command struct {
	josi       [][]string
	returnNone bool
	fn         stdlib.Impl
}

func (p *Plugin) FuncList() lexer.FuncList {
	list := lexer.FuncList{}
	for name, cmd := range p.commands() {
		list[name] = &lexer.FuncItem{Name: name, Type: "func", Josi: cmd.josi, Pure: false, ReturnNone: cmd.returnNone}
	}
	return list
}

func (p *Plugin) Impls() map[string]stdlib.Impl {
	out := map[string]stdlib.Impl{}
	for name, cmd := range p.commands() {
		out[name] = cmd.fn
	}
	return out
}

func (p *Plugin) commands() map[string]command {
	return map[string]command{
		"画像新規作成":  {josi: [][]string{{"の", "で"}}, fn: p.create},
		"画像開":     {josi: [][]string{{"を", "の", "から"}}, fn: p.open},
		"画像切替":    {josi: [][]string{{"に", "へ"}}, returnNone: true, fn: p.selectImage},
		"画像保存":    {josi: [][]string{{"へ", "に"}}, returnNone: true, fn: p.save},
		"画像背景色設定": {josi: [][]string{{"に", "へ", "で"}}, returnNone: true, fn: p.fill},
		"画像点設定":   {josi: [][]string{{"を"}, {"に", "へ"}}, returnNone: true, fn: p.setPixel},
		"画像点取得":   {josi: [][]string{{"の", "から"}}, fn: p.getPixel},
		"画像線描画":   {josi: [][]string{{"を"}, {"で"}}, returnNone: true, fn: p.line},
		"画像矩形描画":  {josi: [][]string{{"を"}, {"で"}}, returnNone: true, fn: p.rect},
		"画像円描画":   {josi: [][]string{{"を"}, {"で"}}, returnNone: true, fn: p.circle},
		"画像文字描画":  {josi: [][]string{{"を"}, {"で"}}, returnNone: true, fn: p.text},
		"画像リサイズ":  {josi: [][]string{{"に", "へ"}}, returnNone: true, fn: p.resize},
		"画像幅取得":   {fn: p.width},
		"画像高取得":   {fn: p.height},
		"画像高さ取得":  {fn: p.height},
		"画像閉":     {returnNone: true, fn: p.close},
	}
}

func arg(args []value.Value, i int) value.Value {
	if i < 0 || i >= len(args) {
		return value.Undefined()
	}
	return args[i]
}

func (p *Plugin) create(_ stdlib.Context, args []value.Value) (value.Value, error) {
	size, err := ints(arg(args, 0), 2, "画像サイズ")
	if err != nil {
		return value.Undefined(), err
	}
	if size[0] <= 0 || size[1] <= 0 || size[0] > 32768 || size[1] > 32768 {
		return value.Undefined(), errors.New("画像サイズは1以上32768以下で指定してください。")
	}
	return p.add(image.NewRGBA(image.Rect(0, 0, size[0], size[1]))), nil
}

func (p *Plugin) open(_ stdlib.Context, args []value.Value) (value.Value, error) {
	filename := value.ToString(arg(args, 0))
	f, err := os.Open(filename)
	if err != nil {
		return value.Undefined(), fmt.Errorf("画像『%s』を開けません: %w", filename, err)
	}
	decoded, _, decodeErr := image.Decode(f)
	closeErr := f.Close()
	if decodeErr != nil {
		return value.Undefined(), fmt.Errorf("画像『%s』を読めません: %w", filename, decodeErr)
	}
	if closeErr != nil {
		return value.Undefined(), closeErr
	}
	rgba := image.NewRGBA(image.Rect(0, 0, decoded.Bounds().Dx(), decoded.Bounds().Dy()))
	draw.Draw(rgba, rgba.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	return p.add(rgba), nil
}

func (p *Plugin) add(img *image.RGBA) value.Value {
	p.next++
	p.images[p.next] = &canvas{img: img}
	p.active = p.next
	return value.Number(float64(p.next))
}

func (p *Plugin) selectImage(_ stdlib.Context, args []value.Value) (value.Value, error) {
	id := int64(value.ToNumber(arg(args, 0)))
	if p.images[id] == nil {
		return value.Undefined(), errors.New("画像切替で指定された画像は開かれていません。")
	}
	p.active = id
	return value.Undefined(), nil
}

func (p *Plugin) save(_ stdlib.Context, args []value.Value) (value.Value, error) {
	c, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	filename := value.ToString(arg(args, 0))
	f, err := os.Create(filename)
	if err != nil {
		return value.Undefined(), fmt.Errorf("画像『%s』を保存できません: %w", filename, err)
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		err = jpeg.Encode(f, c.img, &jpeg.Options{Quality: 90})
	case ".gif":
		err = gif.Encode(f, c.img, nil)
	case ".png", "":
		err = png.Encode(f, c.img)
	default:
		err = errors.New("画像保存はPNG、JPEG、GIF形式に対応しています。")
	}
	closeErr := f.Close()
	if err != nil {
		return value.Undefined(), err
	}
	return value.Undefined(), closeErr
}

func (p *Plugin) fill(_ stdlib.Context, args []value.Value) (value.Value, error) {
	c, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	col, err := parseColor(value.ToString(arg(args, 0)))
	if err != nil {
		return value.Undefined(), err
	}
	draw.Draw(c.img, c.img.Bounds(), &image.Uniform{C: col}, image.Point{}, draw.Src)
	return value.Undefined(), nil
}

func (p *Plugin) setPixel(_ stdlib.Context, args []value.Value) (value.Value, error) {
	c, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	pos, err := ints(arg(args, 0), 2, "画像点設定の座標")
	if err != nil {
		return value.Undefined(), err
	}
	col, err := parseColor(value.ToString(arg(args, 1)))
	if err != nil {
		return value.Undefined(), err
	}
	c.img.SetRGBA(pos[0], pos[1], col)
	return value.Undefined(), nil
}

func (p *Plugin) getPixel(_ stdlib.Context, args []value.Value) (value.Value, error) {
	c, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	pos, err := ints(arg(args, 0), 2, "画像点取得の座標")
	if err != nil {
		return value.Undefined(), err
	}
	if !image.Pt(pos[0], pos[1]).In(c.img.Bounds()) {
		return value.Undefined(), errors.New("画像点取得の座標が画像の外です。")
	}
	col := c.img.RGBAAt(pos[0], pos[1])
	return value.String(fmt.Sprintf("#%02X%02X%02X%02X", col.R, col.G, col.B, col.A)), nil
}

func (p *Plugin) line(_ stdlib.Context, args []value.Value) (value.Value, error) {
	c, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	points, err := ints(arg(args, 0), 4, "画像線描画の座標")
	if err != nil {
		return value.Undefined(), err
	}
	col, err := parseColor(value.ToString(arg(args, 1)))
	if err != nil {
		return value.Undefined(), err
	}
	drawLine(c.img, points[0], points[1], points[2], points[3], col)
	return value.Undefined(), nil
}

func (p *Plugin) rect(_ stdlib.Context, args []value.Value) (value.Value, error) {
	c, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	r, err := ints(arg(args, 0), 4, "画像矩形描画の領域")
	if err != nil {
		return value.Undefined(), err
	}
	col, err := parseColor(value.ToString(arg(args, 1)))
	if err != nil {
		return value.Undefined(), err
	}
	draw.Draw(c.img, image.Rect(r[0], r[1], r[0]+r[2], r[1]+r[3]), &image.Uniform{C: col}, image.Point{}, draw.Src)
	return value.Undefined(), nil
}

func (p *Plugin) circle(_ stdlib.Context, args []value.Value) (value.Value, error) {
	c, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	v, err := ints(arg(args, 0), 3, "画像円描画の座標")
	if err != nil {
		return value.Undefined(), err
	}
	if v[2] < 0 {
		return value.Undefined(), errors.New("画像円描画の半径は0以上で指定してください。")
	}
	col, err := parseColor(value.ToString(arg(args, 1)))
	if err != nil {
		return value.Undefined(), err
	}
	r2 := v[2] * v[2]
	for y := -v[2]; y <= v[2]; y++ {
		dx := int(math.Sqrt(float64(r2 - y*y)))
		for x := -dx; x <= dx; x++ {
			c.img.SetRGBA(v[0]+x, v[1]+y, col)
		}
	}
	return value.Undefined(), nil
}

// text takes [x, y, size, color, fontFile] as its options array. y is the
// baseline. fontFile may be empty for the built-in ASCII font; a Japanese TTF
// or OTF file enables Japanese glyphs.
func (p *Plugin) text(_ stdlib.Context, args []value.Value) (value.Value, error) {
	c, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	opts, ok := arg(args, 1).Array()
	if !ok || opts.Len() < 2 {
		return value.Undefined(), errors.New("画像文字描画の設定は[x,y,size,色,フォントファイル]で指定してください。")
	}
	x, y := int(value.ToNumber(opts.Get(0))), int(value.ToNumber(opts.Get(1)))
	size := 13.0
	if opts.Len() > 2 && value.ToNumber(opts.Get(2)) > 0 {
		size = value.ToNumber(opts.Get(2))
	}
	colorName := "黒"
	if opts.Len() > 3 && opts.Get(3).Kind() != value.KindUndefined {
		colorName = value.ToString(opts.Get(3))
	}
	col, err := parseColor(colorName)
	if err != nil {
		return value.Undefined(), err
	}
	face := font.Face(basicfont.Face7x13)
	var closeFace func() error
	if opts.Len() > 4 && value.ToString(opts.Get(4)) != "" {
		fontFile := value.ToString(opts.Get(4))
		data, readErr := os.ReadFile(fontFile)
		if readErr != nil {
			return value.Undefined(), fmt.Errorf("画像フォント『%s』を読めません: %w", fontFile, readErr)
		}
		parsed, parseErr := opentype.Parse(data)
		if parseErr != nil {
			return value.Undefined(), fmt.Errorf("画像フォント『%s』を解析できません: %w", fontFile, parseErr)
		}
		otFace, faceErr := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		if faceErr != nil {
			return value.Undefined(), faceErr
		}
		face = otFace
		closeFace = otFace.Close
	}
	if closeFace != nil {
		defer closeFace()
	}
	drawer := &font.Drawer{Dst: c.img, Src: image.NewUniform(col), Face: face, Dot: fixed.P(x, y)}
	drawer.DrawString(value.ToString(arg(args, 0)))
	return value.Undefined(), nil
}

func (p *Plugin) resize(_ stdlib.Context, args []value.Value) (value.Value, error) {
	c, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	size, err := ints(arg(args, 0), 2, "画像リサイズのサイズ")
	if err != nil {
		return value.Undefined(), err
	}
	if size[0] <= 0 || size[1] <= 0 || size[0] > 32768 || size[1] > 32768 {
		return value.Undefined(), errors.New("画像サイズは1以上32768以下で指定してください。")
	}
	dst := image.NewRGBA(image.Rect(0, 0, size[0], size[1]))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), c.img, c.img.Bounds(), draw.Src, nil)
	c.img = dst
	return value.Undefined(), nil
}

func (p *Plugin) width(_ stdlib.Context, _ []value.Value) (value.Value, error) {
	c, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	return value.Number(float64(c.img.Bounds().Dx())), nil
}

func (p *Plugin) height(_ stdlib.Context, _ []value.Value) (value.Value, error) {
	c, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	return value.Number(float64(c.img.Bounds().Dy())), nil
}

func (p *Plugin) close(_ stdlib.Context, _ []value.Value) (value.Value, error) {
	if _, err := p.current(); err != nil {
		return value.Undefined(), err
	}
	delete(p.images, p.active)
	p.active = 0
	return value.Undefined(), nil
}

func (p *Plugin) current() (*canvas, error) {
	if p.active == 0 || p.images[p.active] == nil {
		return nil, errors.New("画像関連の命令を使う前に『画像新規作成』または『画像開』を実行してください。")
	}
	return p.images[p.active], nil
}

func ints(v value.Value, want int, label string) ([]int, error) {
	a, ok := v.Array()
	if !ok || a.Len() < want {
		return nil, fmt.Errorf("%sは%d要素の配列で指定してください。", label, want)
	}
	out := make([]int, want)
	for i := range out {
		out[i] = int(value.ToNumber(a.Get(i)))
	}
	return out, nil
}

func parseColor(s string) (color.RGBA, error) {
	named := map[string]string{
		"透明": "00000000", "黒": "000000FF", "白": "FFFFFFFF", "赤": "FF0000FF",
		"緑": "008000FF", "青": "0000FFFF", "黄": "FFFF00FF", "紫": "800080FF",
		"水色": "00FFFFFF", "灰": "808080FF", "橙": "FFA500FF",
	}
	s = strings.TrimSpace(s)
	if c, ok := named[s]; ok {
		s = c
	}
	s = strings.TrimPrefix(s, "#")
	if len(s) == 6 {
		s += "FF"
	}
	if len(s) != 8 {
		return color.RGBA{}, fmt.Errorf("色『%s』は色名、#RRGGBB、#RRGGBBAAのいずれかで指定してください。", s)
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("色『%s』は色名、#RRGGBB、#RRGGBBAAのいずれかで指定してください。", s)
	}
	return color.RGBA{R: uint8(n >> 24), G: uint8(n >> 16), B: uint8(n >> 8), A: uint8(n)}, nil
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 int, col color.RGBA) {
	dx := int(math.Abs(float64(x1 - x0)))
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	dy := -int(math.Abs(float64(y1 - y0)))
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		img.SetRGBA(x0, y0, col)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}
