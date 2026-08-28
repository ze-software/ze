// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// The composition imports this package from internal/le/register.go. Its own
// init keeps command registration beside the action shape.

package htmxupgrade

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "htmx 4 upgrade findings: check the explained list against every htmx-bearing package, or report every scanner issue",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeMap)
}
