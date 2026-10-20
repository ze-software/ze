package radius

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
	interoplab.RegisterCatalog(interoplab.Catalog{Suite: "radius",
		Scenarios: func(root string) ([]interoplab.ScenarioSource, error) {
			return interoplab.Discover(filepath.Join(root, filepath.FromSlash(scenarioDirectory)), "", scenarioCheckerMap(interoplab.ReadEnvironment(interoplab.EnvironmentOptions{}).SessionTimeout))
		},
		RunScenario: func(ctx context.Context, root, scenario string) interoplab.SuiteReport {
			report, _ := RunAt(ctx, root, scenario)
			return report.SuiteReport
		},
		Package:          "internal/le/interoplab/radius/",
		ScenarioRoot:     scenarioDirectory,
		SelectorVariable: selectorVariable,
		Action:           "test integration/interop-radius",
	})
}

const (
	// scenarioDirectory holds one directory per scenario, under the checkout.
	scenarioDirectory = labDirectory + "/scenarios"
	// selectorVariable names the one scenario a run selects.
	selectorVariable = "RADIUS_INTEROP_SCENARIO"
)
