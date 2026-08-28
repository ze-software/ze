// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package commandownership

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(name, leroot.GroupGate, Answer, registry.Meta{
		Description: "each command is owned by exactly one plugin or component: owners are cmd/ze-free, root handlers are internal, and every central root states why it has no owner",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
	})

	// The answer IS the rows, so the row operators act on the violations rather
	// than being refused (internal/component/command/answer_shape.go).
	leroot.RegisterShape(name, command.ShapeMap)

	// The census counts this gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.

}
