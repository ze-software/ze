// Design: docs/architecture/core-design.md -- the unit-test area as one command
// Overview: groups.go -- the six package groups and their order
//
// A bare `le test-unit` lists the groups and runs nothing. `le test-unit all`
// runs all six. Named actions run in command-line order, and every sweep
// returns the first failure's exit code.
package testunit

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// allVerb runs every group of the table. It is dispatched beside the table
// rather than declared in it, because a table row would appear in its own
// expansion and would sweep itself.
const allVerb = "all"

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

// Actions returns the command surface as structured data. The listing carries
// `all` beside the groups, because that is where a bare command line lands and
// where every other action states its reason.
func Actions() leaction.List {
	list := metadataOnly().Actions()
	list.Actions = append(list.Actions, leaction.Row{
		Verb: allVerb,
		Why:  "every group above, in table order, whatever any of them answers",
	})
	return list
}

// Subs returns the one-line action hint for command help. `all` is appended as
// a bare word so the hint stays a verb table a reader can complete against.
func Subs() string {
	var tb textbuf.Buffer
	return tb.Str(metadataOnly().Subs()).Str(" | ").Str(allVerb).String()
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

	return sweep(args, table(tc, run))
}

// sweep expands `all` into the same action table that dispatches every named
// selection, so the run list and the listing cannot disagree about which groups
// "all" means. The expansion happens in place, which keeps `all` a word the
// area holds wherever a developer types it. Gate identities remain metadata and
// are never passed back through verb matching.
func sweep(args []string, command leaction.Area) (any, int) {
	rows := command.Actions().Actions
	selected := make([]string, 0, len(args))
	for _, name := range args {
		if name != allVerb {
			selected = append(selected, name)
			continue
		}
		for _, row := range rows {
			selected = append(selected, row.Verb)
		}
	}
	return command.Sweep(selected, leaction.RunEveryAction)
}
