package rsvpte

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The catalog resolves through the same Discover call and checker map this
// lab's runner builds its scenarios from, so a scenario a feature cites is one
// the lab runs. RunScenario runs one of them through this lab's own runner,
// with the scenario name as its selector. The last four fields say where this
// lab lives and how the repository runs it.
func init() {
	interoplab.RegisterCatalog(interoplab.Catalog{Suite: catalogSuite,
		Scenarios: func(root string) ([]interoplab.ScenarioSource, error) {
			return interoplab.Discover(filepath.Join(root, filepath.FromSlash(scenarioDirectory)), "", scenarioCheckerMap(interoplab.ReadEnvironment(interoplab.EnvironmentOptions{}).SessionTimeout))
		},
		RunScenario:      RunAt,
		Package:          "internal/le/interoplab/rsvpte/",
		ScenarioRoot:     scenarioDirectory,
		SelectorVariable: selectorVariable,
		Action:           "test integration/interop-rsvpte",
	})
}

const (
	// scenarioDirectory holds one directory per scenario, under the checkout.
	scenarioDirectory = labDirectory + "/scenarios"
	// selectorVariable names the one scenario a run selects.
	selectorVariable = "RSVPTE_INTEROP_SCENARIO"
)
