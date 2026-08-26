// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in cmd/le/register.go, and nothing else.

package ste

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
	"github.com/ze-software/ze/letools/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "the repository's writing, against ASD-STE100 Simplified Technical English: " +
			"review every surface, and gate each changed file against its own HEAD version",
		Mode: "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// Every answer this command can give carries one row set, the actions, the
	// growth rows or the findings, so the row operators act on them.
	command.RegisterShape([]string{area}, command.ShapeMap)

	// The census counts these gates as ported from here, in the same init()
	// that registers the command. A claim whose command never registered is
	// red, so the count cannot fall for a tool nothing can reach.
	parity.Claim(area, Gates()...)
}
