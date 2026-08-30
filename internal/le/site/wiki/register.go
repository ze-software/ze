// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.
package sitewiki

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "the wiki page index the website references: derive it into website/data/wiki.json from a wiki checkout, or check what has gone stale in it",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command,
		// which is where internal/le/site/facts registers for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, actions).
		SubsFunc: Subs,
	})
	leroot.RegisterShape(area, command.ShapeMap)
}
