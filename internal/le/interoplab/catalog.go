// Design: docs/architecture/testing/interop.md -- scenario discovery and selector contract
// Related: discover.go -- every catalog answers through Discover
//
// A catalog is one interop suite's scenario list as a reader outside the suite
// asks for it: `./le feature check` resolves a declaration's Interop entry
// `<suite>/<scenario>` here. Each suite registers its own catalog, built on the
// same Discover call and checker map its runner uses, so the names a feature
// cites are the names the runner runs and no second list exists. The catalog
// also declares how to run ONE of those scenarios through the suite's own
// runner, which is how `./le feature record-run` records an interop green run
// without a second resolver.

package interoplab

import (
	"context"
	"slices"
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
