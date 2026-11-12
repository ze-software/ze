// register.go publishes the netlab check at le test netlab.
package testnetlab

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		Description: "Render and validate the netlab integration, export a self-contained lab, or prove its live protocols, routes and traffic on the Ze Docker host.",
		ShortHelp:   "render, package and run the netlab integration",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)

}
