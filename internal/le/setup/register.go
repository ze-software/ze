// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package setup

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "Install and verify every tool a Ze development or test workflow needs, bring vendor/ in step with go.mod, and provision the generated files and servers those workflows use.",
		ShortHelp:   "install and check the dev and test tools",
		// The machine is what it reads and writes, so nothing here talks to a
		// running daemon.
		Mode: "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action writes.
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)

	// Every possible answer from this command contains one row set: the actions
	// or the outcomes of a run. Therefore, the row operators process these rows
	// instead of rejecting them.
	leroot.RegisterShape(area, command.ShapeMap)

	// No parity.Claim: setup has no migration-census gate, so it declares none
	// of those rows.
}
