// Design: docs/architecture/doctor-and-health-checks.md -- recent events: an L2TP hidden AVP left hidden
// Related: register_hidden_diagnostic.go -- the code, its registration and reportHiddenLeft
package l2tp

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// Goal: an AVP left hidden is a problem ze doctor reports, naming the peer.
// Method: log one message's hiddenLeft with no secret configured, then decode
// the warning line and read the error ring: the line carries the doctor code
// and the peer, and the event carries the code, the peer as subject and the
// fix of its reason. A second message from the same peer within the window
// writes its line and raises no second event; another peer raises its own.
func TestHiddenLeftCarriesCodeAndRaisesEventForPeer(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	peer := netip.MustParseAddrPort("192.0.2.10:1701")
	left := hiddenLeft{count: 2, attrType: AVPCallingNumber, mandatory: true, err: errHiddenNoSecret}

	left.log(logger, peer)

	var warning map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &warning); err != nil {
		t.Fatalf("decode the warning line: %v\n%s", err, logs.Bytes())
	}
	if warning["code"] != codeL2TPHiddenAVPUnrevealed {
		t.Fatalf("the warning carries code %v, want %s", warning["code"], codeL2TPHiddenAVPUnrevealed)
	}
	if warning["peer"] != peer.String() {
		t.Fatalf("the warning names peer %v, want %s", warning["peer"], peer)
	}
	entries := report.Errors(0)
	if len(entries) != 1 {
		t.Fatalf("one message left AVPs hidden and retained %d events, want one: %+v", len(entries), entries)
	}
	entry := entries[0]
	if entry.Source != SubsystemName || entry.Code != codeL2TPHiddenAVPUnrevealed || entry.Subject != peer.String() {
		t.Fatalf("the event lost its source, code or peer: %+v", entry)
	}
	if entry.Fix != hiddenRevealFix(errHiddenNoSecret) || !strings.Contains(entry.Fix, "shared-secret") {
		t.Fatalf("the event does not carry the no-secret fix: %q", entry.Fix)
	}
	if !strings.Contains(entry.Message, errHiddenNoSecret.Error()) {
		t.Fatalf("the event message does not carry the reason: %q", entry.Message)
	}
	if diagnostic.Lookup(entry.Code) == nil || !slices.Contains(diagnostic.EventCodes(), entry.Code) {
		t.Fatalf("the code is not described and declared to recent-events: %s", entry.Code)
	}

	left.log(logger, peer)
	if got := len(report.Errors(0)); got != 1 {
		t.Fatalf("a second message from the same peer within the window raised again: %d events", got)
	}
	if got := strings.Count(logs.String(), codeL2TPHiddenAVPUnrevealed); got != 2 {
		t.Fatalf("each message writes its line: %d lines carry the code, want 2", got)
	}

	other := netip.MustParseAddrPort("192.0.2.11:1701")
	left.log(logger, other)
	entries = report.Errors(0)
	if len(entries) != 2 || entries[0].Subject != other.String() {
		t.Fatalf("another peer did not raise its own event: %+v", entries)
	}
}
