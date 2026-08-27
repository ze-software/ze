// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package pylint

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "lint and type-check the Python half of the tree",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action writes.
		SubsFunc: Subs,
	})

	// Each answer has one row set: either actions or run stages.
	// The row operators therefore act on that set instead of being refused.
	leroot.RegisterShape(area, command.ShapeMap)

	// No parity.Claim: no Make target runs this tool, so it declares none of
	// the 156 gates the census counts.
}
