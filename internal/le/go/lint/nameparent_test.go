// Related: actions.go -- nameParent, which names the claimed slot in every pass's environment

package golint

import (
	"slices"
	"testing"
)

// TestLintPassesRunInsideTheClaimedSlot asks whether every pass of a lint
// plan, whose environment was taken before the claim, gets the environment
// that names the claimed slot as its linter's parent, so a shimmed linter
// started through `le exec` runs inside that slot and does not wait for a
// second one. Its method: a plan of two passes, one skipped, and a naming
// function that stands for job.Admission.ChildEnviron (whose own spelling
// rules job's tests prove), with each environment read after nameParent.
//
// VALIDATES: spec-native-action-job-admission decision 3: inside a parent's
// slot (`le go lint run` starting the linter) `le exec` holds nothing.
// PREVENTS: a lint run that holds one slot while its linter queues for
// another behind it, which deadlocks when it held the last one.
func TestLintPassesRunInsideTheClaimedSlot(t *testing.T) {
	const named = "ZE_RUN_JOB=/checkout/tmp/.ze-jobs/go-lint.1.claim.entry"
	plan := LintPlan{Passes: []PassPlan{
		{Name: "default", Environment: []string{"HOME=/h"}},
		{Name: "skipped", Skipped: true, Environment: []string{"HOME=/h", "GOOS=linux"}},
	}}
	seen := 0
	plan.nameParent(func(environ []string) []string {
		seen++
		return append(slices.Clone(environ), named)
	})
	if seen != len(plan.Passes) {
		t.Errorf("the naming function was asked %d times, want once per pass (%d)", seen, len(plan.Passes))
	}
	for _, pass := range plan.Passes {
		if !slices.Contains(pass.Environment, named) {
			t.Errorf("pass %s runs with %v, want it to name the claimed slot as %s", pass.Name, pass.Environment, named)
		}
		if !slices.Contains(pass.Environment, "HOME=/h") {
			t.Errorf("pass %s lost its own environment: %v", pass.Name, pass.Environment)
		}
	}
}
