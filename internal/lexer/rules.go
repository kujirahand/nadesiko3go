package lexer

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kujirahand/nadesiko3go/internal/lexer/josi"
)

// Ported from nako_lex_rules.mts. Character classes that the TypeScript version
// writes with surrogate ranges are written here as the rune ranges they stand
// for, since Go matches runes.
var (
	kanakanjiRE     = regexp.MustCompile(`^[\x{3005}\x{4E00}-\x{9FCF}_a-zA-Z0-9ァ-ヶー\x{2460}-\x{24FF}\x{2776}-\x{277F}\x{3251}-\x{32BF}]+`)
	hiraRE          = regexp.MustCompile(`^[ぁ-ん]`)
	allHiraganaRE   = regexp.MustCompile(`^[ぁ-ん]+$`)
	hiraRunRE       = regexp.MustCompile(`[ぁ-ん]+`)
	hiraTailRE      = regexp.MustCompile(`[ぁ-ん]+$`)
	wordHasIjoIkaRE = regexp.MustCompile(`^.+(以上|以下|超|未満)$`)
	wordSpecialRE   = regexp.MustCompile(`^(かつ|または)`)
	hiraKanRE       = regexp.MustCompile(`[ぁ-ん]間$`)
	// TS版の範囲コメントは /(^\s+|\s+$)/ を g フラグなしで置換するので、
	// 前後の空白のうち先に見つかった片方しか落ちない。
	trimOnceRE = regexp.MustCompile(`^\s+|\s+$`)

	// UnitRE drops the unit that may follow a number (#994).
	UnitRE = regexp.MustCompile(`^(円|ドル|元|歩|㎡|坪|度|℃|°|個|つ|本|冊|才|歳|匹|枚|皿|セット|羽|人|件|行|列|機|品|m|mm|cm|km|g|kg|t|b|mb|kb|gb)`)
	// CSSUnitRE turns a number with a CSS unit into a string (#1811).
	CSSUnitRE = regexp.MustCompile(`^(px|em|ex|rem|vw|vh|vmin|vmax)`)
)

// parseResult mirrors NakoLexParseResult: what a callback-driven rule consumed.
type parseResult struct {
	src    string // 残りのソース
	res    string // トークンの値
	josi   string // 直後の助詞
	numEOL int    // 消費した改行の数
}

type parserFunc func(src string, trimOkurigana bool) (parseResult, error)

type rule struct {
	name     TokenType
	pattern  *regexp.Regexp
	readJosi bool
	cb       func(string) any
	parser   parserFunc
}

// errorRead builds the parser for a stray closing quote.
func errorRead(ch string) parserFunc {
	return func(string, bool) (parseResult, error) {
		return parseResult{}, fmt.Errorf("突然の『%s』があります。", ch)
	}
}

func stringParser(begin, close string) parserFunc {
	return func(src string, _ bool) (parseResult, error) {
		return cbString(begin, close, src)
	}
}

// rules is tried from the top down; the first match wins.
var rules []rule

func init() {
	rules = []rule{
		{name: TypeKokomade, pattern: regexp.MustCompile(`^;;;`)}, // #925
		{name: TypeEOL, pattern: regexp.MustCompile(`^\n`)},
		{name: TypeEOL, pattern: regexp.MustCompile(`^;`)},
		{name: TypeSpace, pattern: regexp.MustCompile(`^(\x20|\t|　|・|⎿ |└|｜)+`)}, // #877,#1015
		{name: TypeComma, pattern: regexp.MustCompile(`^,`)},
		{name: TypeLineComment, pattern: regexp.MustCompile(`^#[^\n]*`)},
		{name: TypeLineComment, pattern: regexp.MustCompile(`^//[^\n]*`)},
		{name: TypeRangeComment, pattern: regexp.MustCompile(`^/\*`), parser: cbRangeComment},
		{name: TypeDefTest, pattern: regexp.MustCompile(`^●テスト:`)},
		{name: TypeDefFunc, pattern: regexp.MustCompile(`^●`)},
		{name: "…", pattern: regexp.MustCompile(`^…`)},       // 範囲オブジェクト(#1704)
		{name: "…", pattern: regexp.MustCompile(`^\.{2,3}`)}, // 範囲オブジェクト(#1704)
		// 多倍長整数リテラル。整数の末尾に「n」が付くだけなので数値より先に判定する。
		{name: TypeBigInt, pattern: regexp.MustCompile(`^0[xX][0-9a-fA-F]+(_[0-9a-fA-F]+)*n`), readJosi: true},
		{name: TypeBigInt, pattern: regexp.MustCompile(`^0[oO][0-7]+(_[0-7]+)*n`), readJosi: true},
		{name: TypeBigInt, pattern: regexp.MustCompile(`^0[bB][0-1]+(_[0-1]+)*n`), readJosi: true},
		{name: TypeBigInt, pattern: regexp.MustCompile(`^\d+(_\d+)*?n`), readJosi: true},
		// 16進/8進/2進。この後に単位を読む処理が入る(#994)
		{name: TypeNumber, pattern: regexp.MustCompile(`^0[xX][0-9a-fA-F]+(_[0-9a-fA-F]+)*`), readJosi: true, cb: parseNumberValue},
		{name: TypeNumber, pattern: regexp.MustCompile(`^0[oO][0-7]+(_[0-7]+)*`), readJosi: true, cb: parseNumberValue},
		{name: TypeNumber, pattern: regexp.MustCompile(`^0[bB][0-1]+(_[0-1]+)*`), readJosi: true, cb: parseNumberValue},
		// 小数点を挟む場合、小数点で始まる場合、小数点がない場合
		{name: TypeNumber, pattern: regexp.MustCompile(`^\d+(_\d+)*\.(\d+(_\d+)*)?([eE][+|\-]?\d+(_\d+)*)?`), readJosi: true, cb: parseNumberValue},
		{name: TypeNumber, pattern: regexp.MustCompile(`^\.\d+(_\d+)*([eE][+|\-]?\d+(_\d+)*)?`), readJosi: true, cb: parseNumberValue},
		{name: TypeNumber, pattern: regexp.MustCompile(`^\d+(_\d+)*([eE][+|\-]?\d+(_\d+)*)?`), readJosi: true, cb: parseNumberValue},
		{name: TypeKokokara, pattern: regexp.MustCompile(`^(ここから),?`)},
		{name: TypeKokomade, pattern: regexp.MustCompile(`^(ここまで|💧)`)},
		{name: TypeMoshi, pattern: regexp.MustCompile(`^もしも?`)},
		// 「ならば」は助詞として扱う
		{name: TypeChigaeba, pattern: regexp.MustCompile(`^違(えば)?`)},
		// 「回」「間」「繰返」などは replaceWord で word から変換する
		{name: "shift_r0", pattern: regexp.MustCompile(`^>>>`)},
		{name: "shift_r", pattern: regexp.MustCompile(`^>>`)},
		{name: "shift_l", pattern: regexp.MustCompile(`^<<`)},
		{name: "===", pattern: regexp.MustCompile(`^===`)}, // #999
		{name: "!==", pattern: regexp.MustCompile(`^!==`)}, // #999
		{name: "gteq", pattern: regexp.MustCompile(`^(≧|>=|=>)`)},
		{name: "lteq", pattern: regexp.MustCompile(`^(≦|<=|=<)`)},
		{name: "noteq", pattern: regexp.MustCompile(`^(≠|<>|!=)`)},
		{name: "←", pattern: regexp.MustCompile(`^(←|<--)`)}, // core#140で廃止された演算子(#891,#899)
		{name: "eq", pattern: regexp.MustCompile(`^(==|🟰🟰)`)},
		{name: "eq", pattern: regexp.MustCompile(`^(=|🟰)`)},
		{name: TypeLineComment, pattern: regexp.MustCompile(`^(!|💡)(インデント構文|ここまでだるい|DNCLモード|DNCL2モード|DNCL2)[^\n]*`)}, // #1184
		{name: TypeNot, pattern: regexp.MustCompile(`^(!|💡)`)}, // #1184 #1457
		{name: "gt", pattern: regexp.MustCompile(`^>`)},
		{name: "lt", pattern: regexp.MustCompile(`^<`)},
		{name: "and", pattern: regexp.MustCompile(`^(かつ|&&|and\s)`)},
		{name: "or", pattern: regexp.MustCompile(`^(または|或いは|あるいは|or\s|\|\|)`)},
		{name: "@", pattern: regexp.MustCompile(`^@`)},
		{name: "+", pattern: regexp.MustCompile(`^\+`)},
		{name: "-", pattern: regexp.MustCompile(`^-`)},
		{name: "**", pattern: regexp.MustCompile(`^(××|\*\*)`)}, // Python風べき乗
		{name: "*", pattern: regexp.MustCompile(`^(×|\*)`)},
		{name: "÷÷", pattern: regexp.MustCompile(`^÷÷`)}, // 整数の割り算
		{name: "÷", pattern: regexp.MustCompile(`^(÷|/)`)},
		{name: "%", pattern: regexp.MustCompile(`^%`)},
		{name: "^", pattern: regexp.MustCompile(`^\^`)},
		{name: "&", pattern: regexp.MustCompile(`^&`)},
		{name: "[", pattern: regexp.MustCompile(`^\[`)},
		{name: "]", pattern: regexp.MustCompile(`^]`), readJosi: true},
		{name: "(", pattern: regexp.MustCompile(`^\(`)},
		{name: ")", pattern: regexp.MustCompile(`^\)`), readJosi: true},
		{name: "|", pattern: regexp.MustCompile(`^\|`)},
		{name: "??", pattern: regexp.MustCompile(`^\?\?`)},                             // 「表示」のエイリアス #1745
		{name: TypeWord, pattern: regexp.MustCompile(`^\$\{.+?\}`), parser: cbExtWord}, // 特別名前トークン(#1836)(#672)
		{name: "$", pattern: regexp.MustCompile(`^(\$|\.)`)},                           // プロパティアクセス (#1793)(#1807)
		{name: TypeString, pattern: regexp.MustCompile(`^🌿`), parser: stringParser("🌿", "🌿")},
		{name: TypeStringEx, pattern: regexp.MustCompile(`^🌴`), parser: stringParser("🌴", "🌴")},
		{name: TypeStringEx, pattern: regexp.MustCompile(`^「`), parser: stringParser("「", "」")},
		{name: TypeString, pattern: regexp.MustCompile(`^『`), parser: stringParser("『", "』")},
		{name: TypeStringEx, pattern: regexp.MustCompile(`^“`), parser: stringParser("“", "”")},
		{name: TypeStringEx, pattern: regexp.MustCompile(`^"`), parser: stringParser(`"`, `"`)},
		{name: TypeString, pattern: regexp.MustCompile(`^'`), parser: stringParser("'", "'")},
		{name: "」", pattern: regexp.MustCompile(`^」`), parser: errorRead("」")},
		{name: "』", pattern: regexp.MustCompile(`^』`), parser: errorRead("』")},
		{name: TypeFunc, pattern: regexp.MustCompile(`^\{関数\},?`)},
		{name: "{", pattern: regexp.MustCompile(`^\{`)},
		{name: "}", pattern: regexp.MustCompile(`^\}`), readJosi: true},
		{name: ":", pattern: regexp.MustCompile(`^:`)},
		{name: TypeEOLUnderscore, pattern: regexp.MustCompile(`^_\s*\n`)},
		{name: TypeDecLineNo, pattern: regexp.MustCompile(`^‰`)},
		// 絵文字変数。TS版は [\uD800-\uDBFF][\uDC00-\uDFFF] というサロゲート対の
		// 並びで書いているので、rune ではBMP外の1文字にあたる。
		{name: TypeWord, pattern: regexp.MustCompile(`^[\x{10000}-\x{10FFFF}][_a-zA-Z0-9]*`), readJosi: true},
		{name: TypeWord, pattern: regexp.MustCompile(`^[\x{1F60}-\x{1F6F}][_a-zA-Z0-9]*`), readJosi: true}, // 絵文字
		{name: TypeWord, pattern: regexp.MustCompile(`^《.+?》`), readJosi: true},                            // 《特別名前トークン》(#672)
		// 単語句
		{
			name:    TypeWord,
			pattern: regexp.MustCompile(`^[_a-zA-Z\x{3005}\x{4E00}-\x{9FCF}ぁ-んァ-ヶ\x{2460}-\x{24FF}\x{2776}-\x{277F}\x{3251}-\x{32BF}]`),
			parser:  cbWordParser,
		},
	}
}

// TrimOkurigana drops okurigana so that 「置換する」 and 「置換」 name the same thing.
func TrimOkurigana(s string) string {
	// ひらがなから始まらない場合、送り仮名を削除する。(例)置換する
	if !hiraRE.MatchString(s) {
		return hiraRunRE.ReplaceAllString(s, "")
	}
	// 全てひらがな？ (例)どうぞ
	if allHiraganaRE.MatchString(s) {
		return s
	}
	// 末尾のひらがなのみ (例)お願いします → お願
	return hiraTailRE.ReplaceAllString(s, "")
}

// trimOnce removes leading or trailing whitespace, whichever the pattern finds
// first, but never both. This mirrors a missing /g flag in the TypeScript
// version, and the range comment token value depends on it.
func trimOnce(s string) string {
	if loc := trimOnceRE.FindStringIndex(s); loc != nil {
		return s[:loc[0]] + s[loc[1]:]
	}
	return s
}

func cbRangeComment(src string, _ bool) (parseResult, error) {
	src = strings.TrimPrefix(src, "/*")
	var res string
	if i := strings.Index(src, "*/"); i < 0 {
		res, src = src, ""
	} else {
		res, src = src[:i], src[i+2:]
	}
	numEOL := strings.Count(res, "\n")
	res = trimOnce(res)
	return parseResult{src: src, res: res, josi: "", numEOL: numEOL}, nil
}

// readJosi reads the particle that follows a token. The caller decides how to
// normalize it, because the TypeScript version does it in two different orders.
func readJosi(src string) (particle, rest string, ok bool) {
	m := josi.RE.FindStringIndex(src)
	if m == nil {
		return "", src, false
	}
	particle = strings.TrimLeft(src[:m[1]], " \t　")
	// 助詞の直後にある「,」を飛ばす #877
	rest = strings.TrimPrefix(src[m[1]:], ",")
	return particle, rest, true
}

// normalizeJosiWord is the order cbWordParser uses: 「もの」 first, then the
// removable-particle check.
func normalizeJosiWord(particle string) string {
	if strings.HasPrefix(particle, "もの") { // #1614
		particle = strings.TrimPrefix(particle, "もの")
	}
	if josi.RemovableMap[particle] { // #936 #939 #974
		return ""
	}
	return particle
}

// normalizeJosiAfter is the order cbString and the readJosi rules use: the
// removable-particle check first, then 「もの」. Keeping the two orders apart
// matters for a particle like 「ものこと」.
func normalizeJosiAfter(particle string) string {
	if josi.RemovableMap[particle] { // #936 #939 #974
		particle = ""
	}
	if strings.HasPrefix(particle, "もの") { // #1614
		particle = strings.TrimPrefix(particle, "もの")
	}
	return particle
}

func cbWordParser(src string, trimOkurigana bool) (parseResult, error) {
	res := ""
	particle := ""
	for src != "" {
		// 1文字以上のとき
		if res != "" {
			// 「かつ」「または」なら分割する (#1379 core#84)
			if wordSpecialRE.MatchString(src) {
				break
			}
			if j, rest, ok := readJosi(src); ok {
				particle = j
				src = rest
				break
			}
		}
		// カタカナ漢字英数字か？
		if m := kanakanjiRE.FindString(src); m != "" {
			res += m
			src = src[len(m):]
			continue
		}
		// ひらがな？
		if hiraRE.MatchString(src) {
			_, size := utf8.DecodeRuneInString(src)
			res += src[:size]
			src = src[size:]
			continue
		}
		break // other chars
	}

	// --- 単語分割における特殊ルール ---
	// 「間」の特殊ルール (#831)
	// 「等しい間」や「一致する間」なら「間」をsrcに戻す。「システム時間」はそのまま。
	if hiraKanRE.MatchString(res) {
		r, size := utf8.DecodeLastRuneInString(res)
		src = string(r) + src
		res = res[:len(res)-size]
	}
	// 「以上」「以下」「超」「未満」 #918
	if ii := wordHasIjoIkaRE.FindStringSubmatch(res); ii != nil {
		src = ii[1] + particle + src
		particle = ""
		res = res[:len(res)-len(ii[1])]
	}
	particle = normalizeJosiWord(particle)

	// 送り仮名の省略ルール。漢字カタカナ英語から始まる語句が対象。
	if trimOkurigana {
		res = TrimOkurigana(res)
	}
	// 助詞だけの語句の場合
	if res == "" && particle != "" {
		res = particle
		particle = ""
	}
	return parseResult{src: src, res: res, josi: particle, numEOL: 0}, nil
}

func cbString(beginTag, closeTag, src string) (parseResult, error) {
	src = strings.TrimPrefix(src, beginTag)
	i := strings.Index(src, closeTag)
	if i < 0 {
		return parseResult{}, fmt.Errorf("『%s』で始めた文字列の終端記号『%s』が見つかりません。", beginTag, closeTag)
	}
	res := src[:i]
	src = src[i+len(closeTag):]
	// res の中に beginTag があればエラーにする #953
	if strings.Contains(res, beginTag) {
		if beginTag == "『" {
			return parseResult{}, errors.New("「『」で始めた文字列の中に「『」を含めることはできません。")
		}
		return parseResult{}, fmt.Errorf("『%s』で始めた文字列の中に『%s』を含めることはできません。", beginTag, beginTag)
	}

	particle, src, _ := readJosi(src)
	particle = normalizeJosiAfter(particle)
	return parseResult{src: src, res: res, josi: particle, numEOL: strings.Count(res, "\n")}, nil
}

func cbExtWord(src string, _ bool) (parseResult, error) {
	src = strings.TrimPrefix(src, "${")
	i := strings.Index(src, "}")
	if i < 0 {
		return parseResult{}, errors.New("変数名の終わりが見つかりません。")
	}
	res := src[:i]
	src = src[i+1:]

	// cbExtWord は助詞の正規化を行わない。TS版と同じ。
	particle, src, _ := readJosi(src)
	if strings.Contains(res, "\n") {
		return parseResult{}, errors.New("変数名に改行を含めることはできません。")
	}
	return parseResult{src: src, res: res, josi: particle, numEOL: 0}, nil
}

func parseNumberValue(n string) any { return parseNumber(n) }

// parseNumber matches JavaScript's Number() for the literals the rules accept.
// Anything Number() would call NaN is NaN here too.
func parseNumber(s string) float64 {
	s = strings.ReplaceAll(s, "_", "")
	if len(s) > 2 && s[0] == '0' {
		base := 0
		switch s[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			// 桁数が多いとuint64に収まらないので大きい整数として読む。
			// JSのNumber()も同じく浮動小数点数に丸める。
			if v, ok := new(big.Int).SetString(s[2:], base); ok {
				f, _ := new(big.Float).SetInt(v).Float64()
				return f
			}
			return math.NaN()
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return v
}

// isIndentChars reports the indent width of ch, or 0 when ch cannot be used for
// indentation (nako_indent_chars.mts equivalent).
func isIndentChars(ch rune) int {
	switch ch {
	case '\t':
		return 4
	case ' ', '|':
		return 1
	case '・', '　':
		return 2
	case '⏋', '⏌': // 0x23CB, 0x23CC
		return 2
	}
	// 罫線
	if ch >= 0x2500 && ch <= 0x257F {
		return 2
	}
	// 記号
	if (ch >= 0x23A0 && ch <= 0x23AF) || (ch >= 0x23B8 && ch <= 0x23BF) {
		return 2
	}
	return 0
}
