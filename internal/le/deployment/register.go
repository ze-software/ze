// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package deployment

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "ze against a real peer daemon in a container: the protocol proofs that need another implementation to mean anything",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go, Subs).
		SubsFunc: Subs,
	})

	// ShapeDoc, because the answers are documents rather than one row set: the
	// listing is rows, while each proof report is one verdict with nested
	// evidence. rowsInKeyed cannot choose between them, so the shape says there
	// are no rows and the engine refuses `| count` by name, before a proof starts.
	// `| json`, `| yaml` and `| table` render every answer.
	leroot.RegisterShape(area, command.ShapeDoc)

	// The census derives every claimed gate from the native action table.
	parity.Claim(area, actions.Gates()...)
}
