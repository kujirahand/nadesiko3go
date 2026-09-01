package prepare_test

import (
	"reflect"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// The expected values below were produced by running the same input through
// nako_prepare.mts and rewriting its UTF-16 offsets as rune offsets. All 239
// compat fixture cases were checked the same way; the only divergence is the
// 🟰 case covered by TestConvert1chNonBMP.
func TestConvert(t *testing.T) {
	tests := []struct {
		name string
		code string
		want []prepare.Result
	}{
		{
			name: "全角数字を半角にする",
			code: "３を表示",
			want: []prepare.Result{{Text: "3", Pos: 0}, {Text: "を", Pos: 1}, {Text: "表", Pos: 2}, {Text: "示", Pos: 3}},
		},
		{
			name: "文字列の中は変換しない",
			code: "「Ａ」を表示",
			want: []prepare.Result{{Text: "「", Pos: 0}, {Text: "Ａ」", Pos: 1}, {Text: "を", Pos: 3}, {Text: "表", Pos: 4}, {Text: "示", Pos: 5}},
		},
		{
			name: "CRLFを改行に揃え、位置は元のまま返す",
			code: "Ａ＝１＋２\r\nＡを表示",
			want: []prepare.Result{
				{Text: "A", Pos: 0}, {Text: "=", Pos: 1}, {Text: "1", Pos: 2}, {Text: "+", Pos: 3}, {Text: "2", Pos: 4},
				{Text: "\n", Pos: 5},
				{Text: "A", Pos: 7}, {Text: "を", Pos: 8}, {Text: "表", Pos: 9}, {Text: "示", Pos: 10},
			},
		},
		{
			name: "CRだけの改行も揃える",
			code: "あ\r\nい\rう",
			want: []prepare.Result{
				{Text: "あ", Pos: 0}, {Text: "\n", Pos: 1},
				{Text: "い", Pos: 3}, {Text: "\n", Pos: 4},
				{Text: "う", Pos: 5},
			},
		},
		{
			name: "行コメントの中は変換しない",
			code: "# コメント　全角\nＡ",
			want: []prepare.Result{{Text: "#", Pos: 0}, {Text: " コメント　全角\n", Pos: 1}, {Text: "A", Pos: 10}},
		},
		{
			name: "全角スラッシュの行コメントは半角に強制変換する",
			code: "Ａ／／ｃ\nＢ",
			want: []prepare.Result{{Text: "A", Pos: 0}, {Text: "//", Pos: 1}, {Text: "ｃ\n", Pos: 3}, {Text: "B", Pos: 5}},
		},
		{
			name: "範囲コメントの中は変換しない",
			code: "/*ａ*/Ｂ",
			want: []prepare.Result{{Text: "/*", Pos: 0}, {Text: "ａ*/", Pos: 2}, {Text: "B", Pos: 5}},
		},
		{
			name: "全角の範囲コメントは半角に強制変換する",
			code: "／＊ａ＊／Ｂ",
			want: []prepare.Result{{Text: "/*", Pos: 0}, {Text: "ａ*/", Pos: 2}, {Text: "B", Pos: 5}},
		},
		{
			name: "絵文字の範囲コメント",
			code: "🌴ａ🌴Ｂ",
			want: []prepare.Result{{Text: "🌴", Pos: 0}, {Text: "ａ🌴", Pos: 1}, {Text: "B", Pos: 3}},
		},
		{
			name: "文字列の入れ子は開き記号を文字として扱う",
			code: "「あ「い」を表示",
			want: []prepare.Result{{Text: "「", Pos: 0}, {Text: "あ「い」", Pos: 1}, {Text: "を", Pos: 5}, {Text: "表", Pos: 6}, {Text: "示", Pos: 7}},
		},
		{
			name: "閉じられていない文字列は捨てる。字句解析側がエラーにする",
			code: "「未閉じ",
			want: []prepare.Result{{Text: "「", Pos: 0}},
		},
		{
			name: "閉じられていない範囲コメントは自動で閉じる",
			code: "/*未閉じ",
			want: []prepare.Result{{Text: "/*", Pos: 0}, {Text: "未閉じ*/", Pos: 2}},
		},
		{
			name: "読点はカンマにする",
			code: "１，２、３",
			want: []prepare.Result{{Text: "1", Pos: 0}, {Text: ",", Pos: 1}, {Text: "2", Pos: 2}, {Text: ",", Pos: 3}, {Text: "3", Pos: 4}},
		},
		{
			name: "句点はセミコロンにする",
			code: "Ａ；Ｂ。Ｃ",
			want: []prepare.Result{{Text: "A", Pos: 0}, {Text: ";", Pos: 1}, {Text: "B", Pos: 2}, {Text: ";", Pos: 3}, {Text: "C", Pos: 4}},
		},
		{
			name: "隅付き括弧は角括弧にする",
			code: "Ａ＝【１】",
			want: []prepare.Result{{Text: "A", Pos: 0}, {Text: "=", Pos: 1}, {Text: "[", Pos: 2}, {Text: "1", Pos: 3}, {Text: "]", Pos: 4}},
		},
		{
			name: "全角ダブルクォートは半角にし、閉じ記号は全角のまま",
			code: "＂ａ＂",
			want: []prepare.Result{{Text: "\"", Pos: 0}, {Text: "ａ＂", Pos: 1}},
		},
		{
			name: "波括弧の引用符",
			code: "“ａｂ”",
			want: []prepare.Result{{Text: "“", Pos: 0}, {Text: "ａｂ”", Pos: 1}},
		},
		{
			name: "サロゲートペアを含む文字列の位置はrune単位",
			code: "「𩸽あ」",
			want: []prepare.Result{{Text: "「", Pos: 0}, {Text: "𩸽あ」", Pos: 1}},
		},
		{
			name: "空文字列",
			code: "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prepare.Convert(tt.code)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Convert(%q)\n got %#v\nwant %#v", tt.code, got, tt.want)
			}
		})
	}
}

func TestConvert1ch(t *testing.T) {
	tests := []struct {
		in   rune
		want rune
	}{
		{'Ａ', 'A'},
		{'３', '3'},
		{'＝', '='},
		{'A', 'A'},
		{'あ', 'あ'},
		{'　', '　'}, // 全角スペースはインデント量2として扱うので変換しない
		{'。', ';'},
		{'、', ','},
		{'※', '#'},
		{'～', '~'},
		{'―', '-'},
		{'❌', '*'},
		{'𩸽', '𩸽'},
	}
	for _, tt := range tests {
		if got := prepare.Convert1ch(tt.in); got != tt.want {
			t.Errorf("Convert1ch(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestConvert1chNonBMP pins the one place where rune-based conversion differs
// from the TypeScript version. TS walks UTF-16 code units, so convert1ch only
// ever sees a lone surrogate for 🟰 and its table entry never fires; the
// character survives unchanged. Go folds it to '=' as #1781 intended.
func TestConvert1chNonBMP(t *testing.T) {
	if got := prepare.Convert1ch('🟰'); got != '=' {
		t.Errorf("Convert1ch('🟰') = %q, want '='", got)
	}
}

func TestText(t *testing.T) {
	code := "Ａ＝「ｂ」\r\nＡを表示"
	want := "A=「ｂ」\nAを表示"
	if got := prepare.Text(prepare.Convert(code)); got != want {
		t.Errorf("Text(Convert(%q)) = %q, want %q", code, got, want)
	}
}

func TestCheckNakoMode(t *testing.T) {
	modes := []string{"!インデント構文", "!ダイレクトモード"}
	tests := []struct {
		name string
		code string
		want bool
	}{
		{"全角の感嘆符", "！インデント構文\nAを表示", true},
		{"電球の絵文字", "💡インデント構文\nAを表示", true},
		{"半角", "!インデント構文\nAを表示", true},
		{"前後の空白は無視する", "  !インデント構文  \nAを表示", true},
		{"句点区切り", "!インデント構文。Aを表示", true},
		{"範囲コメントは取り除く", "/*説明*/!インデント構文\n", true},
		{"宣言なし", "Aを表示", false},
		{"別のモード名", "!なにか\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := prepare.CheckNakoMode(tt.code, modes); got != tt.want {
				t.Errorf("CheckNakoMode(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}
