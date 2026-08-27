// Design: docs/architecture/core-design.md -- verifier dependencies as one command
// Overview: verifydeps.go -- plans, execution, and structured reports
package verifydeps

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

type actionRunner func(context.Context, string, string) (Report, int)

func actionTable(run actionRunner) leaction.Area {
	return leaction.New(Area,
		leaction.Action{
			Verb:   VerbEvidenceVet,
			Why:    "vet the Linux evidence-script package population without feature tags",
			Answer: actionAnswer(VerbEvidenceVet, run),
		},
		leaction.Action{
			Verb:   VerbVulnerability,
			Why:    "scan the Linux/amd64 dependency graph with an installed govulncheck",
			Answer: actionAnswer(VerbVulnerability, run),
		},
		leaction.Action{
			Verb:   VerbUnitCached,
			Why:    "run the full cacheable non-race package pass and bare-core compile-out checks",
			Answer: actionAnswer(VerbUnitCached, run),
		},
		leaction.Action{
			Verb:   VerbUnitRaceChanged,
			Why:    "derive changed Go groups and run them plus bare-core checks with race and cgo",
			Answer: actionAnswer(VerbUnitRaceChanged, run),
		},
		leaction.Action{
			Verb:   VerbAlloc,
			Why:    "measure registered hot-path benchmarks and enforce their allocs/op ceilings",
			Answer: actionAnswer(VerbAlloc, run),
		},
	)
}

func actionAnswer(verb string, run actionRunner) func() (any, int) {
	return func() (any, int) {
		root, err := lepath.Root()
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return run(ctx, root, verb)
	}
}

func metadataOnly() leaction.Area {
	return actionTable(func(context.Context, string, string) (Report, int) {
		return Report{}, 0
	})
}

// Actions returns all five gateless verifier verbs in execution-table order.
func Actions() leaction.List { return metadataOnly().Actions() }

// Subs returns the one-line action hint for command help.
func Subs() string { return metadataOnly().Subs() }

// Answer is the `le verify-deps` command.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return Actions(), 0
	}
	return actionTable(Run).Sweep(args, leaction.RunEveryAction)
}
