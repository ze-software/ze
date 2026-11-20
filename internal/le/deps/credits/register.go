// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Related: gate.go -- the change-set check registered here
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package credits

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "The credits of the third-party works the shipped ze carries: check the declaration and the generated data against the tree, or regenerate the data the binary embeds.",
		ShortHelp:   "check or regenerate the third-party credits",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)

	// The commit gate: `./le commit create` runs it over what the commit adds.
	changecheck.Register(changecheck.Check{Name: area, Judge: judgeCommit})
}
