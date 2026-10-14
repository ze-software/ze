package radiussubscriber

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// catalogSuite is the name a feature declaration cites this lab by.
const catalogSuite = "radius-subscriber"

// The catalog resolves through the same Discover call and checker map this
// lab's runner builds its scenarios from, so a scenario a feature cites is one
// the lab runs.
func init() {
	interoplab.RegisterCatalog(interoplab.Catalog{Suite: catalogSuite,
		Scenarios: func(root string) ([]interoplab.ScenarioSource, error) {
			return interoplab.Discover(filepath.Join(root, labDirectory, "scenarios"), "",
				scenarioCheckerMap(interoplab.ReadEnvironment(interoplab.EnvironmentOptions{}).SessionTimeout))
		},
		RunScenario: RunAt})
}
