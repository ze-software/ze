// Design: docs/architecture/traffic/cos-dynamic.md -- report transport failures
// Related: shared_access_cleanup_registered_test.go -- real registered SDK runner

//go:build ze_l2tp

package cos

import (
	"errors"
	"strings"
	"testing"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Confirmed native cleanup must not replay when only its warning-clear
// publication failed. This needs the explicit publisher seam of the coherent
// candidate; the meaningful RED is a cut of clear-error retention/propagation,
// not an original-production compiler failure.
func TestCoSLegacySharedCleanupRetriesOnlyFailedReportClear(t *testing.T) {
	f, engine, bus := newRegisteredCoSCleanupReview(t)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	f.handler.mu.Lock()
	publish := f.handler.reportIssue
	failClear := true
	f.handler.reportIssue = func(input rpc.ReportIssueInput) error {
		if input.Action == rpc.ReportClear {
			if failClear {
				return errors.New("original report-clear delivery is unavailable")
			}
		}
		return publish(input)
	}
	f.handler.mu.Unlock()
	registeredCoSCleanupReviewUp(t, bus, 63100)
	f.requireMaps(f.shared)
	f.failWrites = 1
	registeredCoSCleanupReviewDown(t, bus, 63100)
	warnings := report.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("native failure produced %d warnings, want one", len(warnings))
	}
	subject := warnings[0].Subject
	writes := f.writes
	err := executeCoSCleanupForReview(t, engine, nil)
	if err == nil {
		t.Fatal("cleanup claimed success after its original warning-clear publication failed")
	}
	if !strings.Contains(err.Error(), "original report-clear delivery is unavailable") {
		t.Fatalf("cleanup lost its actual publication failure: %v", err)
	}
	if f.writes != writes+1 {
		t.Fatal("the first cleanup pass did not restore native maps exactly once")
	}
	f.requireMaps(f.original)
	if _, retained := sessionStore.Load(fixtureCoSKey(sharedCoSTestTunnel, 63100)); retained {
		t.Fatal("confirmed native cleanup kept a departed session solely for its report")
	}
	warnings = report.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("failed warning clear changed warning count to %d", len(warnings))
	}
	if warnings[0].Subject != subject {
		t.Fatal("failed warning-clear publication lost the original bounded subject")
	}
	f.handler.mu.Lock()
	failClear = false
	f.handler.mu.Unlock()
	writes = f.writes
	if err := executeCoSCleanupForReview(t, engine, nil); err != nil {
		t.Fatalf("healthy warning-clear retry failed: %v", err)
	}
	if f.writes != writes {
		t.Fatal("report-only recovery replayed a confirmed native restoration")
	}
	if warnings := report.Warnings(); len(warnings) != 0 {
		t.Fatalf("healthy original warning-clear retry left %+v", warnings)
	}
	if err := executeCoSCleanupForReview(t, engine, nil); err != nil {
		t.Fatal(err)
	}
	if f.writes != writes {
		t.Fatal("empty cleanup pass replayed the native operation")
	}
}

// A failed cleanup command must return both the actual native failure and the
// report transport failure. Merely logging the publication error loses it from
// the operator's command response. This is a coherent-candidate cut question.
func TestCoSLegacySharedCleanupReturnsNativeAndPublicationFailures(t *testing.T) {
	f, engine, bus := newRegisteredCoSCleanupReview(t)
	f.handler.mu.Lock()
	f.handler.reportIssue = func(rpc.ReportIssueInput) error {
		return errors.New("original failure-report delivery is unavailable")
	}
	f.handler.mu.Unlock()
	registeredCoSCleanupReviewUp(t, bus, 63101)
	f.requireMaps(f.shared)
	f.failWrites = 1
	registeredCoSCleanupReviewDown(t, bus, 63101)
	f.failWrites = 1
	err := executeCoSCleanupForReview(t, engine, nil)
	if err == nil {
		t.Fatal("cleanup acknowledged a native failure and failed report publication")
	}
	for _, cause := range []string{"native write outcome is not confirmed", "original failure-report delivery is unavailable"} {
		if !strings.Contains(err.Error(), cause) {
			t.Fatalf("cleanup lost its actual cause %q: %v", cause, err)
		}
	}
}

// Repairing an uncertain legacy write can leave a healthy live subscriber.
// Failed report publication must keep that policy while a later report-only
// cleanup retry clears the same warning with no native rewrite.
func TestCoSLegacySharedCleanupReportRetryPreservesLivePolicy(t *testing.T) {
	f, engine, bus := newRegisteredCoSCleanupReview(t)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	f.handler.mu.Lock()
	publish := f.handler.reportIssue
	failClear := true
	f.handler.reportIssue = func(input rpc.ReportIssueInput) error {
		if input.Action == rpc.ReportClear {
			if failClear {
				return errors.New("live access report-clear delivery is unavailable")
			}
		}
		return publish(input)
	}
	f.handler.mu.Unlock()
	registeredCoSCleanupReviewUp(t, bus, 63102)
	f.requireMaps(f.shared)
	f.failWrites = 2
	if _, err := l2tpevents.SessionCoSChange.Emit(bus, &l2tpevents.SessionCoSChangePayload{Birth: fixtureCoSKey(sharedCoSTestTunnel, 63102).birth, SubscriberID: fixtureCoSID(sharedCoSTestTunnel, 63102),
		TunnelID: sharedCoSTestTunnel, SessionID: 63102,
		AccessInterface: sharedCoSTestInterface, ProfileName: sharedCoSTestDifferent,
	}); err != nil {
		t.Fatal(err)
	}
	f.failWrites = 0
	writes := f.writes
	if err := executeCoSCleanupForReview(t, engine, nil); err == nil {
		t.Fatal("live cleanup acknowledged failed warning-clear delivery")
	}
	if f.writes != writes+1 {
		t.Fatal("uncertain policy cleanup did not restore its immediate beforeimage once")
	}
	f.requireMaps(f.shared)
	key := fixtureCoSKey(sharedCoSTestTunnel, 63102)
	if _, retained := sessionStore.Load(key); !retained {
		t.Fatal("report publication failure discarded a healthy live policy")
	}
	f.handler.mu.Lock()
	failClear = false
	f.handler.mu.Unlock()
	writes = f.writes
	if err := executeCoSCleanupForReview(t, engine, nil); err != nil {
		t.Fatal(err)
	}
	if f.writes != writes {
		t.Fatal("report-only retry rewrote a healthy live policy")
	}
	f.requireMaps(f.shared)
	if _, retained := sessionStore.Load(key); !retained {
		t.Fatal("report-only retry forgot its live subscriber")
	}
	if warnings := report.Warnings(); len(warnings) != 0 {
		t.Fatalf("confirmed report retry retained %+v", warnings)
	}
	registeredCoSCleanupReviewDown(t, bus, 63102)
	f.requireMaps(f.original)
}

// The original rollback token must propagate failed report clearance after
// native compensation, then retry only the report on that same token. A second
// compensation write would replay native work the first attempt confirmed.
func TestCoSLegacySharedRollbackRetriesOnlyFailedReportClear(t *testing.T) {
	f, _, bus := newRegisteredCoSCleanupReview(t)
	f.handler.mu.Lock()
	publish := f.handler.reportIssue
	failClear := true
	clearCalls := 0
	f.handler.reportIssue = func(input rpc.ReportIssueInput) error {
		if input.Action == rpc.ReportClear {
			clearCalls++
			if failClear {
				return errors.New("rollback report-clear delivery is unavailable")
			}
		}
		return publish(input)
	}
	f.handler.mu.Unlock()
	registeredCoSCleanupReviewUp(t, bus, 63103)
	token, err := f.command("prepare", 63103, sharedCoSTestDifferent, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.command("apply", 0, "", token); err != nil {
		t.Fatal(err)
	}
	// Warning clearance requires an actual failed native rollback first;
	// healthy rollback has no warning episode or report transport dependency.
	f.failWrites = 1
	if _, err := f.command("rollback", 0, "", token); err == nil {
		t.Fatal("the original native rollback failure was acknowledged")
	} else if !strings.Contains(err.Error(), "native write outcome is not confirmed") {
		t.Fatalf("rollback lost its original native failure: %v", err)
	}
	if clearCalls != 0 {
		t.Fatalf("unconfirmed native rollback made %d clear publications, want none", clearCalls)
	}
	writes := f.writes
	if _, err := f.command("rollback", 0, "", token); err == nil {
		t.Fatal("rollback claimed success after warning-clear publication failed")
	} else if !strings.Contains(err.Error(), "rollback report-clear delivery is unavailable") {
		t.Fatalf("rollback lost its actual report cause: %v", err)
	}
	if f.writes != writes+1 {
		t.Fatal("the original rollback did not compensate native maps once")
	}
	if clearCalls != 1 {
		t.Fatalf("the first rollback made %d clear publications, want one", clearCalls)
	}
	f.requireMaps(f.shared)
	f.handler.mu.Lock()
	failClear = false
	f.handler.mu.Unlock()
	writes = f.writes
	if _, err := f.command("rollback", 0, "", token); err != nil {
		t.Fatalf("the same original rollback token could not retry its report: %v", err)
	}
	if f.writes != writes {
		t.Fatal("the rollback report retry replayed a confirmed native compensation")
	}
	if clearCalls != 2 {
		t.Fatalf("the same token made %d total clear publications, want failed plus confirmed", clearCalls)
	}
	f.requireMaps(f.shared)
	registeredCoSCleanupReviewDown(t, bus, 63103)
	f.requireMaps(f.original)
}
