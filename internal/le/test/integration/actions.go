// Design: docs/architecture/core-design.md -- the integration area, as one command
// Overview: gates.go -- the table every row here is derived from
//
// actions.go defines this area's action table.
// internal/le/le/action supplies the shared dispatch, listing, help line, and two refusals.
// This file specifies the area's different behavior.
//
// This area has no aggregate run, and bare `le test integration` refuses one.
// Other areas run their complete set when given no gate name.
// This set needs hours of Docker, root, and QEMU work.
// No Make target ever requested all of it, so the shortest spelling must not do so.
// `le test integration list` still lists the gate names and runs no gates.

package testintegration

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	interopbgp "github.com/ze-software/ze/internal/le/interoplab/bgp"
	interopipsec "github.com/ze-software/ze/internal/le/interoplab/ipsec"
	interopradius "github.com/ze-software/ze/internal/le/interoplab/radius"
	interopradiussubscriber "github.com/ze-software/ze/internal/le/interoplab/radiussubscriber"
	interoprsvpte "github.com/ze-software/ze/internal/le/interoplab/rsvpte"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// table builds this invocation's action table. The toolchain is resolved once
// and shared, so actions named together read go.mod and feature-gates.txt once.
func table(tc gotoolchain.Toolchain) leaction.Area {
	declared := Table()
	rows := make([]leaction.Action, 0, len(declared))
	for _, action := range declared {
		row := leaction.Action{Verb: action.Verb, Does: action.Does, Summary: action.Summary, Why: action.Why}
		switch {
		case action.NativeArgs != nil:
			row.Parameters = action.Parameters
			row.AnswerArgs = nativeArgsRunner(tc.Root, action.NativeArgs)
		case action.Native != nil:
			row.Answer = nativeRunner(tc.Root, action.Native)
		default:
			row.Answer = commandRunner(tc, action)
		}
		rows = append(rows, row)
	}
	return leaction.New(Area, rows...)
}

// nativeRunner roots a native verb on a context SIGINT and SIGTERM cancel, so
// a Ctrl-C or a `./le job` stop runs the teardown the lab already defers rather
// than Go's default action ending the process with its containers left behind
// (docs/architecture/testing/runner-architecture.md, "How a suite's child
// processes end").
func nativeRunner(
	root string,
	run func(context.Context, string) (any, int),
) func() (any, int) {
	return func() (any, int) {
		ctx, stop := signalContext()
		defer stop()
		return run(ctx, root)
	}
}

// nativeArgsRunner is nativeRunner for a verb with a keyword grammar.
func nativeArgsRunner(
	root string,
	run func(context.Context, string, leaction.Arguments) (any, int),
) func(leaction.Arguments) (any, int) {
	return func(args leaction.Arguments) (any, int) {
		ctx, stop := signalContext()
		defer stop()
		return run(ctx, root, args)
	}
}

// signalContext is the entry-root shape every lab driver uses (guestContext
// in internal/le/test/qemu): canceled by SIGINT or SIGTERM. The caller MUST
// call the returned stop when the verb ends.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func commandRunner(tc gotoolchain.Toolchain, action Action) func() (any, int) {
	return func() (any, int) {
		return gaterun.Run(action.Verb, action.Argv(), tc.Root,
			tc.Environment(gotoolchain.EnvOptions{Test: true, CGO: action.needsCgo()}))
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
	return interopipsec.RunAt(ctx, root, "")
}

func runRADIUSInterop(ctx context.Context, root string) (any, int) {
	return interopradius.RunAt(ctx, root, "")
}

func runRADIUSSubscriberInterop(ctx context.Context, root string) (any, int) {
	report := interopradiussubscriber.RunAt(ctx, root, "")
	return report, report.Code
}

func runRSVPTEInterop(ctx context.Context, root string) (any, int) {
	report := interoprsvpte.RunAt(ctx, root, "")
	return report, report.Code
}

func runStressBirdGate(_ context.Context, _ string) (any, int) {
	return runStressBirdAction()
}

func runStressGate(ctx context.Context, root string) (any, int) {
	report, code := runStressAction(ctx, root, stressOptions{
		Scenario: env.Get("stress.scenario"), Prefixes: env.GetInt("stress.prefixes", 0),
	})
	return report, code
}

// metadataOnly supplies names but does not resolve a checkout.
// The listing, help line, and census need only the gate names.
// Thus, `le --help` does not read go.mod.
func metadataOnly() leaction.Area { return table(gotoolchain.Toolchain{}) }

// Gates answers the retired Make target of every ported gate this area serves,
// which is what the migration census claims.

// Actions answers the command surface as data.
func Actions() leaction.List { return metadataOnly().Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return metadataOnly().Subs() }

// Answer is the `le test integration` command.
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
	// The caller still receives the first failed gate's exit code (internal/le/le/action, Sweep).
	return table(tc).AnswerOrSweep(args, leaction.RunEveryAction)
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
