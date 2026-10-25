// Design: docs/architecture/core-design.md -- le composition, one import per tool
package verify

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	verifydispatch "github.com/ze-software/ze/internal/le/verify/dispatch"
)

// summary describes the namespace's job, not one member's: its actions verify a
// change set, run the release checks, list the stages and read the reds.
const summary = "verification: the scoped check of a change set, the full release checks only Thomas starts, and the stages and reds of a run"

func init() {
	setActionRunner(verifydispatch.RunAction)
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		ShortHelp: summary,
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeDoc)

}
