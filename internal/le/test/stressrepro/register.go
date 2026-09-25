// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the closed action and argument grammar
//
// One package, one register.go, one init(). Adding this tool to le also needs
// one blank import in internal/le/register.go.

package teststressrepro

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	// The run holds one job slot for all its children (admitRun), so the
	// pretool hook reads it as heavy.
	leroot.RegisterAdmitted(area)
	leroot.Register(area, leroot.GroupSuite, Answer, registry.Meta{
		ShortHelp: "reproduce load-dependent functional-test failures under bounded CPU, GC, and process pressure",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  Subs,
	})
	leroot.RegisterShape(area, command.ShapeDoc)
}
