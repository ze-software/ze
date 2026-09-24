// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package archenumeration

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		ShortHelp: "no Go literal, const block or switch enumerates what a live registry already holds, so a set has one declaration and cannot drift from a copy of itself",
		Mode:      "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)

	// Every answer carries the findings AND the scope that produced them, so it
	// is a document rather than a bare row set: `| json`, `| yaml` and `| table`
	// each render both, and a consumer reading an empty row set can still see
	// what was judged.
	leroot.RegisterShape(area, command.ShapeDoc)
}
