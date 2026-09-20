// VALIDATES: the event a BFD client receives can tell a neighbor that
// administratively disabled its session from a forwarding path that failed.
// Both arrive as a local Down carrying Diag "neighbor signaled session down",
// so State and Diag together answer neither question; StateChange.
// RemoteAdminDown is the field that does.
// PREVENTS: a client tearing its adjacency down because the operator at the
// far end disabled BFD, which RFC 5882 Section 4.2 says it must not do, and
// the opposite defect of a client sleeping through a real path failure
// because the distinction was drawn on the wrong value.
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/session"
	"github.com/ze-software/ze/internal/core/clock"
)

// upSessionWithSubscriber returns an unstarted loop holding one single-hop
// session driven to Up by a synthetic peer, and a subscription to it. The loop
// is never started, so every handleInbound below runs on the test goroutine
// and each event is in the channel before the test reads it.
func upSessionWithSubscriber(t *testing.T) (*Loop, api.Key, *session.Machine, <-chan api.StateChange) {
	t.Helper()
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	req := reqFor(addrB, addrA)
	h, err := l.EnsureSession(req)
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	key := req.Key()
	m := machineFor(t, l, key)
	sub := h.Subscribe()

	// RFC 5880 Section 6.8.6 handshake: Down then Init brings the local end to
	// Up. The first packet carries Your Discriminator zero, the second the
	// discriminator the peer has by then learned.
	l.handleInbound(inboundControlState(key.Peer, key.Local, key.Interface, 0, packet.StateDown))
	l.handleInbound(inboundControlState(key.Peer, key.Local, key.Interface, m.LocalDiscriminator(), packet.StateInit))
	if got := m.State(); got != packet.StateUp {
		t.Fatalf("session did not reach Up before the test began: state = %s", got)
	}
	return l, key, m, sub
}

// lastChange returns the most recent state change on ch. The loop under test
// is unstarted, so every publication has already happened when this runs and
// an empty channel is a failure rather than a race.
func lastChange(t *testing.T, ch <-chan api.StateChange) api.StateChange {
	t.Helper()
	var (
		last api.StateChange
		got  bool
	)
	for {
		select {
		case change := <-ch:
			last, got = change, true
		default:
			if !got {
				t.Fatal("no state change was published")
			}
			return last
		}
	}
}

// RFC requirement: RFC5882-4.2-1 positive -- RFC 5882 Section 4.2: "If a BFD
// session transitions from Up state to AdminDown, or the session transitions
// from Up to Down because the remote system is indicating that the session is
// in state AdminDown, clients SHOULD NOT take any control protocol action."
// The second case is the one a client cannot see for itself, because RFC 5880
// Section 6.8.6 records the neighbor's AdminDown as a LOCAL Down. This checks
// what the BFD service publishes for it: a peer in AdminDown drives the
// session to Down with RemoteAdminDown true, which is the only value in the
// event that separates this case from a path failure. It checks the event, not
// what a client then does with it.
func TestRFC5882RemoteAdminDownIsDistinguished(t *testing.T) {
	l, key, m, sub := upSessionWithSubscriber(t)

	l.handleInbound(inboundControlState(key.Peer, key.Local, key.Interface, m.LocalDiscriminator(), packet.StateAdminDown))

	change := lastChange(t, sub)
	if change.State != packet.StateDown {
		t.Fatalf("state after the neighbor signaled AdminDown = %s, want %s", change.State, packet.StateDown)
	}
	if !change.RemoteAdminDown {
		t.Fatalf("the neighbor signaled AdminDown and the event does not say so: %+v", change)
	}
	if change.Diag != packet.DiagNeighborSignaledDown {
		t.Fatalf("diag = %s, want %s", change.Diag, packet.DiagNeighborSignaledDown)
	}
}

// RFC requirement: RFC5882-4.2-1 negative -- the suppression the sentence above
// asks for is scoped to the neighbor's AdminDown, and nothing wider. RFC 5882
// Section 4.2.1 asks for the opposite on every other Up to Down transition:
// "action SHOULD be taken in the control protocol to signal the lack of
// connectivity for the path over which BFD is running." So a peer signaling
// plain Down drives the session to Down with RemoteAdminDown FALSE. This case
// also pins why the flag had to exist: the Diag asserted here is the same
// DiagNeighborSignaledDown the positive case asserts, so a client reading
// State and Diag cannot tell the two apart. Without it the positive passes on
// code that sets the flag on every Down.
func TestRFC5882NeighborDownIsNotRemoteAdminDown(t *testing.T) {
	l, key, m, sub := upSessionWithSubscriber(t)

	l.handleInbound(inboundControlState(key.Peer, key.Local, key.Interface, m.LocalDiscriminator(), packet.StateDown))

	change := lastChange(t, sub)
	if change.State != packet.StateDown {
		t.Fatalf("state after the neighbor signaled Down = %s, want %s", change.State, packet.StateDown)
	}
	if change.RemoteAdminDown {
		t.Fatalf("a path failure was reported as the neighbor's AdminDown: %+v", change)
	}
	if change.Diag != packet.DiagNeighborSignaledDown {
		t.Fatalf("diag = %s, want %s: the two cases share a diagnostic, which is why RemoteAdminDown exists", change.Diag, packet.DiagNeighborSignaledDown)
	}
}
