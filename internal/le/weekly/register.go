// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package weekly

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register("weekly", leroot.GroupWorkflow, Answer, registry.Meta{
		ShortHelp: "publish the weekly update to Discord; the bare command shows what would be sent",
		Mode:      "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about what the update takes (answer.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions("weekly", Actions)

	// The answer carries one row per post, so the row operators act on the
	// posts rather than being refused. Declaring it lets the engine refuse what
	// the shape cannot support BEFORE a publication starts (ai/rules/cli.md).
	leroot.RegisterShape("weekly", command.ShapeMap)

	// No parity.Claim: the weekly publishing action has no census gate or
	// retained Make target.
}
