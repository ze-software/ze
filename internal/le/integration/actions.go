// Design: docs/architecture/core-design.md -- the integration area, as one command
// Overview: gates.go -- the table every row here is derived from
//
// actions.go defines this area's action table.
// internal/le/leaction supplies the shared dispatch, listing, help line, and two refusals.
// This file specifies the area's different behavior.
//
// This area has no aggregate run, and bare `le integration` refuses one.
// Other areas run their complete set when given no gate name.
// This set needs hours of Docker, root, and QEMU work.
// No Make target ever requested all of it, so the shortest spelling must not do so.
// `le integration list` still lists the gate names and runs no gates.

package integration

import (
	"context"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	interopbgp "github.com/ze-software/ze/internal/le/interoplab/bgp"
	interopipsec "github.com/ze-software/ze/internal/le/interoplab/ipsec"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// table builds this invocation's action table from the gate table. The
// toolchain is resolved once and shared, so twelve gates named together read
// go.mod and feature-gates.txt once between them.
func table(tc gotoolchain.Toolchain) leaction.Area {
	gates := Table()
	rows := make([]leaction.Action, 0, len(gates))
	for _, gate := range gates {
		row := leaction.Action{Gate: gate.Name, Why: gate.Why}
		if gate.Native != nil {
			row.Answer = nativeRunner(tc.Root, gate.Native)
		} else {
			row.Forks = gate.Argv()
			row.Answer = commandRunner(tc, gate)
		}
		rows = append(rows, row)
	}
	return leaction.New(Area, rows...)
}

func nativeRunner(
	root string,
	run func(context.Context, string) (any, int),
) func() (any, int) {
	return func() (any, int) {
		return run(context.Background(), root)
	}
}

func commandRunner(tc gotoolchain.Toolchain, gate Gate) func() (any, int) {
	return func() (any, int) {
		return gaterun.Run(gate.Name, gate.Argv(), tc.Root,
			tc.Environment(gotoolchain.EnvOptions{CGO: gate.NeedsCgo()}))
	}
}

func generalInteropOptions() interopbgp.Options {
	return interopbgp.Options{Scenario: env.Get("interop.scenario")}
}

func runGeneralInterop(ctx context.Context, root string) (any, int) {
	report := interopbgp.RunAt(ctx, root, generalInteropOptions())
	return report, report.Code
}

func runIPsecInterop(ctx context.Context, root string) (any, int) {
	return interopipsec.RunAt(ctx, root)
}

func runStressBirdGate(_ context.Context, _ string) (any, int) {
	return RunStressBird()
}

// metadataOnly supplies names but does not resolve a checkout.
// The listing, help line, and census need only the gate names.
// Thus, `le --help` does not read go.mod.
func metadataOnly() leaction.Area { return table(gotoolchain.Toolchain{}) }

// Gates answers the Make target of every gate this area serves, which is what
// the census claims.
func Gates() []string { return metadataOnly().Gates() }

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
	// The caller still receives the first failed gate's exit code (internal/le/leaction, Sweep).
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
