// Design: docs/guide/route-reflection.md -- route-server replacement on a source departure.
// Related: rs_withdrawal_ownership_test.go -- the recipient TCP projection these tests read.
// Related: rfc7705_live_behavior_test.go -- lowLiveRouter, real sessions over net.Pipe.
// Related: rs_withdrawal_convergence_test.go -- publishLookupTap and lookupSizes.
// RFC 4271 Section 9.1 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// lowLiveReconnect brings a departed live source back on a new session, the
// way a real peer reconnects: the passive peer returns to Active with a new
// Session, the test hands it a new transport, and the remote sends its OPEN
// (same AS and BGP Identifier as before) and its KEEPALIVE. It returns the new
// remote end. The caller MUST have ended the old session (lowLiveDepart)
// first; the old lowLivePeer's reader is already stopped then.
func lowLiveReconnect(t *testing.T, departed *lowLivePeer, identifier uint32) *lowLivePeer {
	t.Helper()
	p := departed.peer
	lowEventually(t, func() bool { return p.currentSession() != nil && p.SessionState() == fsm.StateActive },
		"the departed source is Active again")
	server, remote := net.Pipe()
	v := &lowLivePeer{peer: p, remote: remote, stopped: make(chan struct{})}
	v.startCapture(t)
	t.Cleanup(func() {
		// MUST close before joining: the reader may be waiting for a header.
		_ = remote.Close()
		_ = server.Close()
		<-v.stopped
	})
	if err := p.acceptConnection(server); err != nil {
		t.Fatal(err)
	}
	s := p.Settings()
	open := &message.Open{Version: 4, MyAS: uint16(s.PeerAS), HoldTime: 90, BGPIdentifier: identifier,
		OptionalParams: []byte{2, 6, 65, 4, 0, 0, byte(s.PeerAS >> 8), byte(s.PeerAS), 2, 6, 1, 4, 0, 1, 0, 1}}
	v.send(t, message.PackTo(open, nil))
	lowEventually(t, func() bool { return p.SessionState() == fsm.StateOpenConfirm }, "reconnect OPEN accepted")
	v.send(t, message.PackTo(message.NewKeepalive(), nil))
	lowEventually(t, func() bool { return p.State() == PeerStateEstablished && !p.pendingSync() }, "reconnect initial sync")
	return v
}

// lowLiveConnect brings p up on its first session through lowLiveReconnect,
// whose steps are the same for a first connection: p is Active with a
// Session, the test hands it a transport, and the remote sends its OPEN
// (four-octet AS and IPv4 unicast) and its KEEPALIVE. It returns the remote end
// once p finished its initial sync. A failure there reads "departed source"
// and "reconnect" although p never had a session.
func lowLiveConnect(t *testing.T, p *Peer, identifier uint32) *lowLivePeer {
	t.Helper()
	return lowLiveReconnect(t, &lowLivePeer{peer: p}, identifier)
}

// adjOutOwedAnnouncedBy reports whether the destination's owed record for
// prefix names source among its announcers.
func adjOutOwedAnnouncedBy(t *testing.T, destination *Peer, prefix netip.Prefix, source *Peer) bool {
	t.Helper()
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	var key adjOutKey
	raw := ownershipLiveNLRI([]netip.Prefix{prefix})
	if err := fwdPathKeyFor(&key.path, family.IPv4Unicast, 0, raw, false, scratch[:]); err != nil {
		t.Fatalf("key for %s: %v", prefix, err)
	}
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	record := table.owed[key]
	if record == nil {
		return false
	}
	for i := range record.announcers {
		if record.announcers[i].source == source {
			return true
		}
	}
	return false
}

// TestRSDepartureAfterReconnectKeepsNewerRoute proves AC-18 over the real
// bgp-rib and bgp-rs. A owns P at route-server client C (B offers P too, with
// a longer AS_PATH). A's session ends, so bgp-rs owes P at C to the departure,
// and A reconnects and re-announces P with a newer, shorter route A2.
//
//   - "owe-first": C's departure lookup is held inside bgp-rib's answer
//     until A2 reached C's writer, so A2 merges into the owed record as a
//     current announcer and moves the epoch. The held round's choice (B,
//     made without A2) is refused and the next round, which no longer
//     excludes A, elects A2.
//   - "reconnect-first": A also holds a full departure batch of prefixes
//     that sort before P (bgp-rs withdrawalBatchSize, sorted per group), so
//     bgp-rs sends P's departure in a second command, after the first one
//     is decided. C's lookup for the first batch is held until A2 reached
//     C, so A2 is at C's writer before P's departure is sent. That
//     departure finds P owned by A's current session and owes nothing for
//     P: C is written nothing for P after A2. A's prefix Q, after P in the
//     same second command and not re-announced, is the fence that shows
//     the departure ran.
//
// In both, C's wire ends on A2's attributes: never A's old route, never B.
//
// VALIDATES: AC-18, an old departure never overrides a newer route of the
// reconnected source.
// PREVENTS: the departure's election, made without A's newer route, leaving
// C on the survivor B.
func TestRSDepartureAfterReconnectKeepsNewerRoute(t *testing.T) {
	t.Run("owe-first", func(t *testing.T) { departureAfterReconnect(t, true) })
	t.Run("reconnect-first", func(t *testing.T) { departureAfterReconnect(t, false) })
}

func departureAfterReconnect(t *testing.T, hold bool) {
	settings := []*PeerSettings{
		acceptingLiveSettings(t, "192.0.2.1", 65000, 65002),
		acceptingLiveSettings(t, "192.0.2.2", 65000, 65003),
		acceptingLiveSettings(t, "192.0.2.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	peers := lowLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("10.31.0.0/24")
	attrsOld := firstASAttrs(4, 65002, 65002)
	attrsB := firstASAttrs(4, 65003, 65003, 65003)
	attrsNew := firstASAttrs(4, 65002)

	ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsB)
	ownershipLiveSend(t, a, nil, attrsOld, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsOld)
	lowEventually(t, func() bool { return adjOutPrefixOwner(t, c.peer, prefix) == a.peer }, "A owns P at C")

	only := netip.MustParsePrefix("10.31.1.0/24")
	if !hold {
		ownershipLiveSend(t, a, nil, attrsOld, only) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, only, attrsOld)
		// bgp-rs withdrawalBatchSize (500) prefixes of 10.30.0.0/16 and
		// 10.29.0.0/16, which sort before P, fill the first command alone.
		batch := make([]netip.Prefix, 0, 500)
		for i := range 500 {
			batch = append(batch, netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(30 - i/256), byte(i % 256), 0}), 24))
		}
		for start := 0; start < len(batch); start += 250 {
			ownershipLiveSend(t, a, nil, attrsOld, batch[start:start+250]...) // RFC 4271 Section 4.3.
		}
		ownershipLiveThrough(t, c, batch[len(batch)-1], attrsOld)
	}

	tap := &lookupSizes{destination: c.peer.Settings().Address, seen: make(map[string]struct{}),
		hold: make(chan struct{})}
	publishLookupTap(t, tap.after)

	lowLiveDepart(t, a)
	if hold {
		lowEventually(t, func() bool {
			requests, _, _ := tap.result()
			return requests >= 1
		}, "C's departure lookup for P is held")
	} else {
		lowEventually(t, func() bool {
			requests, _, _ := tap.result()
			return requests >= 1
		}, "C's lookup for A's first departure batch is held")
	}
	// The BGP Identifier lowLiveRouter gave A, the first peer it started.
	a2 := lowLiveReconnect(t, a, 0x0a000001)
	ownershipLiveSend(t, a2, nil, attrsNew, prefix) // RFC 4271 Section 4.3.
	if hold {
		lowEventually(t, func() bool { return adjOutOwedAnnouncedBy(t, c.peer, prefix, a.peer) }, "A2 merged into C's owed record")
		close(tap.hold)
	} else {
		// A2 reached C's writer while P's departure had not been sent: A2's
		// route contests A's old one at C, so it is owed to the election,
		// which waits behind the held first batch.
		lowEventually(t, func() bool { return adjOutOwedAnnouncedBy(t, c.peer, prefix, a.peer) }, "A2 owed at C before P's departure")
		reached := ownershipLiveRead(t, c)
		writes := len(reached.announcements[prefix])
		close(tap.hold)
		// RFC 4271 Section 9.1.3: Q has no other route, so the departure
		// withdraws it; P's departure, made after A2, owes nothing.
		ownershipLiveDecided(t, c, only)
		lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C owes nothing after the departure")
		fence := netip.MustParsePrefix("10.31.2.0/24")
		ownershipLiveSend(t, b, nil, attrsB, fence) // RFC 4271 Section 4.3.
		after := ownershipLiveThrough(t, c, fence, attrsB)
		// One write for P since A2 arrived, and it is A2's: the departure,
		// sent after A2, adds no write and no withdrawal of its own.
		if got := after.announcements[prefix][writes:]; len(got) != 1 || !bytes.Equal(got[0], attrsNew) {
			t.Fatalf("C was sent %x for P after A2 arrived, want only A2's route %x", got, attrsNew)
		}
		if got := after.withdrawals[prefix]; got != reached.withdrawals[prefix] {
			t.Fatalf("C was sent %d withdrawals of P after A2 arrived, want none", got-reached.withdrawals[prefix])
		}
	}

	// RFC 4271 Section 9.1: the Decision Process selects among the routes
	// it holds, and A's newer route is one of them.
	view := ownershipLiveThrough(t, c, prefix, attrsNew)
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C owes nothing")
	if owner := adjOutPrefixOwner(t, c.peer, prefix); owner != a.peer {
		t.Fatalf("P at C is owned by %v, want the reconnected A", owner)
	}
	if got := view.routes[prefix]; !bytes.Equal(got, attrsNew) {
		t.Fatalf("C holds %x for P, want A's newer route %x", got, attrsNew)
	}
	ownershipLiveEstablished(t, []*lowLivePeer{a2, b, c})
}
