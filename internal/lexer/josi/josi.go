// Package josi defines the particles (助詞) recognized by the lexer.
//
// Ported from nako_josi_list.mts. The order of the lists is significant: the
// generated pattern is an alternation, and Go's regexp picks the leftmost
// alternative that matches, exactly as JavaScript does.
package josi

import (
	"regexp"
	"sort"
	"strings"
)

// Base is the basic particle list.
var Base = []string{
	"について", "くらい", "なのか", "までを", "までの", "による", "として",
	"とは", "から", "まで", "だけ", "より", "ほど", "など",
	"いて", "えて", "きて", "けて", "して", "って", "にて", "みて",
	"めて", "ねて", "では", "には", "んで", "ずつ",
	"は", "を", "に", "へ", "で", "と", "が", "の",
}

// Tarareba lists the particles used by 「もし」 conditionals.
var Tarareba = []string{
	"でなければ", "なければ", "ならば", "なら", "たら", "れば",
}

// Removable lists particles that carry no meaning and are dropped (#936 #939 #974).
var Removable = []string{
	"こと", "である", "です", "します", "でした", "にゃん",
}

var (
	// List is every particle the lexer knows, in the order the TypeScript
	// version builds it: Base, then Tarareba, then Removable.
	List []string

	// TararebaMap and RemovableMap answer membership questions without a scan.
	TararebaMap  = map[string]bool{}
	RemovableMap = map[string]bool{}

	// RE matches optional leading whitespace followed by one particle, at the
	// start of the string.
	RE *regexp.Regexp
)

func init() {
	List = append(List, Base...)
	List = append(List, Tarareba...)
	List = append(List, Removable...)
	for _, j := range Tarareba {
		TararebaMap[j] = true
	}
	for _, j := range Removable {
		RemovableMap[j] = true
	}

	// 「もの」構文 (#1614)。「もの」付きと素の助詞を交互に並べる。
	mono := make([]string, 0, len(List)*2)
	for _, j := range List {
		mono = append(mono, "もの"+j, j)
	}
	// 文字数の長い順に並び替える。同じ長さの並びは崩さない。
	sort.SliceStable(mono, func(i, j int) bool {
		return len([]rune(mono[i])) > len([]rune(mono[j]))
	})

	RE = regexp.MustCompile(`^[\t 　]*(` + strings.Join(mono, "|") + `)`)
}

// Match reports the particle at the start of s, skipping leading whitespace.
// consumed is the number of runes matched including that whitespace, so the
// caller can advance past it.
func Match(s string) (particle string, consumed int, ok bool) {
	loc := RE.FindStringSubmatchIndex(s)
	if loc == nil {
		return "", 0, false
	}
	return s[loc[2]:loc[3]], len([]rune(s[:loc[1]])), true
}
