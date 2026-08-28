// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package weekly

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register("weekly", Answer, registry.Meta{
		Description: "publish the weekly update to Discord; the bare command shows what would be sent",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		Subs:    "source, dir, channel, confirm, force, resume-from, date-stamp",
	})

	// The answer carries one row per post, so the row operators act on the
	// posts rather than being refused. Declaring it lets the engine refuse what
	// the shape cannot support BEFORE a publication starts (ai/rules/cli.md).
	leroot.RegisterShape("weekly", command.ShapeMap)

	// No parity.Claim: the weekly publishing action has no census gate or
	// retained Make target.
}
