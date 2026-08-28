// Design: docs/architecture/core-design.md -- le composition and parity claims
package scratch

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "keep disposable scratch and durable caches outside the checkout without overwriting existing work",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeMap)

}
