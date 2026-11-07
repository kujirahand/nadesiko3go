package cliopt

import (
	"reflect"
	"testing"
)

func TestTakeLongtimeError(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
		ok   bool
	}{
		{name: "先頭のオプション", args: []string{"--longtime-error", "main.nako3", "引数"}, want: []string{"main.nako3", "引数"}, ok: true},
		{name: "runの直後のオプション", args: []string{"run", "--longtime-error", "main.nako3", "引数"}, want: []string{"run", "main.nako3", "引数"}, ok: true},
		{name: "-eの直後のオプション", args: []string{"-e", "--longtime-error", "表示"}, want: []string{"-e", "表示"}, ok: true},
		{name: "プログラム引数は保持する", args: []string{"main.nako3", "--longtime-error"}, want: []string{"main.nako3", "--longtime-error"}, ok: false},
		{name: "引数なし", args: nil, want: nil, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := TakeLongtimeError(tt.args)
			if !reflect.DeepEqual(got, tt.want) || ok != tt.ok {
				t.Fatalf("TakeLongtimeError(%q) = (%q, %t), want (%q, %t)", tt.args, got, ok, tt.want, tt.ok)
			}
		})
	}
}
