package reactor

import (
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestRouterIDConflictError verifies the error type used for NOTIFICATION dispatch.
//
// VALIDATES: Error message includes router-ID, ASN, and conflicting address.
// VALIDATES: NotifyCodes returns OPEN Message Error / Bad BGP Identifier.
// VALIDATES: errors.As interface matches session.go dispatch pattern.
// PREVENTS: Silent rejection without proper NOTIFICATION to the peer.
func TestRouterIDConflictError(t *testing.T) {
	err := &routerIDConflictError{
		conflictAddr: netip.MustParseAddr("192.0.2.1"),
		peerAS:       65001,
		bgpID:        0x01020304, // 1.2.3.4
	}

	// Error message should contain all diagnostic info.
	msg := err.Error()
	assert.Contains(t, msg, "1.2.3.4", "should contain router-ID")
	assert.Contains(t, msg, "65001", "should contain ASN")
	assert.Contains(t, msg, "192.0.2.1", "should contain conflicting peer address")

	// NotifyCodes must match RFC 4271 OPEN Message Error / Bad BGP Identifier.
	code, sub := err.NotifyCodes()
	assert.Equal(t, uint8(message.NotifyOpenMessage), code, "code should be OPEN Message Error")
	assert.Equal(t, message.NotifyOpenBadBGPID, sub, "subcode should be Bad BGP Identifier")

	// Must satisfy the interface used by session.go for NOTIFICATION dispatch.
	var valErr interface{ NotifyCodes() (uint8, uint8) }
	assert.True(t, errors.As(err, &valErr), "should satisfy NotifyCodes interface via errors.As")
}

// newClaimReactor builds a reactor with no plugin dispatcher, so validateOpen resolves to
// the RFC 6286 Section 2.1 identifier claim and nothing else.
func newClaimReactor(t *testing.T, allowShared bool) *Reactor {
	t.Helper()
	r := New(&Config{AllowSharedRouterID: allowShared})
	r.eventDispatcher = nil
	return r
}

// claimPeer creates a peer at addr in peerAS, attaches it to r, and runs the production
// OPEN-validation path for a peer presenting bgpID. Returns the peer and validateOpen's error.
func claimPeer(t *testing.T, r *Reactor, addr string, peerAS, bgpID uint32) (*Peer, error) {
	t.Helper()

	const localAS uint32 = 65001
	const localRID uint32 = 0x01020301

	settings := NewPeerSettings(netip.MustParseAddr(addr), localAS, peerAS, localRID)
	peer := NewPeer(settings)
	peer.SetReactor(r)

	r.mu.Lock()
	r.peers[settings.PeerKey()] = peer
	r.mu.Unlock()

	localOpen := &message.Open{Version: 4, MyAS: uint16(localAS), HoldTime: 90, BGPIdentifier: localRID}
	remoteOpen := &message.Open{
		Version:       4,
		MyAS:          uint16(peerAS), //nolint:gosec // Test uses AS numbers < 65536
		HoldTime:      90,
		BGPIdentifier: bgpID,
	}

	err := peer.validateOpen(addr, localOpen, remoteOpen)
	return peer, err
}

// TestValidateOpenAllowSharedRouterID verifies the bgp/session/allow-shared-router-id
// opt-out gates the AS-wide uniqueness rejection in validateOpen.
//
// VALIDATES: default (AllowSharedRouterID=false) rejects a peer whose BGP Identifier
// duplicates a same-AS peer that already claimed it (a routerIDConflictError).
// VALIDATES: opt-in (AllowSharedRouterID=true) accepts it -- the AS112 v4+v6 case.
// PREVENTS: the load-dependent check-then-act race being the only behavior; and a
// zero-value reactor.Config silently DISABLING enforcement (fail-closed: false enforces).
func TestValidateOpenAllowSharedRouterID(t *testing.T) {
	// An ESTABLISHED iBGP peer in AS 65001, router-id 1.2.3.4, is the realistic shape of
	// the holder: it went through the full handshake before the second peer shows up.
	dup, cleanup := makeEstablishedPeerWithID(t, "192.0.2.1", 65001, 0x01020304)
	defer cleanup()

	// The validating peer presents the SAME AS and SAME BGP Identifier from a
	// DIFFERENT address -- the exact shape RFC 6286 permits and 345 exercises.
	localOpen := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020301}
	remoteOpen := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020304}

	cases := []struct {
		name         string
		allow        bool
		wantConflict bool
	}{
		{"default enforces uniqueness (reject)", false, true},
		{"opt-in accepts shared router-id", true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newClaimReactor(t, c.allow)
			dup.SetReactor(r)
			r.mu.Lock()
			r.peers[dup.settings.PeerKey()] = dup
			r.mu.Unlock()
			// The holder claims its identifier the same way production does: through its
			// own OPEN validation.
			require.NoError(t, dup.validateOpen("192.0.2.1", localOpen, remoteOpen),
				"the first peer to present the identifier must be accepted")

			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65001, 65001, 0x01020301)
			p := NewPeer(settings)
			p.SetReactor(r)

			err := p.validateOpen("192.0.2.2", localOpen, remoteOpen)
			var conflictErr *routerIDConflictError
			if c.wantConflict {
				require.ErrorAs(t, err, &conflictErr, "duplicate router-id must be rejected by default")
			} else {
				require.NoError(t, err, "shared router-id must be accepted when opted in")
			}
		})
	}
}

// makeEstablishedPeerWithID creates a peer with an ESTABLISHED session
// and a known remote BGP Identifier. Used for router-ID uniqueness tests.
func makeEstablishedPeerWithID(t *testing.T, addr string, peerAS, remoteRID uint32) (*Peer, func()) {
	t.Helper()

	const localAS uint32 = 65001
	const localRID uint32 = 0x01020301

	settings := NewPeerSettings(netip.MustParseAddr(addr), localAS, peerAS, localRID)
	settings.Connection = ConnectionPassive
	peer := NewPeer(settings)

	session := NewSession(settings)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	err := session.Start()
	require.NoError(t, err)

	client, server := net.Pipe()
	collisionAcceptWithReader(t, session, server, client)

	// Send OPEN → reach OPENCONFIRM.
	peerOpen := &message.Open{
		Version:       4,
		MyAS:          uint16(peerAS), //nolint:gosec // Test uses AS numbers < 65536
		HoldTime:      90,
		BGPIdentifier: remoteRID,
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		if _, err := client.Write(openBytes); err != nil {
			return
		}
		buf := make([]byte, 4096)
		if _, err := client.Read(buf); err != nil {
			return // Drain session's KEEPALIVE response
		}
	}()
	err = session.ReadAndProcess()
	require.NoError(t, err)

	// Send KEEPALIVE → reach ESTABLISHED.
	keepalive := message.NewKeepalive()
	keepaliveBytes := message.PackTo(keepalive, nil)
	go func() {
		if _, err := client.Write(keepaliveBytes); err != nil {
			return
		}
	}()
	err = session.ReadAndProcess()
	require.NoError(t, err)

	require.Equal(t, fsm.StateEstablished, session.State())

	return peer, func() {
		if err := client.Close(); err != nil {
			t.Logf("cleanup client: %v", err)
		}
		if err := server.Close(); err != nil {
			t.Logf("cleanup server: %v", err)
		}
	}
}

// makeOpenConfirmPeerWithID creates a peer with a session in OPENCONFIRM state.
func makeOpenConfirmPeerWithID(t *testing.T, addr string, peerAS, remoteRID uint32) (*Peer, func()) {
	t.Helper()

	const localAS uint32 = 65001
	const localRID uint32 = 0x01020301

	settings := NewPeerSettings(netip.MustParseAddr(addr), localAS, peerAS, localRID)
	settings.Connection = ConnectionPassive
	peer := NewPeer(settings)

	session := NewSession(settings)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	err := session.Start()
	require.NoError(t, err)

	client, server := net.Pipe()
	collisionAcceptWithReader(t, session, server, client)

	// Send OPEN only → OPENCONFIRM (no KEEPALIVE).
	peerOpen := &message.Open{
		Version:       4,
		MyAS:          uint16(peerAS), //nolint:gosec // Test uses AS numbers < 65536
		HoldTime:      90,
		BGPIdentifier: remoteRID,
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		if _, err := client.Write(openBytes); err != nil {
			return
		}
		buf := make([]byte, 4096)
		if _, err := client.Read(buf); err != nil {
			return // Drain session's KEEPALIVE response
		}
	}()
	err = session.ReadAndProcess()
	require.NoError(t, err)

	require.Equal(t, fsm.StateOpenConfirm, session.State())

	return peer, func() {
		if err := client.Close(); err != nil {
			t.Logf("cleanup client: %v", err)
		}
		if err := server.Close(); err != nil {
			t.Logf("cleanup server: %v", err)
		}
	}
}

// TestRouterIDConflictIBGPDuplicate verifies that duplicate router-IDs
// are detected between iBGP peers in the same AS.
//
// VALIDATES: Two iBGP peers with the same remote BGP ID are detected as a conflict.
// VALIDATES: The conflict names the peer that holds the identifier.
// PREVENTS: Silent misconfiguration where iBGP peers share a router-ID,
// breaking ORIGINATOR_ID loop detection.
func TestRouterIDConflictIBGPDuplicate(t *testing.T) {
	r := newClaimReactor(t, false)

	// Peer A: iBGP (local=65001, peer=65001), remote BGP ID = 1.2.3.4
	_, err := claimPeer(t, r, "192.0.2.1", 65001, 0x01020304)
	require.NoError(t, err, "first peer claims the identifier")

	// New peer in same iBGP AS with same router-ID → conflict.
	_, err = claimPeer(t, r, "192.0.2.99", 65001, 0x01020304)
	var conflictErr *routerIDConflictError
	require.ErrorAs(t, err, &conflictErr, "should detect duplicate router-ID in iBGP AS")
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), conflictErr.conflictAddr)
}

// TestRouterIDConflictEBGPSameAS verifies that duplicate router-IDs are
// detected between eBGP peers sharing the same remote AS.
//
// VALIDATES: Two peers in the same remote AS with the same BGP ID conflict.
// VALIDATES: The conflict names the peer that holds the identifier.
// PREVENTS: Two distinct routers in the same AS presenting identical router-IDs.
func TestRouterIDConflictEBGPSameAS(t *testing.T) {
	r := newClaimReactor(t, false)

	// Peer A: eBGP (local=65001, peer=65002), remote BGP ID = 5.6.7.8
	_, err := claimPeer(t, r, "192.0.2.1", 65002, 0x05060708)
	require.NoError(t, err, "first peer claims the identifier")

	// New peer also in AS 65002 with same router-ID → conflict.
	_, err = claimPeer(t, r, "192.0.2.99", 65002, 0x05060708)
	var conflictErr *routerIDConflictError
	require.ErrorAs(t, err, &conflictErr, "should detect duplicate router-ID in same remote AS")
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), conflictErr.conflictAddr)
}

// TestRouterIDConflictDifferentAS verifies that the same router-ID in
// different ASNs does NOT trigger a conflict.
//
// VALIDATES: Router-IDs are scoped per-AS; different ASNs may reuse IDs.
// PREVENTS: False positive conflict detection across AS boundaries.
func TestRouterIDConflictDifferentAS(t *testing.T) {
	r := newClaimReactor(t, false)

	// Peer A: in AS 65002, router-ID 1.2.3.4
	_, err := claimPeer(t, r, "192.0.2.1", 65002, 0x01020304)
	require.NoError(t, err, "first peer claims the identifier")

	// New peer in AS 65003 with same router-ID → no conflict (different AS).
	_, err = claimPeer(t, r, "192.0.2.99", 65003, 0x01020304)
	assert.NoError(t, err, "different ASN should not conflict even with same router-ID")

	// Both peers hold the identifier, each under its own AS: uniqueness is AS-scoped.
	_, heldIn65002 := r.routerIDs.holder(65002, 0x01020304)
	assert.True(t, heldIn65002, "the AS 65002 claim survives")
	_, heldIn65003 := r.routerIDs.holder(65003, 0x01020304)
	assert.True(t, heldIn65003, "the AS 65003 claim coexists with it")
}

// TestRouterIDConflictDifferentRouterID verifies that peers in the same AS
// with different router-IDs do NOT conflict.
//
// VALIDATES: Only duplicate router-IDs trigger conflict.
// PREVENTS: Over-aggressive rejection of valid peer configurations.
func TestRouterIDConflictDifferentRouterID(t *testing.T) {
	r := newClaimReactor(t, false)

	// Peer A: in AS 65001, router-ID 1.2.3.4
	_, err := claimPeer(t, r, "192.0.2.1", 65001, 0x01020304)
	require.NoError(t, err, "first peer claims the identifier")

	// New peer in same AS with different router-ID → no conflict.
	_, err = claimPeer(t, r, "192.0.2.99", 65001, 0x05060708)
	assert.NoError(t, err, "different router-ID in same AS should not conflict")
}

// TestRouterIDConflictNotEstablished covers a peer whose session has NOT reached
// ESTABLISHED (it stopped at OPENCONFIRM) but whose OPEN was validated.
//
// The assertion is deliberately the opposite of what it was before RFC 6286 Section 2.1
// enforcement moved from an established-peers scan to a claim taken during OPEN validation:
// waiting for ESTABLISHED was the check-then-act race, because two peers presenting one
// identifier at the same instant each saw the other as "not established yet" and BOTH were
// accepted. The identifier is now held from OPEN validation onward.
//
// VALIDATES: A peer holds its identifier from OPEN validation, before ESTABLISHED.
// PREVENTS: The load-dependent double-accept of a duplicate router-ID.
func TestRouterIDConflictNotEstablished(t *testing.T) {
	r := newClaimReactor(t, false)

	// Peer A: same AS, same router-ID, only in OPENCONFIRM.
	peerA, cleanupA := makeOpenConfirmPeerWithID(t, "192.0.2.1", 65001, 0x01020304)
	defer cleanupA()

	peerA.SetReactor(r)
	r.mu.Lock()
	r.peers[peerA.settings.PeerKey()] = peerA
	r.mu.Unlock()

	localOpen := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020301}
	remoteOpen := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020304}
	require.NoError(t, peerA.validateOpen("192.0.2.1", localOpen, remoteOpen),
		"the OPENCONFIRM peer claims the identifier at OPEN validation")
	require.NotEqual(t, fsm.StateEstablished, peerA.SessionState(),
		"the holder is deliberately NOT established")

	// Same router-ID from another peer → conflict, even though the holder never established.
	_, err := claimPeer(t, r, "192.0.2.99", 65001, 0x01020304)
	var conflictErr *routerIDConflictError
	require.ErrorAs(t, err, &conflictErr, "a not-yet-established holder must still block a duplicate")
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), conflictErr.conflictAddr)
}

// TestRouterIDConflictSelfExcluded verifies that a peer does not
// conflict with itself.
//
// VALIDATES: The holder re-validating its own OPEN is granted the claim again.
// PREVENTS: Self-conflict when a peer reconnects or re-runs OPEN validation.
func TestRouterIDConflictSelfExcluded(t *testing.T) {
	r := newClaimReactor(t, false)

	// Peer A: claims router-ID 1.2.3.4
	peerA, err := claimPeer(t, r, "192.0.2.1", 65001, 0x01020304)
	require.NoError(t, err, "first claim granted")

	localOpen := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020301}
	remoteOpen := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020304}

	// Re-validating the SAME peer must not conflict with its own claim.
	err = peerA.validateOpen("192.0.2.1", localOpen, remoteOpen)
	assert.NoError(t, err, "peer should not conflict with itself")
}

// TestRouterIDConflictNilSession verifies that peers which never validated an OPEN
// (configured but not connected) hold nothing and cause no false conflict.
//
// VALIDATES: A peer without a session holds no identifier.
// PREVENTS: Nil pointer dereference or a false conflict against an unconnected peer.
func TestRouterIDConflictNilSession(t *testing.T) {
	r := newClaimReactor(t, false)

	// Peer A: configured but no session yet, so it never claimed anything.
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020301)
	peerA := NewPeer(settings)
	peerA.SetReactor(r)
	r.mu.Lock()
	r.peers[settings.PeerKey()] = peerA
	r.mu.Unlock()

	_, held := r.routerIDs.holder(65001, 0x01020304)
	assert.False(t, held, "an unconnected peer holds no identifier")

	// Same AS and same router-ID but the configured peer never validated an OPEN → no conflict.
	_, err := claimPeer(t, r, "192.0.2.99", 65001, 0x01020304)
	assert.NoError(t, err, "peer without session should not trigger conflict")
}

// TestRouterIDConflictMultiplePeers verifies conflict detection across
// multiple peers where only one conflicts.
//
// VALIDATES: Correct peer identified among several in the same AS.
// PREVENTS: Off-by-one or early-exit bugs in claim lookup.
func TestRouterIDConflictMultiplePeers(t *testing.T) {
	r := newClaimReactor(t, false)

	// Peer A: AS 65002, router-ID 1.2.3.4 (different from check).
	_, err := claimPeer(t, r, "192.0.2.1", 65002, 0x01020304)
	require.NoError(t, err)

	// Peer B: AS 65003, router-ID 5.6.7.8 (different AS from check).
	_, err = claimPeer(t, r, "192.0.2.2", 65003, 0x05060708)
	require.NoError(t, err)

	// Peer C: AS 65002, router-ID 5.6.7.8 (THIS one conflicts).
	_, err = claimPeer(t, r, "192.0.2.3", 65002, 0x05060708)
	require.NoError(t, err)

	// New peer in AS 65002 with router-ID 5.6.7.8 → conflicts with Peer C.
	_, err = claimPeer(t, r, "192.0.2.99", 65002, 0x05060708)
	var conflictErr *routerIDConflictError
	require.ErrorAs(t, err, &conflictErr, "should detect conflict with Peer C")
	assert.Equal(t, netip.MustParseAddr("192.0.2.3"), conflictErr.conflictAddr,
		"should identify Peer C as conflicting")
}

// TestRouterIDClaimConcurrentOnlyOneWins is the regression test for the load-dependent
// check-then-act race (spec-fixit-load-dependent-functional-failures, failure 345).
//
// RFC requirement: RFC6286-2.1-2 positive -- the BGP Identifier is kept unique within an
// AS: of N peers of one AS presenting one identifier simultaneously, exactly one is
// accepted and every other is rejected with Bad BGP Identifier.
//
// VALIDATES: N peers of one AS presenting one BGP Identifier at the same instant produce
// exactly ONE accepted peer; every other one is rejected with a routerIDConflictError.
// VALIDATES: The registry names the winner as the holder afterwards.
// PREVENTS: The scheduling-dependent outcome where none (or several) of the racing peers
// is rejected because none had reached ESTABLISHED when the others validated their OPEN.
func TestRouterIDClaimConcurrentOnlyOneWins(t *testing.T) {
	const racers = 16
	const peerAS uint32 = 65001
	const sharedID uint32 = 0x01020304

	r := newClaimReactor(t, false)

	peers := make([]*Peer, racers)
	for i := range peers {
		settings := NewPeerSettings(netip.AddrFrom4([4]byte{192, 0, 2, byte(10 + i)}), 65001, peerAS, 0x01020301)
		peers[i] = NewPeer(settings)
		peers[i].SetReactor(r)
	}

	localOpen := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020301}
	remoteOpen := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: sharedID}

	start := make(chan struct{})
	results := make([]error, racers)
	var wg sync.WaitGroup
	for i, p := range peers {
		wg.Go(func() {
			<-start
			results[i] = p.validateOpen(p.settings.Address.String(), localOpen, remoteOpen)
		})
	}
	close(start)
	wg.Wait()

	accepted := 0
	for i, err := range results {
		if err == nil {
			accepted++
			continue
		}
		var conflictErr *routerIDConflictError
		require.ErrorAs(t, err, &conflictErr, "racer %d must be rejected with a router-id conflict", i)
	}
	assert.Equal(t, 1, accepted, "exactly one racer may hold the identifier")

	holder, held := r.routerIDs.holder(peerAS, sharedID)
	require.True(t, held, "the winner must hold the identifier in the registry")
	assert.NotNil(t, holder)

	// Nothing claimed a neighboring identifier: the racers contended for exactly one key.
	_, strayHeld := r.routerIDs.holder(peerAS, sharedID+1)
	assert.False(t, strayHeld, "no racer may claim an identifier nobody presented")
}

// TestRouterIDClaimReleasedOnTeardown verifies the claim does not outlive its session.
//
// VALIDATES: releaseRouterIDClaim frees the identifier for a different peer.
// VALIDATES: The registry reports no holder once released.
// PREVENTS: A leaked claim permanently rejecting a legitimate later peer -- the failure
// mode that makes claim-at-OPEN safe only when every teardown path releases.
func TestRouterIDClaimReleasedOnTeardown(t *testing.T) {
	r := newClaimReactor(t, false)

	holderPeer, err := claimPeer(t, r, "192.0.2.1", 65001, 0x01020304)
	require.NoError(t, err, "first peer claims the identifier")

	// While held, another peer is rejected.
	_, err = claimPeer(t, r, "192.0.2.99", 65001, 0x01020304)
	var conflictErr *routerIDConflictError
	require.ErrorAs(t, err, &conflictErr, "the identifier is held")

	// Drive the PRODUCTION teardown, not the release helper. cleanup() is what
	// the session-goroutine defer and the peer-removal paths actually run
	// (peer_run.go); calling releaseRouterIDClaim() directly here would pass
	// even if every production release site were deleted, which is exactly the
	// leak this test exists to prevent.
	holderPeer.cleanup()
	_, held := r.routerIDs.holder(65001, 0x01020304)
	assert.False(t, held, "teardown must remove the registry entry")

	// A different peer may now claim it.
	_, err = claimPeer(t, r, "192.0.2.98", 65001, 0x01020304)
	assert.NoError(t, err, "the identifier is available after release")
}

// TestRouterIDClaimKeyedByPeerNotSettings verifies the claim survives a settings change.
//
// VALIDATES: A dynamic peer whose settings are replaced after establishment still has its
// claim released, because the registry keys ownership on the *Peer, not on its address/port.
// PREVENTS: A leaked claim after resolveDynamicPeerSettings rewrites peer settings.
func TestRouterIDClaimKeyedByPeerNotSettings(t *testing.T) {
	r := newClaimReactor(t, false)

	peer, err := claimPeer(t, r, "192.0.2.1", 65001, 0x01020304)
	require.NoError(t, err)

	// Simulate a dynamic peer publishing new settings after the claim was taken.
	peer.mu.Lock()
	replaced := *peer.settings
	replaced.PeerAS = 65010
	peer.settings = &replaced
	peer.mu.Unlock()

	peer.releaseRouterIDClaim()
	_, held := r.routerIDs.holder(65001, 0x01020304)
	assert.False(t, held, "release must find the claim even after settings changed")
}

// TestRouterIDClaimReleasedOnPeerRemoval drives the PEER-REMOVAL paths, which
// are the ones a claim can leak through without any session teardown running.
//
// VALIDATES: RemovePeer and removeDynamicPeer release the AS-wide BGP
// Identifier claim synchronously, so a peer removed and immediately re-added
// (re-address, router-id move, dynamic remove/recreate) does not meet its own
// stale claim.
//
// PREVENTS: the sibling-call-site gap. The reload-remove path
// (reactor_api.go) was given a synchronous releaseRouterIDClaim(); these two
// were not. Peer.Stop() only cancels a context, so the outgoing peer's claim
// outlived its removal until its goroutine was scheduled, and the legitimate
// new session was answered with OPEN Message Error / Bad BGP Identifier --
// decided purely by scheduling, which is why no existing test caught it.
// Driving removal rather than releaseRouterIDClaim() is the whole point: the
// helper-level test passes with every production release site deleted.
func TestRouterIDClaimReleasedOnPeerRemoval(t *testing.T) {
	const (
		peerAS uint32 = 65001
		bgpID  uint32 = 0x01020304
	)

	t.Run("RemovePeer frees the identifier for a re-add", func(t *testing.T) {
		r := newClaimReactor(t, false)

		holder, err := claimPeer(t, r, "192.0.2.1", peerAS, bgpID)
		require.NoError(t, err, "first peer claims the identifier")
		require.NotNil(t, holder)

		require.NoError(t, r.RemovePeer(netip.MustParseAddr("192.0.2.1")))

		if _, held := r.routerIDs.holder(peerAS, bgpID); held {
			t.Fatal("RemovePeer left the claim held; a re-added peer would be refused")
		}
		_, err = claimPeer(t, r, "192.0.2.2", peerAS, bgpID)
		assert.NoError(t, err, "the identifier must be claimable after removal")
	})

	t.Run("removeDynamicPeer frees the identifier", func(t *testing.T) {
		r := newClaimReactor(t, false)

		holder, err := claimPeer(t, r, "192.0.2.10", peerAS, bgpID)
		require.NoError(t, err)

		r.mu.Lock()
		r.removeDynamicPeer(holder)
		r.mu.Unlock()

		if _, held := r.routerIDs.holder(peerAS, bgpID); held {
			t.Fatal("removeDynamicPeer left the claim held; the recreated peer would be refused")
		}
	})
}

// TestRouterIDClaimConcurrentWithRelease races claim against release, which is
// the interleaving production actually runs.
//
// VALIDATES: the registry stays consistent when a claim on one goroutine
// overlaps a release on another -- an identifier is either held by exactly one
// peer or by none, never recorded as held by a peer that released.
//
// PREVENTS: the gap the existing concurrency test left. claim-vs-claim was
// never in doubt; the production shape is claim on the SESSION goroutine while
// release runs from teardown, peer removal or reload. That is the interleaving
// the peer-removal fix lives in, and nothing exercised it. Run under -race.
func TestRouterIDClaimConcurrentWithRelease(t *testing.T) {
	const (
		peerAS uint32 = 65001
		bgpID  uint32 = 0x0A0B0C0D
	)

	for range 50 {
		r := newClaimReactor(t, false)

		holder, err := claimPeer(t, r, "192.0.2.1", peerAS, bgpID)
		require.NoError(t, err)

		settings := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), peerAS, peerAS, 0x01020301)
		challenger := NewPeer(settings)
		challenger.SetReactor(r)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			holder.releaseRouterIDClaim()
		}()
		go func() {
			defer wg.Done()
			// Ignore the result: whether the challenger wins depends on the
			// interleaving, and both outcomes are legal. What must hold is the
			// invariant checked below.
			_, _ = r.routerIDs.claim(challenger, settings.Address, peerAS, bgpID)
		}()
		wg.Wait()

		// Invariant: whoever the registry says holds the identifier must be a
		// peer that did not release it. The holder released unconditionally, so
		// the only legal states are "challenger holds it" or "nobody does".
		if owner, held := r.routerIDs.holder(peerAS, bgpID); held && owner == holder {
			t.Fatal("registry reports the identifier held by the peer that released it")
		}
	}
}

// TestOpenAdvertisedASReadsAS4Capability pins the AS a peer is judged by when
// its settings carry none.
//
// VALIDATES: RFC 6793 -- a 4-byte-AS peer is identified by the ASN in its AS4
// capability, not by the AS_TRANS placeholder in the two-octet My AS field.
//
// PREVENTS: two defects that shared one cause. message.Open.ASN4 is NEVER set on
// a RECEIVED open (UnpackOpen does not populate it and nothing else assigns it),
// so the previous `remote.ASN4 > 0` test was dead and every such peer fell back
// to My AS = AS_TRANS (23456). That put every 4-byte-AS peer's BGP Identifier
// claim in one shared 23456 bucket, so two peers in genuinely different ASes
// that legitimately share an identifier collided and the second was refused --
// a rejection RFC 6286 Section 2.1 does not license, since it scopes uniqueness
// per-AS. The same value decides "is this an internal peer" for Section 2.2.
func TestOpenAdvertisedASReadsAS4Capability(t *testing.T) {
	const as4 uint32 = 4200000001 // > 65535, so My AS must carry AS_TRANS

	withAS4 := &message.Open{Version: 4, MyAS: message.AS_TRANS, HoldTime: 90, BGPIdentifier: 0x01020304}
	withAS4.OptionalParams = buildOptionalParams([]capability.Capability{&capability.ASN4{ASN: as4}})

	assert.Equal(t, as4, openAdvertisedAS(withAS4),
		"a 4-byte-AS peer must be identified by its AS4 capability, not AS_TRANS")

	// A two-octet speaker advertises no AS4 capability; My AS is the answer.
	plain := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020304}
	assert.Equal(t, uint32(65001), openAdvertisedAS(plain),
		"a two-octet peer is identified by My AS")

	// Malformed optional params must not panic or fabricate an AS: fall back to
	// My AS and let capability negotiation report the error with its own subcode.
	malformed := &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020304}
	malformed.OptionalParams = []byte{0xff, 0xff, 0xff}
	assert.Equal(t, uint32(65002), openAdvertisedAS(malformed),
		"a capability parse error must fall back to My AS, not fabricate one")
}
