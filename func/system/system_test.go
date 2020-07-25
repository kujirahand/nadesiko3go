package system

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/value"
)

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
	h.HashRemove("a")
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
