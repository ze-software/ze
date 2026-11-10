// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package archerrorwrap

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "A changed error wrap adds only context its error lacks: a fmt.Errorf whose literal segments before %w repeat one the wrapped error already carries is refused, naming the wrap and the place that segment was first written.",
		ShortHelp:   "refuse a wrap that repeats its error's prefix",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)

	// The check answers the findings AND the scope that produced them, so a
	// reader of an empty row set still sees what was judged.
	leroot.RegisterShape(area, command.ShapeDoc)
}
