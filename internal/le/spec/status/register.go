// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package specstatus

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	// The table is built once here, so a table that cannot be dispatched
	// panics at init rather than on the first call.
	actions := table()
	leroot.Register(name, leroot.GroupReport, Answer, registry.Meta{
		Description: "The spec inventory: release bucket, status and stale-skeleton flag for every spec.",
		ShortHelp:   "list every spec with its bucket and status",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section:  registry.SectionTest,
		SubsFunc: actions.Subs,
	})
	leroot.RegisterActions(name, actions.Actions)

	// The answer IS the rows, so the row operators act on the specs. Declaring
	// the shape lets the engine refuse an operator the answer cannot support
	// before the tool reads the tree (ai/rules/cli.md).
	leroot.RegisterShape(name, command.ShapeMap)

	// NO parity.Claim. The retired spec-status Make targets were not part of the
	// gate census, so this native command must not claim a row the census did not
	// hold.
}
