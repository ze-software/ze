// Design: docs/architecture/core-design.md -- the unit-test area as one command
// Overview: groups.go -- the six package groups and their order
//
// A bare `le test-unit` runs all six groups. Named actions run in command-line
// order. Every sweep returns the first failure's exit code.
package testunit

import (
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

type actionRunner func(string, []string, string, []string) (gaterun.ActionReport, int)
type rootResolver func() (string, error)

type toolchainLoader func(string) (gotoolchain.Toolchain, error)

// table derives dispatch, listing, and help from the group table.
func table(tc gotoolchain.Toolchain, run actionRunner) leaction.Area {
	groups := Table()
	actions := make([]leaction.Action, 0, len(groups))
	for _, group := range groups {
		actions = append(actions, leaction.Action{
			Verb:   group.Verb,
			Why:    group.Why,
			Answer: runner(tc, group, run),
		})
	}
	return leaction.New(Area, actions...)
}

// runner executes one group with the race detector's cgo requirement and the
// test process concurrency cap.
func runner(tc gotoolchain.Toolchain, group Group, run actionRunner) func() (any, int) {
	return func() (any, int) {
		return run(group.Verb, group.Argv(tc), tc.Root, tc.Environment(group.EnvOptions()))
	}
}

// metadataOnly builds the command surface without reading the checkout.
func metadataOnly() leaction.Area {
	return table(gotoolchain.Toolchain{}, gaterun.Run)
}

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
func answer(args []string, resolveRoot rootResolver, loadToolchain toolchainLoader, run actionRunner) (any, int) {
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

	return sweep(args, table(tc, run))
}

// sweep derives the bare command's selection from the same action table that
// dispatches named selections. Gate identities remain metadata and are never
// passed back through verb matching.
func sweep(args []string, command leaction.Area) (any, int) {
	selected := args
	if len(selected) == 0 {
		rows := command.Actions().Actions
		selected = make([]string, 0, len(rows))
		for _, row := range rows {
			selected = append(selected, row.Verb)
		}
	}
	return command.Sweep(selected, leaction.RunEveryAction)
}
