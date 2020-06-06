package system

import (
	"testing"
)

func TestBase(t *testing.T) {
	comp(t, "30", "30")
	comp(t, "true", "true")
	comp(t, "false", "false")
	comp(t, "null", "null")
	comp(t, "\"a\"", "\"a\"")
	comp(t, "[1,2,3]", "[1,2,3]")
	comp(t, "{'a':1}", "{\"a\":1}")
	comp(t, "{'a':[1,2,3,4,5]}", "{\"a\":[1,2,3,4,5]}")
	comp(t, "{'a':'\\u65b0\\u6f5f'}", "{\"a\":\"新潟\"}")
}
func TestBase2(t *testing.T) {
	comp(t, "[\\x4e\\x47\\x54']", "[\"NGT\"]")
}

func comp(t *testing.T, json, expected string) {
	v, err := JSONDecode(json)
	if err != nil {
		t.Errorf("%s %s != %s", err.Error(), json, expected)
		return
	}
	jsonv := v.ToJSONString()
	if jsonv != expected {
		t.Errorf("error : [%s] != [%s]", jsonv, expected)
	}
}
