package main

import (
	"slices"
	"strings"
	"testing"
)

// ひな形の正規タグ集合（#445）。ui/app.js の templateGenres と合わせる。
var templateCanonicalGenres = []string{
	"文法", "プログラミング学習", "GUI", "ダイアログ",
	"仕事自動化", "ファイル", "CUI", "WEB",
}

// 実行モードID（#445）。ui/index.html の select-app-type の値と合わせる。
var templateCanonicalModes = []string{"window", "newwindow", "cli", "wnako3"}

func TestParseTemplateMeta(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		wantDesc  string
		wantTags  []string
		wantModes []string
	}{
		{
			name:      "ヘッダ一式",
			code:      "### もし文のサンプル\n### tags = 文法,GUI,CUI\n### mode = window,cli\nA=30\n",
			wantDesc:  "もし文のサンプル",
			wantTags:  []string{"文法", "GUI", "CUI"},
			wantModes: []string{"window", "cli"},
		},
		{
			name:      "説明文にイコールが含まれても説明として扱う",
			code:      "### A >= B = {A} のサンプル\n### tags = 文法\n### mode = cli\nA=30\n",
			wantDesc:  "A >= B = {A} のサンプル",
			wantTags:  []string{"文法"},
			wantModes: []string{"cli"},
		},
		{
			name: "ヘッダなし（従来形式）",
			code: "// 従来の説明\n「こんにちは」と表示\n",
		},
		{
			name:      "コード行より後の ### は読まない",
			code:      "### 先頭の説明\nA=30\n### tags = 文法\n",
			wantDesc:  "先頭の説明",
			wantTags:  nil,
			wantModes: nil,
		},
		{
			name:      "空行と // 行を挟んでもヘッダは続く",
			code:      "### 説明\n\n// 補足コメント\n### tags = GUI\n### mode = wnako3\nカメ作成\n",
			wantDesc:  "説明",
			wantTags:  []string{"GUI"},
			wantModes: []string{"wnako3"},
		},
		{
			name:      "mode の未知の値は捨てる",
			code:      "### 説明\n### tags = 文法\n### mode = foo,cli\nA=1\n",
			wantDesc:  "説明",
			wantTags:  []string{"文法"},
			wantModes: []string{"cli"},
		},
		{
			name:      "mode が未知の値だけなら空になる（全モード表示）",
			code:      "### 説明\n### mode = foo\nA=1\n",
			wantDesc:  "説明",
			wantTags:  nil,
			wantModes: []string{},
		},
		{
			name:      "tags と mode の重複行は後勝ち",
			code:      "### 説明\n### tags = 文法\n### tags = GUI,CUI\n### mode = cli\n### mode = window\nA=1\n",
			wantDesc:  "説明",
			wantTags:  []string{"GUI", "CUI"},
			wantModes: []string{"window"},
		},
		{
			name:      "シェバン行の後ろのヘッダも読む",
			code:      "#!/usr/bin/env gonako\n### シェバン付きサンプル\n### tags = 文法,CUI\n### mode = cli\n「表示」と表示\n",
			wantDesc:  "シェバン付きサンプル",
			wantTags:  []string{"文法", "CUI"},
			wantModes: []string{"cli"},
		},
		{
			name:      "CRLFでも読める",
			code:      "### 説明です\r\n### tags = 文法, CUI\r\n### mode = cli\r\nA=1\r\n",
			wantDesc:  "説明です",
			wantTags:  []string{"文法", "CUI"},
			wantModes: []string{"cli"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, tags, modes := parseTemplateMeta(tt.code)
			if desc != tt.wantDesc {
				t.Errorf("desc = %q, want %q", desc, tt.wantDesc)
			}
			if !slices.Equal(tags, tt.wantTags) {
				t.Errorf("tags = %v, want %v", tags, tt.wantTags)
			}
			if !slices.Equal(modes, tt.wantModes) {
				t.Errorf("modes = %v, want %v", modes, tt.wantModes)
			}
		})
	}
}

// 同梱サンプル全件にタグと対応モードが付き、値が正規集合に収まること（#445）。
func TestAllSamplesHaveTemplateMeta(t *testing.T) {
	list := getTemplateList()
	if len(list) == 0 {
		t.Fatal("ひな形が1件も取得できません")
	}
	for _, item := range list {
		if item.Desc == "" || item.Desc == item.Title {
			t.Errorf("%s: 説明がヘッダから読めていません (desc=%q)", item.ID, item.Desc)
		}
		if len(item.Tags) == 0 {
			t.Errorf("%s: tags がありません", item.ID)
		}
		for _, tag := range item.Tags {
			if !slices.Contains(templateCanonicalGenres, tag) {
				t.Errorf("%s: 正規タグ集合にないタグ %q", item.ID, tag)
			}
		}
		if len(item.Modes) == 0 {
			t.Errorf("%s: mode がありません", item.ID)
		}
		for _, mode := range item.Modes {
			if !slices.Contains(templateCanonicalModes, mode) {
				t.Errorf("%s: 未知の実行モード %q", item.ID, mode)
			}
		}
		// カテゴリは互換のため先頭タグと一致する。
		if len(item.Tags) > 0 && item.Category != item.Tags[0] {
			t.Errorf("%s: category = %q, want 先頭タグ %q", item.ID, item.Category, item.Tags[0])
		}
		// コード本体にヘッダの tags 行がそのまま残っていること。
		if !strings.Contains(item.Code, "### tags =") {
			t.Errorf("%s: コードに ### tags 行がありません", item.ID)
		}
	}
}

func TestGetTemplateListCarriesTagsAndModes(t *testing.T) {
	byID := map[string]TemplateItem{}
	for _, item := range getTemplateList() {
		byID[item.ID] = item
	}

	dom, ok := byID["12_DOM操作"]
	if !ok {
		t.Fatal("12_DOM操作 がありません")
	}
	if !slices.Contains(dom.Tags, "GUI") || !slices.Contains(dom.Modes, "window") {
		t.Errorf("12_DOM操作 = tags %v / modes %v", dom.Tags, dom.Modes)
	}

	excel, ok := byID["06_Excelブック作成"]
	if !ok {
		t.Fatal("06_Excelブック作成 がありません")
	}
	if !slices.Contains(excel.Tags, "仕事自動化") || !slices.Equal(excel.Modes, []string{"cli"}) {
		t.Errorf("06_Excelブック作成 = tags %v / modes %v", excel.Tags, excel.Modes)
	}

	turtle, ok := byID["15_タートル(wnako3)"]
	if !ok {
		t.Fatal("15_タートル(wnako3) がありません")
	}
	if !slices.Equal(turtle.Modes, []string{"wnako3"}) {
		t.Errorf("15_タートル(wnako3) modes = %v, want [wnako3]", turtle.Modes)
	}
}
