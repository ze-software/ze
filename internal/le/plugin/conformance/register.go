// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package pluginconformance

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// checkColumns is the reading order of one row of the report's checks.
var checkColumns = command.ColumnOrder{"check", "stage", "verdict", "reason", "fix"}

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "A program started in declare mode behaves as the plugin protocol documents: it writes the Stage 1 declaration line alone, exits 0, and declares what the daemon accepts.",
		ShortHelp:   "check a program behaves as a plugin",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/le/plugin/declarations registers under it for the same
		// reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)

	// The answer is one report holding one row set, the checks, so the row
	// operators act on them rather than being refused.
	leroot.RegisterShape(area, command.ShapeMap)

	// The rows read in the order `ze show plugin conformance run` declares
	// (internal/component/plugin/conformance/register.go), so a developer and
	// an operator read one table.
	command.RegisterColumns([]string{leroot.CommandPath(area)}, checkColumns)
}
