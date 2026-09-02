package gogen

import (
	"encoding/json"

	"github.com/kujirahand/nadesiko3go/internal/ir"
)

// encodeProgram is json.Marshal, named so the reader does not have to wonder
// whether this is the same wire format bundle.go and compat use (it is —
// AGENTS.md §6 calls IR "the versioned, serializable boundary").
func encodeProgram(prog *ir.Program) ([]byte, error) {
	return json.Marshal(prog)
}
