// Design: docs/architecture/testing/verify-freshness-scope.md -- detached-worktree verification command
package verifyworktree

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
	"github.com/ze-software/ze/internal/le/verify"
)

const area = "verify"

var configured struct {
	sync.RWMutex
	runner verify.GateRunner
}

var actions = leaction.New(area, leaction.Action{
	Gate:   gateName,
	Why:    "run the full pre-commit population against a fixed commit in a fresh detached worktree; commit defaults to HEAD and keep leaves the worktree",
	Writes: false,
	Parameters: []leaction.Parameter{
		{Keyword: "commit", Value: "revision"},
		{Keyword: "keep"},
	},
	AnswerArgs: runHere,
})

// SetGateRunner supplies the in-process native gate dispatcher. Composition
// calls this before invoking the command. Until then the gate fails closed on
// the first unregistered stage.
func SetGateRunner(runner verify.GateRunner) {
	configured.Lock()
	configured.runner = runner
	configured.Unlock()
}

func gateRunner() verify.GateRunner {
	configured.RLock()
	defer configured.RUnlock()
	return configured.runner
}

// Actions returns the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs returns the one-line command help derived from the action table.
func Subs() string { return actions.Subs() }

// Gates returns the historical gate claimed by this native command.
func Gates() []string { return actions.Gates() }

// Answer is the `le verify` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	options := optionsFrom(args, os.Getenv)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	report := Run(ctx, root, options, gateRunner())
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

// Text renders lifecycle diagnostics in the same order as the Python producer.
func (r Report) Text() string {
	if len(r.Diagnostics) == 0 {
		return ""
	}
	var text textbuf.Buffer
	return text.Join(r.Diagnostics, "\n").Byte('\n').String()
}
