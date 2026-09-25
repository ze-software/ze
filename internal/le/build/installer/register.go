// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the two architectures this registration exposes
//
// One package owns one command. Composition imports it from
// internal/le/register.go.

package buildinstaller

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		ShortHelp: "cross-build the installer initrd PID 1 for amd64 or arm64",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  Subs,
	})
	leroot.RegisterActions(area, Actions)

	leroot.RegisterShape(area, command.ShapeMap)
}
