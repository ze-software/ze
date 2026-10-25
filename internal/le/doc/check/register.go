// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the four documentation verifier actions.

package doccheck

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "Native documentation links, aggregate verification, templ output, and retired-name checks.",
		ShortHelp:   "run the documentation gates",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeDoc)
}
