// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package pluginregistrationsites

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "Every call to a registering function of the plugin registry sits in package initialization of a register file, so no registration can run after the registry seals.",
		ShortHelp:   "check plugin registrations sit in package initialization",
		Mode:        "offline",
		// SectionTest is where ze files a tool rather than a product command.
		Section:  registry.SectionTest,
		SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeDoc)
}
