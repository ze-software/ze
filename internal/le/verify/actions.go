// Design: docs/architecture/testing/verify-freshness-scope.md -- detached-worktree verification command
package verify

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

const area = "verify"

// commitKeyword is the parameter keyword that names the revision a worktree
// verification judges. The action declares it, the action reads it, and
// worktreeArgv writes it into the work identity a second session attaches to,
// so one spelling serves all three.
const commitKeyword = "commit"

var configured struct {
	sync.RWMutex
	runner verifyengine.ActionRunner
}
var actions = leaction.New(area,
	leaction.Action{
		Verb:   actionName,
		Why:    "run the full native verification population against a fixed commit in a fresh detached worktree; commit defaults to HEAD and keep leaves the worktree",
		Writes: false,
		Parameters: []leaction.Parameter{
			{Keyword: commitKeyword, Value: "revision", Requirement: leaction.Optional},
			{Keyword: "keep"},
			{Keyword: keywordFirstRed},
			{Keyword: keywordPart, Value: "index", Requirement: leaction.Optional},
			{Keyword: keywordOf, Value: "pieces", Requirement: leaction.Optional},
		},
		AnswerArgs: runHere,
	},
	leaction.Action{
		Verb:       "current",
		Why:        "verify the tree: full (the default) runs the full population in a fresh detached worktree at HEAD, as worktree does; fast runs the changed-files population in place over the shared checkout",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: keywordFull}, {Keyword: keywordFast}, {Keyword: keywordFirstRed}},
		AnswerArgs: currentHere,
	},
	leaction.Action{
		Verb:       "reds",
		Why:        "read the stage logs a verification run has written so far and answer whether any red names one file; it never says a run passed",
		Writes:     false,
		Parameters: []leaction.Parameter{{Keyword: "file", Value: "path", Requirement: leaction.Required}},
		AnswerArgs: redsHere,
	},
	leaction.Action{
		Verb:       "list",
		Why:        "list the native stages of the full population (the default) or the fast one",
		Parameters: []leaction.Parameter{{Keyword: keywordFull}, {Keyword: keywordFast}},
		AnswerArgs: listHere,
	},
)

// setActionRunner supplies the native action dispatcher. Composition
// calls this before invoking the command. Until then verification fails closed
// on the first unregistered stage.
func setActionRunner(runner verifyengine.ActionRunner) {
	configured.Lock()
	configured.runner = runner
	configured.Unlock()
}

func actionRunner() verifyengine.ActionRunner {
	configured.RLock()
	defer configured.RUnlock()
	return configured.runner
}

// Actions returns the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs returns the one-line command help derived from the action table.
func Subs() string { return actions.Subs() }

// Answer is the `le verify` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runHere(args leaction.Arguments) (any, int) {
	options, err := optionsFrom(args, os.Getenv)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return runWorktree(options)
}

// runWorktree runs the full population in a fresh detached worktree. Both
// `le verify worktree` and a bare `le verify current` answer through it, so the
// worktree lifecycle has one entry.
func runWorktree(options Options) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if err := options.HostPreflight(); err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	ctx, stop := signalContext()
	defer stop()
	report := Run(ctx, root, options, actionRunner())
	return report, report.Code
}

// optionsFrom reads a `le verify worktree` line. A piece number that is not a
// positive integer, a piece without its count, and first-red with a piece are
// refused here, before any work, with the reason and the spellings that work.
func optionsFrom(args leaction.Arguments, getenv func(string) string) (Options, error) {
	commit := strings.TrimSpace(getenv("COMMIT"))
	if args.Has(commitKeyword) {
		commit = strings.TrimSpace(args.One(commitKeyword))
	}
	keep := strings.TrimSpace(getenv("KEEP")) != ""
	if args.Has("keep") {
		keep = true
	}
	options := Options{Commit: commit, Keep: keep, FirstRed: args.Has(keywordFirstRed)}
	var err error
	if options.Part, err = pieceNumber(args, keywordPart); err != nil {
		return Options{}, err
	}
	if options.Parts, err = pieceNumber(args, keywordOf); err != nil {
		return Options{}, err
	}
	if _, err := options.part(); err != nil {
		return Options{}, err
	}
	return options, nil
}

// pieceNumber reads the value of a part or of keyword, zero when it is absent.
func pieceNumber(args leaction.Arguments, keyword string) (int, error) {
	if !args.Has(keyword) {
		return 0, nil
	}
	value := strings.TrimSpace(args.One(keyword))
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, notAPieceNumber(keyword, value)
	}
	if number < 1 {
		return 0, notAPieceNumber(keyword, value)
	}
	return number, nil
}

func notAPieceNumber(keyword, value string) error {
	var text textbuf.Buffer
	return errors.New(text.Str("le verify worktree: ").Str(keyword).Str(" takes a positive number, got ").
		Quoted(value).Str("; as in: le verify worktree part 1 of 6").String())
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// Text renders the lifecycle diagnostics in the order the run produced them,
// which ends with the verdict line: it is written by the last deferred call, so
// every branch that can move Report.Code has already run when it renders.
func (r Report) Text() string {
	if len(r.Diagnostics) == 0 {
		return ""
	}
	var text textbuf.Buffer
	return text.Join(r.Diagnostics, "\n").Byte('\n').String()
}
