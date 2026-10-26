// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Composition imports this package in
// the later integration cut; this file owns registration and parity claims.
package testweakened

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "Detect committed and proposed test weakenings, and record the ze-test-judge approvals that admit them, so no test loses an assertion without an independent verdict.",
		ShortHelp:   "detect test weakening and record approvals",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)

}
