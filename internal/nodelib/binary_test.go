package nodelib

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// callCmd は文脈を使わない命令を直接呼び出す。
func callCmd(t *testing.T, name string, args ...value.Value) (value.Value, error) {
	t.Helper()
	c, ok := commands()[name]
	if !ok {
		t.Fatalf("命令『%s』がありません", name)
	}
	return c.fn(nil, args)
}

func numArray(ns ...int) value.Value {
	items := make([]value.Value, len(ns))
	for i, n := range ns {
		items[i] = value.Number(float64(n))
	}
	return value.ArrayValue(value.NewArray(items...))
}

func toInts(t *testing.T, v value.Value) []int {
	t.Helper()
	arr, ok := v.Array()
	if !ok {
		t.Fatalf("配列ではありません: %v", v)
	}
	out := make([]int, arr.Len())
	for i := range out {
		n, _ := arr.Get(i).Number()
		out[i] = int(n)
	}
	return out
}

func TestCRC32KnownVector(t *testing.T) {
	v, err := callCmd(t, "CRC32", value.String("123456789"))
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := v.Number(); uint32(n) != 0xCBF43926 {
		t.Errorf("CRC32 = %x", uint32(n))
	}
	// バイト配列でも同じ値
	v, err = callCmd(t, "CRC32", numArray('1', '2', '3', '4', '5', '6', '7', '8', '9'))
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := v.Number(); uint32(n) != 0xCBF43926 {
		t.Errorf("CRC32(配列) = %x", uint32(n))
	}
}

func TestBinaryPackKnownVectors(t *testing.T) {
	cases := []struct {
		format string
		vals   value.Value
		want   []int
	}{
		{"<I", numArray(0x04034b50), []int{80, 75, 3, 4}},
		{">I", numArray(0x04034b50), []int{4, 3, 75, 80}},
		{"=H", numArray(0x0102), []int{2, 1}},
		{"3B", numArray(1, 2, 3), []int{1, 2, 3}},
		{"<bh", numArray(-1, -2), []int{255, 254, 255}},
		{"<q", numArray(-1), []int{255, 255, 255, 255, 255, 255, 255, 255}},
		{"<4s", value.ArrayValue(value.NewArray(value.String("ab"))), []int{97, 98, 0, 0}},
		{"<2s", value.ArrayValue(value.NewArray(value.String("abc"))), []int{97, 98}},
		{"<2s", value.ArrayValue(value.NewArray(numArray(9, 8, 7))), []int{9, 8}},
	}
	for _, c := range cases {
		v, err := callCmd(t, "バイナリ変換", value.String(c.format), c.vals)
		if err != nil {
			t.Errorf("%s: %v", c.format, err)
			continue
		}
		if got := toInts(t, v); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s = %v, want %v", c.format, got, c.want)
		}
	}
}

func TestBinaryRoundTrip(t *testing.T) {
	format := ">BHIQbhiq"
	vals := numArray(200, 60000, 4000000000, 1<<52, -100, -30000, -2000000000, -(1 << 52))
	packed, err := callCmd(t, "バイナリ変換", value.String(format), vals)
	if err != nil {
		t.Fatal(err)
	}
	out, err := callCmd(t, "バイナリ取得", value.String(format), packed)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := toInts(t, out), toInts(t, vals); !reflect.DeepEqual(got, want) {
		t.Errorf("往復不一致: %v != %v", got, want)
	}

	// s は配列で返り、余ったデータは無視される
	out, err = callCmd(t, "バイナリ取得", value.String("<2s"), numArray(1, 2, 3))
	if err != nil {
		t.Fatal(err)
	}
	arr, _ := out.Array()
	if arr.Len() != 1 || !reflect.DeepEqual(toInts(t, arr.Get(0)), []int{1, 2}) {
		t.Errorf("s の取得結果が不正: %v", toInts(t, arr.Get(0)))
	}
}

func TestBinaryErrors(t *testing.T) {
	packErrs := []struct {
		name   string
		format string
		vals   value.Value
	}{
		{"範囲外(B)", "<B", numArray(256)},
		{"範囲外(B負)", "<B", numArray(-1)},
		{"範囲外(b)", "<b", numArray(128)},
		{"範囲外(h)", "<h", numArray(-32769)},
		{"範囲外(I)", "<I", numArray(1 << 32)},
		{"値が足りない", "<II", numArray(1)},
		{"値が多い", "<I", numArray(1, 2)},
		{"不正な書式文字", "<X", numArray(1)},
		{"型文字なし", "<3", numArray(1)},
	}
	for _, c := range packErrs {
		if _, err := callCmd(t, "バイナリ変換", value.String(c.format), c.vals); err == nil {
			t.Errorf("%s: エラーになるべき", c.name)
		}
	}
	if _, err := callCmd(t, "バイナリ取得", value.String("<I"), numArray(1, 2, 3)); err == nil {
		t.Error("データ不足はエラーになるべき")
	}
	if _, err := callCmd(t, "バイナリ取得", value.String("<Z"), numArray(1, 2, 3)); err == nil {
		t.Error("不正な書式はエラーになるべき")
	}
}

func TestHashAcceptsByteArray(t *testing.T) {
	s, err := callCmd(t, "ハッシュ値計算", value.String("abc"), value.String("md5"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := callCmd(t, "ハッシュ値計算", numArray('a', 'b', 'c'), value.String("md5"))
	if err != nil {
		t.Fatal(err)
	}
	if value.ToString(s) != "900150983cd24fb0d6963f7d28e17f72" || value.ToString(b) != value.ToString(s) {
		t.Errorf("md5: 文字列=%v 配列=%v", value.ToString(s), value.ToString(b))
	}
}

func TestEncryptDecrypt(t *testing.T) {
	plain := "こんにちは、なでしこ"
	enc, err := callCmd(t, "暗号化", value.String(plain), value.String("pass"))
	if err != nil {
		t.Fatal(err)
	}
	enc2, _ := callCmd(t, "暗号化", value.String(plain), value.String("pass"))
	if value.ToString(enc) == value.ToString(enc2) {
		t.Error("ソルト・ノンスが毎回変わるべき")
	}
	dec, err := callCmd(t, "復号化", enc, value.String("pass"))
	if err != nil {
		t.Fatal(err)
	}
	if value.ToString(dec) != plain {
		t.Errorf("復号結果 = %q", value.ToString(dec))
	}

	// バイト配列の平文
	enc, _ = callCmd(t, "暗号化", numArray(104, 105), value.String("pw"))
	if dec, err := callCmd(t, "復号化", enc, value.String("pw")); err != nil || value.ToString(dec) != "hi" {
		t.Errorf("配列平文の往復に失敗: %v %v", dec, err)
	}

	// パスワード違い
	if _, err := callCmd(t, "復号化", enc, value.String("wrong")); err == nil {
		t.Error("パスワード違いはエラーになるべき")
	}
	// 改ざん: 暗号文部分の1文字を変える
	s := value.ToString(enc)
	raw := []byte(s)
	i := len(raw) - 6
	if raw[i] == 'A' {
		raw[i] = 'B'
	} else {
		raw[i] = 'A'
	}
	if _, err := callCmd(t, "復号化", value.String(string(raw)), value.String("pw")); err == nil {
		t.Error("改ざんはエラーになるべき")
	}
	if _, err := callCmd(t, "復号化", value.String("###"), value.String("pw")); err == nil {
		t.Error("不正なBASE64はエラーになるべき")
	}
}

func TestZipAddListRead(t *testing.T) {
	zp := filepath.Join(t.TempDir(), "t.zip")
	add := func(name string, data value.Value) error {
		_, err := callCmd(t, "ZIP追加", value.String(zp), value.String(name), data)
		return err
	}
	if err := add("日本語/ファイル.txt", value.String("こんにちは")); err != nil {
		t.Fatal(err)
	}
	if err := add("bin.dat", numArray(0, 1, 255)); err != nil {
		t.Fatal(err)
	}
	if err := add("bin.dat", value.String("dup")); err == nil {
		t.Error("同名エントリはエラーになるべき")
	}
	for _, bad := range []string{"/abs.txt", "../x.txt", "a/../../x", "C:/x.txt", ""} {
		if err := add(bad, value.String("x")); err == nil {
			t.Errorf("エントリ名『%s』はエラーになるべき", bad)
		}
	}

	list, err := callCmd(t, "ZIP一覧取得", value.String(zp))
	if err != nil {
		t.Fatal(err)
	}
	arr, _ := list.Array()
	if arr.Len() != 2 {
		t.Fatalf("エントリ数 = %d", arr.Len())
	}
	d, _ := arr.Get(0).Dict()
	get := func(k string) value.Value { v, _ := d.Get(k); return v }
	if value.ToString(get("名前")) != "日本語/ファイル.txt" {
		t.Errorf("名前 = %v", value.ToString(get("名前")))
	}
	if n, _ := get("サイズ").Number(); int(n) != len("こんにちは") {
		t.Errorf("サイズ = %v", n)
	}
	for _, k := range []string{"圧縮後サイズ", "更新日時", "フォルダ"} {
		if _, ok := d.Get(k); !ok {
			t.Errorf("キー『%s』がありません", k)
		}
	}
	if ts := value.ToString(get("更新日時")); len(ts) != len("2006-01-02 15:04:05") || !strings.Contains(ts, ":") {
		t.Errorf("更新日時の形式が不正: %q", ts)
	}
	if b, ok := get("フォルダ").Bool(); !ok || b {
		t.Error("フォルダは false のはず")
	}

	data, err := callCmd(t, "ZIPエントリ読", value.String(zp), value.String("日本語/ファイル.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(intsToBytes(toInts(t, data))); got != "こんにちは" {
		t.Errorf("内容 = %q", got)
	}
	data, _ = callCmd(t, "ZIPエントリ読", value.String(zp), value.String("bin.dat"))
	if got := toInts(t, data); !reflect.DeepEqual(got, []int{0, 1, 255}) {
		t.Errorf("bin.dat = %v", got)
	}
	if _, err := callCmd(t, "ZIPエントリ読", value.String(zp), value.String("none")); err == nil {
		t.Error("存在しないエントリはエラーになるべき")
	}
}

func intsToBytes(ns []int) []byte {
	b := make([]byte, len(ns))
	for i, n := range ns {
		b[i] = byte(n)
	}
	return b
}

func TestBinaryQRangeLimit(t *testing.T) {
	// 2^53 は正確に表せるので往復できる
	got, err := callCmd(t, "バイナリ変換", value.String("<Q"), numArray(9007199254740992))
	if err != nil {
		t.Fatalf("2^53 のバイナリ変換がエラー: %v", err)
	}
	back, err := callCmd(t, "バイナリ取得", value.String("<Q"), got)
	if err != nil {
		t.Fatalf("2^53 のバイナリ取得がエラー: %v", err)
	}
	arr, _ := back.Array()
	if n, _ := arr.Get(0).Number(); n != 9007199254740992 {
		t.Fatalf("2^53 の往復結果が違う: %v", n)
	}
	// uint64 最大値は float64 で正確に表せないのでエラーにする
	if _, err := callCmd(t, "バイナリ取得", value.String("<Q"), numArray(255, 255, 255, 255, 255, 255, 255, 255)); err == nil {
		t.Fatal("uint64 最大値のバイナリ取得がエラーにならなかった")
	}
	// int64 の -2^53-2 も範囲外としてエラーにする (LE: FE FF FF FF FF FF DF FF)
	if _, err := callCmd(t, "バイナリ取得", value.String("<q"), numArray(254, 255, 255, 255, 255, 255, 223, 255)); err == nil {
		t.Fatal("int64 の範囲外の値のバイナリ取得がエラーにならなかった")
	}
	// int64 の -1 は範囲内なので読める
	v, err := callCmd(t, "バイナリ取得", value.String("<q"), numArray(255, 255, 255, 255, 255, 255, 255, 255))
	if err != nil {
		t.Fatalf("-1 のバイナリ取得がエラー: %v", err)
	}
	arr, _ = v.Array()
	if n, _ := arr.Get(0).Number(); n != -1 {
		t.Fatalf("-1 の取得結果が違う: %v", n)
	}
}
