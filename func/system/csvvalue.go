package system

import (
	"strings"

	"github.com/kujirahand/nadesiko3go/value"
)

// GetCSVToValue : CSV文字列をValueに変換
func GetCSVToValue(csv string, splitter rune) *value.Value {
	res := value.NewArrayPtr()
	// 改行コードを統一
	csv = strings.Replace(csv, "\r\n", "\n", 0)
	csv = strings.Replace(csv, "\r", "\n", 0)
	// 解析開始
	cols := value.NewArrayPtr()
	col := ""
	insideQuote := false
	// 最終的に改行を追加する
	if csv != "" && csv[len(csv)-1] != '\n' { // 末尾に改行がなければ改行を追加
		csv += "\n"
	}
	src := []rune(csv)
	i := 0
	for i < len(src) {
		c := src[i]
		if !insideQuote {
			if c == splitter {
				pcol := value.NewStrPtr(col)
				cols.Append(pcol)
				col = ""
				i++
				continue
			}
			if col == "" && c == '"' {
				insideQuote = true
				i++
				continue
			}
			if c == '\n' {
				pcol := value.NewStrPtr(col)
				cols.Append(pcol)
				res.Append(cols)
				println(res.ToJSONString())
				col = ""
				cols = value.NewArrayPtr()
				i++
				continue
			}
			col += string(c)
			i++
			continue
		}
		// inside quote
		if c == '"' {
			if i+1 < len(src) && src[i+1] == '"' {
				col += "\""
				i += 2
				continue
			}
			i++ // skip "
			insideQuote = false
			for i < len(src) { // skip white space
				if src[i] == ' ' {
					i++
					continue
				}
				break
			}
			continue
		}
		col += string(c)
		i++
	}
	return res
}
