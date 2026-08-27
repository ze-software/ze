// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// The composition imports this package from internal/le/register.go. Its own
// init keeps the command registration and both parity claims inseparable.

package htmxupgrade

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "htmx 4 upgrade findings: check the explained list against every htmx-bearing package, or report every scanner issue",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeMap)
	parity.Claim(area, actions.Gates()...)
}
