// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, and one init() are required. To add a tool to
// le, add this file and a blank import in internal/le/register.go. No other change
// is necessary.
//
// No parity claim applies. Admission is not one of the 156 gates declared by
// the Python le, so this command cannot reduce that census. It reduces the
// OTHER census by one shell file. That count decreases when
// internal/le/lejob/answer.go is deleted at the swap, not when this command appears.

package lejob

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(name, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "admit a heavy job before it runs, so the sessions sharing this machine do not oversubscribe it",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		Subs:    usageLine,
	})

	// One answer produces one document. A job report contains no rows, so row
	// operators have no data to process. They are rejected by name instead of
	// producing a plausible answer.
	leroot.RegisterShape(name, command.ShapeDoc)
}
