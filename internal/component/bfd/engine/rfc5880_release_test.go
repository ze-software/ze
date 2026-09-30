// VALIDATES: a session whose last client released it stays in the engine,
// matched by the peer's packets, for one Detection Time after the last Control
// packet it received, and is removed on the first tick after that.
// PREVENTS: ReleaseSession dropping the discriminator and the first-packet key
// the moment the last client goes, which RFC 5880 Section 6.8.1 forbids once a
// packet has arrived from the remote end.
package engine

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/core/clock"
)

// steppedClock is a clock.Clock whose Now only moves when the test moves it.
// The loop under test is never started, so only Now is read: tick and
// handleInbound are called on the test goroutine.
type steppedClock struct {
	clock.RealClock
	now time.Time
}

// Now returns the time the test last set.
func (c *steppedClock) Now() time.Time { return c.now }

// RFC requirement: RFC5880-6.8.1-14 positive -- after the last client releases
// a session that has received Control packets, the engine keeps it: a tick one
// nanosecond before LastReceived plus the Detection Time leaves the entry in
// place in AdminDown, and a peer packet naming its discriminator is counted
// against it (rxPackets rises). That packet restarts the interval, so the entry
// is still present one nanosecond before the new deadline and is gone after the
// tick at it: the session map, the discriminator index and the first-packet
// index no longer hold it, and a further peer packet matches nothing.
func TestRFC5880ReleasedSessionPreservedForOneDetectionTime(t *testing.T) {
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	l := NewLoop(&captureTransport{}, clk)
	req := reqFor(addrB, addrA)
	h, err := l.EnsureSession(req)
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	key := req.Key()
	m := machineFor(t, l, key)
	discr := m.LocalDiscriminator()

	// RFC 5880 Section 6.8.6 handshake: Down then Init brings the session Up.
	l.handleInbound(inboundControlState(key.Peer, key.Local, key.Interface, 0, packet.StateDown))
	l.handleInbound(inboundControlState(key.Peer, key.Local, key.Interface, discr, packet.StateInit))
	if got := m.State(); got != packet.StateUp {
		t.Fatalf("precondition: state = %s, want Up", got)
	}
	detect := m.DetectionInterval()
	if detect <= 0 {
		t.Fatalf("precondition: DetectionInterval = %v, want > 0", detect)
	}

	if err := l.ReleaseSession(h); err != nil {
		t.Fatalf("ReleaseSession: %v", err)
	}

	clk.now = clk.now.Add(detect - time.Nanosecond)
	l.tick()
	entry := releasedEntry(t, l, key, discr)
	if got := entry.machine.State(); got != packet.StateAdminDown {
		t.Fatalf("released session state = %s, want AdminDown", got)
	}

	rxBefore := entry.rxPackets
	l.handleInbound(inboundControlState(key.Peer, key.Local, key.Interface, discr, packet.StateDown))
	if entry.rxPackets != rxBefore+1 {
		t.Fatalf("a peer packet inside the Detection Time did not match the released session: rxPackets %d, want %d", entry.rxPackets, rxBefore+1)
	}

	clk.now = clk.now.Add(detect - time.Nanosecond)
	l.tick()
	releasedEntry(t, l, key, discr)

	clk.now = clk.now.Add(time.Nanosecond)
	l.tick()
	l.mu.Lock()
	_, inSessions := l.sessions[key]
	_, inDiscr := l.byDiscr[discr]
	_, inKey := l.byKey[firstPacketIndex(key)]
	l.mu.Unlock()
	if inSessions || inDiscr || inKey {
		t.Fatalf("released session outlived its Detection Time: sessions=%v byDiscr=%v byKey=%v", inSessions, inDiscr, inKey)
	}
	rxAfter := entry.rxPackets
	l.handleInbound(inboundControlState(key.Peer, key.Local, key.Interface, discr, packet.StateDown))
	if entry.rxPackets != rxAfter {
		t.Fatal("a peer packet after the Detection Time still matched the removed session")
	}
}

// RFC requirement: RFC5880-6.8.1-14 negative -- a client that asks again for
// the key of a released session inside its Detection Time (a reload that
// re-requests the session) is the input that would discard the preserved state,
// and the engine keeps it: the same session, on the same discriminator, still
// holding the peer's bfd.RemoteDiscr, its Required Min RX Interval and the time
// of its last packet, now in Down with the new request's Detect Mult, and a
// tick past the old deadline does not remove it. The new request's Required
// Min RX Interval also differs, which RFC 5880 Section 6.8.3 answers with a
// Poll Sequence.
func TestRFC5880ReleasedSessionAskedAgainKeepsRemoteState(t *testing.T) {
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	l := NewLoop(&captureTransport{}, clk)
	req := reqFor(addrB, addrA)
	h, err := l.EnsureSession(req)
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	key := req.Key()
	m := machineFor(t, l, key)
	discr := m.LocalDiscriminator()
	l.handleInbound(inboundControlState(key.Peer, key.Local, key.Interface, 0, packet.StateDown))
	detect := m.DetectionInterval()
	lastReceived := m.LastReceived()
	if got := m.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("precondition: bfd.RemoteDiscr = %#x, want the peer's %#x", got, peerMyDiscr)
	}

	if err := l.ReleaseSession(h); err != nil {
		t.Fatalf("ReleaseSession: %v", err)
	}
	clk.now = clk.now.Add(detect / 2)
	req.DetectMult = 5
	req.RequiredMinRxInterval = 20_000
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession again: %v", err)
	}
	entry := releasedEntry(t, l, key, discr)
	if entry.machine != m {
		t.Fatal("the session asked for again was rebuilt, discarding the state preserved from the remote end")
	}
	if entry.released {
		t.Fatal("the session a new client asked for is still marked released")
	}
	if got := m.State(); got != packet.StateDown {
		t.Fatalf("the revived session is %s, want Down", got)
	}
	if got := m.RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("the revived session lost bfd.RemoteDiscr: %#x, want %#x", got, peerMyDiscr)
	}
	if got := m.RemoteMinRxInterval(); got != 300*time.Millisecond {
		t.Fatalf("the revived session lost bfd.RemoteMinRxInterval: %v, want the peer's 300ms", got)
	}
	if got := m.LastReceived(); !got.Equal(lastReceived) {
		t.Fatalf("the revived session's last receive time moved: %v, want %v", got, lastReceived)
	}
	if got := m.DetectMult(); got != 5 {
		t.Fatalf("the revived session carries Detect Mult %d, want the new request's 5", got)
	}
	if !m.PollOutstanding() {
		t.Fatal("a changed Required Min RX Interval started no Poll Sequence (RFC 5880 Section 6.8.3)")
	}

	clk.now = clk.now.Add(2 * detect)
	l.tick()
	releasedEntry(t, l, key, discr)
}

// releasedEntry returns the entry the loop still holds for key, failing the test
// when the session map or either lookup index has dropped it.
func releasedEntry(t *testing.T, l *Loop, key api.Key, discr uint32) *sessionEntry {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.sessions[key]
	if entry == nil {
		t.Fatal("released session was removed before one Detection Time had passed since the last packet")
	}
	if l.byDiscr[discr] != entry {
		t.Fatal("released session left the discriminator index before one Detection Time")
	}
	if l.byKey[firstPacketIndex(key)] != entry {
		t.Fatal("released session left the first-packet index before one Detection Time")
	}
	return entry
}

// TestSharedRequestJoinsLiveSessionOverReleasedOne checks that a session kept
// only for its Detection Time does not stop RFC 5882 Section 4.4 sharing. Two
// sessions run to one peer from two local addresses; the second is released
// after a packet arrived, so the engine keeps it. A client that names no local
// address matches both, and it MUST join the live one rather than get a third
// session: the released one has no client left to share with.
func TestSharedRequestJoinsLiveSessionOverReleasedOne(t *testing.T) {
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	l := NewLoop(&captureTransport{}, clk)
	liveReq := reqFor(addrB, addrA)
	if _, err := l.EnsureSession(liveReq); err != nil {
		t.Fatalf("EnsureSession live: %v", err)
	}
	keptReq := reqFor(addrB, "203.0.113.3")
	keptHandle, err := l.EnsureSession(keptReq)
	if err != nil {
		t.Fatalf("EnsureSession kept: %v", err)
	}
	kept := keptReq.Key()
	l.handleInbound(inboundControlState(kept.Peer, kept.Local, kept.Interface, 0, packet.StateDown))
	if err := l.ReleaseSession(keptHandle); err != nil {
		t.Fatalf("ReleaseSession: %v", err)
	}

	narrowReq := reqFor(addrB, addrA)
	narrowReq.Local = netip.Addr{}
	h, err := l.EnsureSession(narrowReq)
	if err != nil {
		t.Fatalf("EnsureSession narrow: %v", err)
	}
	hh, ok := h.(*handle)
	if !ok {
		t.Fatalf("EnsureSession returned %T, want *handle", h)
	}
	if hh.key != liveReq.Key() {
		t.Fatalf("the narrower request got session %+v, want the live session %+v", hh.key, liveReq.Key())
	}
	l.mu.Lock()
	count := len(l.sessions)
	l.mu.Unlock()
	if count != 2 {
		t.Fatalf("the engine holds %d sessions, want 2 (live and kept)", count)
	}
}

// TestSharedRequestRevivesTheOnlyReleasedMatch checks the fallback beside
// TestSharedRequestJoinsLiveSessionOverReleasedOne: when the only session a
// narrower request matches is a released one, the request revives it, out of
// AdminDown, rather than open a second session to the same peer.
func TestSharedRequestRevivesTheOnlyReleasedMatch(t *testing.T) {
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	l := NewLoop(&captureTransport{}, clk)
	keptReq := reqFor(addrB, addrA)
	keptHandle, err := l.EnsureSession(keptReq)
	if err != nil {
		t.Fatalf("EnsureSession kept: %v", err)
	}
	kept := keptReq.Key()
	l.handleInbound(inboundControlState(kept.Peer, kept.Local, kept.Interface, 0, packet.StateDown))
	if err := l.ReleaseSession(keptHandle); err != nil {
		t.Fatalf("ReleaseSession: %v", err)
	}

	narrowReq := reqFor(addrB, addrA)
	narrowReq.Local = netip.Addr{}
	h, err := l.EnsureSession(narrowReq)
	if err != nil {
		t.Fatalf("EnsureSession narrow: %v", err)
	}
	hh, ok := h.(*handle)
	if !ok {
		t.Fatalf("EnsureSession returned %T, want *handle", h)
	}
	if hh.key != kept {
		t.Fatalf("the narrower request got session %+v, want the released session %+v", hh.key, kept)
	}
	if got := machineFor(t, l, kept).State(); got != packet.StateDown {
		t.Fatalf("the revived session is %s, want Down", got)
	}
}
