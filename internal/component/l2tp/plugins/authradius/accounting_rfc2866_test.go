// RFC 2866 (RADIUS Accounting) NAS-side behavioral requirements.
//
// VALIDATES: RFC 2866 accounting-client obligations -- Acct-Status-Type is always
// present (Section 5), Acct-Session-Id is unique across the NAS (Section 5.5), and an
// accounting failure never tears down a subscriber session (Section 3).
// PREVENTS: emitting an Accounting-Request without a status type, colliding session
// ids across the NAS, and tearing down a live session when accounting fails.
//
// ze runs the RADIUS accounting client (NAS) role: it generates session ids,
// builds Accounting-Request packets, and sends Start/Interim/Stop records. The
// producers exercised here are:
//   - buildAcctPacket (acct.go) -- every Accounting-Request carries Acct-Status-Type (RFC 2866 Section 5)
//   - genSessionID   (acct.go) -- unique Acct-Session-Id per NAS (RFC 2866 Section 5.5)
//   - sendAcctPacket / onSessionDown (acct.go) -- accounting failures never tear down sessions (RFC 2866 Section 3)

package l2tpauthradius

import (
	"context"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
)

// newDeadServerAcct returns an accounting manager whose RADIUS client points at a
// bound-but-silent UDP socket. Datagrams are delivered but never answered, so every
// accounting exchange fails after its retries are exhausted.
func newDeadServerAcct(t *testing.T) *radiusAcct {
	t.Helper()
	sink, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sink.Close() }) //nolint:errcheck // test cleanup
	addr := sink.LocalAddr().String()

	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: addr, SharedKey: []byte("k")}},
		Timeout: 50 * time.Millisecond,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() }) //nolint:errcheck // test cleanup

	acct := newRADIUSAcct()
	acct.setClient(client, "nas1", 300*time.Second, addr, nil)
	return acct
}

// RFC requirement: RFC2866-5-1 positive -- an Accounting-Request for each lifecycle event
// carries exactly one Acct-Status-Type attribute set to Start (1), Stop (2), or Interim-Update (3).
func TestRFC2866AcctStatusTypePresent(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{username: "alice", acctSessID: "1-2-1"}

	cases := []struct {
		name   string
		status uint8
	}{
		{"start", radius.AcctStatusStart},
		{"stop", radius.AcctStatusStop},
		{"interim", radius.AcctStatusInterimUpdate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pkt := acct.buildAcctPacket(sess, "nas1", nil, tc.status, 0)
			vals := pkt.FindAllAttr(radius.AttrAcctStatusType)
			if len(vals) != 1 {
				t.Fatalf("Acct-Status-Type count: got %d, want 1", len(vals))
			}
			if len(vals[0]) != 4 {
				t.Fatalf("Acct-Status-Type length: got %d, want 4", len(vals[0]))
			}
			if got := binary.BigEndian.Uint32(vals[0]); got != uint32(tc.status) {
				t.Fatalf("Acct-Status-Type value: got %d, want %d", got, tc.status)
			}
		})
	}
}

// RFC requirement: RFC2866-5-1 negative -- the builder never emits an Accounting-Request
// without an Acct-Status-Type, and never with a value outside the Start/Stop/Interim set;
// a packet missing the attribute or carrying an out-of-range value would be non-conformant.
func TestRFC2866AcctStatusTypeNeverOmitted(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{username: "bob", acctSessID: "9-9-9"}

	for _, status := range []uint8{
		radius.AcctStatusStart,
		radius.AcctStatusStop,
		radius.AcctStatusInterimUpdate,
	} {
		pkt := acct.buildAcctPacket(sess, "nas1", nil, status, 0)
		v := pkt.FindAttr(radius.AttrAcctStatusType)
		if v == nil {
			t.Fatalf("status %d: Accounting-Request omitted Acct-Status-Type", status)
		}
		got := binary.BigEndian.Uint32(v)
		if got < 1 || got > 3 {
			t.Fatalf("status %d: Acct-Status-Type %d outside RFC 2866 Section 5 range 1..3", status, got)
		}
	}
}

// RFC requirement: RFC2866-5.5-1 positive -- genSessionID yields a distinct Acct-Session-Id
// on every call, including under concurrency, so all ids in use across the NAS are unique.
func TestRFC2866AcctSessionIDUnique(t *testing.T) {
	acct := newRADIUSAcct()
	const workers, per = 8, 200

	out := make(chan string, workers*per)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range per {
				out <- acct.genSessionID(uint16(w), uint16(i))
			}
		}(w)
	}
	wg.Wait()
	close(out)

	seen := make(map[string]bool, workers*per)
	for id := range out {
		if seen[id] {
			t.Fatalf("duplicate Acct-Session-Id %q", id)
		}
		seen[id] = true
	}
	if len(seen) != workers*per {
		t.Fatalf("unique ids: got %d, want %d", len(seen), workers*per)
	}
}

// RFC requirement: RFC2866-5.5-1 negative -- reusing the same (tunnel, session) pair does
// not collide: the monotonic counter makes each Acct-Session-Id unique even when the tunnel
// and session identifiers repeat, so a recycled session key can never alias a live session.
func TestRFC2866AcctSessionIDNoCollisionOnReusedKey(t *testing.T) {
	acct := newRADIUSAcct()
	id1 := acct.genSessionID(5, 9)
	id2 := acct.genSessionID(5, 9)
	if id1 == id2 {
		t.Fatalf("reused (tunnel, session) produced identical Acct-Session-Id %q", id1)
	}
}

// RFC requirement: RFC2866-3-1 positive -- when the accounting server is unreachable the
// failed Accounting-Start does not tear down the session: it stays tracked and its context
// (which drives the interim-update loop) is left running.
func TestRFC2866AcctFailureKeepsSession(t *testing.T) {
	acct := newDeadServerAcct(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sess := &acctSession{
		tunnelID:   1,
		sessionID:  2,
		username:   "alice",
		acctSessID: "1-2-1",
		startTime:  time.Now(),
		cancel:     cancel,
	}
	acct.mu.Lock()
	acct.sessions[sessionKey{1, 2}] = sess
	acct.mu.Unlock()

	// Synchronous Accounting-Start that fails against the silent server.
	acct.sendAcctStart(acct.client, sess, "nas1", nil)

	acct.mu.Lock()
	_, stillTracked := acct.sessions[sessionKey{1, 2}]
	acct.mu.Unlock()
	if !stillTracked {
		t.Fatal("accounting Start failure tore down the tracked session")
	}
	select {
	case <-ctx.Done():
		t.Fatal("accounting Start failure canceled the session context")
	default:
	}
}

// RFC requirement: RFC2866-3-1 negative -- session teardown is driven only by the
// session-down lifecycle event, and a failing Accounting-Stop does not block it: the
// session is removed and its context canceled even though the accounting exchange fails.
func TestRFC2866SessionTeardownIndependentOfAccounting(t *testing.T) {
	acct := newDeadServerAcct(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sess := &acctSession{
		tunnelID:   3,
		sessionID:  4,
		username:   "bob",
		acctSessID: "3-4-1",
		startTime:  time.Now(),
		cancel:     cancel,
	}
	acct.mu.Lock()
	acct.sessions[sessionKey{3, 4}] = sess
	acct.mu.Unlock()

	acct.onSessionDown(&events.SessionDownPayload{TunnelID: 3, SessionID: 4})

	acct.mu.Lock()
	_, stillTracked := acct.sessions[sessionKey{3, 4}]
	acct.mu.Unlock()
	if stillTracked {
		t.Fatal("session-down did not remove the session")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("session-down did not cancel the session context")
	}
}
