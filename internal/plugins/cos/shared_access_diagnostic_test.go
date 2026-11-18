// Design: docs/architecture/traffic/cos-dynamic.md -- failures reach the daemon's doctor and report bus
// Related: diagnostic_shared.go -- explicit reporter binding and bounded problem subjects

//go:build ze_l2tp

package cos

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// This fixture reaches the actual registration callback that owns the daemon
// reporter binding. It replaces only the existing native backend boundaries.
// It refers to no future diagnostic helper, callback field or constructor.
type cosReportTestFixture struct {
	*sharedCoSTestOwner
	logs bytes.Buffer
}

func newRegisteredCoSReportFixture(t *testing.T) *cosReportTestFixture {
	t.Helper()
	report.ResetForTest()
	beforeLogger := logger()
	f := &cosReportTestFixture{sharedCoSTestOwner: newSharedCoSTestOwner(t)}
	setLogger(slog.New(slog.NewJSONHandler(&f.logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		setLogger(beforeLogger)
		report.ResetForTest()
	})
	registration := registry.Lookup(Name)
	if registration == nil {
		t.Fatal("the actual class-of-service plugin registration is absent")
	}
	if registration.ConfigureEventBus == nil {
		t.Fatal("the class-of-service registration has no event binding")
	}
	beforeHandler := dynamicHandler
	backendWrite := f.handler.updateQoS
	backendRead := f.handler.readLink
	f.handler.stop()
	f.bus = newTestBus()
	registration.ConfigureEventBus(f.bus)
	if dynamicHandler == nil {
		t.Fatal("the actual registration did not construct its event owner")
	}
	f.handler = dynamicHandler
	f.handler.updateQoS = backendWrite
	f.handler.readLink = backendRead
	t.Cleanup(func() {
		f.handler.stop()
		dynamicHandler = beforeHandler
	})
	return f
}

// Cleanup runs before the underlying metadata cleanup, so a failed assertion
// cannot leave this fixture's native custody in a retained process owner.
func (f *cosReportTestFixture) up(session uint16, profile string) {
	f.t.Helper()
	f.sharedCoSTestOwner.up(session, profile)
	f.t.Cleanup(func() {
		f.failWrites = 0
		f.down(session)
	})
}

func (f *cosReportTestFixture) requireProblem(entries []report.Issue, code string, warningCount int) report.Issue {
	f.t.Helper()
	if len(entries) != 1 {
		f.t.Fatalf("the actual failed operation has %d retained reports, want one: %+v", len(entries), entries)
	}
	entry := entries[0]
	if entry.Source != Name {
		f.t.Fatalf("failure report source=%q, want %q", entry.Source, Name)
	}
	if entry.Code != code {
		f.t.Fatalf("failure report code=%q, want %q", entry.Code, code)
	}
	if entry.Subject == "" {
		f.t.Fatal("the failed operation lost its bounded problem subject")
	}
	if len(entry.Subject) > report.SubjectMax {
		f.t.Fatalf("problem subject is %d bytes, above %d", len(entry.Subject), report.SubjectMax)
	}
	if entry.Fix == "" {
		f.t.Fatal("the failed operation has no recovery command")
	}
	if len(entry.Fix) > report.FixMax {
		f.t.Fatalf("recovery command is %d bytes, above %d", len(entry.Fix), report.FixMax)
	}
	if !strings.Contains(entry.Message, sharedCoSTestInterface) {
		f.t.Fatalf("problem message lost the actual access: %q", entry.Message)
	}
	if diagnostic.Lookup(code) == nil {
		f.t.Fatalf("doctor cannot describe the problem code %q", code)
	}
	if !slices.Contains(diagnostic.EventCodes(), code) {
		f.t.Fatalf("recent-events does not declare problem code %q", code)
	}
	decoder := json.NewDecoder(bytes.NewReader(f.logs.Bytes()))
	count := 0
	for {
		var row map[string]any
		err := decoder.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			f.t.Fatalf("decode the actual structured warning: %v", err)
		}
		count++
		if row["level"] != "WARN" {
			f.t.Fatalf("the failed operation did not log at WARN: %+v", row)
		}
		if row["code"] != code {
			f.t.Fatalf("warning lost the reported doctor code: %+v", row)
		}
		if row["fix"] != entry.Fix {
			f.t.Fatalf("warning and report disagree on recovery: %+v / %+v", row, entry)
		}
	}
	if count != warningCount {
		f.t.Fatalf("structured warnings=%d, want %d", count, warningCount)
	}
	return entry
}

// Goal: a failed final native restore reaches doctor through the real event owner.
// Method: install a healthy policy, fail its final restore, and inspect the
// daemon report and structured warning for the actual access and backend cause.
func TestCoSLegacySharedRestoreFailureHasDoctorReport(t *testing.T) {
	f := newRegisteredCoSReportFixture(t)
	f.up(60211, sharedCoSTestFirst)
	f.requireMaps(f.shared)
	if entries := report.Warnings(); len(entries) != 0 {
		t.Fatalf("healthy initial install raised a problem: %+v", entries)
	}
	f.failWrites = 1
	f.down(60211)
	entry := f.requireProblem(report.Warnings(), "doctor-cos-cleanup-unconfirmed", 1)
	if !strings.Contains(entry.Message, "native write outcome is not confirmed") {
		t.Fatalf("restore report lost the real backend cause: %q", entry.Message)
	}
}

// Goal: a write that fails before membership publication still reports its
// retained native compensation. Method: both the initial write and its
// compensation fail through the existing backend callback, without a token.
func TestCoSLegacySharedUnpublishedFailureHasDoctorReport(t *testing.T) {
	f := newRegisteredCoSReportFixture(t)
	f.failWrites = 2
	f.up(60212, sharedCoSTestFirst)
	entry := f.requireProblem(report.Warnings(), "doctor-cos-cleanup-unconfirmed", 1)
	if !strings.Contains(entry.Message, "native write outcome is not confirmed") {
		t.Fatalf("unpublished compensation report lost its native cause: %q", entry.Message)
	}
}

// Goal: repeated failed attempts retain one warning until restoration succeeds.
// Method: repeat the same original Down twice under native failure, then let
// that same native restore succeed. This tests report lifetime, not an operator
// recovery entry point, which needs its own registered-path test.
func TestCoSLegacySharedRestoreReportTracksConfirmedRecovery(t *testing.T) {
	f := newRegisteredCoSReportFixture(t)
	f.up(60213, sharedCoSTestFirst)
	f.requireMaps(f.shared)
	f.failWrites = 2
	f.down(60213)
	first := f.requireProblem(report.Warnings(), "doctor-cos-cleanup-unconfirmed", 1)
	f.down(60213)
	second := f.requireProblem(report.Warnings(), "doctor-cos-cleanup-unconfirmed", 2)
	if second.Subject != first.Subject {
		t.Fatalf("the same retained cleanup changed subject: %q / %q", first.Subject, second.Subject)
	}
	key := sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: 60213}
	if _, retained := sessionStore.Load(key); !retained {
		t.Fatal("the unconfirmed restore lost its original session before recovery")
	}
	writes := f.writes
	f.down(60213)
	if f.writes != writes+1 {
		t.Fatalf("confirmed recovery made %d native attempts, want one", f.writes-writes)
	}
	f.requireMaps(f.original)
	if _, retained := sessionStore.Load(key); retained {
		t.Fatal("confirmed recovery kept the departed session's custody")
	}
	if entries := report.Warnings(); len(entries) != 0 {
		t.Fatalf("confirmed restoration left its warning active: %+v", entries)
	}
}

// Goal: a conflicting initial profile is diagnosed while the first user survives.
// Method: reach a healthy policy, request different maps on the same access,
// and require a registered refusal report without changing the installed maps.
func TestCoSLegacySharedPolicyRefusalHasDoctorReport(t *testing.T) {
	f := newRegisteredCoSReportFixture(t)
	f.up(60214, sharedCoSTestFirst)
	f.requireMaps(f.shared)
	f.up(60215, sharedCoSTestDifferent)
	entry := f.requireProblem(report.Errors(0), "doctor-cos-policy-refused", 1)
	if !strings.Contains(entry.Message, "another subscriber") {
		t.Fatalf("refusal report lost the actual shared-policy conflict: %q", entry.Message)
	}
	f.requireMaps(f.shared)
}

// Goal: successful legacy events do not manufacture operator problems.
// Method: install two compatible users and remove both through the actual
// registered event owner, with no failing backend call or command.
func TestCoSLegacySharedHealthyEventsHaveNoDoctorReport(t *testing.T) {
	f := newRegisteredCoSReportFixture(t)
	f.up(60216, sharedCoSTestFirst)
	f.up(60217, sharedCoSTestAlias)
	f.requireMaps(f.shared)
	for _, session := range []uint16{60216, 60217} {
		key := sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: session}
		if _, admitted := sessionStore.Load(key); !admitted {
			t.Fatalf("healthy session %d was not admitted", session)
		}
	}
	f.down(60216)
	f.down(60217)
	for _, session := range []uint16{60216, 60217} {
		key := sessionKey{tunnelID: sharedCoSTestTunnel, sessionID: session}
		if _, retained := sessionStore.Load(key); retained {
			t.Fatalf("healthy session %d was not removed", session)
		}
	}
	if entries := report.Warnings(); len(entries) != 0 {
		t.Fatalf("healthy events raised active problems: %+v", entries)
	}
	if entries := report.Errors(0); len(entries) != 0 {
		t.Fatalf("healthy events raised error events: %+v", entries)
	}
	if f.logs.Len() != 0 {
		t.Fatalf("healthy events logged a warning: %s", f.logs.Bytes())
	}
}
