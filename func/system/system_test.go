package system

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/value"
)

func TestSysStr(t *testing.T) {
	eq(t, indexOf, value.NewTArrayFromStrings([]string{"abcde", "c"}), "3")
	eq(t, indexOf, value.NewTArrayFromStrings([]string{"あいうえお", "う"}), "3")
	eq(t, indexOf, value.NewTArrayFromStrings([]string{"abcde", "f"}), "0")
	//
	eq(t, strFind, value.NewTArrayFromStrings([]string{"abcd", "1", "b"}), "2")
	eq(t, strFind, value.NewTArrayFromStrings([]string{"abcabc", "3", "b"}), "5")
	eq(t, strFind, value.NewTArrayFromStrings([]string{"abcdef", "3", "b"}), "0")
	//
	eq(t, strRepeat, value.NewTArrayFromStrings([]string{"あ", "3"}), "あああ")
	eq(t, strCountStr, value.NewTArrayFromStrings([]string{"あいうあいうあいう", "あいう"}), "3")
	eq(t, strCountStr, value.NewTArrayFromStrings([]string{":::あいう:::あいう:::", "あいう"}), "2")
	//
	eq(t, mid, value.NewTArrayFromStrings([]string{"あいうえお", "2", "3"}), "いうえ")
	eq(t, left, value.NewTArrayFromStrings([]string{"あいうえお", "2"}), "あい")
	eq(t, right, value.NewTArrayFromStrings([]string{"あいうえお", "2"}), "えお")
	//
	eq(t, strDelete, value.NewTArrayFromStrings([]string{"あいうえお", "2", "1"}), "あうえお")
	eq(t, strDelete, value.NewTArrayFromStrings([]string{"あいうえお", "3", "2"}), "あいお")
	//
	eq(t, toUpper, value.NewTArrayFromStrings([]string{"aaa"}), "AAA")
	eq(t, toLower, value.NewTArrayFromStrings([]string{"ABC"}), "abc")
}

func TestSys1(t *testing.T) {
	eq(t, urlEncode, value.NewTArrayFromStrings([]string{"123/456"}), "123%2F456")
	eq(t, urlDecode, value.NewTArrayFromStrings([]string{"123%2F456"}), "123/456")
	eq(t, urlAnalizeParams, value.NewTArrayFromStrings([]string{"http://example.com/?a=12%2F&b=45"}), "{\"a\":\"12/\",\"b\":\"45\"}")
}
func TestHashKeys(t *testing.T) {
	h := value.NewHashPtr()
	h.HashSet("a", value.NewIntPtr(11))
	h.HashSet("b", value.NewIntPtr(22))
	a := value.NewTArray()
	a.Append(h)
	eq(t, hashKeys, a, "[\"a\",\"b\"]")
	eq(t, hashValues, a, "[11,22]")
	h.HashDeleteKey("a")
	eq(t, hashKeys, a, "[\"b\"]")
	h.HashDeleteKey("f") // 存在しないキーを削除しても平気か
	eq(t, hashKeys, a, "[\"b\"]")
}

func eq(t *testing.T, f value.TFunction, args *value.TArray, expected string) {
	answer, err := f(args)
	if err != nil {
		t.Errorf("error run expected : %s", expected)
	}
	s := answer.ToString()
	if s != expected {
		t.Errorf("%s != %s", s, expected)
	}
}
