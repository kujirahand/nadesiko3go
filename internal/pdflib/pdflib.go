// Package pdflib implements PDF creation commands for the CUI runtime.
// Documents are represented by numeric handles; fpdf objects never cross the
// language value boundary.
package pdflib

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

type document struct {
	pdf      *fpdf.Fpdf
	fontName string
	fontSize float64
}

// Plugin owns open PDF documents for one runtime.
type Plugin struct {
	docs   map[int64]*document
	active int64
	next   int64
}

func New() *Plugin { return &Plugin{docs: map[int64]*document{}} }

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
		"PDF新規作成":    {fn: p.create},
		"PDF切替":      {josi: [][]string{{"に", "へ"}}, returnNone: true, fn: p.selectDoc},
		"PDFページ追加":   {returnNone: true, fn: p.addPage},
		"PDFフォント設定":  {josi: [][]string{{"を"}, {"で", "に"}}, returnNone: true, fn: p.setFont},
		"PDF文字描画":    {josi: [][]string{{"を"}, {"へ", "に"}}, returnNone: true, fn: p.drawText},
		"PDF複数行文字描画": {josi: [][]string{{"を"}, {"へ", "に"}}, returnNone: true, fn: p.drawMultiline},
		"PDF線描画":     {josi: [][]string{{"を"}, {"で"}}, returnNone: true, fn: p.drawLine},
		"PDF矩形描画":    {josi: [][]string{{"を"}, {"で"}}, returnNone: true, fn: p.drawRect},
		"PDF画像描画":    {josi: [][]string{{"を", "の"}, {"へ", "に"}}, returnNone: true, fn: p.drawImage},
		"PDFタイトル設定":  {josi: [][]string{{"に", "へ", "を"}}, returnNone: true, fn: p.setTitle},
		"PDF保存":      {josi: [][]string{{"へ", "に"}}, returnNone: true, fn: p.save},
		"PDF閉":       {returnNone: true, fn: p.close},
	}
}

func arg(args []value.Value, i int) value.Value {
	if i < 0 || i >= len(args) {
		return value.Undefined()
	}
	return args[i]
}

func (p *Plugin) create(_ stdlib.Context, _ []value.Value) (value.Value, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("", true)
	pdf.SetCreator("gonako", true)
	pdf.SetFont("Helvetica", "", 12)
	pdf.AddPage()
	p.next++
	p.docs[p.next] = &document{pdf: pdf, fontName: "Helvetica", fontSize: 12}
	p.active = p.next
	return value.Number(float64(p.next)), nil
}

func (p *Plugin) selectDoc(_ stdlib.Context, args []value.Value) (value.Value, error) {
	id := int64(value.ToNumber(arg(args, 0)))
	if p.docs[id] == nil {
		return value.Undefined(), errors.New("PDF切替で指定されたPDFは開かれていません。")
	}
	p.active = id
	return value.Undefined(), nil
}

func (p *Plugin) addPage(_ stdlib.Context, _ []value.Value) (value.Value, error) {
	d, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	d.pdf.AddPage()
	return value.Undefined(), d.pdf.Error()
}

// setFont accepts a font file and a point size. An empty file selects the
// built-in Helvetica font. TTF/OTF files are embedded, enabling Japanese text
// when the selected font contains Japanese glyphs.
func (p *Plugin) setFont(_ stdlib.Context, args []value.Value) (value.Value, error) {
	d, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	fontFile := value.ToString(arg(args, 0))
	size := value.ToNumber(arg(args, 1))
	if size <= 0 {
		size = 12
	}
	if fontFile == "" || strings.EqualFold(fontFile, "Helvetica") {
		d.fontName, d.fontSize = "Helvetica", size
		d.pdf.SetFont(d.fontName, "", size)
		return value.Undefined(), d.pdf.Error()
	}
	fontBytes, err := os.ReadFile(fontFile)
	if err != nil {
		return value.Undefined(), fmt.Errorf("PDFフォント『%s』を読めません: %w", fontFile, err)
	}
	name := "font" + strconv.FormatInt(p.active, 10)
	d.pdf.AddUTF8FontFromBytes(name, "", fontBytes)
	d.pdf.SetFont(name, "", size)
	d.fontName, d.fontSize = name, size
	return value.Undefined(), d.pdf.Error()
}

func (p *Plugin) drawText(_ stdlib.Context, args []value.Value) (value.Value, error) {
	d, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	pos, err := numbers(arg(args, 1), 2, "PDF文字描画の位置")
	if err != nil {
		return value.Undefined(), err
	}
	d.pdf.Text(pos[0], pos[1], value.ToString(arg(args, 0)))
	return value.Undefined(), d.pdf.Error()
}

func (p *Plugin) drawMultiline(_ stdlib.Context, args []value.Value) (value.Value, error) {
	d, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	rect, err := numbers(arg(args, 1), 4, "PDF複数行文字描画の領域")
	if err != nil {
		return value.Undefined(), err
	}
	d.pdf.SetXY(rect[0], rect[1])
	lineHeight := d.fontSize * 0.45
	d.pdf.MultiCell(rect[2], lineHeight, value.ToString(arg(args, 0)), "", "L", false)
	return value.Undefined(), d.pdf.Error()
}

func (p *Plugin) drawLine(_ stdlib.Context, args []value.Value) (value.Value, error) {
	d, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	line, err := numbers(arg(args, 0), 4, "PDF線描画の座標")
	if err != nil {
		return value.Undefined(), err
	}
	r, g, b, err := rgb(value.ToString(arg(args, 1)))
	if err != nil {
		return value.Undefined(), err
	}
	d.pdf.SetDrawColor(r, g, b)
	d.pdf.Line(line[0], line[1], line[2], line[3])
	return value.Undefined(), d.pdf.Error()
}

func (p *Plugin) drawRect(_ stdlib.Context, args []value.Value) (value.Value, error) {
	d, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	rect, err := numbers(arg(args, 0), 4, "PDF矩形描画の領域")
	if err != nil {
		return value.Undefined(), err
	}
	r, g, b, err := rgb(value.ToString(arg(args, 1)))
	if err != nil {
		return value.Undefined(), err
	}
	d.pdf.SetFillColor(r, g, b)
	d.pdf.Rect(rect[0], rect[1], rect[2], rect[3], "F")
	return value.Undefined(), d.pdf.Error()
}

func (p *Plugin) drawImage(_ stdlib.Context, args []value.Value) (value.Value, error) {
	d, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	rect, err := numbers(arg(args, 1), 4, "PDF画像描画の領域")
	if err != nil {
		return value.Undefined(), err
	}
	d.pdf.ImageOptions(value.ToString(arg(args, 0)), rect[0], rect[1], rect[2], rect[3], false,
		fpdf.ImageOptions{ReadDpi: true}, 0, "")
	return value.Undefined(), d.pdf.Error()
}

func (p *Plugin) setTitle(_ stdlib.Context, args []value.Value) (value.Value, error) {
	d, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	d.pdf.SetTitle(value.ToString(arg(args, 0)), true)
	return value.Undefined(), nil
}

func (p *Plugin) save(_ stdlib.Context, args []value.Value) (value.Value, error) {
	d, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	filename := value.ToString(arg(args, 0))
	if err := d.pdf.OutputFileAndClose(filename); err != nil {
		return value.Undefined(), fmt.Errorf("PDF『%s』を保存できません: %w", filename, err)
	}
	delete(p.docs, p.active)
	p.active = 0
	return value.Undefined(), nil
}

func (p *Plugin) close(_ stdlib.Context, _ []value.Value) (value.Value, error) {
	if _, err := p.current(); err != nil {
		return value.Undefined(), err
	}
	delete(p.docs, p.active)
	p.active = 0
	return value.Undefined(), nil
}

func (p *Plugin) current() (*document, error) {
	if p.active == 0 || p.docs[p.active] == nil {
		return nil, errors.New("PDF関連の命令を使う前に『PDF新規作成』を実行してください。")
	}
	return p.docs[p.active], nil
}

func numbers(v value.Value, want int, label string) ([]float64, error) {
	a, ok := v.Array()
	if !ok || a.Len() < want {
		return nil, fmt.Errorf("%sは%d要素の配列で指定してください。", label, want)
	}
	out := make([]float64, want)
	for i := range out {
		out[i] = value.ToNumber(a.Get(i))
	}
	return out, nil
}

func rgb(s string) (int, int, int, error) {
	named := map[string]string{
		"黒": "000000", "白": "FFFFFF", "赤": "FF0000", "緑": "008000", "青": "0000FF",
		"黄": "FFFF00", "紫": "800080", "水色": "00FFFF", "灰": "808080", "橙": "FFA500",
	}
	s = strings.TrimSpace(s)
	if c, ok := named[s]; ok {
		s = c
	}
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return 0, 0, 0, fmt.Errorf("色『%s』は色名または#RRGGBBで指定してください。", s)
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("色『%s』は色名または#RRGGBBで指定してください。", s)
	}
	return int(n >> 16), int((n >> 8) & 0xff), int(n & 0xff), nil
}
