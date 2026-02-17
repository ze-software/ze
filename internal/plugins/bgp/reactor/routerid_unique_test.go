package reactor

import (
	"errors"
	"net"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// makeEstablishedPeerWithID creates a peer with an ESTABLISHED session
// and a known remote BGP Identifier. Used for router-ID uniqueness tests.
func makeEstablishedPeerWithID(t *testing.T, addr string, localAS, peerAS, localRID, remoteRID uint32) (*Peer, func()) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr(addr), localAS, peerAS, localRID)
	settings.Passive = true
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
func makeOpenConfirmPeerWithID(t *testing.T, addr string, localAS, peerAS, localRID, remoteRID uint32) (*Peer, func()) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr(addr), localAS, peerAS, localRID)
	settings.Passive = true
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
// PREVENTS: Silent misconfiguration where iBGP peers share a router-ID,
// breaking ORIGINATOR_ID loop detection.
func TestRouterIDConflictIBGPDuplicate(t *testing.T) {
	// Peer A: iBGP (local=65001, peer=65001), remote BGP ID = 1.2.3.4
	peerA, cleanupA := makeEstablishedPeerWithID(t,
		"192.0.2.1", 65001, 65001, 0x01020301, 0x01020304)
	defer cleanupA()

	peers := map[string]*Peer{
		peerA.settings.PeerKey(): peerA,
	}

	// New peer in same iBGP AS with same router-ID → conflict.
	addr, conflict := checkRouterIDConflict(peers, "192.0.2.99:179", 65001, 0x01020304)
	assert.True(t, conflict, "should detect duplicate router-ID in iBGP AS")
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), addr)
}

// TestRouterIDConflictEBGPSameAS verifies that duplicate router-IDs are
// detected between eBGP peers sharing the same remote AS.
//
// VALIDATES: Two peers in the same remote AS with the same BGP ID conflict.
// PREVENTS: Two distinct routers in the same AS presenting identical router-IDs.
func TestRouterIDConflictEBGPSameAS(t *testing.T) {
	// Peer A: eBGP (local=65001, peer=65002), remote BGP ID = 5.6.7.8
	peerA, cleanupA := makeEstablishedPeerWithID(t,
		"192.0.2.1", 65001, 65002, 0x01020301, 0x05060708)
	defer cleanupA()

	peers := map[string]*Peer{
		peerA.settings.PeerKey(): peerA,
	}

	// New peer also in AS 65002 with same router-ID → conflict.
	addr, conflict := checkRouterIDConflict(peers, "192.0.2.99:179", 65002, 0x05060708)
	assert.True(t, conflict, "should detect duplicate router-ID in same remote AS")
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), addr)
}

// TestRouterIDConflictDifferentAS verifies that the same router-ID in
// different ASNs does NOT trigger a conflict.
//
// VALIDATES: Router-IDs are scoped per-AS; different ASNs may reuse IDs.
// PREVENTS: False positive conflict detection across AS boundaries.
func TestRouterIDConflictDifferentAS(t *testing.T) {
	// Peer A: in AS 65002, router-ID 1.2.3.4
	peerA, cleanupA := makeEstablishedPeerWithID(t,
		"192.0.2.1", 65001, 65002, 0x01020301, 0x01020304)
	defer cleanupA()

	peers := map[string]*Peer{
		peerA.settings.PeerKey(): peerA,
	}

	// New peer in AS 65003 with same router-ID → no conflict (different AS).
	_, conflict := checkRouterIDConflict(peers, "192.0.2.99:179", 65003, 0x01020304)
	assert.False(t, conflict, "different ASN should not conflict even with same router-ID")
}

// TestRouterIDConflictDifferentRouterID verifies that peers in the same AS
// with different router-IDs do NOT conflict.
//
// VALIDATES: Only duplicate router-IDs trigger conflict.
// PREVENTS: Over-aggressive rejection of valid peer configurations.
func TestRouterIDConflictDifferentRouterID(t *testing.T) {
	// Peer A: in AS 65001, router-ID 1.2.3.4
	peerA, cleanupA := makeEstablishedPeerWithID(t,
		"192.0.2.1", 65001, 65001, 0x01020301, 0x01020304)
	defer cleanupA()

	peers := map[string]*Peer{
		peerA.settings.PeerKey(): peerA,
	}

	// New peer in same AS with different router-ID → no conflict.
	_, conflict := checkRouterIDConflict(peers, "192.0.2.99:179", 65001, 0x05060708)
	assert.False(t, conflict, "different router-ID in same AS should not conflict")
}

// TestRouterIDConflictNotEstablished verifies that peers that haven't
// reached ESTABLISHED state are not considered for conflict detection.
//
// VALIDATES: Only ESTABLISHED sessions count for router-ID uniqueness.
// PREVENTS: Premature rejection during concurrent connection setup.
func TestRouterIDConflictNotEstablished(t *testing.T) {
	// Peer A: same AS, same router-ID, but only in OPENCONFIRM.
	peerA, cleanupA := makeOpenConfirmPeerWithID(t,
		"192.0.2.1", 65001, 65001, 0x01020301, 0x01020304)
	defer cleanupA()

	peers := map[string]*Peer{
		peerA.settings.PeerKey(): peerA,
	}

	// Same router-ID but peer not established → no conflict.
	_, conflict := checkRouterIDConflict(peers, "192.0.2.99:179", 65001, 0x01020304)
	assert.False(t, conflict, "non-ESTABLISHED peer should not trigger conflict")
}

// TestRouterIDConflictSelfExcluded verifies that a peer does not
// conflict with itself.
//
// VALIDATES: excludeKey correctly skips the peer being checked.
// PREVENTS: Self-conflict when checking after own OPEN is processed.
func TestRouterIDConflictSelfExcluded(t *testing.T) {
	// Peer A: established with router-ID 1.2.3.4
	peerA, cleanupA := makeEstablishedPeerWithID(t,
		"192.0.2.1", 65001, 65001, 0x01020301, 0x01020304)
	defer cleanupA()

	peerKey := peerA.settings.PeerKey()
	peers := map[string]*Peer{
		peerKey: peerA,
	}

	// Check with own key → should be excluded, no conflict.
	_, conflict := checkRouterIDConflict(peers, peerKey, 65001, 0x01020304)
	assert.False(t, conflict, "peer should not conflict with itself")
}

// TestRouterIDConflictNilSession verifies that peers without a session
// (configured but not connected) are safely skipped.
//
// VALIDATES: Nil sessions don't cause panics or false conflicts.
// PREVENTS: Nil pointer dereference on unconnected peers.
func TestRouterIDConflictNilSession(t *testing.T) {
	// Peer A: configured but no session yet.
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020301)
	peerA := NewPeer(settings)

	peers := map[string]*Peer{
		peerA.settings.PeerKey(): peerA,
	}

	// Same AS and same router-ID but no session → no conflict.
	_, conflict := checkRouterIDConflict(peers, "192.0.2.99:179", 65001, 0x01020304)
	assert.False(t, conflict, "peer without session should not trigger conflict")
}

// TestRouterIDConflictMultiplePeers verifies conflict detection across
// multiple peers where only one conflicts.
//
// VALIDATES: Correct peer identified among several in the same AS.
// PREVENTS: Off-by-one or early-exit bugs in peer iteration.
func TestRouterIDConflictMultiplePeers(t *testing.T) {
	// Peer A: AS 65002, router-ID 1.2.3.4 (different from check).
	peerA, cleanupA := makeEstablishedPeerWithID(t,
		"192.0.2.1", 65001, 65002, 0x01020301, 0x01020304)
	defer cleanupA()

	// Peer B: AS 65003, router-ID 5.6.7.8 (different AS from check).
	peerB, cleanupB := makeEstablishedPeerWithID(t,
		"192.0.2.2", 65001, 65003, 0x01020301, 0x05060708)
	defer cleanupB()

	// Peer C: AS 65002, router-ID 5.6.7.8 (THIS one conflicts).
	peerC, cleanupC := makeEstablishedPeerWithID(t,
		"192.0.2.3", 65001, 65002, 0x01020301, 0x05060708)
	defer cleanupC()

	peers := map[string]*Peer{
		peerA.settings.PeerKey(): peerA,
		peerB.settings.PeerKey(): peerB,
		peerC.settings.PeerKey(): peerC,
	}

	// New peer in AS 65002 with router-ID 5.6.7.8 → conflicts with Peer C.
	addr, conflict := checkRouterIDConflict(peers, "192.0.2.99:179", 65002, 0x05060708)
	assert.True(t, conflict, "should detect conflict with Peer C")
	assert.Equal(t, netip.MustParseAddr("192.0.2.3"), addr, "should identify Peer C as conflicting")
}
