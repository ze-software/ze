// Design: docs/guide/bmp.md -- Loc-RIB sender behavior and collector status.
// Related: bmp_locrib.go -- handleBestChange fans out each best change.
// Related: sender_drain.go -- enqueueLocked returns a disconnected sender's error.

package bmp

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// TestLocRIBSendFailureWarnsAndKeepsHealthyCollector drives a valid best change
// through a disconnected sender and a healthy sender. A WARN-level logger must
// expose the failed collector and original error while the other collector
// receives the route. No test replaces writeRouteMonitoring or enqueueLocked.
func TestLocRIBSendFailureWarnsAndKeepsHealthyCollector(t *testing.T) {
	// This package's tests are serial, so no other test produces report-bus
	// events while this fixture owns the bus. Callers MUST NOT add t.Parallel.
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	logs := newRecordingConn()
	previous := logger()
	setLogger(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { setLogger(previous) })

	failed := newTestSession(t, "failed-locrib-collector", nil)
	conn := newRecordingConn()
	healthy := newTestSession(t, "healthy-locrib-collector", conn)
	// Both sessions were primed before this event; the failed connection has
	// since gone away. Peer Up failure is a separate producer from this test.
	failed.locRIBUpSent.Store(true)
	healthy.locRIBUpSent.Store(true)
	bp := &BMPPlugin{
		senders:  []*senderSession{failed, healthy},
		identity: &localIdentity{asn: 65000, routerID: 0x01020305},
	}

	bp.handleBestChange(locRIBBatch(1, false))
	waitQueueDrained(t, healthy)

	messages := decodeBMPStream(t, conn.written())
	if len(messages) != 1 {
		t.Fatalf("healthy collector received %d messages, want one route: %T", len(messages), messages)
	}
	route, ok := messages[0].(*RouteMonitoring)
	if !ok {
		t.Fatalf("healthy collector received %T, want RouteMonitoring", messages[0])
	}
	if route.Peer.PeerType != PeerTypeLocRIB {
		t.Fatalf("healthy collector peer type = %d, want Loc-RIB", route.Peer.PeerType)
	}
	if len(route.BGPUpdate) < message.HeaderLen {
		t.Fatalf("healthy collector received a truncated BGP message: %x", route.BGPUpdate)
	}
	body := route.BGPUpdate[message.HeaderLen:]
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatalf("healthy collector received an invalid UPDATE: %v", err)
	}
	if got := sections.NLRI(body); !bytes.Equal(got, []byte{24, 10, 20, 0}) {
		t.Fatalf("healthy collector NLRI = %x, want 10.20.0.0/24", got)
	}

	out := string(logs.written())
	if !strings.Contains(out, "level=WARN") {
		t.Fatalf("Loc-RIB send failure was hidden below WARN: %q", out)
	}
	if !strings.Contains(out, "code="+diagnostic.CodeDoctorRecentEvents) {
		t.Fatalf("warning does not name the actual recent-events doctor check: %q", out)
	}
	if !strings.Contains(out, "collector=failed-locrib-collector") {
		t.Fatalf("warning does not name the failed collector: %q", out)
	}
	if !strings.Contains(out, errNotConnected.Error()) {
		t.Fatalf("warning lost the original sender error %q: %q", errNotConnected, out)
	}
	if !strings.Contains(out, "ze show bmp collectors") {
		t.Fatalf("warning names no collector diagnosis command: %q", out)
	}
	if strings.Contains(out, "collector=healthy-locrib-collector") {
		t.Fatalf("healthy collector was reported as failed: %q", out)
	}

	issues := report.Errors(0)
	if len(issues) != 1 {
		t.Fatalf("report bus has %d errors, want only the failed collector's event: %+v", len(issues), issues)
	}
	issue := issues[0]
	if issue.Source != "bgp-bmp" {
		t.Fatalf("report source = %q, want bgp-bmp", issue.Source)
	}
	if issue.Code != diagnostic.CodeDoctorRecentEvents {
		t.Fatalf("report code = %q, want %q", issue.Code, diagnostic.CodeDoctorRecentEvents)
	}
	if issue.Subject != failed.name {
		t.Fatalf("report subject = %q, want failed collector %q", issue.Subject, failed.name)
	}
	if issue.Severity != report.SeverityError {
		t.Fatalf("report severity = %v, want an error event", issue.Severity)
	}
	if !strings.Contains(issue.Message, errNotConnected.Error()) {
		t.Fatalf("report event lost the original sender error: %q", issue.Message)
	}
	if !strings.Contains(issue.Message, "ze show bmp collectors") {
		t.Fatalf("report event names no collector diagnosis command: %q", issue.Message)
	}
}
