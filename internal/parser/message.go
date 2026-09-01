package parser

import (
	"fmt"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// nodeToStr describes a node or token in Japanese for an error message
// (nako_parser_message.mts equivalent).
//
// depth controls how far the description expands: at depth 0 an operator is
// named on its own, at depth 1 its operands are described too. typeName
// overrides the leading noun when it is not empty.
func nodeToStr(node any, depth int, typeName string) string {
	depth--
	named := func(def string) string {
		if typeName != "" {
			return typeName
		}
		return def
	}

	switch n := node.(type) {
	case nil:
		return "(NULL)"
	case *lexer.Token:
		if n == nil {
			return "(NULL)"
		}
		return tokenToStr(n, depth, typeName)
	case *ast.Node:
		if n == nil {
			return "(NULL)"
		}
		switch n.Type {
		case ast.Not:
			if depth >= 0 {
				return fmt.Sprintf("%s『%sに演算子『not』を適用した式』", named(""), nodeToStr(n.Block(0), depth, ""))
			}
			return named("演算子") + "『not』"
		case ast.Op:
			op := operatorLabel(n.Operator)
			if depth >= 0 {
				left := nodeToStr(n.Block(0), depth, "")
				right := nodeToStr(n.Block(1), depth, "")
				if n.Operator == "eq" {
					return fmt.Sprintf("%s『%sと%sが等しいかどうかの比較』", named(""), left, right)
				}
				return fmt.Sprintf("%s『%sと%sに演算子『%s』を適用した式』", named(""), left, right, op)
			}
			return fmt.Sprintf("%s『%s』", named("演算子"), op)
		case ast.Number:
			return named("数値") + literalToStr(n.Value)
		case ast.BigInt:
			return named("巨大整数") + literalToStr(n.Value)
		case ast.String:
			return fmt.Sprintf("%s『%s』", named("文字列"), literalToStr(n.Value))
		case ast.Word:
			return fmt.Sprintf("%s『%s』", named("単語"), literalToStr(n.Value))
		case ast.Func:
			return fmt.Sprintf("%s『%s』", named("関数"), n.Name)
		case ast.EOL:
			return "行の末尾"
		}
		name := n.Name
		if name == "" {
			name = literalToStr(n.Value)
		}
		if name == "" {
			name = string(n.Type)
		}
		return fmt.Sprintf("%s『%s』", named(""), name)
	}
	return "(NULL)"
}

func tokenToStr(t *lexer.Token, depth int, typeName string) string {
	named := func(def string) string {
		if typeName != "" {
			return typeName
		}
		return def
	}
	switch t.Type {
	case lexer.TypeNumber:
		return named("数値") + literalToStr(t.Value)
	case lexer.TypeBigInt:
		return named("巨大整数") + literalToStr(t.Value)
	case lexer.TypeString:
		return fmt.Sprintf("%s『%s』", named("文字列"), literalToStr(t.Value))
	case lexer.TypeWord:
		return fmt.Sprintf("%s『%s』", named("単語"), literalToStr(t.Value))
	case lexer.TypeFunc:
		return fmt.Sprintf("%s『%s』", named("関数"), literalToStr(t.Value))
	case lexer.TypeEOL:
		return "行の末尾"
	case lexer.TypeEOF:
		return "ファイルの末尾"
	}
	name := literalToStr(t.Value)
	if name == "" {
		name = string(t.Type)
	}
	return fmt.Sprintf("%s『%s』", named(""), name)
}

// operatorLabel renders the operators that read better as symbols or words.
var operatorLabels = map[string]string{
	"eq": "＝", "not": "!", "gt": ">", "lt": "<", "and": "かつ", "or": "または",
}

func operatorLabel(op string) string {
	if label, ok := operatorLabels[op]; ok {
		return label
	}
	return op
}

// literalToStr renders a token or node value the way JavaScript's string
// conversion would, so error messages read the same as the TypeScript ones.
func literalToStr(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return value.NumberToString(x)
	case int:
		return fmt.Sprintf("%d", x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	return fmt.Sprintf("%v", v)
}

// makeStackBalanceReport explains what was left on the calculation stack, and
// lists how the functions called nearby expect to be used (#1093).
func makeStackBalanceReport(stack []*ast.Node, recentlyCalledFunc []*lexer.FuncItem) string {
	words := make([]string, 0, len(stack))
	for _, t := range stack {
		w := nodeToStr(t, 1, "")
		if t.Josi != "" {
			w += t.Josi
		}
		words = append(words, w)
	}

	var descFunc strings.Builder
	for _, f := range recentlyCalledFunc {
		descFunc.WriteString(" - ")
		for i, arg := range f.Josi {
			descFunc.WriteRune(rune('A' + i))
			if len(arg) == 1 {
				descFunc.WriteString(arg[0])
			} else {
				descFunc.WriteString("(" + strings.Join(arg, "|") + ")")
			}
		}
		descFunc.WriteString(f.Name + "\n")
	}
	return fmt.Sprintf("未解決の単語があります: [%s]\n次の命令の可能性があります:\n%s",
		strings.Join(words, ","), descFunc.String())
}
