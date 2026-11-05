package stdlib_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// referenceCurrencyFormat は修正前の『通貨形式』の実装そのもの。
// 等価性テストの参照としてだけ使う。
func referenceCurrencyFormat(s string) string {
	parts := strings.SplitN(s, ".", 2)
	start := 0
	if strings.HasPrefix(parts[0], "-") || strings.HasPrefix(parts[0], "+") {
		start = 1
	}
	for i := len(parts[0]) - 3; i > start; i -= 3 {
		parts[0] = parts[0][:i] + "," + parts[0][i:]
	}
	return strings.Join(parts, ".")
}

func lookupCurrency(t *testing.T) (func(value.Value) string, func(string) string) {
	t.Helper()
	r := stdlib.NewRegistry()
	e, ok := r.Lookup("通貨形式")
	if !ok || e.Fn == nil {
		t.Fatal("通貨形式 が登録されていない")
	}
	call := func(v value.Value) string {
		t.Helper()
		got, err := e.Fn(newContext(), []value.Value{v})
		if err != nil {
			t.Fatalf("通貨形式(%v): %v", v, err)
		}
		return value.ToString(got)
	}
	return call, func(s string) string { return call(value.String(s)) }
}

// Issue #361 の回帰テスト: 『通貨形式』の出力を従来どおりに維持すること。
// 期待値は修正前の実装で確認したもの。グループ化はバイト単位で行われる
// 現行仕様（全角数字でも3バイトごとに区切る）を含めて固定する。
func TestCurrencyFormatOutputsPreserved(t *testing.T) {
	call, _ := lookupCurrency(t)

	tests := []struct {
		name string
		in   value.Value
		want string
	}{
		{"通常金額", value.String("12345"), "12,345"},
		{"千", value.String("1000"), "1,000"},
		{"大きい金額", value.String("123456789"), "123,456,789"},
		{"区切り不要", value.String("123"), "123"},
		{"ゼロ", value.String("0"), "0"},
		{"負号", value.String("-12345"), "-12,345"},
		{"負号4桁", value.String("-1234"), "-1,234"},
		{"正号", value.String("+12345"), "+12,345"},
		{"正号4桁", value.String("+1234"), "+1,234"},
		{"小数点", value.String("123456.789"), "123,456.789"},
		{"小数部は無加工", value.String("123456.7891234"), "123,456.7891234"},
		{"小数だけ", value.String("3.1415"), "3.1415"},
		{"小数点が2つ", value.String("1234.5.6"), "1,234.5.6"},
		{"末尾小数点", value.String("123."), "123."},
		{"先頭小数点", value.String(".5"), ".5"},
		{"英字", value.String("abcdefg"), "a,bcd,efg"},
		{"英字短い", value.String("abc"), "abc"},
		{"空", value.String(""), ""},
		{"符号のみ負", value.String("-"), "-"},
		{"符号のみ正", value.String("+"), "+"},
		{"小数点のみ", value.String("."), "."},
		{"既存カンマ", value.String("1,234"), "1,,234"},
		{"全角数字はバイト単位", value.String("１２３４５"), "１,２,３,４,５"},
		{"6桁の境界", value.String("123456"), "123,456"},
		{"7桁の境界", value.String("1234567"), "1,234,567"},
		{"負号6桁", value.String("-123456"), "-123,456"},
		{"負号3桁", value.String("-123"), "-123"},
		{"符号と小数点", value.String("-.5"), "-.5"},
		{"正号と小数点のみ", value.String("+."), "+."},
		{"符号が2つ", value.String("+-1234"), "+-1,234"},
		{"符号と全角数字", value.String("-１２３４"), "-１,２,３,４"},
		{"不正なUTF-8", value.String("\xff\xfe\xfd\xfc"), "\xff,\xfe\xfd\xfc"},
		{"数値入力", value.Number(12345), "12,345"},
		{"数値入力小数", value.Number(123456.789), "123,456.789"},
		{"負の数値入力", value.Number(-12345.678), "-12,345.678"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := call(tt.in); got != tt.want {
				t.Errorf("通貨形式(%q) = %q, want %q", value.ToString(tt.in), got, tt.want)
			}
		})
	}

	// 長い入力でも先頭・末尾の区切り位置が従来どおりであること。
	long := strings.Repeat("1234567890", 1000) // 10,000桁
	got := call(value.String(long))
	wantLen := len(long) + (len(long)-1)/3
	if len(got) != wantLen {
		t.Fatalf("通貨形式(10000桁) の長さ = %d, want %d", len(got), wantLen)
	}
	if !strings.HasPrefix(got, "1,234,567,890,") || !strings.HasSuffix(got, ",890") {
		t.Errorf("通貨形式(10000桁) の先頭・末尾が想定と違う: %.20q ... %.10q", got, got[len(got)-10:])
	}
	if strings.Count(got, ",") != (len(long)-1)/3 {
		t.Errorf("通貨形式(10000桁) のカンマ数 = %d, want %d", strings.Count(got, ","), (len(long)-1)/3)
	}
	if stripped := strings.ReplaceAll(got, ",", ""); stripped != long {
		t.Errorf("通貨形式(10000桁) からカンマを除くと元の入力に戻らない")
	}
}

// TestCurrencyFormatMatchesReference は、修正前の実装（参照実装）と
// 出力が完全一致することを、シード固定のランダム入力と、桁数の剰余が
// 0・1・2 の全パターンになる長い入力で確かめる (#361)。
func TestCurrencyFormatMatchesReference(t *testing.T) {
	_, callStr := lookupCurrency(t)

	alphabet := []string{"0", "1", "9", "-", "+", ".", ",", "a", "Z", " ", "１", "あ", ""}
	rng := rand.New(rand.NewSource(361))
	for iter := 0; iter < 20000; iter++ {
		n := rng.Intn(30)
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteString(alphabet[rng.Intn(len(alphabet))])
		}
		s := sb.String()
		if got, want := callStr(s), referenceCurrencyFormat(s); got != want {
			t.Fatalf("通貨形式(%q) = %q, want %q (参照実装)", s, got, want)
		}
	}

	// 長い入力: 桁数 mod 3 の全パターン、符号・小数部付きも含める。
	for _, n := range []int{9999, 10000, 10001} {
		digits := strings.Repeat("1234567890", n/10+1)[:n]
		for _, s := range []string{digits, "-" + digits, "+" + digits + ".987", digits + ".5"} {
			if got, want := callStr(s), referenceCurrencyFormat(s); got != want {
				t.Fatalf("通貨形式(長さ%d) が参照実装と一致しない", len(s))
			}
		}
	}
}

// Issue #361 の受け入れ条件用ベンチマーク: 1万・2万・4万桁の入力を
// 『通貨形式』へ変換する時間と割当を測る。通常長も併せて測り、
// 性能悪化がないことを確認する。
// 実行例: go test ./internal/stdlib/ -bench BenchmarkCurrencyFormat -benchmem
func BenchmarkCurrencyFormat(b *testing.B) {
	r := stdlib.NewRegistry()
	e, _ := r.Lookup("通貨形式")
	ctx := newContext()
	bench := func(name, in string) {
		b.Run(name, func(b *testing.B) {
			wantLen := len(referenceCurrencyFormat(in))
			inVal := value.String(in)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got, err := e.Fn(ctx, []value.Value{inVal})
				if err != nil {
					b.Fatal(err)
				}
				if len(value.ToString(got)) != wantLen {
					b.Fatalf("len = %d, want %d", len(value.ToString(got)), wantLen)
				}
			}
		})
	}
	bench("normal9", "123456789")
	for _, n := range []int{10000, 20000, 40000} {
		digits := strings.Repeat("1234567890", n/10)
		bench(fmt.Sprintf("digits%d", n), digits)
		bench(fmt.Sprintf("digits%d_signed_decimal", n), "-"+digits+".123")
	}
}
