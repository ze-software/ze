// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the table this registration points at
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.
//
// No parity claim exists. Neither ported script is among the 156 Python gates
// that the Python le declares. Thus, these verbs cannot leave that census. They
// remove two shell files from the OTHER census. That count falls when the swap
// deletes the scripts, not when these commands appear.

package changed

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupReport, Answer, registry.Meta{
		Description: "what this checkout changed: the test groups it touches, and the packages a scoped verify must cover",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which verbs exist (actions.go, Subs).
		SubsFunc: Subs,
	})

	// ShapeDoc, because the answers are not one row set. The listing is rows,
	// a group selection is two lists, and a scope answer is a list plus the
	// verdict that says whether it widened. rowsInKeyed cannot choose among
	// them, so the shape says there are no rows and the engine refuses
	// `| count` by name rather than counting something plausible. `| json`,
	// `| yaml` and `| table` render every one of them.
	leroot.RegisterShape(area, command.ShapeDoc)
}
