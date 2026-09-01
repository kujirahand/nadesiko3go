package josi_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/lexer/josi"
)

// Expected values come from running the same inputs through josiRE in
// nako_josi_list.mts. The generated pattern string is byte-identical to the
// TypeScript one.
func TestMatch(t *testing.T) {
	tests := []struct {
		code     string
		particle string
		consumed int
		ok       bool
	}{
		{"を表示", "を", 1, true},
		{"は1", "は", 1, true},
		{"について調べる", "について", 4, true},
		{"がある", "が", 1, true},
		{"ずつ増やす", "ずつ", 2, true},
		// 「もの」構文 (#1614)
		{"ものを表示", "ものを", 3, true},
		{"ものについて", "ものについて", 6, true},
		// 長い助詞を優先する
		{"までを", "までを", 3, true},
		{"まで", "まで", 2, true},
		// 「もし」文の助詞
		{"ならば", "ならば", 3, true},
		{"なければ", "なければ", 4, true},
		{"でなければ", "でなければ", 5, true},
		// 意味のない助詞
		{"こと", "こと", 2, true},
		{"にゃん", "にゃん", 3, true},
		// 前置きの空白は読み飛ばして consumed に含める
		{"  を表示", "を", 3, true},
		{"\tを表示", "を", 2, true},
		{"　を表示", "を", 2, true},
		// 助詞でないもの
		{"表示", "", 0, false},
		{"", "", 0, false},
		{"もの", "", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			particle, consumed, ok := josi.Match(tt.code)
			if ok != tt.ok || particle != tt.particle || consumed != tt.consumed {
				t.Errorf("Match(%q) = (%q, %d, %v), want (%q, %d, %v)",
					tt.code, particle, consumed, ok, tt.particle, tt.consumed, tt.ok)
			}
		})
	}
}

func TestListOrder(t *testing.T) {
	want := len(josi.Base) + len(josi.Tarareba) + len(josi.Removable)
	if len(josi.List) != want {
		t.Fatalf("len(List) = %d, want %d", len(josi.List), want)
	}
	if josi.List[0] != "について" {
		t.Errorf("List[0] = %q, want 助詞リストの先頭が基本助詞であること", josi.List[0])
	}
	if josi.List[len(josi.List)-1] != "にゃん" {
		t.Errorf("List末尾 = %q, want 意味のない助詞が最後に並ぶこと", josi.List[len(josi.List)-1])
	}
}

func TestMaps(t *testing.T) {
	if !josi.TararebaMap["ならば"] || josi.TararebaMap["を"] {
		t.Error("TararebaMapが「もし」文の助詞だけを持っていない")
	}
	if !josi.RemovableMap["こと"] || josi.RemovableMap["を"] {
		t.Error("RemovableMapが意味のない助詞だけを持っていない")
	}
}
