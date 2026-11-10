// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package testredwalk

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupSuite, Answer, registry.Meta{
		Description: "Run one Go break against one unit test, one .ci or one interop scenario, clean then broken, through a Go overlay that leaves the shared tree untouched, and say whether the test discriminates the break.",
		ShortHelp:   "check that a test goes red under a Go break",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	// One document: the walk's verdict, its two lines and its overlay.
	leroot.RegisterShape(area, command.ShapeDoc)
}
