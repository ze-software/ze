// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package protocolskeleton

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupReport, Answer, registry.Meta{
		Description: "which protocol implementations are still a skeleton rather than a daemon, classified against ai/rules/protocol.md",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about what this command holds (actions.go, Subs).
		SubsFunc: Subs,
	})

	// Every answer this command can give carries one row set -- the actions,
	// the protocols, or the selftest cases -- so the row operators act on them.
	leroot.RegisterShape(area, command.ShapeMap)

	// The census counts the gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.

}
