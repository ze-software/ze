package radius

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The catalog resolves through the same Discover call and checker map this
// lab's runner builds its scenarios from, so a scenario a feature cites is one
// the lab runs. A catalog names scenarios and runs none of them.
func init() {
	interoplab.RegisterCatalog(interoplab.Catalog{Suite: "radius",
		Scenarios: func(root string) ([]interoplab.ScenarioSource, error) {
			return interoplab.Discover(filepath.Join(root, labDirectory, "scenarios"), "", scenarioCheckerMap(interoplab.ReadEnvironment(interoplab.EnvironmentOptions{}).SessionTimeout))
		}})
}
