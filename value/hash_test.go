package value

import (
	"testing"
)

func TestHash(t *testing.T) {
	h := NewHashPtr()
	v30 := NewIntPtr(30)
	v50 := NewIntPtr(50)
	h.HashSet("aaa", v30)
	h.HashSet("bbb", v50)
	s := h.HashGet("aaa").ToString()
	if s != "30" {
		t.Errorf("hash get aaa")
	}
	v := h.HashGet("bbb").ToInt()
	if v != 50 {
		t.Errorf("hash get bbb")
	}
	//
	j := h.ToString()
	if j == "{\"aaa\":30,\"bbb\":50}" || j == "{\"bbb\":50,\"aaa\":30}" {
	} else {
		t.Errorf("hash.ToString=" + j)
	}
}

func TestHash2(t *testing.T) {
	h := NewHashPtr()
	v30 := NewIntPtr(30)
	h.HashSet("aaa", v30)
	json := h.ToString()
	if json != "{\"aaa\":30}" {
		t.Errorf("hash.ToString=" + json)
	}
}
