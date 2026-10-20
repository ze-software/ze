package bgp

import (
	"context"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The catalog resolves through the same Discover call and checker map this
// lab's runner builds its scenarios from, so a scenario a feature cites is one
// the lab runs. RunScenario runs one of them through this lab's own runner,
// with the scenario name as its selector. The last four fields say where this
// lab lives and how the repository runs it.
func init() {
	interoplab.RegisterCatalog(interoplab.Catalog{Suite: "bgp",
		Scenarios: func(root string) ([]interoplab.ScenarioSource, error) {
			return interoplab.Discover(filepath.Join(root, filepath.FromSlash(scenarioDirectory)), "", checkers())
		},
		RunScenario: func(ctx context.Context, root, scenario string) interoplab.SuiteReport {
			return RunAt(ctx, root, Options{Scenario: scenario})
		},
		Package:          "internal/le/interoplab/bgp/",
		ScenarioRoot:     scenarioDirectory,
		SelectorVariable: selectorVariable,
		Action:           "test integration/interop",
	})
}

const (
	// scenarioDirectory holds one directory per scenario, under the checkout.
	scenarioDirectory = "test/interop/scenarios"
	// selectorVariable names the one scenario a run selects.
	selectorVariable = "INTEROP_SCENARIO"
)
