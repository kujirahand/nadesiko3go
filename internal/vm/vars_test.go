package vm_test

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// TestVarsFrameSpecials pins that Vars reports the final value of the
// frame-owned system values — 『それ』・『対象』・『対象キー』・『回数』・
// 『エラーメッセージ』 — which the run kept in the top frame rather than in
// the program-wide slot (issue #217).
func TestVarsFrameSpecials(t *testing.T) {
	r, err := vm.RunSource("それ=42", "main.nako3", []string{"それ"})
	if err != nil {
		t.Fatal(err)
	}
	if got := value.ToString(r.Vars["それ"]); got != "42" {
		t.Errorf("それ = %q, want \"42\"", got)
	}
}

// TestVarsFrameSpecialsAfterError pins that an エラー監視 block leaves
// 『エラーメッセージ』 readable through Vars once the run is over.
func TestVarsFrameSpecialsAfterError(t *testing.T) {
	code := "エラー監視\n「失敗した」のエラー発生\nエラーならば\nここまで"
	r, err := vm.RunSource(code, "main.nako3", []string{"エラーメッセージ"})
	if err != nil {
		t.Fatal(err)
	}
	if got := value.ToString(r.Vars["エラーメッセージ"]); got != "失敗した" {
		t.Errorf("エラーメッセージ = %q, want \"失敗した\"", got)
	}
}
