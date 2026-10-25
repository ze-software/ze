// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package repobootstraps

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "Build le from what git holds, rather than from the working tree, and check it still starts, so a commit that leaves out a file le needs is caught before every session loses its launcher.",
		ShortHelp:   "check le builds from what git holds",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table (actions.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)

	// The answer carries one row per finding, so the row operators act on the
	// broken packages rather than being refused.
	leroot.RegisterShape(area, command.ShapeMap)

	// No parity.Claim applies. The retired ze-le-tracked-import-check Make
	// target was not one of the Python registry's census gates, so this port has
	// no gate row to move.
}
