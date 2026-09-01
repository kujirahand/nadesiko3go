// Package csvlib implements the TypeScript plugin_csv commands used by the
// command-line runtime. It stays separate from stdlib because plugin_system is
// the only compatibility-guaranteed command set.
package csvlib

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) FuncList() lexer.FuncList {
	return lexer.FuncList{
		"CSV取得": {Name: "CSV取得", Type: "func", Josi: [][]string{{"を", "の", "で"}}, Pure: true},
		"TSV取得": {Name: "TSV取得", Type: "func", Josi: [][]string{{"を", "の", "で"}}, Pure: true},
	}
}

func (p *Plugin) Impls() map[string]stdlib.Impl {
	return map[string]stdlib.Impl{
		"CSV取得": parse(','),
		"TSV取得": parse('\t'),
	}
}

func parse(delimiter rune) stdlib.Impl {
	return func(_ stdlib.Context, args []value.Value) (value.Value, error) {
		input := ""
		if len(args) > 0 {
			input = strings.TrimSpace(value.ToString(args[0]))
		}
		if input == "" {
			return value.ArrayValue(value.NewArray()), nil
		}
		r := csv.NewReader(strings.NewReader(input))
		r.Comma = delimiter
		r.FieldsPerRecord = -1
		rows := make([]value.Value, 0)
		for {
			record, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return value.Undefined(), err
			}
			cells := make([]value.Value, len(record))
			for i, cell := range record {
				cells[i] = csvValue(cell)
			}
			rows = append(rows, value.ArrayValue(value.NewArray(cells...)))
		}
		return value.ArrayValue(value.NewArray(rows...)), nil
	}
}

func csvValue(s string) value.Value {
	if n, err := strconv.ParseFloat(s, 64); err == nil && isNumeric(s) {
		return value.Number(n)
	}
	return value.String(s)
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r >= '0' && r <= '9' || r == '.' || r == 'e' || r == 'E' {
			continue
		}
		if (r == '-' || r == '+') && (i == 0 || s[i-1] == 'e' || s[i-1] == 'E') {
			continue
		}
		return false
	}
	return true
}
