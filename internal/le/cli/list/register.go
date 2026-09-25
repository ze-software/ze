// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package clilist

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register("cli list", leroot.GroupReport, Answer, registry.Meta{
		ShortHelp: "every registered command, by verb, read from the live handlers and schemas",
		Mode:      "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section: registry.SectionTest,
	})

	// The answer IS the rows, so the row operators act on the commands rather
	// than being refused. Declaring it lets the engine refuse what the shape
	// cannot support BEFORE the tool loads the registry (ai/rules/cli.md).
	leroot.RegisterShape("cli list", command.ShapeMap)

	// The census counts this gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.

}
