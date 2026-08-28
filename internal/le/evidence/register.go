// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package evidence

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "release-candidate evidence: run the verify gate over a clean clone of this checkout, inside a container",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// ShapeDoc, because ONE of the two answers is a document rather than a row
	// set: the listing is rows, and the run's report is one verdict carrying a
	// list of dirty paths. rowsInKeyed cannot choose between them, so the shape
	// says there are no rows and the engine refuses `| count` by name, before
	// anything is started. `| json`, `| yaml` and `| table` render both.
	leroot.RegisterShape(area, command.ShapeDoc)

	// The census counts the gate as ported from here, read out of the action
	// table rather than from a second hand-typed list.

}
