// Design: docs/contributing/error-inventory.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package archerrorinventory

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "Every place a message reaches a user, with its surface, owner, door area, status against ai/rules/user-facing-errors.md and a pre-rank, derived from the code; and a check that refuses an added plain message at a surface writer no other gate judges.",
		ShortHelp:   "list user-visible message sites and their fix status",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)

	// Every answer carries the rows AND the counts that summarize them, so a
	// reader of an empty row set still sees every surface counted.
	leroot.RegisterShape(area, command.ShapeDoc)
}
