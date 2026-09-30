// Package csvlib implements the TypeScript plugin_csv commands used by the
// command-line runtime. It stays separate from stdlib because plugin_system is
// the only compatibility-guaranteed command set.
package csvlib

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

type Options struct {
	Delimiter         string
	EOL               string
	AutoConvertNumber bool
}

type Plugin struct {
	options Options
}

func New() *Plugin {
	return &Plugin{
		options: Options{
			Delimiter:         ",",
			EOL:               "\r\n",
			AutoConvertNumber: true,
		},
	}
}

func (p *Plugin) ResetEnv() {
	p.options.Delimiter = ","
	p.options.EOL = "\r\n"
	p.options.AutoConvertNumber = true
}

func (p *Plugin) FuncList() lexer.FuncList {
	return lexer.FuncList{
		"CSV取得":     {Name: "CSV取得", Type: "func", Josi: [][]string{{"を", "の", "で", "から"}}, Pure: true},
		"TSV取得":     {Name: "TSV取得", Type: "func", Josi: [][]string{{"を", "の", "で", "から"}}, Pure: true},
		"表CSV変換":    {Name: "表CSV変換", Type: "func", Josi: [][]string{{"を", "の", "から", "で"}}, Pure: true},
		"CSV変換":     {Name: "CSV変換", Type: "func", Josi: [][]string{{"を", "の", "から", "で"}}, Pure: true},
		"表TSV変換":    {Name: "表TSV変換", Type: "func", Josi: [][]string{{"を", "の", "から", "で"}}, Pure: true},
		"TSV変換":     {Name: "TSV変換", Type: "func", Josi: [][]string{{"を", "の", "から", "で"}}, Pure: true},
		"CSVオプション設定": {Name: "CSVオプション設定", Type: "func", Josi: [][]string{{"を", "で"}}, Pure: true, ReturnNone: true},
	}
}

func (p *Plugin) Impls() map[string]stdlib.Impl {
	return map[string]stdlib.Impl{
		"CSV取得":     p.csvGet,
		"TSV取得":     p.tsvGet,
		"表CSV変換":    p.csvStringify,
		"CSV変換":     p.csvStringify,
		"表TSV変換":    p.tsvStringify,
		"TSV変換":     p.tsvStringify,
		"CSVオプション設定": p.csvSetOptions,
	}
}

func (p *Plugin) csvGet(_ stdlib.Context, args []value.Value) (value.Value, error) {
	p.options.Delimiter = ","
	txt := ""
	if len(args) > 0 {
		txt = value.ToString(args[0])
	}
	res := p.parse(txt, p.options.Delimiter)
	return value.ArrayValue(res), nil
}

func (p *Plugin) tsvGet(_ stdlib.Context, args []value.Value) (value.Value, error) {
	p.options.Delimiter = "\t"
	txt := ""
	if len(args) > 0 {
		txt = value.ToString(args[0])
	}
	res := p.parse(txt, p.options.Delimiter)
	return value.ArrayValue(res), nil
}

func (p *Plugin) csvStringify(_ stdlib.Context, args []value.Value) (value.Value, error) {
	p.options.Delimiter = ","
	var v value.Value = value.Undefined()
	if len(args) > 0 {
		v = args[0]
	}
	s := p.stringify(v, p.options.Delimiter, p.options.EOL)
	return value.String(s), nil
}

func (p *Plugin) tsvStringify(_ stdlib.Context, args []value.Value) (value.Value, error) {
	p.options.Delimiter = "\t"
	var v value.Value = value.Undefined()
	if len(args) > 0 {
		v = args[0]
	}
	s := p.stringify(v, p.options.Delimiter, p.options.EOL)
	return value.String(s), nil
}

func (p *Plugin) csvSetOptions(_ stdlib.Context, args []value.Value) (value.Value, error) {
	if len(args) == 0 {
		return value.Undefined(), nil
	}
	v := args[0]
	if dict, ok := v.Dict(); ok && dict != nil {
		for _, key := range dict.Keys() {
			val, _ := dict.Get(key)
			switch key {
			case "delimiter", "区切文字":
				p.options.Delimiter = value.ToString(val)
			case "eol":
				p.options.EOL = value.ToString(val)
			case "auto_convert_number":
				p.options.AutoConvertNumber = value.ToBool(val)
			}
		}
	}
	return value.Undefined(), nil
}

var numericRe = regexp.MustCompile(`^-?\d+(\.\d+)?([eE][-+]?\d+)?$`)

func isNumeric(s string) bool {
	return numericRe.MatchString(s)
}

func (p *Plugin) parse(txt string, delimiter string) *value.Array {
	if delimiter == "" {
		delimiter = p.options.Delimiter
	}
	txt = txt + "\n"
	txt = strings.ReplaceAll(txt, "\r\n", "\n")
	txt = strings.ReplaceAll(txt, "\r", "\n")
	txt = strings.TrimRight(txt, " \t\v\f\r\n") + "\n"

	convType := func(v string) value.Value {
		if p.options.AutoConvertNumber && isNumeric(v) {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return value.Number(f)
			}
		}
		return value.String(v)
	}

	// 非引用セルの終端となる文字の集合（区切り文字のいずれかの文字か改行）
	stopChars := delimiter + "\n"

	var res []value.Value
	var cells []value.Value

	delimRune, _ := utf8.DecodeRuneInString(delimiter)
	if delimiter == "" {
		delimRune = 0
	}
	// runeAt は txt[i:] 先頭のruneとそのバイト長を返す（末尾なら 0, 0）
	runeAt := func(txt string, i int) (rune, int) {
		if i >= len(txt) {
			return 0, 0
		}
		return utf8.DecodeRuneInString(txt[i:])
	}

	// txt は残りの入力を指すスライスで、先頭から読み進めるだけにする。
	// []rune への変換や string の再生成をしないので、入力長に対して線形で処理できる
	for len(txt) > 0 {
		// first check delimiter (because /^\s+/ skip delimiter '\t') (#3)
		if strings.HasPrefix(txt, delimiter) {
			txt = txt[len(delimiter):]
			cells = append(cells, value.String(""))
			continue
		}
		// second check LF (#7)
		if txt[0] == '\n' {
			cells = append(cells, value.String(""))
			res = append(res, value.ArrayValue(value.NewArray(cells...)))
			cells = nil
			txt = txt[1:]
			continue
		}

		// trim white space
		if delimiter == "\t" {
			txt = strings.TrimLeft(txt, " \v\f\r")
		} else {
			txt = strings.TrimLeft(txt, " \t\v\f\r")
		}
		if len(txt) == 0 {
			break
		}

		// no data
		if strings.HasPrefix(txt, delimiter) {
			cells = append(cells, value.String(""))
			txt = txt[len(delimiter):]
			continue
		}

		// written using the dialect of Excel
		if strings.HasPrefix(txt, `="`) {
			txt = txt[1:]
			continue
		}

		// number or simple string
		if txt[0] != '"' {
			idx := strings.IndexAny(txt, stopChars)
			if idx < 0 {
				cells = append(cells, convType(txt))
				res = append(res, value.ArrayValue(value.NewArray(cells...)))
				cells = nil
				break
			}
			valStr := txt[:idx]
			_, size := utf8.DecodeRuneInString(txt[idx:])
			sep := txt[idx : idx+size]
			if sep == "\n" {
				cells = append(cells, convType(valStr))
				res = append(res, value.ArrayValue(value.NewArray(cells...)))
				cells = nil
			} else if sep == delimiter {
				cells = append(cells, convType(valStr))
			}
			txt = txt[idx+size:]
			continue
		}

		// "" ... 空引用符フィールドか、Excel方言の空セルかを判定する (#2476)
		// """" のように隣接する引用符エスケープは、通常の引用フィールド解析に任せる
		if len(txt) >= 3 && txt[1] == '"' && txt[2] != '"' {
			rest := txt[2:]
			// 区切り文字・改行に達するまで空白文字をスキップして判定する
			idx := 0
			for idx < len(rest) {
				ch, size := utf8.DecodeRuneInString(rest[idx:])
				if ch == delimRune || ch == '\n' || !unicode.IsSpace(ch) {
					break
				}
				idx += size
			}
			if ch, _ := runeAt(rest, idx); idx < len(rest) && (ch == delimRune || ch == '\n') {
				// 空引用符フィールドとして扱い、""と後続の空白を捨てて区切り/改行の処理に委ねる
				txt = rest[idx:]
				continue
			}
			// Excel方言の空セルとして扱い、続きを通常のセルとして解析する
			cells = append(cells, value.String(""))
			txt = rest
			continue
		}

		// "..."
		i := 1
		var sb strings.Builder
		for i < len(txt) {
			// 次の引用符までは通常の文字なので、まとめて書き込む
			q := strings.IndexByte(txt[i:], '"')
			if q < 0 {
				sb.WriteString(txt[i:])
				i = len(txt)
				break
			}
			sb.WriteString(txt[i : i+q])
			i += q + 1 // 引用符の次へ
			c2, size2 := runeAt(txt, i)
			// 2quote => 1quote char
			if c2 == '"' {
				i += size2
				sb.WriteByte('"')
				continue
			}
			if c2 == delimRune {
				i += size2
				cells = append(cells, convType(sb.String()))
				sb.Reset()
				break
			}
			if c2 == '\n' {
				i += size2
				cells = append(cells, convType(sb.String()))
				res = append(res, value.ArrayValue(value.NewArray(cells...)))
				cells = nil
				break
			}
			// 閉じていない単独の引用符は、直後の1文字ごと読み捨てる（本家 nako_csv.mts と同じ）
			i += size2
		}
		txt = txt[i:]
	}
	if len(cells) > 0 {
		res = append(res, value.ArrayValue(value.NewArray(cells...)))
	}
	return value.NewArray(res...)
}

func (p *Plugin) stringify(v value.Value, delimiter, eol string) string {
	if delimiter == "" {
		delimiter = p.options.Delimiter
	}
	if eol == "" {
		eol = p.options.EOL
	}
	arr, ok := v.Array()
	if !ok || arr == nil {
		return ""
	}
	valueConv := func(s string) string {
		fQuot := false
		if strings.ContainsAny(s, "\r\n") || strings.Contains(s, delimiter) || strings.Contains(s, "\"") {
			fQuot = true
			s = strings.ReplaceAll(s, "\"", "\"\"")
		}
		if fQuot {
			s = "\"" + s + "\""
		}
		return s
	}

	var r strings.Builder
	for i := 0; i < arr.Len(); i++ {
		rowVal := arr.Get(i)
		rowArr, rowOK := rowVal.Array()
		if !rowOK || rowArr == nil {
			r.WriteString(eol)
			continue
		}
		var cells []string
		for j := 0; j < rowArr.Len(); j++ {
			cells = append(cells, valueConv(value.ToString(rowArr.Get(j))))
		}
		r.WriteString(strings.Join(cells, delimiter))
		r.WriteString(eol)
	}
	res := r.String()
	res = strings.ReplaceAll(res, "\r\n", "\n")
	res = strings.ReplaceAll(res, "\r", "\n")
	res = strings.ReplaceAll(res, "\n", eol)
	return res
}
