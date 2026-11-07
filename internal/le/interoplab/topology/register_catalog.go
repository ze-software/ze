package topology

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// catalogSuite is the name a feature declaration cites this lab by.
const catalogSuite = "topology"

// The catalog resolves through the same Discover call and checker map this
// lab's runner builds its scenarios from, so a scenario a feature cites is
// one the lab runs.
func init() {
	interoplab.RegisterCatalog(interoplab.Catalog{Suite: catalogSuite,
		Scenarios: func(root string) ([]interoplab.ScenarioSource, error) {
			return interoplab.Discover(filepath.Join(root, filepath.FromSlash(scenarioDirectory)), "", checkers())
		},
		RunScenario:      RunScenario,
		Package:          "internal/le/interoplab/topology/",
		ScenarioRoot:     scenarioDirectory,
		SelectorVariable: selectorVariable,
		Action:           "test integration/interop-topology",
	})
}

const (
	// scenarioDirectory holds one directory per scenario, under the checkout.
	scenarioDirectory = "test/interop-topology/scenarios"
	// selectorVariable names the one scenario a run selects.
	selectorVariable = "TOPOLOGY_INTEROP_SCENARIO"
)
