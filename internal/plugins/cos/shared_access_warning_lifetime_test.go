// Design: docs/architecture/traffic/cos-dynamic.md -- original warning lifetime
// Related: shared_access_command_report_test.go -- registered SDK command fixture

//go:build ze_l2tp

package cos

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: healthy rollback and departure do not acquire a warning dependency.
// Method: after healthy apply, fail every report call. Original rollback, its
// token retry and final Down must succeed with zero report calls and correct
// native maps. This needs the candidate publisher seam, not a compiler RED.
func TestCoSLegacySharedHealthyRollbackDoesNotClearAbsentWarning(t *testing.T) {
	f := newCoSCommandReportFixture(t)
	registeredCoSCleanupReviewUp(t, f.bus, 63235)
	token, err := f.command("prepare", 63235, sharedCoSTestDifferent, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.command("apply", 0, "", token); err != nil {
		t.Fatal(err)
	}
	calls := 0
	f.handler.mu.Lock()
	f.handler.reportIssue = func(rpc.ReportIssueInput) error {
		calls++
		return errors.New("unused warning transport is unavailable")
	}
	f.handler.mu.Unlock()
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatalf("healthy rollback depended on an absent warning: %v", err)
	}
	f.requireMaps(f.shared)
	writes := f.writes
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatalf("healthy completed token depended on an absent warning: %v", err)
	}
	if f.writes != writes {
		t.Fatal("healthy completed rollback token replayed native work")
	}
	registeredCoSCleanupReviewDown(t, f.bus, 63235)
	f.requireMaps(f.original)
	if calls != 0 {
		t.Fatalf("healthy original owner made %d unnecessary report calls", calls)
	}
	if warnings := report.Warnings(); len(warnings) != 0 {
		t.Fatalf("healthy original owner manufactured a warning: %+v", warnings)
	}
	if _, retained := sessionStore.Load(fixtureCoSKey(sharedCoSTestTunnel, 63235)); retained {
		t.Fatal("healthy departure retained its original subscriber")
	}
}

// Goal: unknown Raise delivery still requires original-owner warning clearance.
// Method: deliver the real failed-rollback warning but lose its acknowledgement;
// confirmed rollback must clear that same warning, then its retry is a noop.
// A latch set only after acknowledged Raise would abandon this visible warning.
func TestCoSLegacySharedUnknownWarningDeliveryStillClearsOriginalOwner(t *testing.T) {
	f := newCoSCommandReportFixture(t)
	registeredCoSCleanupReviewUp(t, f.bus, 63236)
	token, err := f.command("prepare", 63236, sharedCoSTestDifferent, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.command("apply", 0, "", token); err != nil {
		t.Fatal(err)
	}
	raises := 0
	clears := 0
	var raisedSubject, clearedSubject string
	f.handler.mu.Lock()
	publish := f.handler.reportIssue
	f.handler.reportIssue = func(input rpc.ReportIssueInput) error {
		if input.Action == rpc.ReportRaise {
			raises++
			raisedSubject = input.Subject
			if err := publish(input); err != nil {
				return err
			}
			return errors.New("original warning delivery acknowledgement was lost")
		}
		if input.Action == rpc.ReportClear {
			clears++
			clearedSubject = input.Subject
		}
		return publish(input)
	}
	f.handler.mu.Unlock()
	f.failures = []error{errors.New("original native rollback failed")}
	_, rollbackErr := f.command("rollback", 0, "", token)
	if rollbackErr == nil {
		t.Fatal("failed rollback with unknown report delivery was acknowledged")
	}
	for _, cause := range []string{"original native rollback failed", "original warning delivery acknowledgement was lost"} {
		if !strings.Contains(rollbackErr.Error(), cause) {
			t.Fatalf("failed rollback lost cause %q: %v", cause, rollbackErr)
		}
	}
	subject := requireCoSCommandWarning(t, "", "original native rollback failed")
	if raises != 1 {
		t.Fatalf("one failed rollback made %d Raise attempts", raises)
	}
	if clears != 0 {
		t.Fatal("unconfirmed native rollback cleared its warning")
	}
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatal(err)
	}
	f.requireMaps(f.shared)
	if clears != 1 {
		t.Fatalf("confirmed original rollback made %d CLEAR attempts", clears)
	}
	if raisedSubject != subject {
		t.Fatal("Raise did not use the original warning subject")
	}
	if clearedSubject != subject {
		t.Fatal("unknown-delivery recovery cleared another warning lifetime")
	}
	if warnings := report.Warnings(); len(warnings) != 0 {
		t.Fatalf("confirmed same-owner cleanup left %+v", warnings)
	}
	writes := f.writes
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatal(err)
	}
	if f.writes != writes {
		t.Fatal("completed unknown-delivery recovery replayed native work")
	}
	if clears != 1 {
		t.Fatal("completed unknown-delivery recovery repeated CLEAR")
	}
	registeredCoSCleanupReviewDown(t, f.bus, 63236)
	f.requireMaps(f.original)
	if clears != 1 {
		t.Fatal("healthy final departure repeated an acknowledged CLEAR")
	}
}
