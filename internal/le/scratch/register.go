// Design: docs/architecture/core-design.md -- le composition and parity claims
package scratch

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "keep disposable scratch and durable caches outside the checkout without overwriting existing work",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeMap)
	parity.Claim(area, actions.Gates()...)
}
