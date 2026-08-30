// Design: ai/rules/cli.md -- the suite table, as data
// Overview: suites.go -- the table this renders
//
// catalog.go prevents a port from hiding data from two external guards.
// The rerun-target guard reads internal/le/verify/engine/verifyengine_test.go.
// The .ci evidence tier reads internal/le/rfc/actions.go.
// Both previously read `all_suites="..."` from a Makefile recipe.
// A recipe that delegates to a program provides no suite names.
// Without this catalog, both guards CAN pass over an empty population.
//
// The guards read the `gating` field, which is derived from the run list.
// A suite therefore cannot claim to gate unless the run includes it.

package functional

import "slices"

// SuiteRow is one suite, as a plain record. `le functional list | json` prints
// the whole table, and the field names are the ones the two guards already
// read.
type SuiteRow struct {
	Name           string   `json:"name"`
	Gating         bool     `json:"gating"`
	Action         string   `json:"action"`
	Rerun          string   `json:"rerun"`
	Budget         string   `json:"budget"`
	BudgetVariable string   `json:"budget-variable"`
	Command        []string `json:"command"`
	Why            string   `json:"why"`
}

// GatingNames answers the authoritative gating run list in declaration order.
//
// The copy makes the exported answer read-only: RFC evidence classification and
// other consumers can inspect the catalog without receiving the mutable slice
// the runner owns.
func GatingNames() []string { return slices.Clone(Gating) }

// Catalog answers every suite, in declaration order.
func Catalog() []SuiteRow {
	rows := make([]SuiteRow, 0, len(Suites))
	for _, suite := range Suites {
		rows = append(rows, SuiteRow{
			Name:           suite.Name,
			Gating:         slices.Contains(Gating, suite.Name),
			Action:         suite.Name,
			Rerun:          suite.Rerun(),
			Budget:         suite.Budget(),
			BudgetVariable: suite.BudgetVar(),
			Command:        suite.Command(),
			Why:            suite.Why,
		})
	}
	return rows
}
