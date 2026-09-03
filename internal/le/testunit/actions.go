// Design: docs/architecture/core-design.md -- the unit-test area as one command
// Overview: groups.go -- the named package groups, and what `all` sweeps
//
// A bare `le test-unit` lists the actions and runs nothing. `le test-unit all`
// runs the whole checkout, then each group whose build tags hide its tests from
// that run. Named actions run in command-line order, and every sweep returns
// the first failure's exit code.
package testunit

import (
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// allVerb is the word that runs the whole checkout. It is an action beside the
// six groups rather than a row of the table, because it names no component
// group: it runs the population those groups are subsets of.
const allVerb = "all"

// allWhy is the reason the listing and the help print for that word.
const allWhy = "the whole checkout race-instrumented, then each group above whose build tags hide its tests from that run"

type actionRunner func(string, []string, string, []string) (gaterun.ActionReport, int)
type rootResolver func() (string, error)

type toolchainLoader func(string) (gotoolchain.Toolchain, error)

// table derives dispatch, listing, and help from the group table, with `all`
// appended as the action that runs the whole checkout.
func table(tc gotoolchain.Toolchain, run actionRunner) leaction.Area {
	actions := actionsFor(tc, Table(), run)
	actions = append(actions, leaction.Action{
		Verb:   allVerb,
		Why:    allWhy,
		Answer: allRunner(tc, run),
	})
	return leaction.New(Area, actions...)
}

// actionsFor turns a group population into the actions that run it. The spare
// slot is for the `all` action its first caller appends.
func actionsFor(tc gotoolchain.Toolchain, rows []Group, run actionRunner) []leaction.Action {
	actions := make([]leaction.Action, 0, len(rows)+1)
	for _, group := range rows {
		actions = append(actions, leaction.Action{
			Verb:   group.Verb,
			Why:    group.Why,
			Answer: runner(tc, group, run),
		})
	}
	return actions
}

// allRunner runs the whole checkout, then every group whose build tags hide its
// tests from that run. It sweeps an area of its own, so each command is
// reported by name and the first failure's code is answered, which is what a
// named multi-action selection already does.
func allRunner(tc gotoolchain.Toolchain, run actionRunner) func() (any, int) {
	return func() (any, int) {
		actions := actionsFor(tc, allGroups(), run)
		verbs := make([]string, 0, len(actions))
		for _, action := range actions {
			verbs = append(verbs, action.Verb)
		}
		return leaction.New(Area, actions...).Sweep(verbs, leaction.RunEveryAction)
	}
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

// Actions returns the command surface as structured data. `all` is one of the
// area's actions, so the listing carries it beside the groups, with the reason
// every other action states.
func Actions() leaction.List {
	return metadataOnly().Actions()
}

// Subs returns the one-line action hint for command help.
func Subs() string {
	return metadataOnly().Subs()
}

// Answer is the `le test-unit` command.
func Answer(args []string) (any, int) {
	return answer(args, lepath.Root, gotoolchain.New, gaterun.Run)
}

// answer keeps checkout and process seams replaceable by package tests. The
// production path above always uses the repository root, native Go toolchain,
// and gaterun subprocess execution.
func answer(args []string, resolveRoot rootResolver, loadToolchain toolchainLoader, run actionRunner) (any, int) {
	// The listing is answered before the checkout is read, so a developer who
	// types the area name to see the groups starts no test run and waits on no
	// toolchain probe (owner directive, 2026-09-02).
	if len(args) == 0 {
		return Actions(), 0
	}

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

	return table(tc, run).Sweep(args, leaction.RunEveryAction)
}
