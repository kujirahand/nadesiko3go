package format

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// symbolForms は、記号トークンの種類ごとに、全角→半角の折り畳み(prepare.
// Convert1ch)をした後の綴りとして認める形と、その書き換え先を表す。
// 折り畳んだ結果がここに無い綴り(『≧』など)は書き換えずに残す。
//
// 掛け算・割り算・比較は、なでしこらしい『×』『÷』『≧』『≦』『≠』に揃える(#120)。
var symbolForms = map[lexer.TokenType]map[string]string{
	"(":     {"(": "("},
	")":     {")": ")"},
	"[":     {"[": "["},
	"]":     {"]": "]"},
	"{":     {"{": "{"},
	"}":     {"}": "}"},
	"+":     {"+": "+"},
	"-":     {"-": "-"},
	"*":     {"*": "×", "×": "×"},
	"÷":     {"/": "÷", "÷": "÷"},
	"%":     {"%": "%"},
	"^":     {"^": "^"},
	"&":     {"&": "&"},
	":":     {":": ":"},
	"eq":    {"=": "=", "==": "=="},
	"gt":    {">": ">"},
	"lt":    {"<": "<"},
	"gteq":  {">=": "≧", "=>": "≧", "≧": "≧"},
	"lteq":  {"<=": "≦", "=<": "≦", "≦": "≦"},
	"noteq": {"!=": "≠", "<>": "≠", "≠": "≠"},
}

// openers・closersは、内側に空白を置かない括弧。
var openers = map[lexer.TokenType]bool{"(": true, "[": true, "{": true}
var closers = map[lexer.TokenType]bool{")": true, "]": true, "}": true}

// spaceChars は、トークンの間の空白として扱う文字。
const spaceChars = " \t　"

// textEdit は、元のソースのrune範囲[start, end)をtextに置き換える編集。
type textEdit struct {
	start, end int
	text       string
}

// applyEdits は、重ならない編集をまとめてcodeに適用する。
func applyEdits(code string, edits []textEdit) string {
	if len(edits) == 0 {
		return code
	}
	// 同じ位置では、挿入(start == end)を置き換えより先に適用する。
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].start != edits[j].start {
			return edits[i].start < edits[j].start
		}
		return edits[i].end < edits[j].end
	})
	runes := []rune(code)
	var b strings.Builder
	pos := 0
	for _, e := range edits {
		if e.start < pos || e.end > len(runes) {
			continue // 重なる編集は捨てる(起きないはずだが念のため)
		}
		b.WriteString(string(runes[pos:e.start]))
		b.WriteString(e.text)
		pos = e.end
	}
	b.WriteString(string(runes[pos:]))
	return b.String()
}

// normalize は、行の中身の書き方を揃える(#120)。
//
//   - 演算子(『+』『=』『&』『×』『≧』など)の前後には空白を1つ置く。ただし
//     両隣が全角記号(『「あ」&「い」』の『」』『「』など)なら空白を置かない。
//     単項の『-』『+』の直後にも空白を置かない。
//   - 開き括弧の直後・閉じ括弧の直前・読点やコロンの直前の空白は取り除く。
//   - それ以外のトークンの間の連続した空白は1つにまとめる(空白の無い所には
//     足さない)。
//   - 数値と記号(括弧・演算子)の全角文字は半角にする。『*』は『×』に、
//     『/』は『÷』に、『>=』『<=』『!=』『<>』は『≧』『≦』『≠』にする。
//   - joinJosiがtrueなら、値と助詞の間の空白(『3 を』)を取り除く。
//
// 文字列リテラル・コメント・単語の中身には触れない。行頭のインデントと
// 行末の空白はSourceが扱うので、ここでは行の途中だけを書き換える。
// 行の数は変えないので、整形前の構文木の行番号はそのまま使える。
//
// 呼び出し側は、書き換えた結果の構文構造が変わらないことを確かめる。
func normalize(code, filename string, joinJosi bool) string {
	tokens, ok := scanTokens(code, filename)
	if !ok {
		return code
	}
	runes := []rune(code)
	var edits []textEdit

	// 各トークンの整形後の綴り。隣の文字が全角記号かどうかの判定にも使う。
	texts := make([]string, len(tokens))
	for i, t := range tokens {
		raw := string(runes[t.start:t.end])
		texts[i] = raw
		if t.startLine == t.endLine {
			texts[i] = normalizeToken(t, raw, joinJosi)
			if texts[i] != raw {
				edits = append(edits, textEdit{t.start, t.end, texts[i]})
			}
		}
	}
	// sameLine(i, j)は、トークンi・jが同じ行で隣り合う中身のトークンかどうか。
	sameLine := func(i, j int) bool {
		if i < 0 || j >= len(tokens) {
			return false
		}
		return tokens[i].significant() && tokens[j].significant() && tokens[j].startLine == tokens[i].endLine
	}
	// unary(i)は、演算子iが単項の『-』『+』かどうか。直前が値でなければ単項。
	unary := func(i int) bool {
		t := tokens[i]
		if t.Type != "-" && t.Type != "+" {
			return false
		}
		if !sameLine(i-1, i) {
			return true
		}
		p := tokens[i-1]
		if p.Josi != "" {
			return true // 『Aに -1を足す』のように助詞の後は単項
		}
		switch {
		case p.Type == lexer.TypeNumber, p.Type == lexer.TypeBigInt,
			p.Type == lexer.TypeString, p.Type == lexer.TypeStringEx,
			p.Type == lexer.TypeWord, closers[p.Type]:
			return false
		}
		return true
	}
	// spaced(i)は、二項演算子iの前後に空白を置くかどうか。
	spaced := func(i int) bool {
		if sameLine(i-1, i) && sameLine(i, i+1) &&
			isWideSymbol(lastRune(texts[i-1])) && isWideSymbol(firstRune(texts[i+1])) {
			return false // 『「あ」&「い」』のように全角記号に挟まれている
		}
		return true
	}

	for i, t := range tokens {
		if !sameLine(i, i+1) {
			continue
		}
		n := tokens[i+1]
		if n.start < t.end {
			continue
		}
		gap := string(runes[t.end:n.start])
		if strings.Trim(gap, spaceChars) != "" {
			continue // 空白以外が挟まっている(字句解析が読み飛ばした文字など)
		}
		if t.Type == "$" || n.Type == "$" {
			continue // プロパティアクセス(『A.b』)の空白はそのまま
		}
		want := gap
		if want != "" {
			want = " "
		}
		switch {
		case openers[t.Type] && t.Josi == "":
			want = ""
		case closers[n.Type], n.Type == ":", n.Type == lexer.TypeComma:
			want = ""
		case operators[t.Type] && unary(i):
			want = ""
		case operators[t.Type] && !spaced(i), operators[n.Type] && !unary(i+1) && !spaced(i+1):
			want = ""
		case operators[t.Type], operators[n.Type] && !unary(i+1):
			want = " "
		case t.Type == lexer.TypeComma && texts[i] != ",":
			want = "" // 『、』の後ろには空白を置かない
		}
		if gap != want {
			edits = append(edits, textEdit{t.end, n.start, want})
		}
	}
	return applyEdits(code, edits)
}

// operators は、前後の空白を揃える演算子。
var operators = map[lexer.TokenType]bool{
	"+": true, "-": true, "*": true, "÷": true, "÷÷": true, "**": true,
	"%": true, "^": true, "&": true,
	"eq": true, "gt": true, "lt": true, "gteq": true, "lteq": true, "noteq": true,
	"===": true, "!==": true, "shift_l": true, "shift_r": true, "shift_r0": true,
}

// isWideSymbol は、rが全角の記号(『「』『」』『（』など)かどうかを返す。
// 漢字・かななどの文字は含まない。
func isWideSymbol(r rune) bool {
	return r >= 0x80 && (unicode.IsPunct(r) || unicode.IsSymbol(r))
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

func lastRune(s string) rune {
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

// normalizeToken は、1行に収まるトークン1つの綴りを揃えた結果を返す。
// rawはそのトークンの元のソース上の綴り(助詞を含む)。
func normalizeToken(t srcToken, raw string, joinJosi bool) string {
	body, josi := splitJosi(t, raw)
	switch {
	case t.Type == lexer.TypeNumber:
		body = foldRunes(body)
	case symbolForms[t.Type] != nil:
		folded := foldRunes(strings.TrimRight(body, spaceChars))
		to, ok := symbolForms[t.Type][folded]
		if !ok {
			return raw
		}
		// 記号と助詞の間の空白は、助詞を繋げるかどうかで扱いを決める。
		body = to + body[len(strings.TrimRight(body, spaceChars)):]
	}
	if josi != "" && joinJosi {
		body = strings.TrimRight(body, spaceChars)
	}
	return body + josi
}

// splitJosi は、トークンの綴りを本体(末尾の空白を含む)と助詞に分ける。
// 助詞が綴りの末尾に見つからない場合は、全体を本体として返す。
func splitJosi(t srcToken, raw string) (body, josi string) {
	for _, j := range []string{t.RawJosi, t.Josi} {
		if j == "" || !strings.HasSuffix(raw, j) {
			continue
		}
		body = strings.TrimSuffix(raw, j)
		if strings.TrimRight(body, spaceChars) == "" {
			continue // 本体が無い(助詞だけ)なら分けない
		}
		return body, j
	}
	return raw, ""
}

// foldRunes は、prepareが字句解析の前に行う1文字ずつの折り畳み(全角英数・
// 記号を半角に)を文字列全体に適用する。
func foldRunes(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(prepare.Convert1ch(r))
	}
	return b.String()
}
