package tomllib

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// レビュー指摘: 辞書が自身を要素として持つ（循環参照）場合、無限再帰で
// プロセスが停止せず、エラーを返すことを確認する。
func TestEncodeTOML_CyclicDict(t *testing.T) {
	d := value.NewDict()
	d.Set("self", value.DictValue(d))
	if _, err := encodeTOML(value.DictValue(d)); err == nil {
		t.Fatal("循環参照する辞書はエラーになるべき")
	}
}

// 配列が自身を要素として持つ場合も同様。
func TestEncodeTOML_CyclicArray(t *testing.T) {
	a := value.NewArray(value.Undefined())
	a.Set(0, value.ArrayValue(a))
	d := value.NewDict()
	d.Set("a", value.ArrayValue(a))
	if _, err := encodeTOML(value.DictValue(d)); err == nil {
		t.Fatal("循環参照する配列はエラーになるべき")
	}
}

// 同じ辞書を循環でなく複数箇所から共有しているだけなら許容する。
func TestEncodeTOML_SharedNonCyclic(t *testing.T) {
	shared := value.NewDict()
	shared.Set("v", value.Number(1))
	d := value.NewDict()
	d.Set("a", value.DictValue(shared))
	d.Set("b", value.DictValue(shared))
	out, err := encodeTOML(value.DictValue(d))
	if err != nil {
		t.Fatalf("共有だけの非循環な値はエラーにならないはず: %v", err)
	}
	if !strings.Contains(out, "v = 1") {
		t.Fatalf("out=%q", out)
	}
}

// レビュー指摘: 2^63は math.MaxInt64 をfloat64に丸めた値と一致してしまい、
// 範囲チェックを素通りしてint64へ変換すると符号が反転する。
func TestEncodeTOML_LargePositiveNumber(t *testing.T) {
	d := value.NewDict()
	d.Set("n", value.Number(9223372036854775808)) // 2^63
	out, err := encodeTOML(value.DictValue(d))
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(out, "-") {
		t.Fatalf("正の数値が負として出力された: out=%q", out)
	}
}

// レビュー指摘: オフセットの無いTOMLのローカル日付・時刻・日時が、
// 誤った文字列（存在しない年月日や時刻を含むRFC3339表記）にならないことを確認する。
func TestDecodeTOML_LocalDateTime(t *testing.T) {
	tests := []struct {
		name string
		toml string
		want string
	}{
		{"日付のみ", `d = 1979-05-27`, "1979-05-27"},
		{"時刻のみ", `d = 07:32:00`, "07:32:00"},
		{"ローカル日時", `d = 1979-05-27T07:32:00`, "1979-05-27T07:32:00"},
		{"オフセット付き日時", `d = 1979-05-27T07:32:00Z`, "1979-05-27T07:32:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := decodeTOML(tt.toml)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			d, ok := v.Dict()
			if !ok {
				t.Fatal("dict ではない")
			}
			item, _ := d.Get("d")
			s, _ := item.String()
			if s != tt.want {
				t.Errorf("got=%q want=%q", s, tt.want)
			}
		})
	}
}
