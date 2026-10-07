// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package feature

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		ShortHelp: "feature maturity: check each features/<id>.md declaration against its evidence, " +
			"and report every feature's level, ceiling and the gap to the next level",
		Mode: "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing (actions.go).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)
	// Both answers carry row sets, so the row operators act on them.
	leroot.RegisterShape(area, command.ShapeMap)
}
