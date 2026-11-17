// Design: docs/architecture/doctor-and-health-checks.md -- captured control-send failures remain visible
// Related: teardown.go -- the exact captured session remains the teardown authority
package l2tp

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"testing"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// Goal: a failed captured-session CDN send remains visible after local teardown.
// Method: the real unstarted listener refuses its send; the test inspects the
// retained event, diagnostic registration, structured warning and unchanged Down.
func TestCapturedSessionTeardownSendFailureReportsControlDiagnostic(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	var logs bytes.Buffer
	r := newReactorForSnapshot(t)
	r.logger = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	r.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), r.logger)
	bus := newCauseBus()
	r.eventBus = bus
	peer := netip.MustParseAddrPort("10.0.0.9:1701")
	session := &L2TPSession{localSID: 7, remoteSID: 70, state: L2TPSessionEstablished, username: "grace"}
	insertEstablishedTunnel(t, r, 11, 22, peer, session)
	tunnel := r.tunnelsByLocalID[11]

	if !r.teardownCapturedSession(tunnel, session, nil, cdnResultGeneralError, l2tpevents.TerminateCauseNASError) {
		t.Fatal("the original captured session was not torn down")
	}
	entries := report.Errors(0)
	if len(entries) != 1 {
		t.Fatalf("failed captured teardown send retained %d events, want one: %+v", len(entries), entries)
	}
	entry := entries[0]
	if entry.Source != SubsystemName || entry.Code != "doctor-l2tp-control-send" || entry.Subject != peer.String() {
		t.Fatalf("control-send event lost its actual peer or diagnostic class: %+v", entry)
	}
	if entry.Fix != "ze show errors" || entry.Detail["error"] != errListenerNotStarted.Error() || entry.Detail["peer"] != peer.String() {
		t.Fatalf("control-send event lost its real cause or next command: %+v", entry)
	}
	if !strings.Contains(entry.Message, peer.String()) || !strings.Contains(entry.Message, errListenerNotStarted.Error()) {
		t.Fatalf("control-send message does not explain the actual failure: %+v", entry)
	}
	if diagnostic.Lookup(entry.Code) == nil || !slices.Contains(diagnostic.EventCodes(), entry.Code) {
		t.Fatalf("the control-send code is not described and declared to recent-events: %s", entry.Code)
	}
	var warning map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &warning); err != nil {
		t.Fatalf("decode the actual warning: %v\n%s", err, logs.Bytes())
	}
	if warning["code"] != entry.Code || warning["fix"] != entry.Fix {
		t.Fatalf("warning and retained event do not share the diagnostic and repair: %+v", warning)
	}
	deaths := bus.downEvents()
	if len(deaths) != 1 || deaths[0].TunnelID != 11 || deaths[0].SessionID != 7 || deaths[0].Cause != l2tpevents.TerminateCauseNASError {
		t.Fatalf("diagnostic changed the original local teardown event: %+v", deaths)
	}
}

// Goal: a stale captured owner cannot report a send it never attempted.
// Method: retain an earlier session pointer at a reused SID, then require the
// real pointer guard to preserve its replacement with no log or report event.
func TestCapturedSessionTeardownStaleOwnerReportsNoSendFailure(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	var logs bytes.Buffer
	r := newReactorForSnapshot(t)
	r.logger = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	r.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), r.logger)
	bus := newCauseBus()
	r.eventBus = bus
	peer := netip.MustParseAddrPort("10.0.0.9:1701")
	original := &L2TPSession{localSID: 7, remoteSID: 70, state: L2TPSessionEstablished}
	replacement := &L2TPSession{localSID: 7, remoteSID: 71, state: L2TPSessionEstablished}
	insertEstablishedTunnel(t, r, 11, 22, peer, replacement)
	tunnel := r.tunnelsByLocalID[11]

	if r.teardownCapturedSession(tunnel, original, nil, cdnResultGeneralError, l2tpevents.TerminateCauseNASError) {
		t.Fatal("a stale captured session acquired teardown authority")
	}
	if tunnel.lookupSession(7) != replacement {
		t.Fatal("the current transport session was changed by the stale owner")
	}
	if entries := report.Errors(0); len(entries) != 0 {
		t.Fatalf("stale owner reported a control send that was not attempted: %+v", entries)
	}
	if logs.Len() != 0 || len(bus.downEvents()) != 0 {
		t.Fatalf("stale owner logged a send failure or emitted Down: %s", logs.Bytes())
	}
}
