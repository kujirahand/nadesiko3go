package value

import (
	"testing"
)

func TestHash(t *testing.T) {
	h := NewValueHash()
	h.Set("aaa", NewValueInt(30))
	h.Set("bbb", NewValueInt(50))
	s := h.Get("aaa").ToString()
	if s != "30" {
		t.Errorf("hash get aaa")
	}
	v := h.Get("bbb").ToInt()
	if v != 50 {
		t.Errorf("hash get bbb")
	}
}
