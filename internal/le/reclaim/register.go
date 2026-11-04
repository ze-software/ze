// Design: docs/contributing/running-commands.md -- abandoned artifacts without verification.
// Related: ../verify/cleanup.go -- the shared reclamation producer.

package reclaim

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/verify"
)

const area = "reclaim"

var actions = leaction.New(area, leaction.Action{
	Verb:    "worktrees",
	Does:    "remove",
	Summary: "reclaim abandoned verification artifacts",
	Why:     "remove abandoned worktrees and directoryless dead-owner markers without verification; preserve live owners, changed worktrees and unknown identities",
	Writes:  true,
	Answer:  worktrees,
})

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		ShortHelp:   "reclaim abandoned worktrees without verification",
		Description: "Reclaim abandoned verification worktrees and owner markers without creating a checkout or running a suite.",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeDoc)
}

// Actions returns the registered cleanup action contract.
func Actions() leaction.List { return actions.Actions() }

// Subs returns completion and help from the action contract.
func Subs() string { return actions.Subs() }

// Answer dispatches the standalone cleanup action.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func worktrees() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	report := verify.Reclaim(ctx, root)
	return report, report.Code
}
