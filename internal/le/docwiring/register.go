// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package docwiring

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
)

func init() {
	leroot.Register(name, Answer, registry.Meta{
		Description: "the changed-file wiring, documentation, command and inventory gate",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// The keywords this command takes, so help names them where a developer
		// reads what to type.
		SubsFunc: Subs,
	})

	// The answer carries several lists -- the changed files, the selected
	// targets, the per-check results and the failure groups -- so no one key
	// holds the rows. A document shape refuses the row operators by name rather
	// than letting one of the lists stand in for the answer.
	leroot.RegisterShape(name, command.ShapeDoc)

	// The census counts the gate as converted from here, in the same init()
	// that registers the command. Every selected repository check is a linked
	// Go callback in delegate.go.
	parity.Claim(name, gateTarget)
}
