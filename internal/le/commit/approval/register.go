// Design: docs/contributing/committing.md -- native approval sub-area
// Related: ../approval_actions.go -- the one action table and its handlers
package commitapproval

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/commit"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

const area = "commit approval"

func init() {
	leroot.Register(area, leroot.GroupWorkflow, commit.ApprovalAnswer, registry.Meta{
		Description: "Check captured approval verdicts against a publication parent and clean exact consumed rows after successful publication.",
		ShortHelp:   "check and clean commit approvals",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    commit.ApprovalSubs,
	})
	leroot.RegisterActions(area, commit.ApprovalActions)
	leroot.RegisterShape(area, command.ShapeMap)
}
