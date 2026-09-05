package lexer

import (
	"path"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/lexer/josi"
)

// FuncItem describes a name the lexer and parser know about: a function, or a
// variable or constant. The implementation of a function lives in stdlib; only
// what the lexer and parser need is kept here.
type FuncItem struct {
	Name string
	Type string     // "func" / "var" / "const"
	Josi [][]string // 引数ごとに受け付ける助詞

	// IsVariableJosi marks a function whose last parameter takes any number
	// of arguments.
	IsVariableJosi bool

	VarNames     []string
	FuncPointers []string // 関数ポインタで受け取る引数名。それ以外は ""
	IsExport     *bool    // nil はモジュールの既定値に従う

	// AsyncFn is set once the parser finds the function needs to await.
	// It propagates outward: a function calling an async function is async.
	AsyncFn bool

	// Value holds the current value of a variable or constant.
	Value any

	// Pure marks a function with no side effects (実行速度優先 で使う).
	Pure bool
	// ReturnNone marks a function that returns nothing.
	ReturnNone bool
}

// FuncList maps a namespaced function name (`mod__name`) to its definition.
type FuncList map[string]*FuncItem

// Lexer holds the state that survives across the token replacement passes:
// the function table built while reading, the module list, and each module's
// export default.
type Lexer struct {
	FuncList     FuncList
	ModList      []string
	ModuleExport map[string]bool
	// Warnings collects the messages the TypeScript version sends to its
	// logger. They are not errors and never stop lexing.
	Warnings []string
}

func NewLexer() *Lexer {
	return &Lexer{
		FuncList:     FuncList{},
		ModuleExport: map[string]bool{},
	}
}

// FilenameToModName strips the directory and the .nako/.nako3 suffix.
func FilenameToModName(filename string) string {
	if filename == "" {
		return "main"
	}
	// Windowsのパス記号を / に置換する
	filename = strings.NewReplacer(`\`, "/", ":", "/").Replace(filename)
	if strings.Contains(filename, "/") {
		filename = path.Base(filename)
	}
	filename = strings.TrimSuffix(filename, ".nako3")
	filename = strings.TrimSuffix(filename, ".nako")
	return filename
}

// reservedWords maps a word to the token type it becomes
// (nako_reserved_words.mts equivalent).
var reservedWords = map[string]TokenType{
	"もし":           "もし",
	"回":            "回",
	"回繰返":          "回", // (#924)
	"間":            "間",
	"間繰返":          "間", // (#927)
	"繰返":           "繰返",
	"増繰返":          "増繰返", // (#1140)
	"減繰返":          "減繰返",
	"後判定":          "後判定", // (#1147)
	"反復":           "反復",
	"抜":            "抜ける",
	"続":            "続ける",
	"戻":            "戻る",
	"先":            "先に",
	"次":            "次に",
	"代入":           "代入",
	"実行速度優先":       "実行速度優先",
	"パフォーマンスモニタ適用": "パフォーマンスモニタ適用", // (#986)
	"定":            "定める",
	"逐次実行":         "逐次実行", // 廃止 #1611 ただし念のため残してある
	"条件分岐":         "条件分岐",
	"増":            "増",
	"減":            "減",
	"変数":           "変数",
	"定数":           "定数",
	"エラー監視":        "エラー監視", // 例外処理:エラーならばと対
	"エラー":          "エラー",
	"それ":           TypeWord,
	"そう":           TypeWord,    // 「それ」のエイリアス
	"関数":           TypeDefFunc, // 無名関数の定義用
	"インデント構文":      "インデント構文",
	"非同期モード":       "非同期モード",  // (#637)
	"DNCLモード":      "DNCLモード", // (#1140)
	"DNCL2モード":     "DNCL2モード",
	"モード設定":        "モード設定", // (#1020)
	"取込":           "取込",
	"モジュール公開既定値":   "モジュール公開既定値",
	"厳チェック":        "厳チェック", // (#1698)
}

// reservedWordsList lists every reserved word in insertion order matching nako_reserved_words.mjs.
var reservedWordsList = []string{
	"もし", "回", "回繰返", "間", "間繰返", "繰返", "増繰返", "減繰返",
	"後判定", "反復", "抜", "続", "戻", "先", "次", "代入",
	"実行速度優先", "パフォーマンスモニタ適用", "定", "逐次実行",
	"条件分岐", "増", "減", "変数", "定数", "エラー監視", "エラー",
	"それ", "そう", "関数", "インデント構文", "非同期モード", "DNCLモード",
	"DNCL2モード", "モード設定", "取込", "モジュール公開既定値", "厳チェック",
}

// ReservedWords lists every reserved word, for plugin_system::予約語一覧取得.
func ReservedWords() []string {
	out := make([]string, len(reservedWordsList))
	copy(out, reservedWordsList)
	return out
}

// opPriority gives each operator its binding power (nako_parser_const.mts).
// The lexer only needs to know whether a token is an operator at all, when it
// decides if a minus sign belongs to the number that follows it.
var opPriority = map[TokenType]int{
	"and": 1, "or": 1,
	"eq": 2, "noteq": 2, "===": 2, "!==": 2, "gt": 2, "gteq": 2, "lt": 2, "lteq": 2,
	"&": 3,
	"+": 4, "-": 4, "shift_l": 4, "shift_r": 4, "shift_r0": 4,
	"*": 5, "/": 5, "÷": 5, "÷÷": 5, "%": 5,
	"^": 6, "**": 6,
}

// ReplaceTokens runs the two correction passes the parser expects, and appends
// the trailing eol and eof when this is the first (outermost) call.
func (l *Lexer) ReplaceTokens(tokens []Token, isFirst bool, filename string) ([]Token, error) {
	tokens = l.preDefineFunc(tokens, filename)
	tokens, err := l.replaceWord(tokens)
	if err != nil {
		return nil, err
	}
	if !isFirst {
		return tokens, nil
	}

	eol := Token{Type: TypeEOL, Value: "---", Indent: -1}
	eof := Token{Type: TypeEOF, Value: "", Indent: -1}
	if len(tokens) > 0 {
		last := tokens[len(tokens)-1]
		eol.Line, eol.File, eol.Offset, eol.Length = last.Line, last.File, last.Offset, last.Length
		eof.Line, eof.File, eof.Offset, eof.Length = last.Line, last.File, last.Offset, last.Length
	}
	return append(tokens, eol, eof), nil
}

// ExpandCodeTokens tokenizes expressions embedded in interpolated strings.
// ReplaceTokens creates TypeCode placeholders first, matching the two-pass
// pipeline in the TypeScript tokenizer.
func (l *Lexer) ExpandCodeTokens(tokens []Token, filename string) ([]Token, error) {
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Type != TypeCode {
			continue
		}
		parent := tokens[i]
		raw, err := Tokenize(parent.StringValue(), parent.Line, filename)
		if err != nil {
			return nil, err
		}
		children, err := l.ReplaceTokens(raw, false, filename)
		if err != nil {
			return nil, err
		}
		for j := range children {
			children[j].Offset += parent.Offset
		}
		tokens = append(tokens[:i], append(children, tokens[i+1:]...)...)
		i--
	}
	return tokens, nil
}

// preDefineFunc reads function definitions ahead of parsing and performs the
// token substitutions that depend on them.
func (l *Lexer) preDefineFunc(tokens []Token, filename string) []Token {
	i := 0
	isFuncPointer := false

	// readArgs reads a parenthesised argument list, collapsing repeats of the
	// same name into one entry that accepts every particle seen for it.
	readArgs := func() (josiList [][]string, varNames []string, funcPointers []string) {
		if i >= len(tokens) || tokens[i].Type != "(" {
			return nil, nil, nil
		}
		i++
		type arg struct {
			name      string
			isPointer bool
		}
		var args []arg
		keys := map[string][]string{}
		for i < len(tokens) {
			t := tokens[i]
			i++
			if t.Type == ")" {
				break
			}
			if t.Type == TypeFunc {
				isFuncPointer = true
				continue
			}
			if t.Type == "|" || t.Type == TypeComma {
				continue
			}
			name := t.StringValue()
			args = append(args, arg{name: name, isPointer: isFuncPointer})
			isFuncPointer = false
			keys[name] = append(keys[name], t.Josi)
		}
		already := map[string]bool{}
		for _, a := range args {
			if already[a.name] {
				continue
			}
			already[a.name] = true
			josiList = append(josiList, keys[a.name])
			varNames = append(varNames, a.name)
			if a.isPointer {
				funcPointers = append(funcPointers, a.name)
			} else {
				funcPointers = append(funcPointers, "")
			}
		}
		return josiList, varNames, funcPointers
	}

	for i < len(tokens) {
		t := &tokens[i]

		// モジュール公開既定値の指定
		if t.Type == TypeNot && len(tokens)-i > 3 {
			prevType := TokenType(TypeEOL)
			if i >= 1 {
				prevType = tokens[i-1].Type
			}
			if prevType == TypeEOL && tokens[i+1].Type == TypeWord && tokens[i+1].StringValue() == "モジュール公開既定値" {
				tokens[i+1].Type = "モジュール公開既定値"
				next := tokens[i+2]
				if next.Type == TypeString {
					switch next.StringValue() {
					case "非公開":
						l.ModuleExport[FilenameToModName(t.File)] = false
						i += 3
						continue
					case "公開":
						l.ModuleExport[FilenameToModName(t.File)] = true
						i += 3
						continue
					}
				}
			}
		}

		// 「xxには**」は暗黙的な無名関数の定義とする
		if t.Type == TypeWord && t.Josi == "には" {
			def := Token{
				Type: TypeDefFunc, Value: "関数", Indent: t.Indent, Line: t.Line,
				Column: t.Column, File: t.File, Offset: t.Offset + t.Length,
			}
			tokens = insertToken(tokens, i+1, def)
			i++
			continue
		}
		// 「永遠に繰り返す」を「永遠の間」に置換 #1686
		if t.Type == TypeWord && t.StringValue() == "永遠" && t.Josi == "に" {
			if i+1 < len(tokens) && tokens[i+1].StringValue() == "繰返" {
				tokens[i+1].Value = "間"
				tokens[i+1].Josi = "の"
			}
			i++
			continue
		}
		// 「N回」を「N」「回」に分割する
		if t.Type == TypeWord && t.Josi == "" && runeLen(t.StringValue()) >= 2 && strings.HasSuffix(t.StringValue(), "回") {
			v := t.StringValue()
			t.Value = strings.TrimSuffix(v, "回")
			kai := Token{
				Type: "回", Value: "回", Indent: t.Indent, Line: t.Line,
				Column: t.Column, File: t.File, Offset: t.Offset + t.Length - 1, Length: 1,
			}
			t.Length--
			tokens = insertToken(tokens, i+1, kai)
			i++
			t = &tokens[i-1]
		}
		// 予約語の置換
		if t.Type == TypeWord {
			if rtype, ok := reservedWords[t.StringValue()]; ok {
				t.Type = rtype
			}
			if t.StringValue() == "そう" {
				t.Value = "それ"
			}
		}

		// 関数定義の確認
		if t.Type != TypeDefTest && t.Type != TypeDefFunc {
			i++
			continue
		}
		// 無名関数か普通の関数定義かは、一つ前が改行かどうかで決まる。
		// 先頭のトークンは改行の後ろとみなす。
		prevType := TokenType(TypeEOL)
		if i >= 1 {
			prevType = tokens[i-1].Type
		}
		isMumei := prevType != TypeEOL
		defIndex := i
		i++ // skip "●" or "関数"

		var josiList [][]string
		var varNames, funcPointers []string
		funcName := ""
		funcNameIndex := -1
		var isExport *bool

		// 関数の属性指定
		if i < len(tokens) && tokens[i].Type == "{" {
			i++
			attr := ""
			if i < len(tokens) && tokens[i].Type == TypeWord {
				attr = tokens[i].StringValue()
			}
			switch attr {
			case "公開", "エクスポート":
				v := true
				isExport = &v
			case "非公開":
				v := false
				isExport = &v
			default:
				l.Warnings = append(l.Warnings, "不明な関数属性『"+attr+"』が指定されています。")
			}
			i++
			if i < len(tokens) && tokens[i].Type == "}" {
				i++
			}
		}
		// 関数名の前に引数定義
		if i < len(tokens) && tokens[i].Type == "(" {
			josiList, varNames, funcPointers = readArgs()
		}
		// 関数名
		if !isMumei && i < len(tokens) && tokens[i].Type == TypeWord {
			funcNameIndex = i
			funcName = tokens[i].StringValue()
			i++
		}
		// 関数名の後で引数定義
		if len(josiList) == 0 && i < len(tokens) && tokens[i].Type == "(" {
			josiList, varNames, funcPointers = readArgs()
		}

		// 名前のある関数だけを関数テーブルに登録する
		if funcName != "" && funcNameIndex >= 0 {
			modName := FilenameToModName(tokens[defIndex].File)
			if modName == "" {
				modName = FilenameToModName(filename)
			}
			funcName = modName + "__" + funcName
			if _, exists := l.FuncList[funcName]; exists {
				// main__ は省略して表示する #1223
				l.Warnings = append(l.Warnings,
					"関数『"+strings.TrimPrefix(funcName, "main__")+"』は既に定義されています。")
			}
			tokens[funcNameIndex].Value = funcName
			l.FuncList[funcName] = &FuncItem{
				Type: "func", Josi: josiList, IsExport: isExport,
				VarNames: varNames, FuncPointers: funcPointers,
			}
		}
	}
	return tokens
}

// replaceWord turns words into function calls, folds particles into tokens the
// parser understands, and drops comments.
func (l *Lexer) replaceWord(tokens []Token) ([]Token, error) {
	var comment []string
	i := 0
	isFuncPointer := false
	var namespaceStack []string

	lastType := func() TokenType {
		if i <= 0 {
			return TypeEOL
		}
		return tokens[i-1].Type
	}
	modSelf := "main"
	if len(tokens) > 0 {
		modSelf = FilenameToModName(tokens[0].File)
	}

	for i < len(tokens) {
		t := &tokens[i]

		// 名前空間の切り替え
		if t.Type == TypeWord || t.Type == TypeFunc {
			switch t.StringValue() {
			case "名前空間設定":
				if isFuncPointer {
					return nil, &errs.NakoError{Kind: errs.Lexer, File: t.File, Line: t.Line,
						Msg: "名前空間設定の関数参照を取得することはできません。"}
				}
				namespaceStack = append(namespaceStack, modSelf)
				if i >= 1 {
					modSelf = tokens[i-1].StringValue()
				}
			case "名前空間ポップ":
				if isFuncPointer {
					return nil, &errs.NakoError{Kind: errs.Lexer, File: t.File, Line: t.Line,
						Msg: "名前空間ポップの関数参照を取得することはできません。"}
				}
				if n := len(namespaceStack); n > 0 {
					modSelf = namespaceStack[n-1]
					namespaceStack = namespaceStack[:n-1]
				}
			}
		}

		// 関数を強制的に置換( word => func )
		if t.Type == TypeWord && t.StringValue() != "それ" {
			funcName := t.StringValue()
			if !strings.Contains(funcName, "__") {
				// まず自身のモジュールを探す
				gname := modSelf + "__" + funcName
				if fo, ok := l.FuncList[gname]; ok && fo.Type == "func" {
					tokens, i = l.markAsFunc(tokens, i, gname, &isFuncPointer)
					continue
				}
				// 次に取り込んだモジュールを探す
				for _, mod := range l.ModList {
					gname := mod + "__" + funcName
					fo, ok := l.FuncList[gname]
					if !ok || fo.Type != "func" || !exported(fo, l.ModuleExport, mod) {
						continue
					}
					tokens, i = l.markAsFunc(tokens, i, gname, &isFuncPointer)
					break
				}
			}
			// 名前空間なしでも探す。TS版は上でモジュール関数に置換していても
			// 元の名前で引き直すので、そこも合わせてある。
			if fo, ok := l.FuncList[funcName]; ok && fo.Type == "func" {
				wasPointer := isFuncPointer
				tokens, i = l.markAsFunc(tokens, i, "", &isFuncPointer)
				if wasPointer {
					continue
				}
			}
			t = &tokens[i]
		}

		isFuncPointer = false
		if t.Type == TypeFunc && t.StringValue() == "{関数}" {
			i++
			isFuncPointer = true
			continue
		}

		// 数字につくマイナス記号を判定する
		// (ng) 5 - 3 || word - 3
		// (ok) (行頭)-3 || 1 * -3 || Aに -3を 足す
		if t.Type == "-" && i+1 < len(tokens) {
			nextType := tokens[i+1].Type
			if nextType == TypeNumber || nextType == TypeBigInt {
				// 一つ前が行頭・演算子・助詞付きの語句なら負数
				lt := lastType()
				_, isOp := opPriority[lt]
				if lt == TypeEOL || isOp || (i >= 1 && tokens[i-1].Josi != "") {
					tokens = removeToken(tokens, i)
					if nextType == TypeNumber {
						tokens[i].Value = -tokens[i].NumberValue()
					} else {
						tokens[i].Value = "-" + tokens[i].StringValue()
					}
					t = &tokens[i]
				}
			}
		}

		// 助詞の「は」を = に展開する
		if t.Josi == "は" {
			if t.RawJosi == "" {
				t.RawJosi = t.Josi
			}
			offset := t.Offset + t.Length - runeLen(t.RawJosi)
			tokens = insertToken(tokens, i+1, Token{
				Type: "eq", Indent: t.Indent, Line: t.Line, Column: t.Column,
				File: t.File, Offset: offset,
			})
			tokens[i].Josi, tokens[i].RawJosi = "", ""
			tokens[i].Length = offset - tokens[i].Offset
			i += 2
			continue
		}
		// 「とは」を一つの単語にする
		if t.Josi == "とは" {
			if t.RawJosi == "" {
				t.RawJosi = t.Josi
			}
			offset := t.Offset + t.Length - runeLen(t.RawJosi)
			tokens = insertToken(tokens, i+1, Token{
				Type: "とは", Indent: t.Indent, Line: t.Line, Column: t.Column,
				File: t.File, Offset: offset,
			})
			tokens[i].Josi, tokens[i].RawJosi = "", ""
			tokens[i].Length = offset - tokens[i].Offset
			i += 2
			continue
		}
		// 「ならば」「たら」などをトークンにする
		if josi.TararebaMap[t.Josi] {
			value := "ならば"
			if t.Josi == "でなければ" || t.Josi == "なければ" {
				value = "でなければ"
			}
			if t.RawJosi == "" {
				t.RawJosi = value
			}
			offset := t.Offset + t.Length - runeLen(t.RawJosi)
			tokens = insertToken(tokens, i+1, Token{
				Type: "ならば", Value: value, Indent: t.Indent, Line: t.Line,
				Column: t.Column, File: t.File, Offset: offset,
			})
			tokens[i].Josi, tokens[i].RawJosi = "", ""
			tokens[i].Length = offset - tokens[i].Offset
			i += 2
			continue
		}

		// '_' + 改行 を飛ばす(演算子の直後に改行を入れたいときに使う)
		if t.Type == TypeEOLUnderscore {
			tokens = removeToken(tokens, i)
			continue
		}
		// コメントを飛ばし、次の改行に埋め込む
		if t.Type == TypeLineComment || t.Type == TypeRangeComment {
			comment = append(comment, t.StringValue())
			tokens = removeToken(tokens, i)
			continue
		}
		if t.Type == TypeEOL {
			t.Value = strings.Join(comment, "/")
			comment = nil
		}
		i++
	}
	return tokens, nil
}

// markAsFunc turns tokens[at] into a func (or func_pointer) token. When it is a
// function pointer, the preceding 「{関数}」 token is dropped and the index moves
// back with it. An empty name leaves the token value alone.
func (l *Lexer) markAsFunc(tokens []Token, at int, name string, isFuncPointer *bool) ([]Token, int) {
	tokens[at].Type = TypeFunc
	if name != "" {
		tokens[at].Value = name
	}
	if *isFuncPointer {
		tokens[at].Type = "func_pointer"
		*isFuncPointer = false
		if at >= 1 {
			tokens = removeToken(tokens, at-1)
			at--
		}
	}
	return tokens, at
}

// exported reports whether a module function is visible outside its module.
// An explicit isExport wins; otherwise the module default applies, and that
// default only hides the function when it is explicitly false.
func exported(fo *FuncItem, moduleExport map[string]bool, mod string) bool {
	if fo.IsExport != nil {
		return *fo.IsExport
	}
	def, ok := moduleExport[mod]
	return !ok || def
}

func insertToken(tokens []Token, at int, t Token) []Token {
	tokens = append(tokens, Token{})
	copy(tokens[at+1:], tokens[at:])
	tokens[at] = t
	return tokens
}

func removeToken(tokens []Token, at int) []Token {
	return append(tokens[:at], tokens[at+1:]...)
}
