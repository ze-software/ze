// Design: docs/architecture/testing/verify-freshness-scope.md -- detached-worktree verification command
package verify

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

const area = "verify"

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
			{Keyword: "commit", Value: "revision"},
			{Keyword: "keep"},
		},
		AnswerArgs: runHere,
	},
	leaction.Action{
		Verb:       "current",
		Why:        "verify the current shared checkout; mode defaults to full",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: "mode", Value: "full|changed"}},
		AnswerArgs: currentHere,
	},
	leaction.Action{
		Verb:       "list",
		Why:        "list the native current-checkout stages; mode defaults to full",
		Parameters: []leaction.Parameter{{Keyword: "mode", Value: "full|changed"}},
		AnswerArgs: listHere,
	},
)

// setActionRunner supplies the in-process native action dispatcher. Composition
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
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	options := optionsFrom(args, os.Getenv)
	ctx, stop := signalContext()
	defer stop()
	report := Run(ctx, root, options, actionRunner())
	return report, report.Code
}

func optionsFrom(args leaction.Arguments, getenv func(string) string) Options {
	commit := strings.TrimSpace(getenv("COMMIT"))
	if value, ok := args["commit"]; ok {
		commit = strings.TrimSpace(value)
	}
	keep := strings.TrimSpace(getenv("KEEP")) != ""
	if args.Has("keep") {
		keep = true
	}
	return Options{Commit: commit, Keep: keep}
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
