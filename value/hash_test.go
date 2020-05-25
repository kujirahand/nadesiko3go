package value

import (
	"testing"
)

func TestHash(t *testing.T) {
	h := NewValueHash()
	v30 := NewValueInt(30)
	v50 := NewValueInt(50)
	h.Set("aaa", &v30)
	h.Set("bbb", &v50)
	s := h.Get("aaa").ToString()
	if s != "30" {
		t.Errorf("hash get aaa")
	}
	v := h.Get("bbb").ToInt()
	if v != 50 {
		t.Errorf("hash get bbb")
	}
	//
	json := h.ToString()
	if json != "{\"aaa\":30,\"bbb\":50}" {
		t.Errorf("hash.ToString=" + json)
	}
}
