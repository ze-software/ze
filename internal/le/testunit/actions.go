// Design: docs/architecture/core-design.md -- the unit-test area as one command
// Overview: groups.go -- the five package groups and their order
//
// A bare `le test-unit` runs all five groups. Named gates run in command-line
// order. Every sweep runs all selected groups and returns the first failure's
// exit code, matching scripts/le/gateapp.py.
package testunit

import (
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

type gateRunner func(string, []string, string, []string) (gaterun.GateReport, int)

type rootResolver func() (string, error)

type toolchainLoader func(string) (gotoolchain.Toolchain, error)

// table derives dispatch, listing, help, and parity claims from the group table.
func table(tc gotoolchain.Toolchain, run gateRunner) leaction.Area {
	gates := Table()
	actions := make([]leaction.Action, 0, len(gates))
	for _, gate := range gates {
		actions = append(actions, leaction.Action{
			Gate:   gate.Name,
			Why:    gate.Why,
			Forks:  gate.Argv(tc),
			Answer: runner(tc, gate, run),
		})
	}
	return leaction.New(Area, actions...)
}

// runner executes one group with the race detector's cgo requirement and the
// test process concurrency cap.
func runner(tc gotoolchain.Toolchain, gate Gate, run gateRunner) func() (any, int) {
	return func() (any, int) {
		return run(gate.Name, gate.Argv(tc), tc.Root, tc.Environment(gate.EnvOptions()))
	}
}

// metadataOnly builds the command surface without reading the checkout.
func metadataOnly() leaction.Area {
	return table(gotoolchain.Toolchain{}, gaterun.Run)
}

// Gates returns the five Make gate names in execution order.
func Gates() []string { return metadataOnly().Gates() }

// Actions returns the command surface as structured data.
func Actions() leaction.List { return metadataOnly().Actions() }

// Subs returns the one-line action hint for command help.
func Subs() string { return metadataOnly().Subs() }

// Answer is the `le test-unit` command.
func Answer(args []string) (any, int) {
	return answer(args, lepath.Root, gotoolchain.New, gaterun.Run)
}

// answer keeps checkout and process seams replaceable by package tests. The
// production path above always uses the repository root, native Go toolchain,
// and gaterun subprocess execution.
func answer(args []string, resolveRoot rootResolver, loadToolchain toolchainLoader, run gateRunner) (any, int) {
	root, err := resolveRoot()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	tc, err := loadToolchain(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	selected := args
	if len(selected) == 0 {
		selected = Gates()
	}
	return table(tc, run).Sweep(selected, leaction.RunEveryAction)
}
