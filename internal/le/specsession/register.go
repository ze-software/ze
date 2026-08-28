// Design: docs/architecture/core-design.md -- le's composition, one import per tool

package specsession

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(commandName, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "spec ownership, per-spec state paths, transcript model facts, and independent review artifacts",
		Mode:        "offline",
		Section:     registry.SectionTest,
		Subs:        "current | claim | release | wip | state | model | review",
	})
	leroot.RegisterShape(commandName, command.ShapeDoc)
}
