// Design: docs/architecture/core-design.md -- the integration area, as one command
// Overview: gates.go -- the table every row here is derived from
//
// actions.go defines this area's action table.
// letools/leaction supplies the shared dispatch, listing, help line, and two refusals.
// This file specifies the area's different behavior.
//
// This area has no aggregate run, and bare `le integration` refuses one.
// Other areas run their complete set when given no gate name.
// This set needs hours of Docker, root, and QEMU work.
// No Make target ever requested all of it, so the shortest spelling must not do so.
// `le integration list` still lists the gate names and runs no gates.

package integration

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/letools/gaterun"
	"github.com/ze-software/ze/letools/gotoolchain"
	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
)

// table builds this invocation's action table from the gate table. The
// toolchain is resolved once and shared, so twelve gates named together read
// go.mod and feature-gates.txt once between them.
func table(tc gotoolchain.Toolchain) leaction.Area {
	gates := Table()
	rows := make([]leaction.Action, 0, len(gates))
	for _, gate := range gates {
		rows = append(rows, leaction.Action{
			Gate: gate.Name,
			Why:  gate.Why,
			// The argv this gate starts, published so letools/parity can tell
			// the nine that run `go test` from the three that start a lab's
			// Python runner. Nothing here decides which is which: the census
			// reads the argv (letools/leaction, forksAScript).
			Forks:  gate.Argv(),
			Answer: runner(tc, gate),
		})
	}
	return leaction.New(Area, rows...)
}

// runner answers the action that runs one gate under the environment its own
// command asks for.
func runner(tc gotoolchain.Toolchain, gate Gate) func() (any, int) {
	return func() (any, int) {
		return gaterun.Run(gate.Name, gate.Argv(), tc.Root, tc.Environment(gotoolchain.EnvOptions{CGO: gate.NeedsCgo()}))
	}
}

// metadataOnly supplies names but does not resolve a checkout.
// The listing, help line, and census need only the gate names.
// Thus, `le --help` does not read go.mod.
func metadataOnly() leaction.Area { return table(gotoolchain.Toolchain{}) }

// Gates answers the Make target of every gate this area serves, which is what
// the census claims.
func Gates() []string { return metadataOnly().Gates() }

// ForkedGates returns the gates whose drivers remain scripts.
// They are two interop labs and the stress runner, each backed by test/<lab>/run.py.
// The census counts them as claimed instead of converted.
func ForkedGates() []string { return metadataOnly().ForkedGates() }

// Actions answers the command surface as data.
func Actions() leaction.List { return metadataOnly().Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return metadataOnly().Subs() }

// Answer is the `le integration` command.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return refuseAggregateRun()
	}

	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	tc, err := gotoolchain.New(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	// RunEveryAction returns the complete list instead of one problem per invocation.
	// The caller still receives the first failed gate's exit code (letools/leaction, Sweep).
	return table(tc).Sweep(args, leaction.RunEveryAction)
}

// refuseAggregateRun answers the listing and the code that says nothing ran.
//
// 2 rather than 1, which is what the Python area answered for the same mistake:
// a caller reading gate codes apart can tell "you named nothing" from "a gate
// ran and failed".
func refuseAggregateRun() (any, int) {
	listing := Actions()
	gaterun.Note("integration has no aggregate run: name the gate you want.")

	verbs := make([]string, 0, len(listing.Actions))
	for _, row := range listing.Actions {
		verbs = append(verbs, row.Verb)
	}
	var tb textbuf.Buffer
	gaterun.Note(tb.Str("  ").Join(verbs, ", ").String())
	return listing, 2
}
