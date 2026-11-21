//go:build linux

package trend

import (
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// VALIDATES: AC-18 -- the kernel-memory rule's own runtime check reports the
// standing warning with the bus's message and fix while the rule is active,
// and no row while it is quiet. It declares the rule's code, which is what
// lets recent-events skip that warning (ownedCodes) rather than add a second
// row.
// Method: the check registered from register_kernel_memory.go, called over the
// real run loop driven through the same leak as the run-loop test: quiet at
// start, active at hour 9, quiet again at hour 14.
func TestTrendDoctorCheckRowFollowsRuleState(t *testing.T) {
	check := registeredRuntimeCheck(t, kernelMemoryCheckName)
	if !slices.Contains(check.Codes, CodeKernelMemoryGrowth) {
		t.Fatalf("check %s declares %v, want %s", check.Name, check.Codes, CodeKernelMemoryGrowth)
	}
	isolateTrend(t, []rule{kernelMemoryRuleOnly(t)})
	box := newKernelMemoryBox(t)
	box.set(0)
	run := startTrend(t, &host.Detector{Root: box.root})
	if rows := check.Check(diagnostic.DoctorCheckContext{}); len(rows) != 0 {
		t.Fatalf("rows while warming up = %+v, want none", rows)
	}
	minute := 0
	advanceTo := func(target int) {
		t.Helper()
		for minute < target {
			minute++
			box.set(uint64(min(minute, 600)))
			run.step(time.Minute)
		}
	}

	advanceTo(9 * 60)
	issue, raised := warningOf(CodeKernelMemoryGrowth, kernelMemoryRuleName)
	if !raised {
		t.Fatal("no kernel-memory warning at hour 9")
	}
	rows := check.Check(diagnostic.DoctorCheckContext{})
	if len(rows) != 1 {
		t.Fatalf("rows while active = %d, want 1: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.Code != CodeKernelMemoryGrowth || row.Severity != diagnostic.SeverityWarning {
		t.Errorf("row code/severity = %s/%v, want %s/warning", row.Code, row.Severity, CodeKernelMemoryGrowth)
	}
	if row.Message != issue.Message {
		t.Errorf("row message = %q, want the bus message %q", row.Message, issue.Message)
	}
	if row.Fix != issue.Fix {
		t.Errorf("row fix = %q, want the bus fix %q", row.Fix, issue.Fix)
	}

	advanceTo(14 * 60)
	if rows := check.Check(diagnostic.DoctorCheckContext{}); len(rows) != 0 {
		t.Errorf("rows once quiet = %+v, want none", rows)
	}
}
