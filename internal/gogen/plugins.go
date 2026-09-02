package gogen

import (
	"fmt"
	"sort"

	"github.com/kujirahand/nadesiko3go/internal/csvlib"
	"github.com/kujirahand/nadesiko3go/internal/imagelib"
	"github.com/kujirahand/nadesiko3go/internal/mathlib"
	"github.com/kujirahand/nadesiko3go/internal/nodelib"
	"github.com/kujirahand/nadesiko3go/internal/officelib"
	"github.com/kujirahand/nadesiko3go/internal/pdflib"
	"github.com/kujirahand/nadesiko3go/internal/sqlitelib"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
)

// pluginRT names the pkg/runtime constructor that stands in for one plugin,
// so that Generate can build a registry in generated code identical to the
// one BuildRegistry gives the caller for compiling — the two must never
// disagree, because a stdlib command's ID depends on the full, sorted set of
// commands the registry was built with (AGENTS.md §12: a mismatch here calls
// the wrong command silently instead of failing loudly).
var pluginConstructors = map[string]struct {
	plugin func() stdlib.Plugin
	rtCall string
}{
	"nodelib":   {func() stdlib.Plugin { return nodelib.New() }, "rt.NodeLib()"},
	"csvlib":    {func() stdlib.Plugin { return csvlib.New() }, "rt.CSVLib()"},
	"mathlib":   {func() stdlib.Plugin { return mathlib.New() }, "rt.MathLib()"},
	"sqlitelib": {func() stdlib.Plugin { return sqlitelib.New() }, "rt.SQLiteLib()"},
	"officelib": {func() stdlib.Plugin { return officelib.New() }, "rt.OfficeLib()"},
	"pdflib":    {func() stdlib.Plugin { return pdflib.New() }, "rt.PDFLib()"},
	"imagelib":  {func() stdlib.Plugin { return imagelib.New() }, "rt.ImageLib()"},
}

// PluginNames lists the plugins BuildRegistry and Generate know, sorted.
func PluginNames() []string {
	names := make([]string, 0, len(pluginConstructors))
	for name := range pluginConstructors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// BuildRegistry builds the registry a caller should compile source with in
// order to match a Generate call given the same names.
func BuildRegistry(names []string) (*stdlib.Registry, error) {
	plugins := make([]stdlib.Plugin, 0, len(names))
	for _, name := range names {
		c, ok := pluginConstructors[name]
		if !ok {
			return nil, fmt.Errorf("知らないプラグインです: %s（使えるのは %v）", name, PluginNames())
		}
		plugins = append(plugins, c.plugin())
	}
	return stdlib.NewRegistry(plugins...), nil
}

// registryCallFor turns Options.Plugins into the rt.NewRegistry(...) call
// generated code builds its Machine's registry with.
func registryCallFor(names []string) (string, error) {
	args := make([]string, 0, len(names))
	for _, name := range names {
		c, ok := pluginConstructors[name]
		if !ok {
			return "", fmt.Errorf("知らないプラグインです: %s（使えるのは %v）", name, PluginNames())
		}
		args = append(args, c.rtCall)
	}
	call := "rt.NewRegistry("
	for i, a := range args {
		if i > 0 {
			call += ", "
		}
		call += a
	}
	call += ")"
	return call, nil
}
