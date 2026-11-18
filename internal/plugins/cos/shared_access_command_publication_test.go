// Design: docs/architecture/traffic/cos-dynamic.md -- rollback report publication
// Related: shared_access_command_report_test.go -- actual registered SDK fixture

//go:build ze_l2tp

package cos

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: the actual SDK command propagates failed warning clearance after
// confirmed native rollback, then its same original token retries only CLEAR.
// Method: first retain a real failed rollback warning, fail only its original
// publisher's CLEAR, and observe the SDK answer and native write count.
// This needs the candidate reportIssue seam. Missing-field compilation against
// original production is not a semantic RED; cut clear propagation/retention
// only after a coherent candidate runs this actual named body GREEN.
func TestCoSLegacySharedRegisteredRollbackRetriesOnlyReportClear(t *testing.T) {
	f := newCoSCommandReportFixture(t)
	registeredCoSCleanupReviewUp(t, f.bus, 63234)
	token, err := f.command("prepare", 63234, sharedCoSTestDifferent, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.command("apply", 0, "", token); err != nil {
		t.Fatal(err)
	}
	f.failures = []error{errors.New("registered rollback native failure")}
	if _, err := f.command("rollback", 0, "", token); err == nil {
		t.Fatal("failed original rollback was acknowledged")
	}
	subject := requireCoSCommandWarning(t, "", "registered rollback native failure")
	f.handler.mu.Lock()
	publish := f.handler.reportIssue
	failClear := true
	clearCalls := 0
	f.handler.reportIssue = func(input rpc.ReportIssueInput) error {
		if input.Action == rpc.ReportClear {
			clearCalls++
			if failClear {
				return errors.New("registered rollback warning-clear delivery failed")
			}
		}
		return publish(input)
	}
	f.handler.mu.Unlock()
	writes := f.writes
	if _, err := f.command("rollback", 0, "", token); err == nil {
		t.Fatal("SDK command acknowledged failed warning clearance")
	} else if !strings.Contains(err.Error(), "registered rollback warning-clear delivery failed") {
		t.Fatalf("SDK command lost the original publication cause: %v", err)
	}
	if f.writes != writes+1 {
		t.Fatal("rollback did not compensate native maps once")
	}
	f.requireMaps(f.shared)
	requireCoSCommandWarning(t, subject, "registered rollback native failure")
	if clearCalls != 1 {
		t.Fatalf("confirmed rollback made %d CLEAR attempts, want one", clearCalls)
	}
	f.handler.mu.Lock()
	failClear = false
	f.handler.mu.Unlock()
	writes = f.writes
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatal(err)
	}
	if f.writes != writes {
		t.Fatal("SDK report-only retry replayed confirmed native rollback")
	}
	if clearCalls != 2 {
		t.Fatalf("same token made %d CLEAR attempts, want failed then successful", clearCalls)
	}
	if len(report.Warnings()) != 0 {
		t.Fatal("confirmed same-owner publication kept its warning")
	}
	f.requireMaps(f.shared)
	registeredCoSCleanupReviewDown(t, f.bus, 63234)
	f.requireMaps(f.original)
}
