// Design: docs/architecture/core-design.md -- hook selftests and runtime are one le action area
// Overview: hookcheck.go -- native selftest implementation
package hookcheck

import (
	"os"

	"github.com/ze-software/ze/internal/le/hookruntime"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "hook-check"

var actions = func() leaction.Area {
	verbs := [...]string{
		"session-start", "compaction-reminder", "verify-claim-reminder", categoryDelegationReminder,
		"block-until-lsp", "pretool-bash", "pretool-writeedit", "pretool-agent-skill",
		"pre-compact-save", "block-premature-stop", "rule-coverage-report", "session-end-summary",
		"session-end-deferrals", categorySubagentContext, "mark-lsp-invoked", categoryMarkSourceRead,
		"mark-agent-spawned", categoryValidateSpec, "posttool-writeedit", categorySessionID,
	}
	list := make([]leaction.Action, 0, 1+len(verbs))
	list = append(list, leaction.Action{
		Verb:   "unit",
		Why:    "run every hook dispatcher golden row and every behavioral fixture category in-process",
		Answer: runHere,
	})
	for _, verb := range verbs {
		kind := verb
		list = append(list, leaction.Action{
			Verb: kind,
			Why:  "run the " + kind + " hook JSON protocol in the native le process",
			Answer: func() (any, int) {
				return nil, hookruntime.Run(kind, os.Stdin, os.Stdout, os.Stderr)
			},
		})
	}
	return leaction.New(area, list...)
}()

// Actions answers the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le hook-check` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, code := Run(root)
	return report, code
}
