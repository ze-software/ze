// Design: docs/architecture/core-design.md -- native session lifecycle command
// Related: seed.go -- session store seeding
// Related: summary.go -- session recovery snapshot
package session

import (
	"errors"
	"os"
	"time"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "session"

var actions = leaction.New(area,
	leaction.Action{
		Verb:       "seed-store",
		Why:        "seed this session's isolated ze store once from a ze_core binary",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: "binary", Value: "path"}},
		AnswerArgs: seedHere,
	},
	leaction.Action{
		Verb:       "scratch",
		Why:        "print this session's private scratch path; ensure creates it",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: "ensure"}, {Keyword: "clean"}},
		AnswerArgs: scratchHere,
	},
	leaction.Action{
		Verb:       "reap",
		Why:        "remove only session directories whose owners are provably gone",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: "dry"}},
		AnswerArgs: reapHere,
	},
	leaction.Action{
		Verb:   "end-summary",
		Why:    "write a compact recovery snapshot and preserve phase handoffs",
		Writes: true,
		Answer: endSummaryHere,
	},
)

// Actions returns the session command inventory.
func Actions() leaction.List { return actions.Actions() }

// Subs returns the command hint derived from the action table.
func Subs() string { return actions.Subs() }

// Answer dispatches one session lifecycle action.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func seedHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, code, err := seedStore(root, args["binary"], streams{Out: os.Stdout, Err: os.Stderr})
	if err != nil {
		leaction.ReportError(err)
	}
	return report, code
}

func scratchHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if args.Has("ensure") && args.Has("clean") {
		leaction.ReportError(errors.New("session scratch: ensure and clean are mutually exclusive"))
		return nil, 1
	}
	if args.Has("clean") {
		report, err := cleanScratch(root)
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		return report, 0
	}
	report, err := Scratch(root, args.Has("ensure"))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}

func reapHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	dry := reapDry(args, os.Getenv)
	report, err := Reap(root, os.Getenv("CLAUDE_CONFIG_DIR"), dry)
	if err != nil {
		// No report on a failure. A zero one reads as an answer -- "0 dead
		// session directories, 0 kept running" is what a clean tree looks
		// like, and it was printed for years beside an error that said the
		// process scan never ran. A reader cannot tell those apart, and the
		// one that means "nothing was examined" is the dangerous one.
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}

func reapDry(args leaction.Arguments, getenv func(string) string) bool {
	return args.Has("dry") || getenv("DRY") != ""
}

func endSummaryHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	paths, err := lepath.ResolveSession(root, false)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := EndSummary(root, paths, time.Now())
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	return report, 0
}
