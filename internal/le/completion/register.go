// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init().

package completion

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	area := table()
	leroot.Register(name, leroot.GroupWorkflow, area.Answer, registry.Meta{
		ShortHelp: shortHelp,
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  area.Subs,
	})
	leroot.RegisterActions(name, area.Actions)
	// A script and a candidate list are each one document the shell reads.
	leroot.RegisterShape(name, command.ShapeDoc)
	// A shell runs the callback on a keystroke: the root handler runs no
	// repair and no hook before it, so stdout holds only the candidates.
	leroot.RegisterShellCallback(name)
}
