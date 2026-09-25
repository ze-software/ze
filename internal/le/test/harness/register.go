// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: harness.go -- build bin/le-test when absent, then exec it
//
// One package, one register.go, one init(). Adding this tool to le also needs
// one blank import in internal/le/register.go.

package testharness

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(Area, leroot.GroupSuite, Answer, registry.Meta{
		ShortHelp: "run the test harness bin/le-test with the trailing argv, building it first when absent",
		Mode:      "offline",
		Section:   registry.SectionTest,
	})
	leroot.RegisterShape(Area, command.ShapeDoc)

	// Every word after the name is the program's own command line, so a
	// trailing help word reaches the program and it prints its own help.
	leroot.RegisterForwarding(Area)
}
