package stdlib_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// 上流Issue #213 の回帰テスト。本家の __str2date は空白・コロン・
// ハイフン・T をすべて '/' に置き換えて解釈するので、TなしISO日付
// （YYYY-MM-DD）や空白区切りの日時も受理する。Go版の parseDate にも
// 同じレイアウトを加える。
// さらに『曜日』『曜日番号取得』は解析失敗時に ctx.Now() で隠蔽して
// いたが、日数差・UNIXTIME変換・和暦変換・日時書式変換と同じく
// エラーで返す挙動に揃える。
//
// fakeContext の Now は 2026-09-01（火曜）なので、ctx.Now() への
// フォールバックを見抜けるよう火曜以外の日付も使う。
func TestISODateLayoutAndWeekdayError(t *testing.T) {
	r := stdlib.NewRegistry()
	ctx := newContext()

	call := func(cmd string, args ...value.Value) (value.Value, error) {
		e, ok := r.Lookup(cmd)
		if !ok || e.Fn == nil {
			t.Fatalf("%s が登録されていない", cmd)
		}
		return e.Fn(ctx, args)
	}
	str := func(s string) value.Value { return value.String(s) }

	tests := []struct {
		name string
		cmd  string
		args []value.Value
		want string
	}{
		// --- TなしISO日付を受理する ---
		{"曜日/ISO日付(火)", "曜日", []value.Value{str("2024-01-02")}, "火"},
		{"曜日/ISO日付(木)", "曜日", []value.Value{str("2024-01-04")}, "木"},
		{"曜日番号取得/ISO日付", "曜日番号取得", []value.Value{str("2024-01-04")}, "4"},
		{"曜日/ゼロ埋めなしISO日付", "曜日", []value.Value{str("2024-1-2")}, "火"},
		{"日数差/ISO日付", "日数差", []value.Value{str("2024-01-01"), str("2024-01-10")}, "9"},
		{"時間差/空白区切りISO日時", "時間差", []value.Value{str("2024-01-01 00:00:00"), str("2024-01-01 01:00:00")}, "1"},
		// --- 従来の書式は変わらない ---
		{"曜日/スラッシュ日付", "曜日", []value.Value{str("2024/01/02")}, "火"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := call(tt.cmd, tt.args...)
			if err != nil {
				t.Fatalf("%s%v: %v", tt.cmd, tt.args, err)
			}
			if s := value.ToString(got); s != tt.want {
				t.Errorf("%s%v = %q, want %q", tt.cmd, tt.args, s, tt.want)
			}
		})
	}

	// --- 解析不能な入力は ctx.Now() で隠蔽せずエラーにする ---
	errTests := []struct {
		name string
		cmd  string
		args []value.Value
	}{
		{"曜日/不正入力", "曜日", []value.Value{str("xyz")}},
		{"曜日番号取得/不正入力", "曜日番号取得", []value.Value{str("xyz")}},
	}
	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := call(tt.cmd, tt.args...); err == nil {
				t.Errorf("%s%v がエラーにならなかった（ctx.Now() で隠蔽されている）", tt.cmd, tt.args)
			}
		})
	}
}
