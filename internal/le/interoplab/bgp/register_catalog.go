package bgp

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The catalog resolves through the same Discover call and checker map this
// lab's runner builds its scenarios from, so a scenario a feature cites is one
// the lab runs. A catalog names scenarios and runs none of them.
func init() {
	interoplab.RegisterCatalog(interoplab.Catalog{Suite: "bgp",
		Scenarios: func(root string) ([]interoplab.ScenarioSource, error) {
			return interoplab.Discover(filepath.Join(root, "test", "interop", "scenarios"), "", checkers())
		}})
}
