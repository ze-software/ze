// Design: docs/architecture/core-design.md -- le composition, one import per tool
package verify

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/verifydispatch"
)

func init() {
	setActionRunner(verifydispatch.RunAction)
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		Description: "the full native verification population against a fixed commit in a detached worktree",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeDoc)

}
