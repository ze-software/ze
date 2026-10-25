// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package goextract

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register("go extract", leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "Move named declarations from one Go file to another with their comments and formatting intact, so a file split is one mechanical step.",
		ShortHelp:   "move Go declarations between files",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about what the move takes (answer.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions("go extract", Actions)

	// The answer carries one row per declaration that moved, so the row
	// operators act on those rather than being refused. Declaring it lets the
	// engine refuse what the shape cannot support BEFORE a file is rewritten
	// (ai/rules/cli.md).
	leroot.RegisterShape("go extract", command.ShapeMap)

	// No parity.Claim: this developer action has no census gate or retained
	// Make target.
}
