// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPF event bus types
// Related: events.go -- the sink whose refused emits these tests read
package ospf

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// refusingBus answers every emit with an error, as the plugin server answers an event
// type that no namespace registered.
type refusingBus struct{}

func (refusingBus) Emit(namespace, eventType string, _ any) (int, error) {
	return 0, errors.New("unknown event: " + namespace + "/" + eventType)
}

func (refusingBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

// captureOSPFLog points the package logger at a buffer that keeps WARN and above for
// the test's duration, and puts the previous logger back after it.
func captureOSPFLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	sink := &bytes.Buffer{}
	previous := logger()
	loggerPtr.Store(slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { loggerPtr.Store(previous) })
	return sink
}

// VALIDATES: spec-dropped-error-6 AC-3 -- an event the bus refuses reaches the operator
// at WARN, one line per refused emit, naming the event and the interface or neighbor,
// why it matters and the command to follow, under the internal-defect doctor code, and
// each is raised as an error event through the SDK publisher under the same code.
// Method: drive a neighbor event, an interface event and the RFC 5709 last-key
// notification into a sink whose bus refuses every emit, then read the WARN lines and
// the raises a recording publisher kept.
// PREVENTS: an adjacency change other plugins never hear of, recorded only at debug,
// which the default level hides; and a raise into the process-local report bus, which a
// forked OSPF holds in its own process where no doctor check reads it.
func TestOSPFEventEmitRefusalIsWarned(t *testing.T) {
	sink := captureOSPFLog(t)
	raised := recordIssues(t)
	events := newEventSink(refusingBus{})

	events.NeighborUp(ospfneighbor.Snapshot{Interface: "eth0", RouterID: "10.0.0.2", State: neighborStateFull})
	events.InterfaceStateChanged(ospfiface.Snapshot{Name: "eth0"})
	events.lastKeyExpired(&lastKeyExpirationEvent{Interface: "eth0", KeyID: 7, Direction: "send"})

	var refused []string
	for _, line := range warnLines(sink.String()) {
		if strings.Contains(line, "did not publish") {
			refused = append(refused, line)
		}
	}
	if len(refused) != 3 {
		t.Fatalf("refused-emit WARN lines = %d, want 3 (one per emit):\n%s", len(refused), sink.String())
	}
	want := []struct{ message, fix, subject string }{
		{"OSPF did not publish the neighbor-up event of neighbor 10.0.0.2 on interface eth0", showNeighborFix,
			"neighbor-up neighbor 10.0.0.2 on interface eth0"},
		{"OSPF did not publish the interface-state event of interface eth0", showInterfaceFix,
			"interface-state interface eth0"},
		{"OSPF did not publish the last-key-expiration event of interface eth0", showInterfaceFix,
			"last-key-expiration interface eth0"},
	}
	for index, line := range refused {
		for _, part := range []string{
			`msg="` + want[index].message + `"`,
			`why="` + emitRefusedWhy + `"`,
			`fix="` + want[index].fix + `"`,
			"code=doctor-internal-defect",
			"unknown event: ospf/",
			"report=queued",
		} {
			if !strings.Contains(line, part) {
				t.Errorf("WARN line %d lacks %s:\n%s", index, part, line)
			}
		}
	}
	issues := raised.withCode(diagnostic.CodeDoctorInternalDefect)
	if len(issues) != len(want) {
		t.Fatalf("internal-defect raises = %d, want %d (one per refused emit): %+v", len(issues), len(want), issues)
	}
	for index, issue := range issues {
		if issue.Severity != rpc.ReportError {
			t.Errorf("raise %d severity = %q, want %q", index, issue.Severity, rpc.ReportError)
		}
		if issue.Subject != want[index].subject {
			t.Errorf("raise %d subject = %q, want %q", index, issue.Subject, want[index].subject)
		}
		if issue.Message != want[index].message {
			t.Errorf("raise %d message = %q, want %q", index, issue.Message, want[index].message)
		}
		if issue.Fix != want[index].fix {
			t.Errorf("raise %d fix = %q, want %q", index, issue.Fix, want[index].fix)
		}
	}
}
