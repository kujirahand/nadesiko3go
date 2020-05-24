package system

import (
	"github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/value"
)

// RegisterFunction : 関数を登録
func RegisterFunction(sys *core.Core) {
	sys.AddFunc("足", core.DefArgs{{"と", "に"}, {"を"}}, Add)
}

// Add : 足す
func Add(args value.ValueArray) (*value.Value, error) {
	l := args[0]
	r := args[1]
	v := value.Add(&l, &r)
	return &v, nil
}
