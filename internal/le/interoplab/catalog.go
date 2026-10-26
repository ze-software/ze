// Design: docs/architecture/testing/interop.md -- scenario discovery and selector contract
// Related: discover.go -- every catalog answers through Discover
//
// A catalog is one interop suite's scenario list as a reader outside the suite
// asks for it: `./le feature check` resolves a declaration's Interop entry
// `<suite>/<scenario>` here. Each suite registers its own catalog, built on the
// same Discover call and checker map its runner uses, so the names a feature
// cites are the names the runner runs and no second list exists. The catalog
// also declares how to run ONE of those scenarios through the suite's own
// runner, which is how `./le feature run` records an interop green run
// without a second resolver. Last, it declares where the suite lives and how
// the repository runs it: `./le rfc` maps an `RFC requirement:` tag on a
// checker to its suite through Package and Action, and `./le rfc
// record` drives one scenario through ScenarioRoot,
// SelectorVariable and Action, so neither keeps a table of suites of its own.

package interoplab

import (
	"context"
	"slices"
	"strings"
	"sync"
)

// Catalog answers the scenarios one interop suite runs.
type Catalog struct {
	// Suite is the name a feature declaration cites before the slash.
	Suite string
	// Scenarios answers the suite's scenarios under the checkout at root,
	// through Discover with an empty selector.
	Scenarios func(root string) ([]ScenarioSource, error)
	// RunScenario runs exactly the scenario named scenario under the checkout
	// at root through the suite's own runner, with that name as the runner's
	// selector, and answers the runner's report. It reads no selector variable
	// from the environment: the argument wins. Docker is required.
	RunScenario func(ctx context.Context, root, scenario string) SuiteReport
	// Package is the slash path, relative to the checkout and ending in '/',
	// of the directory holding this suite's checkers. A tagged file under it
	// is carried by this suite.
	Package string
	// ScenarioRoot is the slash path, relative to the checkout, of the
	// directory holding one subdirectory per scenario: the directory the
	// runner hands Discover.
	ScenarioRoot string
	// SelectorVariable is the environment variable the suite's runner reads
	// the one scenario to run from when its caller passes no selector.
	SelectorVariable string
	// Action is the `./le` action whose run executes this suite, written
	// `<command> <area>/<verb>` (for example `test integration/interop`).
	Action string
}

var (
	catalogMu sync.Mutex
	catalogs  = map[string]Catalog{}
)

// RegisterCatalog records one suite's catalog. It MUST be called from the
// suite package's register.go init. A second registration of one name is a
// Ze defect.
func RegisterCatalog(catalog Catalog) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	if catalog.Suite == "" {
		panic("BUG: interoplab.RegisterCatalog with an empty suite name")
	}
	if catalog.Scenarios == nil {
		panic("BUG: interoplab.RegisterCatalog " + catalog.Suite + " without a Scenarios function")
	}
	if catalog.RunScenario == nil {
		panic("BUG: interoplab.RegisterCatalog " + catalog.Suite + " without a RunScenario function")
	}
	if !strings.HasSuffix(catalog.Package, "/") {
		panic("BUG: interoplab.RegisterCatalog " + catalog.Suite + " without a Package ending in '/'")
	}
	if catalog.ScenarioRoot == "" {
		panic("BUG: interoplab.RegisterCatalog " + catalog.Suite + " without a ScenarioRoot")
	}
	if catalog.SelectorVariable == "" {
		panic("BUG: interoplab.RegisterCatalog " + catalog.Suite + " without a SelectorVariable")
	}
	if !strings.Contains(catalog.Action, "/") {
		panic("BUG: interoplab.RegisterCatalog " + catalog.Suite + " without an Action `<command> <area>/<verb>`")
	}
	if _, held := catalogs[catalog.Suite]; held {
		panic("BUG: interoplab.RegisterCatalog " + catalog.Suite + " registered twice")
	}
	catalogs[catalog.Suite] = catalog
}

// CatalogNamed answers the catalog of suite. Safe for concurrent use.
func CatalogNamed(suite string) (Catalog, bool) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	catalog, held := catalogs[suite]
	return catalog, held
}

// CatalogSuites answers every registered suite name, sorted. Safe for
// concurrent use.
func CatalogSuites() []string {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	names := make([]string, 0, len(catalogs))
	for name := range catalogs {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
