// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the closed action and argument grammar
//
// One package, one register.go, one init(). Adding this tool to le also needs
// one blank import in internal/le/register.go.

package stressrepro

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupSuite, Answer, registry.Meta{
		Description: "reproduce load-dependent functional-test failures under bounded CPU, GC, and process pressure",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeDoc)
}
