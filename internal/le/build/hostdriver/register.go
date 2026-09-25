// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: hostdriver.go -- the build this registration exposes
//
// One package owns one command. Composition imports it from
// internal/le/register.go.

package buildhostdriver

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(name, leroot.GroupWorkflow, Answer, registry.Meta{
		ShortHelp: "build ze-host, the `ze appliance ...` driver that runs on the build machine, at the checkout root",
		Mode:      "offline",
		Section:   registry.SectionTest,
	})

	// The answer is one build report, so the map shape renders its fields.
	leroot.RegisterShape(name, command.ShapeMap)
}
