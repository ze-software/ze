// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package sitefacts

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "the numbers the website publishes about this repository: derive them into website/data/repo-facts.json, or check what has gone stale in it",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, actions).
		SubsFunc: Subs,
	})

	// Every answer this command can give carries one row set -- the actions, or
	// the facts the update wrote -- so the row operators act on them rather
	// than being refused. Declaring the shape lets the engine refuse what the
	// shape cannot support BEFORE the tool walks the tree.
	leroot.RegisterShape(area, command.ShapeMap)

	// Both gates are claimed from the same table the dispatch reads, in the
	// init() that registers the command. A gate cannot be counted as ported by
	// a command nothing can reach.

}
