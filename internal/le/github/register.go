// Design: docs/contributing/github-integration.md -- registered read-only GitHub reporting.

package github

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	for _, meta := range []diagnostic.CodeMeta{
		{Code: codeInput, Title: "GitHub evidence selector is invalid", Description: "Select exactly one full commit ID or positive workflow run ID; machine observations may be qualified by a revision.", Examples: []string{"./le help github report"}},
		{Code: codeAPI, Title: "GitHub API evidence is incomplete", Description: "A bounded API read, identity check or state classification failed. Partial rows are observations, not a passing certificate.", Examples: []string{"./le help github report"}},
		{Code: codeMachine, Title: "Local committed-revision handoff is not ready", Description: "Current local source identity, dirty inputs, liveness or capacity prevents a ready handoff. No command or remote work was executed.", Examples: []string{"./le github report machine"}},
	} {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: GitHub report diagnostic was registered twice")
		}
	}
	leroot.Register(area, leroot.GroupReport, Answer, registry.Meta{
		ShortHelp:   "report GitHub evidence for one revision or run",
		Description: "Read the fixed Ze repository's GitHub Actions results for one exact revision or run. Preserve job identities, incomplete results and the distinction between GitHub conclusions and native verification evidence.",
		Mode:        "offline", Section: registry.SectionTest, SubsFunc: Subs,
	})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeDoc)
}
