// Design: docs/architecture/traffic/cos-dynamic.md -- bounded cleanup pass counts
// Related: shared_access_cleanup_registered_test.go -- actual registered SDK route

//go:build ze_l2tp

package cos

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: failed warning clearance reports 1/0 after maps already restored, then
// reports 1/1 and 0/0 without another native write.
// Method: fail only the original publisher's CLEAR after one failed native Down.
// This candidate-only question requires reportIssue; a missing field is not RED.
// MUTATION: omit the failed report-only count or retry native compensation.
func TestCoSLegacySharedCleanupCountsReportOnlyRecovery(t *testing.T) {
	f, engine, bus := newRegisteredCoSCleanupReview(t)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	registeredCoSCleanupReviewUp(t, bus, 63210)
	f.requireMaps(f.shared)
	f.failWrites = 1
	registeredCoSCleanupReviewDown(t, bus, 63210)
	warnings := report.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("original native failure produced %d warnings, want one", len(warnings))
	}
	subject := warnings[0].Subject
	f.handler.mu.Lock()
	publish := f.handler.reportIssue
	failClear := true
	f.handler.reportIssue = func(input rpc.ReportIssueInput) error {
		if input.Action == rpc.ReportClear {
			if failClear {
				return errors.New("counted original warning-clear delivery failed")
			}
		}
		return publish(input)
	}
	f.handler.mu.Unlock()
	writes := f.writes
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err := engine.Execute(ctx, cleanupCommandForReview, nil)
	if err == nil {
		t.Fatal("failed warning clearance was counted as complete")
	}
	for _, label := range []string{`\battempted=1\b`, `\brecovered=0\b`} {
		if !regexp.MustCompile(label).MatchString(err.Error()) {
			t.Fatalf("actual SDK report-only failure lost count %s: %v", label, err)
		}
	}
	if !strings.Contains(err.Error(), "counted original warning-clear delivery failed") {
		t.Fatalf("counted failure lost its publication cause: %v", err)
	}
	if strings.Contains(err.Error(), "until their native maps are confirmed") {
		t.Fatalf("report-only error still claims native maps are unconfirmed: %v", err)
	}
	f.requireMaps(f.original)
	if f.writes != writes+1 {
		t.Fatal("first counted cleanup did not restore native maps once")
	}
	if _, retained := sessionStore.Load(fixtureCoSKey(sharedCoSTestTunnel, 63210)); retained {
		t.Fatal("confirmed native cleanup kept a departed session for its report")
	}
	warnings = report.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("failed CLEAR left %d warnings, want one", len(warnings))
	}
	if warnings[0].Subject != subject {
		t.Fatal("counted report-only failure changed original warning subject")
	}
	f.handler.mu.Lock()
	failClear = false
	f.handler.mu.Unlock()
	writes = f.writes
	ctxFinal, cancelFinal := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelFinal()
	data, err := engine.Execute(ctxFinal, cleanupCommandForReview, nil)
	if err != nil {
		t.Fatal(err)
	}
	requireCoSCleanupCountDocumentForReview(t, data, 1, 1)
	if f.writes != writes {
		t.Fatal("report-only counted recovery replayed native restoration")
	}
	if warnings := report.Warnings(); len(warnings) != 0 {
		t.Fatalf("counted report recovery left %+v", warnings)
	}
	ctxEmpty, cancelEmpty := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelEmpty()
	data, err = engine.Execute(ctxEmpty, cleanupCommandForReview, nil)
	if err != nil {
		t.Fatal(err)
	}
	requireCoSCleanupCountDocumentForReview(t, data, 0, 0)
	if f.writes != writes {
		t.Fatal("zero-count report pass replayed native restoration")
	}
}
