package vm_test

import (
	"github.com/kujirahand/nadesiko3go/internal/imagelib"
	"github.com/kujirahand/nadesiko3go/internal/officelib"
	"github.com/kujirahand/nadesiko3go/internal/pdflib"
	"github.com/kujirahand/nadesiko3go/internal/sqlitelib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

func init() {
	vm.RegisterPlugin(
		sqlitelib.New(),
		officelib.New(),
		pdflib.New(),
		imagelib.New(),
	)
}
