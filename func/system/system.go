package system

import (
	. "github.com/kujirahand/nadesiko3go/core"
	"github.com/kujirahand/nadesiko3go/value"
)

func RegisterFunction(sys *Core) {
	sys.AddFunc("足", DefArgs{Josi{"と", "に"}, Josi{"を"}}, Add)
}

func Add(args value.ValueArray) (*value.Value, error) {
	l := args[0]
	r := args[1]
	v := value.Add(&l, &r)
	return &v, nil
}
