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
	"strconv"
	"strings"
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
	acct.setClient(client, "nas1", 300*time.Second, addr, nil, "")
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

// When the accounting server is unreachable the
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

// Session teardown is driven only by the
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

// capturedAccountant answers an accountant whose client sends to a capture
// server that answers every Accounting-Request, so a test reads the records a
// real exchange put on the wire.
func capturedAccountant(t *testing.T) (*radiusAcct, *acctCapture) {
	t.Helper()
	sharedKey := []byte("accttest")
	capture := newAcctCapture()
	conn, addr := startAcctServer(t, sharedKey, capture)
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup
	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: addr, SharedKey: sharedKey}},
		Timeout: 2 * time.Second,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() }) //nolint:errcheck // test cleanup
	acct := newRADIUSAcct()
	acct.setClient(client, "test-nas", 300*time.Second, addr, nil, "")
	// A test that panics with acct.mu held (a broken producer under
	// ./le rfc record) would deadlock in Stop, which takes the
	// lock, and hang rather than fail; the panic already ends the binary.
	t.Cleanup(func() {
		if !acct.mu.TryLock() {
			return
		}
		acct.mu.Unlock()
		acct.Stop()
	})
	return acct, capture
}

// startedSessionID drives one SessionIPAssigned for tunnel 1, session 2 into
// acct and answers the Acct-Session-Id its Accounting-Start carried.
func startedSessionID(t *testing.T, acct *radiusAcct, capture *acctCapture) string {
	t.Helper()
	acct.onSessionIPAssigned(&events.SessionIPAssignedPayload{
		TunnelID: 1, SessionID: 2, Username: "alice", PeerAddr: "10.0.0.1",
	})
	start := capture.waitN(t, 1)[0]
	if start.statusType != radius.AcctStatusStart {
		t.Fatalf("status type: got %d, want %d (Start)", start.statusType, radius.AcctStatusStart)
	}
	return start.sessionID
}

// setAcctClock fixes acctNow at the second for the rest of the test.
func setAcctClock(t *testing.T, second int64) {
	t.Helper()
	saved := acctNow
	acctNow = func() time.Time { return time.Unix(second, 0) }
	t.Cleanup(func() { acctNow = saved })
}

// setAcctBootRandom makes the random half of every boot tag the byte b.
func setAcctBootRandom(t *testing.T, b byte) {
	t.Helper()
	saved := acctBootRandom
	acctBootRandom = func(random []byte) {
		for i := range random {
			random[i] = b
		}
	}
	t.Cleanup(func() { acctBootRandom = saved })
}

// bootTagOf answers the third dash-separated field of an Acct-Session-Id, the
// boot tag, failing the test when the id has not the four fields
// "tunnel-session-tag-counter".
func bootTagOf(t *testing.T, id string) string {
	t.Helper()
	fields := strings.Split(id, "-")
	if len(fields) != 4 {
		t.Fatalf("Acct-Session-Id %q: got %d dash-separated fields, want 4 (tunnel-session-tag-counter)", id, len(fields))
	}
	if len(fields[2]) != 24 {
		t.Fatalf("Acct-Session-Id %q: boot tag %q is %d hex digits, want 24", id, fields[2], len(fields[2]))
	}
	return fields[2]
}

// TestRFC2866AcctSessionIDUniqueAcrossRestart simulates a process restart: two
// accountants built in turn, each starting a session on the same tunnel and
// session ids.
//
// VALIDATES: the two Accounting-Start records carry different Acct-Session-Id
// values (AC-3a), through the SessionIPAssigned entry point and a real
// exchange.
// PREVENTS: the process-local counter restarting at 1 and repeating "1-2-1" in
// the server's log after every restart.
//
// RFC requirement: RFC2866-5.5-1 positive -- two accountants built in turn, as
// across a restart, give a session on the same tunnel and session ids two
// different Acct-Session-Id values in their Accounting-Start records.
func TestRFC2866AcctSessionIDUniqueAcrossRestart(t *testing.T) {
	before, beforeCapture := capturedAccountant(t)
	first := startedSessionID(t, before, beforeCapture)
	after, afterCapture := capturedAccountant(t)
	second := startedSessionID(t, after, afterCapture)

	if first == second {
		t.Fatalf("Acct-Session-Id repeated across a restart: %q", first)
	}
}

// TestRFC2866AcctSessionIDUniqueWhenClockRepeats is the restart of a box with no
// RTC whose first session is assigned before NTP answers: both boots read the
// same second.
//
// VALIDATES: the two Acct-Session-Id values differ, their eight time digits are
// equal and their sixteen random digits differ (AC-3d).
// PREVENTS: a tag built from the clock alone, which repeats when two boots
// restore the same saved time.
//
// RFC requirement: RFC2866-5.5-1 positive -- two accountants built in turn
// while the clock reads the same second still give a session on the same
// tunnel and session ids two different Acct-Session-Id values.
func TestRFC2866AcctSessionIDUniqueWhenClockRepeats(t *testing.T) {
	setAcctClock(t, 1756900000)
	before, beforeCapture := capturedAccountant(t)
	first := startedSessionID(t, before, beforeCapture)
	after, afterCapture := capturedAccountant(t)
	second := startedSessionID(t, after, afterCapture)

	if first == second {
		t.Fatalf("Acct-Session-Id repeated across a restart at one clock second: %q", first)
	}
	firstTag, secondTag := bootTagOf(t, first), bootTagOf(t, second)
	if firstTag[:8] != secondTag[:8] {
		t.Errorf("time digits: got %q and %q, want equal at one clock second", firstTag[:8], secondTag[:8])
	}
	if firstTag[8:] == secondTag[8:] {
		t.Errorf("random digits repeated across a restart: %q", firstTag[8:])
	}
}

// TestRFC2866AcctSessionIDTagCarriesBootTime pins the time half of the boot
// tag and when it is read. Untagged: uniqueness is the claim of the two tests
// above, and this one orders boots.
//
// VALIDATES: with the random half fixed, two accountants whose first session
// reads two different seconds give different ids, each tag opening with the
// Unix seconds in hex read at that accountant's first genSessionID (AC-3e).
// A clock moved between construction and the first session, and again before
// the second session, leaves both tags at the first-session second (AC-3f).
// PREVENTS: a tag minted at construction, which runs at package init before
// NTP restores the clock, and a tag minted again per session, which is no boot
// tag.
func TestRFC2866AcctSessionIDTagCarriesBootTime(t *testing.T) {
	setAcctBootRandom(t, 0x5a)
	const s1, s2, s3 = 1756900000, 1756900100, 1756900200

	setAcctClock(t, s1)
	acct := newRADIUSAcct()
	setAcctClock(t, s2)
	first := acct.genSessionID(1, 2)
	setAcctClock(t, s3)
	second := acct.genSessionID(1, 3)

	wantS2 := strconv.FormatInt(s2, 16)
	if got := bootTagOf(t, first)[:8]; got != wantS2 {
		t.Errorf("first session's tag opens %q, want %q (the second read at the first genSessionID)", got, wantS2)
	}
	if got := bootTagOf(t, second)[:8]; got != wantS2 {
		t.Errorf("second session's tag opens %q, want %q (the tag is minted once)", got, wantS2)
	}
	if want := strings.Repeat("5a", 8); bootTagOf(t, first)[8:] != want {
		t.Errorf("random digits: got %q, want %q", bootTagOf(t, first)[8:], want)
	}

	later := newRADIUSAcct()
	third := later.genSessionID(1, 2)
	if third == first {
		t.Fatalf("two accountants reading two seconds share Acct-Session-Id %q", first)
	}
	if got, want := bootTagOf(t, third)[:8], strconv.FormatInt(s3, 16); got != want {
		t.Errorf("later accountant's tag opens %q, want %q", got, want)
	}
}

// TestRFC2866AcctSessionIDKeepsTunnelSessionPrefix keeps the two contracts the
// id format carries.
//
// VALIDATES: one session's Start, Interim-Update and Stop records carry one
// Acct-Session-Id beginning "<tunnel>-<session>-" (AC-3b), and a
// Disconnect-Request carrying an id genSessionID wrote matches the live
// session through findSessions (AC-3c).
// PREVENTS: a format change that splits a session's records or stops a CoA
// matching its session.
func TestRFC2866AcctSessionIDKeepsTunnelSessionPrefix(t *testing.T) {
	acct, capture := capturedAccountant(t)
	startID := startedSessionID(t, acct, capture)
	if !strings.HasPrefix(startID, "1-2-") {
		t.Errorf("Acct-Session-Id %q does not begin with the tunnel-session prefix %q", startID, "1-2-")
	}
	acct.mu.Lock()
	sess := acct.sessions[sessionKey{tunnelID: 1, sessionID: 2}]
	acct.mu.Unlock()
	if sess == nil {
		t.Fatal("the started session is not tracked")
	}
	interim := string(acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusInterimUpdate, 30).FindAttr(radius.AttrAcctSessionID))
	acct.onSessionDown(&events.SessionDownPayload{TunnelID: 1, SessionID: 2})
	records := capture.waitN(t, 1) // one more after the Start
	stop := records[len(records)-1]
	if stop.statusType != radius.AcctStatusStop {
		t.Fatalf("status type: got %d, want %d (Stop)", stop.statusType, radius.AcctStatusStop)
	}
	if interim != startID || stop.sessionID != startID {
		t.Errorf("one session's ids: Start %q, Interim-Update %q, Stop %q, want one value", startID, interim, stop.sessionID)
	}

	secret := []byte("test-rfc2866-prefix-secret")
	fake := oneSessionService()
	addr := coaTestListener(t, secret, nil, fake)
	id := newRADIUSAcct().genSessionID(10, 20)
	resp := sendRawCoAPacket(t, addr, buildCoAPacket(t, radius.CodeDisconnectRequest, secret,
		[]radius.Attr{{Type: radius.AttrAcctSessionID, Value: radius.AttrString(id)}}, time.Now()))
	if resp.Code != radius.CodeDisconnectACK {
		t.Errorf("Disconnect-Request for %q: code %d, want %d (Disconnect-ACK)", id, resp.Code, radius.CodeDisconnectACK)
	}
	if got := fake.teardowns.Load(); got != 1 {
		t.Errorf("teardowns for %q: got %d, want 1", id, got)
	}
}
