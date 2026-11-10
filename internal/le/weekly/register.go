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

// The event report renders itself for a bare line, as the publication does.
var _ leroot.Prose = EventReport{}

func init() {
	leroot.Register("weekly", leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "Publish the weekly update to Discord. Without confirm it answers what would be published and posts nothing, so the bare command is always safe. `report from <rev> to <rev>` answers the tooling events of the update's release range.",
		ShortHelp:   "publish the weekly update, or report a range's tooling events",
		Mode:        "offline",
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
	// The report's rows are its sections (events.go, EventReport).
	leroot.RegisterShape("weekly report", command.ShapeMap)

	// No parity.Claim: the weekly publishing action has no census gate or
	// retained Make target.
}
