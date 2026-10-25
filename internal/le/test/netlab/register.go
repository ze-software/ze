// register.go publishes the netlab check at le test netlab.
package testnetlab

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "The netlab daemon integration: render contrib/netlab with the installed netlab, compare every node with its golden file, or rewrite the goldens from a real render.",
		ShortHelp:   "render and validate the netlab integration",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)

}
