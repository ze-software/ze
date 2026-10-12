// Design: docs/architecture/bfd.md -- session registry, shared sessions
// Related: shared_session.go -- retimeLocked and fastestRequest
//
// VALIDATES: a session shared by several clients runs the timers of the
// client whose request gives the shortest Detection Time, recomputed when a
// client joins, leaves, or edits its request (UpdateSession), whatever the
// order the clients came in (owner decision of 2026-10-12, "Fastest
// detection", after RFC 5882 Section 4.1); UpdateSession changes the timers
// and the authentication of a live session in place, keeping its
// discriminator.
// PREVENTS: a shared session that keeps the first client's slow timers while a
// faster client relies on it, a session that stays fast after the fast client
// left, and a profile edit that cannot reach a live session.
package engine

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
)

// timedReq is reqFor with the timers and profile a client names.
func timedReq(profile string, intervalUs uint32, mult uint8) api.SessionRequest {
	req := reqFor(addrB, addrA)
	req.Profile = profile
	req.DesiredMinTxInterval = intervalUs
	req.RequiredMinRxInterval = intervalUs
	req.DetectMult = mult
	return req
}

// sessionTimers reads what the session's next Control packet advertises and
// the profile the snapshot names, under the loop's lock.
func sessionTimers(t *testing.T, l *Loop, key api.Key) (packet.Control, string) {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.sessions[key]
	if !ok {
		t.Fatalf("no session for %+v", key)
	}
	return entry.machine.Build(), entry.profile
}

// TestSharedSessionRunsFastestDetection opens one session for a slow client
// (300ms x 5) and lets a fast client (50ms x 3) join, then leave, and runs the
// same pair in the other order. The session advertises the fast client's
// Required Min RX and Detect Mult while it is attached, and the slow client's
// once it has left.
// MUTATION: retimeLocked returning without ApplyTimers leaves the session on
// the first client's timers, so this test goes red.
func TestSharedSessionRunsFastestDetection(t *testing.T) {
	slow := timedReq("slow", 300_000, 5)
	fast := timedReq("fast", 50_000, 3)
	key := slow.Key()

	l := NewLoop(&captureTransport{}, clock.RealClock{})
	slowHandle, err := l.EnsureSession(slow)
	if err != nil {
		t.Fatalf("EnsureSession slow: %v", err)
	}
	fastHandle, err := l.EnsureSession(fast)
	if err != nil {
		t.Fatalf("EnsureSession fast: %v", err)
	}
	if c, profile := sessionTimers(t, l, key); c.RequiredMinRxInterval != 50_000 || c.DetectMult != 3 || profile != "fast" {
		t.Fatalf("with both clients: rx %d mult %d profile %q, want the fast client's 50000 x 3", c.RequiredMinRxInterval, c.DetectMult, profile)
	}
	if err := l.ReleaseSession(fastHandle); err != nil {
		t.Fatalf("ReleaseSession fast: %v", err)
	}
	if c, profile := sessionTimers(t, l, key); c.RequiredMinRxInterval != 300_000 || c.DetectMult != 5 || profile != "slow" {
		t.Fatalf("after the fast client left: rx %d mult %d profile %q, want the slow client's 300000 x 5", c.RequiredMinRxInterval, c.DetectMult, profile)
	}
	if err := l.ReleaseSession(slowHandle); err != nil {
		t.Fatalf("ReleaseSession slow: %v", err)
	}

	l2 := NewLoop(&captureTransport{}, clock.RealClock{})
	if _, err := l2.EnsureSession(fast); err != nil {
		t.Fatalf("EnsureSession fast first: %v", err)
	}
	if _, err := l2.EnsureSession(slow); err != nil {
		t.Fatalf("EnsureSession slow second: %v", err)
	}
	if c, profile := sessionTimers(t, l2, key); c.RequiredMinRxInterval != 50_000 || c.DetectMult != 3 || profile != "fast" {
		t.Fatalf("slow joining fast: rx %d mult %d profile %q, want the fast client's 50000 x 3", c.RequiredMinRxInterval, c.DetectMult, profile)
	}
}

// TestFastestRequestOrder pins the order fastestRequest ranks requests in:
// Detect Mult times Required Min RX first, then Detect Mult times Desired Min
// TX, then the profile name, so the choice never depends on join order. A zero
// field counts as the engine's default, as the machine reads it.
func TestFastestRequestOrder(t *testing.T) {
	withTx := func(req api.SessionRequest, txUs uint32) api.SessionRequest {
		req.DesiredMinTxInterval = txUs
		return req
	}
	cases := []struct {
		name string
		reqs []api.SessionRequest
		want string
	}{
		{"shorter receive detection wins", []api.SessionRequest{timedReq("a", 100_000, 5), timedReq("b", 200_000, 2)}, "b"},
		{"equal receive detection, shorter transmit wins", []api.SessionRequest{withTx(timedReq("a", 100_000, 3), 300_000), withTx(timedReq("b", 100_000, 3), 200_000)}, "b"},
		{"all equal, profile name decides", []api.SessionRequest{timedReq("b", 100_000, 3), timedReq("a", 100_000, 3)}, "a"},
		{"zero fields read as the defaults", []api.SessionRequest{timedReq("zero", 0, 0), timedReq("second", 1_000_000, 4)}, "zero"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clients := make([]*sessionClient, 0, len(tc.reqs))
			for _, req := range tc.reqs {
				clients = append(clients, &sessionClient{key: req.Key(), req: req})
			}
			if got := fastestRequest(clients).Profile; got != tc.want {
				t.Fatalf("fastestRequest chose %q, want %q", got, tc.want)
			}
		})
	}
}

// TestUpdateSessionRetimesInPlace edits a sole client's timers through
// UpdateSession: the loop still holds one session, with the same local
// discriminator, advertising the new timers.
func TestUpdateSessionRetimesInPlace(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	req := timedReq("p", 300_000, 3)
	h, err := l.EnsureSession(req)
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	before := l.Snapshot()

	if err := l.UpdateSession(h, timedReq("p", 100_000, 4)); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}

	after := l.Snapshot()
	if len(after) != 1 || after[0].LocalDiscr != before[0].LocalDiscr {
		t.Fatalf("after the edit: %d sessions, discriminator %d, want one session keeping %d", len(after), after[0].LocalDiscr, before[0].LocalDiscr)
	}
	if c, _ := sessionTimers(t, l, req.Key()); c.RequiredMinRxInterval != 100_000 || c.DetectMult != 4 {
		t.Fatalf("after the edit: rx %d mult %d, want 100000 x 4", c.RequiredMinRxInterval, c.DetectMult)
	}
}

// TestUpdateSessionAuthEdit gives a session's only client's profile an auth
// block through UpdateSession: a client refused before the edit for asking
// for that authentication joins after it, and the loop holds one session. An
// edit that would change the authentication under a client of another profile
// is refused with ErrAuthMismatch and changes nothing.
func TestUpdateSessionAuthEdit(t *testing.T) {
	secure := &api.AuthSettings{Type: packet.AuthTypeKeyedSHA1, KeyID: 3, Secret: []byte("EDIT-SECRET")}
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	pinned := timedReq("p", 300_000, 3)
	h, err := l.EnsureSession(pinned)
	if err != nil {
		t.Fatalf("EnsureSession pinned: %v", err)
	}
	joiner := timedReq("p", 300_000, 3)
	joiner.Auth = secure
	if _, err := l.EnsureSession(joiner); !errors.Is(err, ErrAuthMismatch) {
		t.Fatalf("join before the edit: err %v, want ErrAuthMismatch", err)
	}

	edited := pinned
	edited.Auth = secure
	if err := l.UpdateSession(h, edited); err != nil {
		t.Fatalf("UpdateSession with auth: %v", err)
	}
	if _, err := l.EnsureSession(joiner); err != nil {
		t.Fatalf("join after the edit: %v", err)
	}
	if snap := l.Snapshot(); len(snap) != 1 || snap[0].Refcount != 2 {
		t.Fatalf("after the join: %d sessions, want one at refcount 2", len(snap))
	}

	l2 := NewLoop(&captureTransport{}, clock.RealClock{})
	h2, err := l2.EnsureSession(timedReq("p", 300_000, 3))
	if err != nil {
		t.Fatalf("EnsureSession p: %v", err)
	}
	if _, err := l2.EnsureSession(timedReq("other", 300_000, 3)); err != nil {
		t.Fatalf("EnsureSession other: %v", err)
	}
	if err := l2.UpdateSession(h2, edited); !errors.Is(err, ErrAuthMismatch) {
		t.Fatalf("edit under a client of another profile: err %v, want ErrAuthMismatch", err)
	}
	l2.mu.Lock()
	hasAuth := l2.sessions[edited.Key()].machine.HasAuth()
	l2.mu.Unlock()
	if hasAuth {
		t.Fatal("a refused edit gave the session authentication")
	}
}

// TestUpdateSessionRefusesAnotherKey keeps a handle bound to its session: an
// edit that names another remote system is a caller defect and is refused.
func TestUpdateSessionRefusesAnotherKey(t *testing.T) {
	lb, _ := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	l := NewLoop(lb, clock.RealClock{})
	h, err := l.EnsureSession(timedReq("p", 300_000, 3))
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	moved := timedReq("p", 300_000, 3)
	moved.Peer = netip.MustParseAddr("203.0.113.77")
	if err := l.UpdateSession(h, moved); err == nil {
		t.Fatal("an edit naming another peer was accepted")
	}
}

// TestReprofileAppliesEchoInterval edits the echo interval of a sole client's
// profile through Reprofile: the live session advertises the new Required
// Min Echo RX Interval in its next Control packet, keeping its discriminator.
// MUTATION: retimeLocked not calling ApplyEcho keeps advertising zero, so this
// test goes red.
func TestReprofileAppliesEchoInterval(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	req := timedReq("p", 300_000, 3)
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	before := l.Snapshot()

	err := l.Reprofile(func(r api.SessionRequest) (api.SessionRequest, error) {
		r.DesiredMinEchoTxInterval = 150_000
		return r, nil
	})
	if err != nil {
		t.Fatalf("Reprofile: %v", err)
	}

	if after := l.Snapshot(); after[0].LocalDiscr != before[0].LocalDiscr {
		t.Fatalf("the edit replaced the session: discriminator %d, want %d", after[0].LocalDiscr, before[0].LocalDiscr)
	}
	if c, _ := sessionTimers(t, l, req.Key()); c.RequiredMinEchoRxInterval != 150_000 {
		t.Fatalf("advertised Required Min Echo RX %d, want 150000", c.RequiredMinEchoRxInterval)
	}
}

// TestReprofileAppliesPassive makes a sole client's profile passive through
// Reprofile while its session is Down and knows no remote discriminator: the
// session stops transmitting, as RFC 5880 Section 6.8.7 requires of a passive
// system, and keeps its discriminator.
// MUTATION: retimeLocked not calling ApplyRole keeps the session sending, so
// this test goes red.
func TestReprofileAppliesPassive(t *testing.T) {
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	req := timedReq("p", 300_000, 3)
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	nextTx := func() time.Time {
		l.mu.Lock()
		defer l.mu.Unlock()
		return l.sessions[req.Key()].machine.NextTxDeadline()
	}
	if nextTx().IsZero() {
		t.Fatal("precondition: the active session has no TX deadline")
	}

	err := l.Reprofile(func(r api.SessionRequest) (api.SessionRequest, error) {
		r.Passive = true
		return r, nil
	})
	if err != nil {
		t.Fatalf("Reprofile: %v", err)
	}

	if !nextTx().IsZero() {
		t.Fatal("after the edit to passive: the session with no remote discriminator still transmits")
	}
}

// TestFastestRequestTieIsOrderFree ranks requests equal in both Detection
// Times and the profile name, which differ only in what the session takes
// from the chosen one besides the timers (the role, the echo interval) or in
// the client's key. The answer is the same whichever client joined first.
// MUTATION: fasterRequest answering false on a full tie keeps the first
// client, so one order of each pair goes red.
func TestFastestRequestTieIsOrderFree(t *testing.T) {
	passive := timedReq("p", 100_000, 3)
	passive.Passive = true
	echo := timedReq("p", 100_000, 3)
	echo.DesiredMinEchoTxInterval = 50_000
	pinned := timedReq("p", 100_000, 3)
	pinned.Local = netip.MustParseAddr("192.0.2.200")
	cases := []struct {
		name  string
		a, b  api.SessionRequest
		check func(api.SessionRequest) bool
	}{
		{"active before passive", timedReq("p", 100_000, 3), passive, func(r api.SessionRequest) bool { return !r.Passive }},
		{"echo before no echo", timedReq("p", 100_000, 3), echo, func(r api.SessionRequest) bool { return r.DesiredMinEchoTxInterval == 50_000 }},
		{"smaller local address first", timedReq("p", 100_000, 3), pinned, func(r api.SessionRequest) bool { return r.Local == pinned.Local }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, order := range [][]api.SessionRequest{{tc.a, tc.b}, {tc.b, tc.a}} {
				clients := []*sessionClient{{key: order[0].Key(), req: order[0]}, {key: order[1].Key(), req: order[1]}}
				if got := fastestRequest(clients); !tc.check(got) {
					t.Fatalf("join order %+v chose %+v", order, got)
				}
			}
		})
	}
}
