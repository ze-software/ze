// Design: docs/contributing/navigating-the-code.md -- the agent-economy figures that copy the call budget
// Related: budget.go -- the declarations the token report copies
//
// The token report (internal/le/ai/tokens) judges the editing agents' call
// budget with copies of this package's unexported declarations, because it
// cannot name them. This file is the check those copies name. The import runs
// in this direction only: aitokens imports nothing of hookruntime, so the test
// binary makes no cycle.

package hookruntime

import (
	"slices"
	"testing"

	aitokens "github.com/ze-software/ze/internal/le/ai/tokens"
)

// TestTheTokenReportCopiesTheCallBudget compares the token report's copies of
// the call budget with what they copy: the budget, the agent type it binds,
// and the tools whose calls it counts. The tools are derived from
// nativeHookActions, as the tools of every action that runs bashCallBudget or
// writeCallBudget, so a tool added to or removed from those actions, or a
// budget check moved to another action, shows here.
func TestTheTokenReportCopiesTheCallBudget(t *testing.T) {
	if aitokens.CallBudget != agentCallBudget {
		t.Errorf("the token report's call budget is %d, the hook's is %d", aitokens.CallBudget, agentCallBudget)
	}
	if aitokens.BudgetedAgentType != budgetedAgentType {
		t.Errorf("the token report's budgeted agent type is %q, the hook's is %q", aitokens.BudgetedAgentType, budgetedAgentType)
	}

	budgetChecks := []string{checkName(bashCallBudget), checkName(writeCallBudget)}
	bound := []string{}
	for _, action := range nativeHookActions {
		for _, check := range action.checks {
			if slices.Contains(budgetChecks, checkName(check)) {
				bound = append(bound, action.tools...)
				break
			}
		}
	}
	if len(bound) == 0 {
		t.Fatal("no action of nativeHookActions runs the call budget")
	}
	slices.Sort(bound)
	bound = slices.Compact(bound)
	copied := slices.Clone(aitokens.BudgetedTools)
	slices.Sort(copied)
	if !slices.Equal(copied, bound) {
		t.Errorf("the token report's budgeted tools are %v, the hook's budget binds %v", copied, bound)
	}
}
