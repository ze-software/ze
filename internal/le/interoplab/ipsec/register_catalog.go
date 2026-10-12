package ipsec

import (
	"context"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The catalog resolves through the same Discover call and checker map this
// lab's runner builds its scenarios from, so a scenario a feature cites is one
// the lab runs. RunScenario runs one of them through this lab's own runner,
// with the scenario name as its selector.
func init() {
	interoplab.RegisterCatalog(interoplab.Catalog{Suite: "ipsec",
		Scenarios: func(root string) ([]interoplab.ScenarioSource, error) {
			return interoplab.Discover(filepath.Join(root, "test", "interop-ipsec", "scenarios"), "", checkerAdapters())
		},
		RunScenario: func(ctx context.Context, root, scenario string) interoplab.SuiteReport {
			report, _ := RunAt(ctx, root, scenario)
			return report.SuiteReport
		}})
}
