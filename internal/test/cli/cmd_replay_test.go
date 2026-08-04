package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/capture"
)

// bgpMarker is the 16-byte all-ones marker every BGP message carries
// (RFC 4271 Section 4.1).
var bgpMarker = []byte{
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
}

// bgpMessage frames a body as a complete BGP message.
func bgpMessage(msgType byte, body []byte) []byte {
	total := 19 + len(body)
	msg := make([]byte, 0, total)
	msg = append(msg, bgpMarker...)
	msg = append(msg, byte(total>>8), byte(total), msgType)
	return append(msg, body...)
}

// replayOpen builds an OPEN from asn with no optional parameters.
func replayOpen(asn uint16) []byte {
	return bgpMessage(1, []byte{
		4,                         // version
		byte(asn >> 8), byte(asn), // my AS
		0, 90, // hold time
		192, 0, 2, 1, // BGP identifier
		0, // optional parameter length
	})
}

// replayUpdate builds an UPDATE announcing 198.51.100.0/24 via 10.0.0.1.
func replayUpdate() []byte {
	attrs := []byte{
		0x40, 1, 1, 0, // ORIGIN igp
		0x40, 2, 0, // AS_PATH empty
		0x40, 3, 4, 10, 0, 0, 1, // NEXT_HOP 10.0.0.1
	}
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	body = append(body, attrs...)
	body = append(body, 24, 198, 51, 100) // NLRI 198.51.100.0/24
	return bgpMessage(2, body)
}

// writeCapture builds a capture file holding the given wire messages.
func writeCapture(t *testing.T, dir string, msgs ...[]byte) string {
	t.Helper()
	return writeCaptureWithHeader(t, dir, capture.Header{
		Peer: "192.0.2.1", Started: "2026-08-03T00:00:00Z", Coalesce: true,
	}, msgs...)
}

// writeCaptureWithHeader is writeCapture with the header spelled out, for the
// tests that care what identity the file records.
func writeCaptureWithHeader(t *testing.T, dir string, hdr capture.Header, msgs ...[]byte) string {
	t.Helper()
	path := filepath.Join(dir, "session.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create capture: %v", err)
	}
	defer func() { _ = f.Close() }()

	w := capture.NewWriter(f, 0)
	if err := w.WriteHeader(hdr); err != nil {
		t.Fatalf("write header: %v", err)
	}
	for i, m := range msgs {
		if err := w.WriteMessage(time.Unix(int64(i), 0), capture.DirectionReceived, m[18], m, 0, 0); err != nil {
			t.Fatalf("write message %d: %v", i, err)
		}
	}
	return path
}

// VALIDATES: I8 -- the replayed session takes its identity from the capture
// header, an explicit flag beats the header, and a header that records no
// identity still replays on the fallback.
// PREVENTS: replaying an iBGP capture as eBGP. Local AS versus peer AS decides
// which branch OPEN validation and the forwarding rules take, so an invented AS
// stops the replay reproducing the run it was fed. Every other test in this file
// writes a zero-identity header and passes no override, so only the fallback
// branch of resolve would ever run and the precedence would be untested.
func TestReplayIdentityPrecedence(t *testing.T) {
	recorded := capture.Header{LocalAS: 64512, PeerAS: 64512, RouterID: 0x0a0b0c0d}

	t.Run("header beats fallback", func(t *testing.T) {
		localAS, peerAS, routerID := replayIdentity{}.resolve(recorded)
		if localAS != 64512 || peerAS != 64512 || routerID != 0x0a0b0c0d {
			t.Fatalf("resolve = %d/%d/%#x, want the header's 64512/64512/0xa0b0c0d", localAS, peerAS, routerID)
		}
		if localAS == replayFallbackLocalAS || peerAS == replayFallbackPeerAS {
			t.Fatal("resolve returned the fallback while the header carried an identity")
		}
	})

	t.Run("flag beats header", func(t *testing.T) {
		override := replayIdentity{localAS: 65001, peerAS: 65002, routerID: 7}
		localAS, peerAS, routerID := override.resolve(recorded)
		if localAS != 65001 || peerAS != 65002 || routerID != 7 {
			t.Fatalf("resolve = %d/%d/%d, want the override 65001/65002/7", localAS, peerAS, routerID)
		}
	})

	t.Run("identity-less header falls back", func(t *testing.T) {
		localAS, peerAS, routerID := replayIdentity{}.resolve(capture.Header{Peer: "192.0.2.1"})
		if localAS != replayFallbackLocalAS || peerAS != replayFallbackPeerAS || routerID != replayFallbackRouterID {
			t.Fatalf("resolve = %d/%d/%#x, want the fallbacks", localAS, peerAS, routerID)
		}
	})
}

// VALIDATES: I8 end to end -- a capture whose header records an iBGP identity
// replays as that identity, and the report says so.
// PREVENTS: the header fields being written and then ignored by the harness.
func TestReplayUsesTheCapturedIdentity(t *testing.T) {
	dir := t.TempDir()
	path := writeCaptureWithHeader(t, dir, capture.Header{
		Peer: "192.0.2.1", Started: "2026-08-03T00:00:00Z",
		LocalAS: 64512, PeerAS: 64512, RouterID: 0x0a0b0c0d,
	})

	report, err := runReplay(path, replayIdentity{})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if report.LocalAS != 64512 || report.PeerAS != 64512 || report.RouterID != 0x0a0b0c0d {
		t.Fatalf("report identity = %d/%d/%#x, want 64512/64512/0xa0b0c0d",
			report.LocalAS, report.PeerAS, report.RouterID)
	}
}

// VALIDATES: AC-3 -- replaying a captured session drives the same FSM through the
// same transitions and reports the routes the peer announced.
// PREVENTS: a harness that reads a capture without feeding it through the real
// read path, which would reproduce nothing.
func TestReplayDrivesSession(t *testing.T) {
	path := writeCapture(t, t.TempDir(),
		replayOpen(65001),
		bgpMessage(4, nil), // KEEPALIVE
		replayUpdate(),
	)

	report, err := runReplay(path, replayIdentity{})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if report.Peer != "192.0.2.1" {
		t.Fatalf("peer = %q", report.Peer)
	}
	if len(report.Steps) != 3 {
		t.Fatalf("got %d steps, want 3: %+v", len(report.Steps), report.Steps)
	}
	if report.Steps[0].MsgType != "OPEN" || report.Steps[2].MsgType != "UPDATE" {
		t.Fatalf("message types = %q, %q", report.Steps[0].MsgType, report.Steps[2].MsgType)
	}
	// The FSM must actually move: a harness that never reached the real read
	// path would report the same state throughout.
	if report.States[0] == report.States[len(report.States)-1] {
		t.Fatalf("FSM never left %q; the replay did not drive the session", report.States[0])
	}
	if report.Steps[2].Error != "" {
		t.Fatalf("UPDATE replay failed: %s", report.Steps[2].Error)
	}
	found := false
	for _, p := range report.Steps[2].Announced {
		if p == "198.51.100.0/24" {
			found = true
		}
	}
	if !found {
		t.Fatalf("replay did not report the announced prefix: %+v", report.Steps[2])
	}
	// Deterministic clock: the same capture replays to the same report.
	again, err := runReplay(path, replayIdentity{})
	if err != nil {
		t.Fatalf("second replay: %v", err)
	}
	if len(again.States) != len(report.States) {
		t.Fatalf("replay is not deterministic: %v vs %v", report.States, again.States)
	}
	for i := range report.States {
		if report.States[i] != again.States[i] {
			t.Fatalf("replay is not deterministic at step %d: %q vs %q", i, report.States[i], again.States[i])
		}
	}
}

// VALIDATES: AC-5 -- a truncated or corrupt capture yields a clear error naming
// the offending line, and never a panic.
// PREVENTS: replay crashing on a partial file, which is exactly the file an
// operator ships after a daemon crash.
func TestReplayRejectsCorruptCapture(t *testing.T) {
	dir := t.TempDir()
	good := writeCapture(t, dir, replayOpen(65001), bgpMessage(4, nil))
	raw, err := os.ReadFile(good)
	if err != nil {
		t.Fatalf("read capture: %v", err)
	}

	truncated := filepath.Join(dir, "truncated.jsonl")
	if err := os.WriteFile(truncated, raw[:len(raw)-40], 0o600); err != nil {
		t.Fatalf("write truncated: %v", err)
	}
	_, err = runReplay(truncated, replayIdentity{})
	if err == nil {
		t.Fatal("a truncated capture must be rejected")
	}
	if !strings.Contains(err.Error(), "line ") {
		t.Fatalf("error must name the offending line: %v", err)
	}
}

// VALIDATES: R-2 -- a capture written by an unknown schema version is refused
// with a clear message rather than misread.
func TestReplayRejectsUnknownVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.jsonl")
	line := `{"format":"ze-capture","version":42,"peer":"192.0.2.1","started":"x","daemon-version":"t","coalesce":false}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := runReplay(path, replayIdentity{})
	if err == nil {
		t.Fatal("an unknown schema version must be refused")
	}
	if !strings.Contains(err.Error(), "42") {
		t.Fatalf("error must name the version it found: %v", err)
	}
}

// VALIDATES: the command surface -- a missing file argument is a usage error,
// and a file that does not exist is reported rather than treated as empty.
func TestCmdReplayArguments(t *testing.T) {
	if code := cmdReplay(nil); code != 2 {
		t.Fatalf("no argument must be a usage error, got exit %d", code)
	}
	if code := cmdReplay([]string{filepath.Join(t.TempDir(), "absent.jsonl")}); code != 1 {
		t.Fatalf("a missing file must be an error, got exit %d", code)
	}
}
