// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package webassets

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "the per-page web asset sets derived from the markup each page renders: check them, write them, or print them",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// The answers hold several row sets between them -- the actions, the
	// derived files, and the pages, which are one map of page to asset list --
	// so ShapeDoc says the answer is a DOCUMENT rather than one table. `| json`,
	// `| yaml` and `| table` still render it: they are anyShape operators
	// (internal/component/command/pipe_catalog.go). `| count` is refused by
	// name, which is right for an answer holding no single row set to count.
	leroot.RegisterShape(area, command.ShapeDoc)

	// The census counts the gate as ported from here, read out of the action
	// table rather than from a second hand-typed list.
	parity.Claim(area, actions.Gates()...)
}
