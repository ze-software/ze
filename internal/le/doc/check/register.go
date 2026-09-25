// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Overview: actions.go -- the four documentation verifier actions.

package doccheck

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		ShortHelp: "native documentation links, aggregate verification, templ output, and retired-name checks",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  Subs,
	})
	leroot.RegisterShape(area, command.ShapeDoc)
}
