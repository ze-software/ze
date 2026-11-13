// Design: docs/architecture/doctor-and-health-checks.md -- reported event application failures.

package connected

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func connectedReplayPayload(t *testing.T, id uint64) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(redistevents.ReplayRequest{ReplayID: id})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// MUTATION: logging a failed replay without the SDK report-issue raise leaves
// show warnings and show doctor with no indication of the missing routes.
func TestConnectedReplayFailureReportsAndClears(t *testing.T) {
	bootstrapModes(t, func(t *testing.T, mode string) {
		for _, recovery := range []string{"complete-replay", "empty-current-inventory"} {
			t.Run(recovery, func(t *testing.T) {
				var logs bytes.Buffer
				previous := logger()
				setLogger(slog.New(slog.NewTextHandler(&logs, nil)))
				t.Cleanup(func() { setLogger(previous) })
				h := startBootstrapConsumer(t, mode, make(map[string]json.RawMessage))
				h.add("192.0.2.1", 24)
				h.add("2001:db8:5::1", 64)
				h.replay(t)
				assertBootstrapPath(t, h, "192.0.2.0/24", true)
				assertBootstrapPath(t, h, "2001:db8:5::/64", true)
				var reports []rpc.ReportIssueInput
				h.mu.Lock()
				h.beforeEmit = func(batch *redistevents.RouteChangeBatch) error {
					if batch.ReplayID == 0 {
						return nil
					}
					if batch.Entries[0].Prefix == netip.MustParsePrefix("192.0.2.0/24") {
						return errors.New("test engine refused connected replay")
					}
					return nil
				}
				h.reportIssue = func(in rpc.ReportIssueInput) error {
					reports = append(reports, in)
					return nil
				}
				h.mu.Unlock()
				h.deliver(t, redistevents.ReplayNamespace, redistevents.ReplayRequestEventType, connectedReplayPayload(t, 91))
				h.deliver(t, redistevents.ReplayNamespace, redistevents.ReplayRequestEventType, connectedReplayPayload(t, 92))
				h.mu.Lock()
				snapshot := slices.Clone(reports)
				h.mu.Unlock()
				if len(snapshot) != 1 {
					t.Fatalf("failed replays produced %d report-issue requests, want one active warning and no clear for the successful IPv6 subset", len(snapshot))
				}
				raised := snapshot[0]
				if raised.Action != rpc.ReportRaise || raised.Severity != rpc.ReportWarning || raised.Code != diagnostic.CodeDoctorPluginPanic {
					t.Fatalf("replay report=%+v, want registered event-handler-failed warning", raised)
				}
				if raised.Subject != "route-change replay" || raised.Fix != "ze show log recent level warn component connected" {
					t.Fatalf("replay warning has no stable replay identity or operator command: %+v", raised)
				}
				for _, message := range []string{"192.0.2.0/24", "test engine refused connected replay"} {
					if !strings.Contains(raised.Message, message) {
						t.Fatalf("replay warning %q omits %q", raised.Message, message)
					}
					if !strings.Contains(logs.String(), message) {
						t.Fatalf("replay log %q omits %q", logs.String(), message)
					}
				}
				if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "code="+diagnostic.CodeDoctorPluginPanic) {
					t.Fatalf("failed replay did not retain its warning and registered code: %s", logs.String())
				}
				if !slices.Contains(diagnostic.EventCodes(), raised.Code) {
					t.Fatalf("replay code %q is not registered for the recent-events doctor check", raised.Code)
				}
				h.mu.Lock()
				h.beforeEmit = nil
				h.mu.Unlock()
				if recovery == "empty-current-inventory" {
					h.remove(t, "192.0.2.1")
					h.remove(t, "2001:db8:5::1")
				}
				h.deliver(t, redistevents.ReplayNamespace, redistevents.ReplayRequestEventType, connectedReplayPayload(t, 93))
				h.mu.Lock()
				defer h.mu.Unlock()
				if len(reports) != 2 || reports[1].Action != rpc.ReportClear || reports[1].Code != raised.Code || reports[1].Subject != raised.Subject {
					t.Fatalf("complete recovery reports=%+v, want one clear of the acknowledged warning", reports)
				}
				if recovery == "complete-replay" {
					prefixes := make(map[netip.Prefix]bool)
					for _, batch := range h.batches {
						if batch.ReplayID == 93 {
							prefixes[batch.Entries[0].Prefix] = true
						}
					}
					if len(prefixes) != 2 || !prefixes[netip.MustParsePrefix("192.0.2.0/24")] || !prefixes[netip.MustParsePrefix("2001:db8:5::/64")] {
						t.Fatalf("recovered replay prefixes=%v, want both live families", prefixes)
					}
				}
			})
		}
	})
}

func TestConnectedReplayReportRefusalReachesEventCaller(t *testing.T) {
	bootstrapModes(t, func(t *testing.T, mode string) {
		for _, action := range []rpc.ReportAction{rpc.ReportRaise, rpc.ReportClear} {
			t.Run(string(action), func(t *testing.T) {
				h := startBootstrapConsumer(t, mode, make(map[string]json.RawMessage))
				h.add("192.0.2.1", 24)
				h.add("2001:db8:5::1", 64)
				h.replay(t)
				assertBootstrapPath(t, h, "192.0.2.0/24", true)
				assertBootstrapPath(t, h, "2001:db8:5::/64", true)
				h.mu.Lock()
				h.beforeEmit = func(batch *redistevents.RouteChangeBatch) error {
					if batch.ReplayID != 0 {
						return errors.New("test engine refused connected replay")
					}
					return nil
				}
				var raised []rpc.ReportIssueInput
				h.reportIssue = func(in rpc.ReportIssueInput) error {
					raised = append(raised, in)
					return nil
				}
				h.mu.Unlock()
				if action == rpc.ReportClear {
					h.deliver(t, redistevents.ReplayNamespace, redistevents.ReplayRequestEventType, connectedReplayPayload(t, 93))
					h.mu.Lock()
					snapshot := slices.Clone(raised)
					h.beforeEmit = nil
					h.mu.Unlock()
					if len(snapshot) != 1 || snapshot[0].Action != rpc.ReportRaise || snapshot[0].Code != diagnostic.CodeDoctorPluginPanic {
						t.Fatalf("clear-refusal setup reports=%+v, want an acknowledged failed-replay warning", snapshot)
					}
				}
				h.mu.Lock()
				h.reportIssue = func(in rpc.ReportIssueInput) error {
					if in.Action != action {
						return errors.New("unexpected report action at refusal boundary")
					}
					return errors.New("test report service unavailable")
				}
				h.mu.Unlock()
				err := h.deliverEvent(redistevents.ReplayNamespace, redistevents.ReplayRequestEventType, connectedReplayPayload(t, 94))
				if err == nil {
					t.Fatal("the replay callback acknowledged success while its warning could not reach the daemon")
				}
				if !strings.Contains(err.Error(), "test report service unavailable") {
					t.Fatalf("replay callback error=%v, want actual report refusal", err)
				}
				if action == rpc.ReportClear {
					h.mu.Lock()
					defer h.mu.Unlock()
					prefixes := make(map[netip.Prefix]bool)
					for _, batch := range h.batches {
						if batch.ReplayID == 94 {
							prefixes[batch.Entries[0].Prefix] = true
						}
					}
					if len(prefixes) != 2 || !prefixes[netip.MustParsePrefix("192.0.2.0/24")] || !prefixes[netip.MustParsePrefix("2001:db8:5::/64")] {
						t.Fatalf("clear was refused before a useful complete recovery: %v", prefixes)
					}
				}
			})
		}
	})
}
