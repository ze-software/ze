// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in cmd/le/register.go, and nothing else.

package portdefaults

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
	"github.com/ze-software/ze/letools/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "the Go listener-default table and the YANG refine port defaults still agree, service by service",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// Every answer this command can give carries one row set -- the actions, the
	// drifts, or the selftest cases -- so the row operators act on them rather
	// than being refused.
	command.RegisterShape([]string{area}, command.ShapeMap)

	// The census counts both gates as ported from here, in the same init() that
	// registers the command.
	parity.Claim(area, actions.Gates()...)
}
