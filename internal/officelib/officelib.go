// Package officelib implements spreadsheet commands for the CUI runtime.
//
// Command names and behaviour follow nadesiko3-office where practical. Go
// objects never cross the Value boundary: workbooks and sheets are represented
// by numeric handles, while commands without a handle operate on the currently
// selected workbook and sheet.
package officelib

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"github.com/xuri/excelize/v2"
)

const (
	officeVersion = "1.0.0-go"
	errNoBook     = "Excel関連の命令を使う時は、最初に『エクセル新規ブック』や『エクセル開』などでブックを用意してください。"
)

type workbook struct {
	file  *excelize.File
	sheet string
}

// Plugin owns spreadsheet handles for one runtime.
type Plugin struct {
	books  map[int64]*workbook
	active int64
	next   int64
}

func New() *Plugin { return &Plugin{books: map[int64]*workbook{}} }

type command struct {
	josi       [][]string
	returnNone bool
	fn         stdlib.Impl
}

func (p *Plugin) FuncList() lexer.FuncList {
	list := lexer.FuncList{
		"OFFICEバージョン": {Name: "OFFICEバージョン", Type: "const", Value: officeVersion},
	}
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
		"エクセル新規ブック": {fn: p.newBook},
		"エクセル開":     {josi: [][]string{{"を", "の", "から"}}, fn: p.open},
		"エクセル保存":    {josi: [][]string{{"へ", "に"}}, returnNone: true, fn: p.save},
		"エクセルCSV保存": {josi: [][]string{{"へ", "に"}}, returnNone: true, fn: p.saveCSV},
		"エクセル閉":     {returnNone: true, fn: p.close},
		"エクセルブック切替": {josi: [][]string{{"に", "へ"}}, returnNone: true, fn: p.switchBook},
		"エクセル新規シート": {josi: [][]string{{"の", "で"}}, fn: p.newSheet},
		"エクセルシート取得": {josi: [][]string{{"の"}}, fn: p.getSheet},
		"エクセルシート注目": {josi: [][]string{{"の", "に", "を"}}, fn: p.selectSheet},
		"エクセルセル設定":  {josi: [][]string{{"へ", "に"}, {"を"}}, returnNone: true, fn: p.setCell},
		"エクセル設定":    {josi: [][]string{{"へ", "に"}, {"を"}}, returnNone: true, fn: p.setCell},
		"エクセル一括設定":  {josi: [][]string{{"へ", "に"}, {"を"}}, returnNone: true, fn: p.setRange},
		"エクセルセル取得":  {josi: [][]string{{"から", "を", "の"}}, fn: p.getCell},
		"エクセル取得":    {josi: [][]string{{"から", "を", "の"}}, fn: p.getCell},
		"エクセル一括取得":  {josi: [][]string{{"から"}, {"までの", "まで", "の"}}, fn: p.getRange},
		"エクセルシート列挙": {fn: p.listSheets},
		"エクセルシート削除": {josi: [][]string{{"の", "を"}}, returnNone: true, fn: p.deleteSheet},
		"エクセルセル幅設定": {josi: [][]string{{"を"}, {"に", "へ"}}, returnNone: true, fn: p.setColWidth},
		"エクセル背景色設定": {josi: [][]string{{"を"}, {"に", "へ"}}, returnNone: true, fn: p.setBackground},
		"エクセル文字色設定": {josi: [][]string{{"を"}, {"に", "へ"}}, returnNone: true, fn: p.setFontColor},
	}
}

func arg(args []value.Value, i int) value.Value {
	if i < 0 || i >= len(args) {
		return value.Undefined()
	}
	return args[i]
}

func (p *Plugin) newBook(_ stdlib.Context, _ []value.Value) (value.Value, error) {
	f := excelize.NewFile()
	p.next++
	p.books[p.next] = &workbook{file: f, sheet: f.GetSheetName(f.GetActiveSheetIndex())}
	p.active = p.next
	return value.Number(float64(p.next)), nil
}

func (p *Plugin) open(_ stdlib.Context, args []value.Value) (value.Value, error) {
	filename := value.ToString(arg(args, 0))
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return value.Undefined(), fmt.Errorf("Excelファイル『%s』を開けません: %w", filename, err)
	}
	p.next++
	sheets := f.GetSheetList()
	selected := ""
	if len(sheets) > 0 {
		selected = sheets[0]
	}
	p.books[p.next] = &workbook{file: f, sheet: selected}
	p.active = p.next
	return value.Number(float64(p.next)), nil
}

func (p *Plugin) save(_ stdlib.Context, args []value.Value) (value.Value, error) {
	b, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	filename := value.ToString(arg(args, 0))
	if err := b.file.SaveAs(filename); err != nil {
		return value.Undefined(), fmt.Errorf("Excelファイル『%s』を保存できません: %w", filename, err)
	}
	return value.Undefined(), nil
}

func (p *Plugin) saveCSV(_ stdlib.Context, args []value.Value) (value.Value, error) {
	b, err := p.currentSheet()
	if err != nil {
		return value.Undefined(), err
	}
	rows, err := b.file.GetRows(b.sheet)
	if err != nil {
		return value.Undefined(), err
	}
	filename := value.ToString(arg(args, 0))
	f, err := os.Create(filename)
	if err != nil {
		return value.Undefined(), fmt.Errorf("CSVファイル『%s』を保存できません: %w", filename, err)
	}
	w := csv.NewWriter(f)
	writeErr := w.WriteAll(rows)
	w.Flush()
	if writeErr == nil {
		writeErr = w.Error()
	}
	closeErr := f.Close()
	if writeErr != nil {
		return value.Undefined(), writeErr
	}
	return value.Undefined(), closeErr
}

func (p *Plugin) close(_ stdlib.Context, _ []value.Value) (value.Value, error) {
	b, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	id := p.active
	p.active = 0
	delete(p.books, id)
	return value.Undefined(), b.file.Close()
}

func (p *Plugin) switchBook(_ stdlib.Context, args []value.Value) (value.Value, error) {
	id := int64(value.ToNumber(arg(args, 0)))
	if p.books[id] == nil {
		return value.Undefined(), errors.New("エクセルブック切替で指定されたブックは開かれていません。")
	}
	p.active = id
	return value.Undefined(), nil
}

func (p *Plugin) newSheet(_ stdlib.Context, args []value.Value) (value.Value, error) {
	if p.active == 0 {
		if _, err := p.newBook(nil, nil); err != nil {
			return value.Undefined(), err
		}
	}
	b, _ := p.current()
	name := value.ToString(arg(args, 0))
	if name == "" {
		name = "Sheet" + strconv.Itoa(len(b.file.GetSheetList())+1)
	}
	idx, err := b.file.NewSheet(name)
	if err != nil {
		return value.Undefined(), err
	}
	b.file.SetActiveSheet(idx)
	b.sheet = name
	return value.String(name), nil
}

func (p *Plugin) getSheet(_ stdlib.Context, args []value.Value) (value.Value, error) {
	b, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	name := value.ToString(arg(args, 0))
	idx, err := b.file.GetSheetIndex(name)
	if err != nil || idx < 0 {
		return value.Undefined(), fmt.Errorf("Excelシート『%s』が見つかりません。", name)
	}
	return value.String(name), nil
}

func (p *Plugin) selectSheet(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	v, err := p.getSheet(ctx, args)
	if err != nil {
		return value.Undefined(), err
	}
	name := value.ToString(v)
	b, _ := p.current()
	idx, _ := b.file.GetSheetIndex(name)
	b.file.SetActiveSheet(idx)
	b.sheet = name
	return v, nil
}

func (p *Plugin) setCell(_ stdlib.Context, args []value.Value) (value.Value, error) {
	b, err := p.currentSheet()
	if err != nil {
		return value.Undefined(), err
	}
	cell := strings.ToUpper(value.ToString(arg(args, 0)))
	v := arg(args, 1)
	if v.Kind() == value.KindString && strings.HasPrefix(value.ToString(v), "=") {
		err = b.file.SetCellFormula(b.sheet, cell, strings.TrimPrefix(value.ToString(v), "="))
	} else {
		err = b.file.SetCellValue(b.sheet, cell, excelValue(v))
	}
	return value.Undefined(), err
}

func (p *Plugin) setRange(_ stdlib.Context, args []value.Value) (value.Value, error) {
	b, err := p.currentSheet()
	if err != nil {
		return value.Undefined(), err
	}
	start := strings.Split(value.ToString(arg(args, 0)), ":")[0]
	col, row, err := excelize.CellNameToCoordinates(start)
	if err != nil {
		return value.Undefined(), err
	}
	rows, ok := arg(args, 1).Array()
	if !ok {
		return value.Undefined(), errors.New("エクセル一括設定には二次元配列を指定してください。")
	}
	for y, rowValue := range rows.Values() {
		cells, ok := rowValue.Array()
		if !ok {
			return value.Undefined(), errors.New("エクセル一括設定には二次元配列を指定してください。")
		}
		for x, cellValue := range cells.Values() {
			name, _ := excelize.CoordinatesToCellName(col+x, row+y)
			if err := b.file.SetCellValue(b.sheet, name, excelValue(cellValue)); err != nil {
				return value.Undefined(), err
			}
		}
	}
	return value.Undefined(), nil
}

func (p *Plugin) getCell(_ stdlib.Context, args []value.Value) (value.Value, error) {
	b, err := p.currentSheet()
	if err != nil {
		return value.Undefined(), err
	}
	s, err := b.file.GetCellValue(b.sheet, strings.ToUpper(value.ToString(arg(args, 0))))
	if err != nil {
		return value.Undefined(), err
	}
	return cellValue(s), nil
}

func (p *Plugin) getRange(_ stdlib.Context, args []value.Value) (value.Value, error) {
	b, err := p.currentSheet()
	if err != nil {
		return value.Undefined(), err
	}
	c1, r1, err := excelize.CellNameToCoordinates(value.ToString(arg(args, 0)))
	if err != nil {
		return value.Undefined(), err
	}
	c2, r2, err := excelize.CellNameToCoordinates(value.ToString(arg(args, 1)))
	if err != nil {
		return value.Undefined(), err
	}
	if c1 > c2 || r1 > r2 {
		return value.Undefined(), errors.New("エクセル一括取得の範囲が逆順です。")
	}
	rows := make([]value.Value, 0, r2-r1+1)
	for row := r1; row <= r2; row++ {
		cells := make([]value.Value, 0, c2-c1+1)
		for col := c1; col <= c2; col++ {
			name, _ := excelize.CoordinatesToCellName(col, row)
			s, getErr := b.file.GetCellValue(b.sheet, name)
			if getErr != nil {
				return value.Undefined(), getErr
			}
			cells = append(cells, cellValue(s))
		}
		rows = append(rows, value.ArrayValue(value.NewArray(cells...)))
	}
	return value.ArrayValue(value.NewArray(rows...)), nil
}

func (p *Plugin) listSheets(_ stdlib.Context, _ []value.Value) (value.Value, error) {
	b, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	items := make([]value.Value, 0, len(b.file.GetSheetList()))
	for _, name := range b.file.GetSheetList() {
		items = append(items, value.String(name))
	}
	return value.ArrayValue(value.NewArray(items...)), nil
}

func (p *Plugin) deleteSheet(_ stdlib.Context, args []value.Value) (value.Value, error) {
	b, err := p.current()
	if err != nil {
		return value.Undefined(), err
	}
	name := value.ToString(arg(args, 0))
	if err := b.file.DeleteSheet(name); err != nil {
		return value.Undefined(), err
	}
	sheets := b.file.GetSheetList()
	if b.sheet == name {
		b.sheet = ""
		if len(sheets) > 0 {
			b.sheet = sheets[0]
		}
	}
	return value.Undefined(), nil
}

func (p *Plugin) setColWidth(_ stdlib.Context, args []value.Value) (value.Value, error) {
	b, err := p.currentSheet()
	if err != nil {
		return value.Undefined(), err
	}
	col := value.ToString(arg(args, 0))
	if n := int(value.ToNumber(arg(args, 0))); arg(args, 0).Kind() == value.KindNumber {
		col, err = excelize.ColumnNumberToName(n)
		if err != nil {
			return value.Undefined(), err
		}
	}
	return value.Undefined(), b.file.SetColWidth(b.sheet, col, col, value.ToNumber(arg(args, 1)))
}

func (p *Plugin) setBackground(_ stdlib.Context, args []value.Value) (value.Value, error) {
	return p.setColor(args, true)
}

func (p *Plugin) setFontColor(_ stdlib.Context, args []value.Value) (value.Value, error) {
	return p.setColor(args, false)
}

func (p *Plugin) setColor(args []value.Value, background bool) (value.Value, error) {
	b, err := p.currentSheet()
	if err != nil {
		return value.Undefined(), err
	}
	rangeName := strings.ToUpper(value.ToString(arg(args, 0)))
	colorCode, err := normalizeColor(value.ToString(arg(args, 1)))
	if err != nil {
		return value.Undefined(), err
	}
	style := &excelize.Style{}
	if background {
		style.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{colorCode}}
	} else {
		style.Font = &excelize.Font{Color: colorCode}
	}
	styleID, err := b.file.NewStyle(style)
	if err != nil {
		return value.Undefined(), err
	}
	ends := strings.SplitN(rangeName, ":", 2)
	end := ends[0]
	if len(ends) == 2 {
		end = ends[1]
	}
	return value.Undefined(), b.file.SetCellStyle(b.sheet, ends[0], end, styleID)
}

func (p *Plugin) current() (*workbook, error) {
	if p.active == 0 || p.books[p.active] == nil {
		return nil, errors.New(errNoBook)
	}
	return p.books[p.active], nil
}

func (p *Plugin) currentSheet() (*workbook, error) {
	b, err := p.current()
	if err != nil {
		return nil, err
	}
	if b.sheet == "" {
		return nil, errors.New("注目中のExcelシートがありません。")
	}
	return b, nil
}

func excelValue(v value.Value) any {
	switch v.Kind() {
	case value.KindUndefined, value.KindNull:
		return nil
	case value.KindBool:
		b, _ := v.Bool()
		return b
	case value.KindNumber:
		n, _ := v.Number()
		return n
	case value.KindString:
		return value.ToString(v)
	default:
		return value.ToString(v)
	}
}

func cellValue(s string) value.Value {
	if s == "" {
		return value.String("")
	}
	if b, err := strconv.ParseBool(s); err == nil {
		return value.Bool(b)
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return value.Number(n)
	}
	return value.String(s)
}

func normalizeColor(s string) (string, error) {
	named := map[string]string{
		"黒": "000000", "白": "FFFFFF", "赤": "FF0000", "緑": "008000", "青": "0000FF",
		"黄": "FFFF00", "紫": "800080", "水色": "00FFFF", "灰": "808080", "橙": "FFA500",
	}
	if c, ok := named[strings.TrimSpace(s)]; ok {
		return c, nil
	}
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) == 8 {
		s = s[2:]
	}
	if len(s) != 6 {
		return "", fmt.Errorf("色『%s』は色名または#RRGGBBで指定してください。", s)
	}
	if _, err := strconv.ParseUint(s, 16, 32); err != nil {
		return "", fmt.Errorf("色『%s』は色名または#RRGGBBで指定してください。", s)
	}
	return strings.ToUpper(s), nil
}
