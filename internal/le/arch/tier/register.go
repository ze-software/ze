// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package archtier

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		ShortHelp: "module-tier placement and plugin import ownership: core import direction, compile-out dependencies, and production imports across plugin ownership boundaries",
		Mode:      "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)

	// Every answer contains one row set: checks, selftest cases, audit rows, or
	// baseline counts. Thus, row operators apply instead of refusing the answer.
	leroot.RegisterShape(area, command.ShapeMap)

	// The census counts both gates as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.

}
