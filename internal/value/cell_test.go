package value_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

// TestCellVariable pins that a variable cell takes any number of writes.
func TestCellVariable(t *testing.T) {
	c := value.NewCell(true)
	if c.Get().Kind() != value.KindUndefined {
		t.Errorf("初期値 = %v, want undefined", c.Get().Kind())
	}
	if !c.Set(value.Number(1)) || !c.Set(value.Number(2)) {
		t.Fatal("変数セルへの書き込みが拒否された")
	}
	if got := value.ToString(c.Get()); got != "2" {
		t.Errorf("値 = %q, want \"2\"", got)
	}
}

// TestCellConstant pins that a constant cell takes exactly one Init and
// refuses every Set, including one arriving through a closure.
func TestCellConstant(t *testing.T) {
	c := value.NewCell(false)
	if !c.Init(value.Number(1)) {
		t.Fatal("最初のInitが拒否された")
	}
	if c.Init(value.Number(2)) {
		t.Error("二度目のInitが通ってしまった")
	}
	if c.Set(value.Number(3)) {
		t.Error("定数セルへのSetが通ってしまった")
	}
	if got := value.ToString(c.Get()); got != "1" {
		t.Errorf("値 = %q, want \"1\"", got)
	}
}

// TestCellIsShared pins that two references to one cell see each other's
// writes. This is what a closure relies on.
func TestCellIsShared(t *testing.T) {
	c := value.NewCell(true)
	alias := c
	c.Set(value.Number(7))
	if got := value.ToString(alias.Get()); got != "7" {
		t.Errorf("共有した先の値 = %q, want \"7\"", got)
	}
}
