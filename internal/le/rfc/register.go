// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package rfc

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "RFC conformance: bind every MUST-level requirement of an enrolled RFC " +
			"to the tests that enforce it, and bound what the summaries missed",
		Mode: "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go).
		SubsFunc: Subs,
	})

	// Every answer this command can give carries one row set, so the row
	// operators act on it.
	leroot.RegisterShape(area, command.ShapeMap)

	// The census counts these gates as ported from here, in the same init()
	// that registers the command. A claim whose command never registered is
	// red, so the count cannot fall for a tool nothing can reach.

}
