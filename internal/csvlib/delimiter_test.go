package csvlib_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/csvlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// プラグインはプロセス内で共有され設定が残るため、VM経由ではなく
// 新しいPluginを直接呼んで、他のテストに設定を漏らさないようにする。

func callImpl(t *testing.T, p *csvlib.Plugin, name string, args ...value.Value) value.Value {
	t.Helper()
	v, err := p.Impls()[name](nil, args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return v
}

func setDelimiter(t *testing.T, p *csvlib.Plugin, d string) {
	t.Helper()
	opt := value.NewDict()
	opt.Set("delimiter", value.String(d))
	callImpl(t, p, "CSVオプション設定", value.DictValue(opt))
}

func firstRowLen(t *testing.T, v value.Value) int {
	t.Helper()
	rows, ok := v.Array()
	if !ok || rows.Len() == 0 {
		t.Fatalf("配列が返りませんでした: %v", v)
	}
	row, ok := rows.Get(0).Array()
	if !ok {
		t.Fatalf("行が配列ではありません")
	}
	return row.Len()
}

func table(cells ...string) value.Value {
	row := make([]value.Value, len(cells))
	for i, c := range cells {
		row[i] = value.String(c)
	}
	return value.ArrayValue(value.NewArray(value.ArrayValue(value.NewArray(row...))))
}

// CSVオプション設定 の delimiter が取得・変換の双方に反映されること (#205)
func TestCSVDelimiterOption(t *testing.T) {
	t.Run("設定した区切り文字でCSV取得", func(t *testing.T) {
		p := csvlib.New()
		setDelimiter(t, p, ";")
		if n := firstRowLen(t, callImpl(t, p, "CSV取得", value.String("a;b"))); n != 2 {
			t.Errorf("列数 = %d, want 2", n)
		}
	})
	t.Run("設定した区切り文字でCSV変換", func(t *testing.T) {
		p := csvlib.New()
		setDelimiter(t, p, ";")
		got := value.ToString(callImpl(t, p, "CSV変換", table("a", "b")))
		if got != "a;b\r\n" {
			t.Errorf("got %q, want %q", got, "a;b\r\n")
		}
	})
	t.Run("未設定ならカンマで取得", func(t *testing.T) {
		p := csvlib.New()
		if n := firstRowLen(t, callImpl(t, p, "CSV取得", value.String("a;b"))); n != 1 {
			t.Errorf("列数 = %d, want 1", n)
		}
		if n := firstRowLen(t, callImpl(t, p, "CSV取得", value.String("a,b"))); n != 2 {
			t.Errorf("列数 = %d, want 2", n)
		}
	})
	t.Run("未設定ならカンマで変換", func(t *testing.T) {
		p := csvlib.New()
		got := value.ToString(callImpl(t, p, "CSV変換", table("a", "b")))
		if got != "a,b\r\n" {
			t.Errorf("got %q, want %q", got, "a,b\r\n")
		}
	})
	t.Run("TSVは設定に関わらずタブで、設定を書き換えない", func(t *testing.T) {
		p := csvlib.New()
		setDelimiter(t, p, ";")
		if n := firstRowLen(t, callImpl(t, p, "TSV取得", value.String("x\ty;z"))); n != 2 {
			t.Errorf("TSV列数 = %d, want 2", n)
		}
		if got := value.ToString(callImpl(t, p, "TSV変換", table("a", "b"))); got != "a\tb\r\n" {
			t.Errorf("TSV変換 = %q", got)
		}
		if n := firstRowLen(t, callImpl(t, p, "CSV取得", value.String("a;b"))); n != 2 {
			t.Errorf("TSV後のCSV列数 = %d, want 2", n)
		}
	})
}
