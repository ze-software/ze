// Design: docs/architecture/core-design.md -- verifier dependencies as one command
// Overview: verifydeps.go -- plans, execution, and structured reports
package verifydeps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

type actionRunner func(context.Context, string, string, verifyengine.Part) (Report, int)

func actionTable(run actionRunner) leaction.Area {
	return leaction.New(Area,
		leaction.Action{
			Verb:    VerbEvidenceVet,
			Summary: "vet the Linux evidence-script packages",
			Why:     "vet the Linux evidence-script package population without feature tags",
			Answer:  actionAnswer(VerbEvidenceVet, run),
		},
		leaction.Action{
			Verb:    VerbVulnerability,
			Summary: "scan the dependency graph with govulncheck",
			Why:     "scan the Linux/amd64 dependency graph with an installed govulncheck",
			Answer:  actionAnswer(VerbVulnerability, run),
		},
		leaction.Action{
			Verb:    VerbUnitCached,
			Summary: "run the cacheable non-race package pass",
			Why:     "run the full cacheable non-race package pass and bare-core compile-out checks",
			Answer:  actionAnswer(VerbUnitCached, run),
		},
		leaction.Action{
			Verb:    VerbUnitChanged,
			Summary: "test and vet the packages the change reaches",
			Why:     "test and vet the packages the change set reaches and run the bare-core compile-out check",
			Answer:  actionAnswer(VerbUnitChanged, run),
		},
		leaction.Action{
			Verb:    VerbUnitRaceChanged,
			Summary: "run the changed groups with race and cgo",
			Why: "derive changed Go groups and run them plus bare-core checks with race and cgo; " +
				"part <index> of <count> runs one piece of the packages, and the pieces together run every package; " +
				"it takes keywords, so it runs on its own and is never swept beside another verb",
			Parameters: []leaction.Parameter{
				{Keyword: "part", Kind: leaction.KindUint, Help: "which piece of the groups this run judges, read with of", Value: "index", Requirement: leaction.Optional},
				{Keyword: "of", Kind: leaction.KindUint, Help: "how many pieces the groups are split into, read with part", Value: "count", Requirement: leaction.Optional},
			},
			AnswerArgs: func(args leaction.Arguments) (any, int) {
				part, err := partFrom(args)
				if err != nil {
					leaction.ReportError(err)
					return nil, 2
				}
				return answerPart(VerbUnitRaceChanged, part, run)
			},
		},
		leaction.Action{
			Verb:    VerbAlloc,
			Summary: "enforce hot-path allocation ceilings",
			Why:     "measure registered hot-path benchmarks and enforce their allocs/op ceilings",
			Answer:  actionAnswer(VerbAlloc, run),
		},
	)
}

func actionAnswer(verb string, run actionRunner) func() (any, int) {
	return func() (any, int) { return answerPart(verb, verifyengine.Uncut(), run) }
}

func answerPart(verb string, part verifyengine.Part, run actionRunner) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, root, verb, part)
}

// partFrom answers the piece of the race population an invocation runs.
//
// Naming neither keyword runs part 1 of 1, the whole population: the uncut
// population IS one piece, so no number stands for "the caller said nothing".
// Naming one keyword without the other is refused, because "part 2" alone
// cannot say how many pieces the packages were dealt into, and a guess would
// silently run the wrong subset.
func partFrom(args leaction.Arguments) (verifyengine.Part, error) {
	hasIndex := args.Has("part")
	hasCount := args.Has("of")
	if !hasIndex && !hasCount {
		return verifyengine.Uncut(), nil
	}
	if hasIndex != hasCount {
		return verifyengine.Part{}, errors.New(
			"a cut race pass needs both keywords: le verify deps unit-race-changed part <index> of <count>")
	}
	index, err := partNumber("part", args.One("part"))
	if err != nil {
		return verifyengine.Part{}, err
	}
	count, err := partNumber("of", args.One("of"))
	if err != nil {
		return verifyengine.Part{}, err
	}
	part := verifyengine.Part{Index: index, Of: count}
	if err := part.Valid(); err != nil {
		return verifyengine.Part{}, fmt.Errorf("%w: le verify deps unit-race-changed part <index> of <count>", err)
	}
	return part, nil
}

// partNumber reads one keyword's value as a counting number.
func partNumber(keyword, declared string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(declared))
	if err != nil {
		return 0, partNumberRefusal(keyword, declared)
	}
	if value < 1 {
		return 0, partNumberRefusal(keyword, declared)
	}
	return value, nil
}

// partNumberRefusal names a keyword whose value is not a counting number.
func partNumberRefusal(keyword, declared string) error {
	return fmt.Errorf("%s needs a whole number of 1 or more, got %q: le verify deps unit-race-changed part <index> of <count>",
		keyword, declared)
}

func metadataOnly() leaction.Area {
	return actionTable(func(context.Context, string, string, verifyengine.Part) (Report, int) {
		return Report{}, 0
	})
}

// Actions returns all five gateless verifier verbs in execution-table order.
func Actions() leaction.List { return metadataOnly().Actions() }

// Subs returns the one-line action hint for command help.
func Subs() string { return metadataOnly().Subs() }

// Answer is the `le verify deps` command.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return Actions(), 0
	}
	return actionTable(Run).AnswerOrSweep(args, leaction.RunEveryAction)
}
