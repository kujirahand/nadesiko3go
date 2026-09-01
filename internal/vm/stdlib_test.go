package vm_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// Expected values come from the compat fixtures, which the TypeScript version
// generated. These keep a failure readable when one command breaks.

func TestStringCommands(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{"文字数", `("あいう"の文字数)を表示`, "3"},
		// サロゲートペアは1文字。TS版もArray.fromで数えるので一致する。
		{"文字数-サロゲートペア", `("𩸽"の文字数)を表示`, "1"},
		{"文字数-ZWJ絵文字", `("👨‍👩‍👦"の文字数)を表示`, "5"},
		{"何文字目", `("abcabc"で"c"が何文字目)を表示`, "3"},
		{"文字検索", `("abcabc"の2から"a"を文字検索)を表示`, "4"},
		{"文字検索-見つからない", `("abc"の1から"z"を文字検索)を表示`, "0"},
		{"文字抜出", `("あいうえお"の2から3を文字抜出)を表示`, "いうえ"},
		{"文字左部分", `("あいうえお"の2だけ文字左部分)を表示`, "あい"},
		{"文字右部分", `("あいうえお"の2だけ文字右部分)を表示`, "えお"},
		{"文字挿入", `("あえお"の2に"いう"を文字挿入)を表示`, "あいうえお"},
		{"文字削除", `("あいうえお"の2から2を文字削除)を表示`, "あえお"},
		{"文字始", `("abc"が"ab"で文字始)を表示`, "true"},
		{"文字終", `("abc"が"bc"で文字終)を表示`, "true"},
		{"出現回数", `("aXbXc"で"X"の出現回数)を表示`, "2"},
		{"置換", `("aXbXc"の"X"を"-"に置換)を表示`, "a-b-c"},
		{"単置換", `("aXbXc"の"X"を"-"に単置換)を表示`, "a-bXc"},
		{"区切", `("a,b,c"を","で区切)を表示`, "a,b,c"},
		{"文字列分解", `("あいう"を文字列分解)を表示`, "あ,い,う"},
		{"リフレイン", `("ab"を3でリフレイン)を表示`, "ababab"},
		{"トリム", `("  a  "をトリム)を表示`, "a"},
		{"右トリム", `("  a  "を右トリム&"|")を表示`, "  a|"},
		{"大文字変換", `("abc"を大文字変換)を表示`, "ABC"},
		{"小文字変換", `("ABC"を小文字変換)を表示`, "abc"},
		{"平仮名変換", `("アイウ"を平仮名変換)を表示`, "あいう"},
		{"カタカナ変換", `("あいう"をカタカナ変換)を表示`, "アイウ"},
		{"英数全角変換", `("a1"を英数全角変換)を表示`, "ａ１"},
		{"英数半角変換", `("ａ１"を英数半角変換)を表示`, "a1"},
		{"ゼロ埋", `(5を3でゼロ埋)を表示`, "005"},
		// 桁数はrune単位。本家も修正済みなので一致する。
		{"ゼロ埋-サロゲートペア", `("𩸽"を4でゼロ埋)を表示`, "000𩸽"},
		{"空白埋", `("a"を3で空白埋)を表示`, "  a"},
		{"ASC", `("A"のASC)を表示`, "65"},
		{"ASC-配列", `(["a","b","c"]のASCをJSONエンコード)を表示`, "[97,98,99]"},
		{"ASC-サロゲートペア", `("𩸽"のASC)を表示`, "171581"},
		{"CHR", `(65のCHR)を表示`, "A"},
		{"CHR-配列", `([97,98,99]のCHRを""で配列結合)を表示`, "abc"},
		{"文字列連結", `("a"と"b"を文字列連結)を表示`, "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestArrayCommands(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{"要素数", `([1,2,3]の要素数)を表示`, "3"},
		{"添字代入-範囲外", "A=[1]\nA[3]=9\nAを表示", "1,,,9"},
		{"配列追加", "A=[1,2]\nAに3を配列追加\nAを表示", "1,2,3"},
		{"配列ポップ", "A=[1,2,3]\nB=Aから配列ポップ\nAを表示\nBを表示", "1,2\n3"},
		{"配列取出", "A=[1,2,3,4]\nB=Aの1から2を配列取出\nAを表示\nBを表示", "1,4\n2,3"},
		{"配列削除", "A=[1,2,3]\nB=Aの1を配列削除\nAを表示\nBを表示", "1,3\n2"},
		{"配列切取-範囲", "A=[0,1,2,3,4,5]\nB=Aの1…3を配列切取\nBをJSONエンコードして表示\nAをJSONエンコードして表示", "[1,2,3]\n[0,4,5]"},
		{"配列挿入", "A=[1,3]\nAの1に2を配列挿入\nAを表示", "1,2,3"},
		{"配列結合", `A=[1,2,3]を"-"で配列結合` + "\nAを表示", "1-2-3"},
		{"配列検索", `([1,2,3]から2を配列検索)を表示`, "1"},
		{"配列検索-見つからない", `([1,2,3]から9を配列検索)を表示`, "-1"},
		// 既定のソートは文字列として比べる
		{"配列ソート", "A=[3,1,2]を配列ソート\nAを表示", "1,2,3"},
		{"配列数値ソート", "A=[10,9,100]を配列数値ソート\nAを表示", "9,10,100"},
		{"配列逆順", "A=[1,2,3]を配列逆順\nAを表示", "3,2,1"},
		{"配列合計", `([1,2,3]の配列合計)を表示`, "6"},
		{"配列最大値", `([1,5,3]の配列最大値)を表示`, "5"},
		{"配列最小値", `([1,5,3]の配列最小値)を表示`, "1"},
		// 複製は深いコピー。元の配列は変わらない。
		{"配列複製", "A=[1,2]\nB=Aを配列複製\nB[0]=9\nAを表示\nBを表示", "1,2\n9,2"},
		// 代入は参照渡し
		{"配列参照", "A=[1,2]\nB=A\nB[0]=9\nAを表示", "9,2"},
		{"配列連番作成", "A=1から5まで配列連番作成\nAを表示", "1,2,3,4,5"},
		{"配列要素作成", `A="x"を3だけ配列要素作成` + "\nAを表示", "x,x,x"},
		{"配列要素作成-多次元", `A=0を[2,2]だけ配列要素作成` + "\nAをJSONエンコードして表示", "[[0,0],[0,0]]"},
		{"配列マップ", "F=関数(V)\n(V*2)で戻る\nここまで\nB=[1,2,3]\nA=FをBに配列マップ\nAを表示", "2,4,6"},
		{"配列フィルタ", "F=関数(V)\n(V%2=0)で戻る\nここまで\nB=[1,2,3,4]\nA=FでBを配列フィルタ\nAを表示", "2,4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestDictCommands(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{"辞書参照", `A={"x":1,"y":2}` + "\nA[\"x\"]を表示", "1"},
		{"辞書代入", `A={"x":1}` + "\nA[\"y\"]=2\nA[\"y\"]を表示", "2"},
		{"存在しないキー", `A={"x":1}` + "\nA[\"none\"]を表示", "undefined"},
		// キーは書いた順を保つ
		{"辞書キー列挙", `B={"z":1,"a":2,"m":3}` + "\nA=Bの辞書キー列挙\nAを表示", "z,a,m"},
		{"辞書キー存在", `A={"x":1}に"x"が辞書キー存在` + "\nAを表示", "true"},
		{"辞書キー削除", `A={"x":1,"y":2}` + "\nAの\"x\"を辞書キー削除\nAの辞書キー列挙を表示", "y"},
		{"入れ子", `A={"p":{"q":7}}` + "\nA[\"p\"][\"q\"]を表示", "7"},
		{"JSONエンコード", `(({"x":1,"y":[1,2]})をJSONエンコード)を表示`, `{"x":1,"y":[1,2]}`},
		// 日本語はエスケープしない
		{"JSONエンコード-日本語", `(({"名":"太郎"})をJSONエンコード)を表示`, `{"名":"太郎"}`},
		{"JSONデコード", "A=『{\"x\":1}』をJSONデコード\nA[\"x\"]を表示", "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestTypeURLAndDateCommands(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{"整数変換-16進", `("0xFF"を整数変換)を表示`, "255"},
		{"RGB", `RGB(255,255,0)を表示`, "#ffff00"},
		{"ファイル名抽出-空", `(""からファイル名抽出)を表示`, ""},
		{"パス抽出-ファイルのみ", `("abc.txt"からパス抽出)を表示`, ""},
		{"日時加算-ISO", `("2024-05-10T10:50"に"10分"を日時加算)を表示`, "2024/05/10 11:00:00"},
		{"日時書式変換", `("2021/12/25"を"YYYY-MM-DD(WWW)"で日時書式変換)を表示`, "2021-12-25(Sat)"},
		{"JSオブジェクト取得-変数", `A=5;("A"のJSオブジェクト取得)を表示`, "5"},
		{"ハテナ関数-命令列", `["JSONエンコード","表示"]をハテナ関数設定;?? [1,2]`, "[1,2]"},
		{"グローバル関数一覧", "●ホゲとは\nそれは1\nここまで\nA=グローバル関数一覧取得\nI=Aから\"main__ホゲ\"を配列検索\nもし、(I>=0)ならば\"OK\"を表示", "OK"},
		{"プラグイン名設定", `"テスト"にプラグイン名設定;プラグイン名を表示`, "テスト"},
		{"名前空間復元", `元=名前空間;"サブ"に名前空間設定;名前空間ポップ;もし、(名前空間=元)ならば"OK"を表示`, "OK"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// TestClosure pins that a nested function shares the variable it closed over,
// rather than getting a copy of it.
func TestClosure(t *testing.T) {
	code := "●(Nで)カウンタ作成とは\nM=N\n(関数()\nM=M+1\nMで戻る\nここまで)で戻る\nここまで\n" +
		"F=10でカウンタ作成\n(F())を表示\n(F())を表示"
	if got := run(t, code); got != "11\n12" {
		t.Errorf("closure = %q, want \"11\\n12\"", got)
	}
}

// TestCallFunctionInVariable pins that a function held in a variable can be
// called with C-style syntax.
func TestCallFunctionInVariable(t *testing.T) {
	code := "F=関数(A)\nA*2で戻る\nここまで\n(F(3))を表示"
	if got := run(t, code); got != "6" {
		t.Errorf("F(3) = %q, want \"6\"", got)
	}
}

func TestRegexpCommands(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{"正規表現置換", `("a1b2c3"の"/[0-9]/g"を"-"に正規表現置換)を表示`, "a-b-c-"},
		{"正規表現置換-日本語", `("あいうえお"の"/[いう]/g"を"*"に正規表現置換)を表示`, "あ**えお"},
		{"正規表現置換-グループ参照", `("2024-01-02"の"/(\d+)-(\d+)-(\d+)/"を"$1年$2月$3日"に正規表現置換)を表示`, "2024年01月02日"},
		{"正規表現区切", `A="a1b22c"を"/[0-9]+/"で正規表現区切` + "\nAを表示", "a,b,c"},
		{"正規表現マッチ", `A=("abc123"を"/[0-9]+/"で正規表現マッチ)` + "\nAを表示", "123"},
		// gが付くと一致した全体の配列を返す
		{"正規表現マッチ-全件", `A=("a1b2"を"/[0-9]/g"で正規表現マッチ)` + "\nAを表示", "1,2"},
		{"正規表現マッチ-なし", `A=("abc"を"/[0-9]+/"で正規表現マッチ)` + "\nAを表示", "null"},
		// 部分マッチは『抽出文字列』に入る
		{"抽出文字列", `A=("2024-01"を"/(\d+)-(\d+)/"で正規表現マッチ)` + "\n抽出文字列を表示", "2024,01"},
		{"文字クラスと日本語", `("あa1"の"/[^0-9]/g"を"*"に正規表現置換)を表示`, "**1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// TestRegexpUnsupported pins the message a pattern RE2 cannot handle produces,
// rather than a Go regexp error leaking out.
func TestRegexpUnsupported(t *testing.T) {
	_, err := vm.RunSource(`("abab"を"/(ab)\1/"で正規表現マッチ)を表示`, "main.nako3", nil)
	if err == nil {
		t.Fatal("エラーになるはずが成功した")
	}
	want := "[実行時エラー]main.nako3(1行目): 正規表現『/(ab)\\1/』の後方参照や先読みには対応していません。"
	if got := err.Error(); got != want {
		t.Errorf("Error()\n got %q\nwant %q", got, want)
	}
}

func TestFunctionParameterParticleAlternativesShareOneSlot(t *testing.T) {
	code := "●英字判定(Sが|Sの|Sを)\n" +
		"Sを「^[A-Za-z]」で正規表現マッチ。\n" +
		"もし、それがNULLならば、いいえで戻る。\n" +
		"違えば、はいで戻る。\nここまで。\n" +
		"「abc」が英字判定して表示。\n「いろは」が英字判定して表示。"
	if got := run(t, code); got != "true\nfalse" {
		t.Fatalf("particle alternatives = %q, want true\\nfalse", got)
	}
}

// TestSoreStartsEmpty pins that 『それ』 starts as an empty string, not as
// undefined. An omitted argument reads it, so the difference is visible.
func TestSoreStartsEmpty(t *testing.T) {
	if got := run(t, "それを表示"); got != "" {
		t.Errorf("それ = %q, want \"\"", got)
	}
}

// TestNestedLoopSystemVars pins that a nested loop puts back the system
// variables it took over. 『回』 saves 『回数』 and 『反復』 saves 『対象』
// 『対象キー』『それ』; 『繰返』 saves nothing, matching nako_gen.mts.
func TestNestedLoopSystemVars(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{"回数", "2回\n2回\nここまで\n回数を表示\nここまで", "1\n2"},
		{"対象", "[1,2]を反復\n[9]を反復\nここまで\n対象を表示\nここまで", "1\n2"},
		{"条件の中の入れ子", "3回\nもし回数=2ならば\n1回\nここまで\nここまで\n回数を表示\nここまで", "1\n2\n3"},
		{"対象キー", `[1,2]を反復` + "\n" + `{"x":9}を反復` + "\nここまで\n対象キーを表示\nここまで", "0\n1"},
		// 『抜ける』で出ても元に戻る
		{"抜けた後", "2回\n3回\n抜ける\nここまで\n回数を表示\nここまで", "1\n2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// TestAsyncCommands pins the observable order of the timer commands. The clock
// is virtual, so these run instantly however long the waits say (AGENTS.md §8).
func TestAsyncCommands(t *testing.T) {
	tests := []struct{ name, code, want string }{
		{
			// 秒後のコールバックは、続きの文より後に動く
			name: "秒後-コールバックは後で動く",
			code: "「A」と表示\n0.01秒後には\n「C」と表示\nここまで\n「B」と表示\n0.1秒待つ",
			want: "A\nB\nC",
		},
		{
			// 待ち時間の短い順。積んだ順ではない。
			name: "秒後-待ち時間の短い順",
			code: "0.05秒後には\n「後」と表示\nここまで\n0.01秒後には\n「先」と表示\nここまで\n0.2秒待つ",
			want: "先\n後",
		},
		{
			name: "秒待つ-同期的に待つ",
			code: "「A」と表示\n0.01秒待つ\n「B」と表示",
			want: "A\nB",
		},
		{
			// コールバックの中から『対象』で自分のタイマーを止める
			name: "秒毎-タイマー停止",
			code: "N=0\nF=関数()\nN=N+1\n「{N}」と表示\nもしN>=3ならば\n対象のタイマー停止\nここまで\nここまで\nFを0.01秒毎\n0.2秒待つ",
			want: "1\n2\n3",
		},
		{
			name: "並列的な逐次実行",
			code: "「1」と表示\n0.01秒待つ\n「2」と表示\n0.01秒待つ\n「3」と表示",
			want: "1\n2\n3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.code); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// TestPendingCallbacksRunAfterMain pins that a one-shot callback still due when
// main ends is run rather than dropped.
func TestPendingCallbacksRunAfterMain(t *testing.T) {
	if got := run(t, "0.01秒後には\n「後」と表示\nここまで\n「先」と表示"); got != "先\n後" {
		t.Errorf("log = %q, want \"先\\n後\"", got)
	}
}

// TestVirtualClockDoesNotSleep pins that a long wait costs no real time.
func TestVirtualClockDoesNotSleep(t *testing.T) {
	started := time.Now()
	if got := run(t, "10秒待つ\n「終」と表示"); got != "終" {
		t.Errorf("log = %q, want \"終\"", got)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("実時間 = %v。仮想時計なら待たないはず。", elapsed)
	}
}

// TestRunawayTimerStops pins that a repeating timer nobody stops ends the run
// with an error instead of hanging.
func TestRunawayTimerStops(t *testing.T) {
	_, err := vm.RunSource("F=関数()\nここまで\nFを0.001秒毎\n1000000秒待つ", "main.nako3", nil)
	if err == nil {
		t.Fatal("止まらないタイマーがエラーにならなかった")
	}
}

// TestRealSleepWaits verifies that when RealSleep option is enabled, Wait pauses in real time.
func TestRealSleepWaits(t *testing.T) {
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	started := time.Now()
	if err := vm.RunProgram("0.05秒待つ\n「完了」と表示", "main.nako3", host); err != nil {
		t.Fatalf("RunProgram failed: %v", err)
	}
	elapsed := time.Since(started)
	if elapsed < 40*time.Millisecond {
		t.Errorf("RealSleep elapsed = %v, expected at least 40ms", elapsed)
	}
	if strings.TrimSpace(out.String()) != "完了" {
		t.Errorf("output = %q, want '完了'", out.String())
	}
}
