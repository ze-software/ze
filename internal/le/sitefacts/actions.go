// Design: docs/architecture/core-design.md -- the repository-facts area, as one command
//
// actions.go is the command surface: `le site-facts update` writes the
// committed file, and `le site-facts check` reports what has gone stale in it.
// The pair is the shape ze-test-health-update and ze-test-health-check already
// have, which is the point -- a generated file nobody gates goes stale in
// silence.
//
// The dispatch, the listing, the help line and the two refusals live in
// internal/le/leaction, which every tool package shares. What stays here is the
// TABLE, because the table is the only part of an area that is about the
// facts the website publishes.
//
// Related: sitefacts.go -- the derivation and the file format.

package sitefacts

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as.
const area = "site-facts"

// actions is the whole command surface. Each action carries its retired target
// identity for the migration census, and leaction derives the native verb
// (internal/le/leaction, Area.verbOf).
var actions = leaction.New(area,
	leaction.Action{Verb: "update", Why: "derive the published numbers about this repository and write website/data/repo-facts.json, the file the site build reads instead of walking this tree",
		Writes: true,
		Answer: runUpdate},
	leaction.Action{Verb: "check", Why: "report which published facts the committed file and the last commit disagree about, and name the action that fixes them",
		Answer: runCheck},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Gates answers the retired Make target of each ported action, so the migration
// census counts them from the same table the dispatch reads.

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le site-facts` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// written is what the update answers: the file it wrote, what it put there, and
// the Go files the tree it read holds that the last commit does not.
type written struct {
	File        string          `json:"file"`
	Facts       map[string]fact `json:"facts"`
	Uncommitted []change        `json:"uncommitted"`
}

// runUpdate is `le site-facts update`.
func runUpdate() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return update(root)
}

// update is runUpdate with the checkout named, which is what a test can call
// against a fixture rather than against the checkout it runs in.
func update(root string) (any, int) {
	derived, err := derive(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	uncommitted, err := uncommittedGoFiles(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	warnUncommitted(uncommitted)

	path, err := write(root, derived)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	return written{File: path, Facts: derived.Facts, Uncommitted: uncommitted}, 0
}

// warnUncommitted names the Go files the counts describe and no commit holds,
// before the file is written.
//
// Several sessions share this checkout, so the tree a person regenerates in
// carries other people's work as often as their own. The warning is the half
// that stops this tool becoming the defect it exists to remove: without it, a
// number that describes somebody else's uncommitted edit lands in a committed
// file that says nothing about where it came from
// (plan/journal/concurrent-session-corruption.md).
func warnUncommitted(changes []change) {
	if len(changes) == 0 {
		return
	}

	subject := " Go files differ"
	if len(changes) == 1 {
		subject = " Go file differs"
	}

	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("warning: ").Int(int64(len(changes))).Str(subject).Str(" from the last commit, so these counts describe this tree and not a commit:").String()) //nolint:errcheck // CLI output
	for _, entry := range changes {
		tb.Reset()
		fmt.Fprintln(os.Stderr, tb.Str("  ").Str(entry.Status).Byte(' ').Str(entry.Path).String()) //nolint:errcheck // CLI output
	}
	tb.Reset()
	fmt.Fprintln(os.Stderr, tb.Str("commit them with ").Str(factsFile).Str(", or run this again once they are committed.").String()) //nolint:errcheck // CLI output
}

// runCheck is `le site-facts check`.
//
// A stale file answers 1 and a checkout that could not be judged answers 2, so
// a caller reads "the numbers are out of date" apart from "the question was
// never put" (internal/le/archmap, run).
func runCheck() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return judge(root)
}

// judge is runCheck with the checkout named, which is what a test can call
// against a fixture rather than against the checkout it runs in.
func judge(root string) (any, int) {
	report, err := check(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if len(report.Stale) > 0 {
		return report, 1
	}
	return report, 0
}
