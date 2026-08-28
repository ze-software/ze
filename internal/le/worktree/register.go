// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the table this registration points at
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.
//
// This area makes no migration-census claim because the retired helper had no
// Make target and was not one of the Python registry's gates.

package worktree

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "bring a linked git worktree up to date with main, stashing and restoring its uncommitted work",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// ShapeMap: the answer is one key holding rows, so the row operators act on
	// the worktrees rather than on the report around them.
	leroot.RegisterShape(area, command.ShapeMap)
}
