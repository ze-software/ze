// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package inventory

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register("inventory", leroot.GroupReport, Answer, registry.Meta{
		Description: "what ze is made of: plugins, families, YANG modules, RPCs, tests and package sizes",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
	})

	// The answer is ONE document holding several row sets -- plugins, families,
	// RPCs, tests -- so no row operator can act on it without being told which
	// set it means. ShapeDoc says so, and the engine then refuses `| count` by
	// name instead of counting whichever set it guessed at (ai/rules/cli.md).
	// `| json`, `| yaml` and `| table` act on any shape and still render it.
	leroot.RegisterShape("inventory", command.ShapeDoc)

	// The census counts this gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.

}
