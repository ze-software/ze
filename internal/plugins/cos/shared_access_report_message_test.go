// Design: docs/architecture/traffic/cos-dynamic.md -- actionable report messages
// Related: shared_access_diagnostic_test.go -- actual registered event owner

//go:build ze_l2tp

package cos

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/report"
)

// Goal: doctor receives the supplied profile and correction in the stored event.
// Method: refuse an unknown profile through the real registration and inspect
// Message, the actual field recent-events renders, rather than code metadata.
func TestCoSLegacySharedPolicyReportCarriesProfileAndCorrection(t *testing.T) {
	f := newRegisteredCoSReportFixture(t)
	profile := "operator-supplied-profile"
	f.up(63220, profile)
	entries := report.Errors(0)
	if len(entries) != 1 {
		t.Fatalf("unknown profile produced %d events, want one: %+v", len(entries), entries)
	}
	entry := entries[0]
	for _, text := range []string{strconv.Quote(profile), "requested subscriber policy was not installed", "correct the conflicting RADIUS assignment or profile", "does not exist"} {
		if !strings.Contains(entry.Message, text) {
			t.Fatalf("doctor's stored policy message omitted %q: %q", text, entry.Message)
		}
	}
	if entry.Fix != "ze show class-of-service" {
		t.Fatalf("policy correction lost its inspection command: %+v", entry)
	}
	if len(report.Warnings()) != 0 {
		t.Fatal("a refused profile manufactured uncertain native cleanup")
	}
	if f.writes != 0 {
		t.Fatal("unknown-profile refusal wrote native maps")
	}
}

// Goal: a shared-policy conflict includes its actual assignment and remedy.
// Method: keep one healthy user and refuse the next user's incompatible profile.
func TestCoSLegacySharedConflictReportCarriesCorrection(t *testing.T) {
	f := newRegisteredCoSReportFixture(t)
	f.up(63221, sharedCoSTestFirst)
	f.up(63222, sharedCoSTestDifferent)
	entries := report.Errors(0)
	if len(entries) != 1 {
		t.Fatalf("conflict produced %d events, want one", len(entries))
	}
	for _, text := range []string{strconv.Quote(sharedCoSTestDifferent), "correct the conflicting RADIUS assignment or profile", "another subscriber"} {
		if !strings.Contains(entries[0].Message, text) {
			t.Fatalf("stored conflict message omitted %q: %q", text, entries[0].Message)
		}
	}
	f.requireMaps(f.shared)
	if len(report.Warnings()) != 0 {
		t.Fatal("policy conflict manufactured uncertain cleanup")
	}
}

// Goal: doctor receives why the retained native owner needs cleanup.
// Method: fail final restoration and inspect the active warning's stored text.
func TestCoSLegacySharedCleanupReportCarriesExplanation(t *testing.T) {
	f := newRegisteredCoSReportFixture(t)
	f.up(63223, sharedCoSTestFirst)
	f.failWrites = 1
	f.down(63223)
	entries := report.Warnings()
	if len(entries) != 1 {
		t.Fatalf("failed restore produced %d warnings, want one", len(entries))
	}
	for _, text := range []string{"original access maps are not confirmed restored", "original owner remains retained", "native write outcome is not confirmed"} {
		if !strings.Contains(entries[0].Message, text) {
			t.Fatalf("stored cleanup message omitted %q: %q", text, entries[0].Message)
		}
	}
	if entries[0].Fix != "ze request class-of-service cleanup" {
		t.Fatalf("cleanup explanation lost its recovery command: %+v", entries[0])
	}
}
