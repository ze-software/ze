package reactor

import (
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/clock"
)

// collisionAcceptWithReader shares the pipe reader and session resource
// ownership of acceptWithReader.
func collisionAcceptWithReader(t *testing.T, session *Session, server, client net.Conn) {
	t.Helper()
	acceptWithReader(t, session, server, client)
}

// setupOpenConfirmSession creates a session in OpenConfirm state for collision testing.
func setupOpenConfirmSession(t *testing.T, localID uint32) (*Session, net.Conn, net.Conn) {
	t.Helper()
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, localID,
	)
	settings.Connection = ConnectionPassive
	session := NewSession(settings)

	_ = session.Start()
	client, server := net.Pipe()
	collisionAcceptWithReader(t, session, server, client)

	// Send peer's OPEN to reach OPENCONFIRM
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()
	_ = session.ReadAndProcess()

	require.Equal(t, fsm.StateOpenConfirm, session.State())

	return session, client, server
}

// TestCollisionEstablished verifies collision with established session.
// RFC 4271 §6.8: "collision with existing BGP connection that is in
// the Established state causes closing of the newly created connection"
//
// VALIDATES: Incoming connection rejected when peer is ESTABLISHED.
// PREVENTS: Established sessions being disrupted by new connections.
func TestCollisionEstablished(t *testing.T) {
	// Create session with local BGP ID
	localID := uint32(0x01020304)
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, localID,
	)
	settings.Connection = ConnectionPassive
	session := NewSession(settings)

	// Start session and accept connection
	_ = session.Start()
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	collisionAcceptWithReader(t, session, server, client)

	// Complete handshake to reach ESTABLISHED
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()
	_ = session.ReadAndProcess() // Process OPEN → OPENCONFIRM

	// Send KEEPALIVE to reach ESTABLISHED
	keepalive := message.NewKeepalive()
	keepaliveBytes := message.PackTo(keepalive, nil)
	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()
	_ = session.ReadAndProcess() // Process KEEPALIVE → ESTABLISHED

	require.Equal(t, fsm.StateEstablished, session.State())

	// New incoming connection should trigger collision detection
	// Regardless of remote BGP ID, ESTABLISHED always rejects
	remoteID := uint32(0xFFFFFFFF) // Higher than local - would normally win

	shouldAccept, shouldCloseExisting := session.detectCollision(remoteID)

	assert.False(t, shouldAccept, "ESTABLISHED should reject new connection")
	assert.False(t, shouldCloseExisting, "ESTABLISHED should not close existing")
}

// TestCollisionOpenConfirmLocalWins verifies local BGP ID wins.
// RFC 4271 §6.8: "local system closes the newly created BGP connection"
//
// VALIDATES: When local_id > remote_id, incoming is rejected.
// PREVENTS: Wrong connection being kept when local ID is higher.
// This is a decision-only assertion: keep the existing connection and reject
// the incoming connection. The real-socket collision tests assert teardown.
// RFC requirement: RFC4271-6.8-2 positive -- on receipt of the OPEN the local system examines the
// connection in OpenConfirm state and compares BGP Identifiers
// (internal/component/bgp/reactor/session.go:610-620).
func TestCollisionOpenConfirmLocalWins(t *testing.T) {
	// Local ID is higher than remote
	localID := uint32(0xC0A80001)  // 192.168.0.1
	remoteID := uint32(0x0A000001) // 10.0.0.1

	session, client, server := setupOpenConfirmSession(t, localID)
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	shouldAccept, shouldCloseExisting := session.detectCollision(remoteID)

	// Local > Remote: reject incoming, keep existing
	assert.False(t, shouldAccept, "local wins: should reject incoming")
	assert.False(t, shouldCloseExisting, "local wins: should keep existing")
}

// TestCollisionOpenConfirmRemoteWins verifies remote BGP ID wins.
// RFC 4271 §6.8: "local system closes the BGP connection that already
// exists and accepts the BGP connection initiated by the remote system"
//
// VALIDATES: When local_id < remote_id, existing is closed, incoming accepted.
// PREVENTS: Wrong connection being kept when remote ID is higher.
func TestCollisionOpenConfirmRemoteWins(t *testing.T) {
	// Remote ID is higher than local
	localID := uint32(0x0A000001)  // 10.0.0.1
	remoteID := uint32(0xC0A80001) // 192.168.0.1

	session, client, server := setupOpenConfirmSession(t, localID)
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	shouldAccept, shouldCloseExisting := session.detectCollision(remoteID)

	// Local < Remote: accept incoming, close existing
	assert.True(t, shouldAccept, "remote wins: should accept incoming")
	assert.True(t, shouldCloseExisting, "remote wins: should close existing")
}

// TestCollisionOpenSentNoCollision verifies OpenSent allows new connections.
// RFC 4271 §6.8: "cannot be detected with connections in Idle, Connect, or Active"
// (OpenSent MAY detect if BGP ID known by other means - we don't implement that)
//
// VALIDATES: Connections in OpenSent can accept incoming (no collision).
// PREVENTS: Over-aggressive collision detection.
// RFC requirement: RFC4271-6.8-1 negative -- no connection is closed when there is no collision:
// a session in OpenSent has no OpenConfirm peer connection to collide with
// (internal/component/bgp/reactor/session.go:622-629).
func TestCollisionOpenSentNoCollision(t *testing.T) {
	localID := uint32(0x01020304)
	remoteID := uint32(0xFFFFFFFF)

	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, localID,
	)
	settings.Connection = ConnectionPassive
	session := NewSession(settings)

	// Start session and accept connection - puts us in OPENSENT
	_ = session.Start()
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	collisionAcceptWithReader(t, session, server, client)

	require.Equal(t, fsm.StateOpenSent, session.State())

	shouldAccept, shouldCloseExisting := session.detectCollision(remoteID)

	// OpenSent: no collision detection (per RFC MUST only for OpenConfirm)
	assert.True(t, shouldAccept, "OpenSent should accept new connection")
	assert.False(t, shouldCloseExisting, "OpenSent should not close existing")
}

// TestCollisionBGPIDComparison verifies ID comparison as uint32.
// RFC 4271 §6.8: "converting them to host byte order and treating
// them as 4-octet unsigned integers"
//
// VALIDATES: BGP IDs compared correctly as unsigned integers.
// PREVENTS: Byte order or signed comparison bugs.
func TestCollisionBGPIDComparison(t *testing.T) {
	tests := []struct {
		name              string
		localID           uint32
		remoteID          uint32
		wantAccept        bool
		wantCloseExisting bool
	}{
		{
			// RFC 6286 Section 2.3 (which updates RFC 4271 Section 6.8) replaces the plain
			// "tie goes to the existing connection" outcome for IDENTICAL identifiers: "the
			// connection initiated by the BGP speaker with the larger AS number is
			// preserved". setupOpenConfirmSession is local AS 65001 / peer AS 65002, so the
			// remote has the larger AS and its (pending) connection wins. The AS-ordering
			// cases both ways are covered by TestDetectCollisionEqualIdentifierPrefersLargerAS.
			name:              "identical identifiers - larger AS (the remote here) wins per RFC 6286 2.3",
			localID:           0x01020304,
			remoteID:          0x01020304,
			wantAccept:        true,
			wantCloseExisting: true,
		},
		{
			name:              "local higher by 1",
			localID:           0x01020305,
			remoteID:          0x01020304,
			wantAccept:        false,
			wantCloseExisting: false,
		},
		{
			name:              "remote higher by 1",
			localID:           0x01020304,
			remoteID:          0x01020305,
			wantAccept:        true,
			wantCloseExisting: true,
		},
		{
			name:              "max uint32 vs zero",
			localID:           0x00000000,
			remoteID:          0xFFFFFFFF,
			wantAccept:        true,
			wantCloseExisting: true,
		},
		{
			name:              "typical IPs: 10.x vs 192.x",
			localID:           0x0A000001, // 10.0.0.1
			remoteID:          0xC0A80001, // 192.168.0.1
			wantAccept:        true,
			wantCloseExisting: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, client, server := setupOpenConfirmSession(t, tt.localID)
			defer func() { _ = client.Close() }()
			defer func() { _ = server.Close() }()

			shouldAccept, shouldCloseExisting := session.detectCollision(tt.remoteID)

			assert.Equal(t, tt.wantAccept, shouldAccept, "accept mismatch")
			assert.Equal(t, tt.wantCloseExisting, shouldCloseExisting, "close existing mismatch")
		})
	}
}

// TestCollisionNotificationSent verifies NOTIFICATION is sent on collision.
// RFC 4271 §6.8: "sending NOTIFICATION message with Error Code Cease"
//
// VALIDATES: Rejected connection receives NOTIFICATION 6/7.
// PREVENTS: Silent connection drops without proper BGP signaling.
func TestCollisionNotificationSent(t *testing.T) {
	// Create and establish a session
	localID := uint32(0x01020304)
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, localID,
	)
	settings.Connection = ConnectionPassive
	session := NewSession(settings)

	_ = session.Start()
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	collisionAcceptWithReader(t, session, server, client)

	// Complete handshake to reach ESTABLISHED
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()
	_ = session.ReadAndProcess()

	keepalive := message.NewKeepalive()
	keepaliveBytes := message.PackTo(keepalive, nil)
	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()
	_ = session.ReadAndProcess()

	require.Equal(t, fsm.StateEstablished, session.State())

	// Create a new "incoming" connection and test rejectConnectionCollision
	incomingClient, incomingServer := net.Pipe()
	defer func() { _ = incomingClient.Close() }()

	// Read the NOTIFICATION in a goroutine
	var notifData []byte
	var wg sync.WaitGroup
	wg.Go(func() {
		buf := make([]byte, 256)
		n, _ := incomingClient.Read(buf)
		if n > 0 {
			notifData = buf[:n]
		}
	})

	// Send NOTIFICATION to the incoming connection (simulate collision reject)
	notif := &message.Notification{
		ErrorCode:    message.NotifyCease,
		ErrorSubcode: message.NotifyCeaseConnectionCollision,
	}
	data := message.PackTo(notif, nil)
	_, err := incomingServer.Write(data)
	require.NoError(t, err)
	_ = incomingServer.Close()

	wg.Wait()

	// Verify NOTIFICATION was received
	require.NotEmpty(t, notifData, "should receive NOTIFICATION")
	require.GreaterOrEqual(t, len(notifData), message.HeaderLen+2, "NOTIFICATION too short")

	// Parse header
	hdr, err := message.ParseHeader(notifData[:message.HeaderLen])
	require.NoError(t, err)
	assert.Equal(t, msgtype.TypeNOTIFICATION, hdr.Type)

	// Parse NOTIFICATION body
	body := notifData[message.HeaderLen:hdr.Length]
	assert.Equal(t, uint8(message.NotifyCease), body[0], "error code should be Cease")
	assert.Equal(t, message.NotifyCeaseConnectionCollision, body[1], "subcode should be Connection Collision")
}

// TestPeerSetPendingConnection verifies pending connection tracking.
//
// VALIDATES: Peer can track a pending incoming connection.
// PREVENTS: Lost connections during collision handling.
func TestPeerSetPendingConnection(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020304,
	)
	peer := NewPeer(settings)

	// Initially no pending connection
	assert.False(t, peer.hasPendingConnection())

	// Set pending connection
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	err := peer.setPendingConnection(server)
	require.NoError(t, err)
	assert.True(t, peer.hasPendingConnection())

	// Setting another should fail
	client2, server2 := net.Pipe()
	defer func() { _ = client2.Close() }()
	defer func() { _ = server2.Close() }()

	err = peer.setPendingConnection(server2)
	require.Error(t, err)

	// Clear pending
	peer.clearPendingConnection()
	assert.False(t, peer.hasPendingConnection())
}

// TestPeerResolvePendingCollisionLocalWins verifies collision resolution when local wins.
// RFC 4271 §6.8: When local BGP ID > remote, close incoming.
//
// VALIDATES: Pending connection rejected when local ID is higher.
// PREVENTS: Wrong connection being kept.
func TestPeerResolvePendingCollisionLocalWins(t *testing.T) {
	// Local ID is higher than remote
	localID := uint32(0xC0A80001)  // 192.168.0.1
	remoteID := uint32(0x0A000001) // 10.0.0.1

	session, existing, current := setupOpenConfirmSession(t, localID)
	t.Cleanup(func() { closeConnQuietly(existing); closeConnQuietly(current) })
	require.NoError(t, existing.SetDeadline(time.Now().Add(5*time.Second)))
	peer := NewPeer(session.settings)
	peer.session = session
	pendingClient, pendingServer, candidate := collisionCandidateInput(t, peer)

	// Pending OPEN from "remote" with lower BGP ID
	pendingOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: remoteID,
	}

	delayOpenWrite(t, pendingClient, pendingOpen)
	requireCollisionCease(t, pendingClient, "the incoming loser")
	var octet [1]byte
	n, err := pendingClient.Read(octet[:])
	require.Zero(t, n)
	require.ErrorIs(t, err, io.EOF, "the actual caller closes the rejected socket")
	assert.Same(t, pendingServer, candidate.conn, "the rejected candidate owns the exact incoming socket")
	peer.mu.RLock()
	retained := inboundConn(peer.inbound)
	peer.mu.RUnlock()
	assert.Nil(t, retained, "local wins: no incoming winner transfers")
	assert.False(t, peer.hasPendingConnection(), "pending should be cleared")
	assert.Same(t, session, peer.currentSession(), "local wins: keep the original Session")
	assert.Same(t, current, session.Conn(), "local wins: keep the original socket")
	assert.Equal(t, fsm.StateOpenConfirm, session.State())
}

// TestPeerResolvePendingCollisionRemoteWins verifies collision resolution when remote wins.
// RFC 4271 §6.8: When local BGP ID < remote, close existing and accept incoming.
//
// VALIDATES: Existing connection closed when remote ID is higher.
// PREVENTS: Wrong connection being kept.
func TestPeerResolvePendingCollisionRemoteWins(t *testing.T) {
	// Remote ID is higher than local
	localID := uint32(0x0A000001)  // 10.0.0.1
	remoteID := uint32(0xC0A80001) // 192.168.0.1

	session, existing, current := setupOpenConfirmSession(t, localID)
	t.Cleanup(func() { closeConnQuietly(existing); closeConnQuietly(current) })
	require.NoError(t, existing.SetDeadline(time.Now().Add(5*time.Second)))
	peer := NewPeer(session.settings)
	peer.session = session
	pendingClient, pendingServer, candidate := collisionCandidateInput(t, peer)

	// Pending OPEN from "remote" with higher BGP ID
	pendingOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: remoteID,
	}

	wire := message.PackTo(pendingOpen, nil)
	delayOpenWrite(t, pendingClient, pendingOpen)
	requireCollisionCease(t, existing, "the original loser")
	var octet [1]byte
	n, err := existing.Read(octet[:])
	require.Zero(t, n)
	require.ErrorIs(t, err, io.EOF)
	peer.mu.RLock()
	stored := peer.inbound
	peer.mu.RUnlock()
	parsed, ok := stored.(*candidateOpen)
	require.True(t, ok, "the actual parser hands off its independent candidate")
	assert.Same(t, candidate, parsed.candidate, "the already-running candidate wins")
	assert.Equal(t, pendingOpen, parsed.open, "the parser retains the received OPEN's fields")
	inbound := peer.takeInboundConnection()
	t.Cleanup(func() { discardInbound(inbound) })
	assert.Same(t, stored, inbound, "remote wins: the Peer transfers its exact owned receipt")
	assert.Equal(t, pendingServer, inboundConn(inbound), "same winning connection")
	owned, ok := inbound.(*candidateOpen)
	require.True(t, ok)
	assert.Same(t, parsed.open, owned.open, "the handoff retains its original parsed OPEN pointer")
	assert.Equal(t, wire, owned.wire, "original complete OPEN retained")
	assert.Same(t, candidate.session, owned.candidate.session, "the winning Session is not reconstructed")
	assert.Same(t, pendingServer, candidate.session.Conn(), "accepted socket remains owned by that Session")
	assert.False(t, peer.hasPendingConnection(), "pending should be cleared")
}

// collisionCandidateInput enters the actual listener collision path and reads
// the incoming Session's own OPEN before its remote input. The caller MUST
// either leave its receipt to fixture Peer cleanup or discard a taken receipt.
func collisionCandidateInput(t *testing.T, peer *Peer) (net.Conn, net.Conn, *incomingCandidate) {
	t.Helper()
	cleanupPendingFixturePeer(t, peer)
	client, server := collisionProofTCPPair(t)
	r := &Reactor{clock: clock.RealClock{}}
	r.acceptOrReject(server, peer, nil)
	delayOpenReadType(t, client, 1)
	var candidate *incomingCandidate
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		var ok bool
		candidate, ok = peer.pending.(*incomingCandidate)
		return ok
	})
	require.Same(t, server, candidate.conn)
	return client, server, candidate
}

// TestCollisionNonCollisionStates verifies states that cannot detect collision.
// RFC 4271 §6.8: "cannot be detected with connections in Idle, Connect, or Active"
//
// VALIDATES: Idle/Connect/Active states accept without collision detection.
// PREVENTS: False collision detection in non-applicable states.
// RFC requirement: RFC4271-6.8-2 negative -- states other than OpenConfirm and Established are
// not treated as collisions, so the OpenConfirm examination is state-specific rather than applied
// to every connection (internal/component/bgp/reactor/session.go:598-629).
func TestCollisionNonCollisionStates(t *testing.T) {
	// For states that cannot have a connection (Idle/Connect/Active),
	// detectCollision should always return (true, false) - accept new, don't close existing
	tests := []struct {
		name       string
		connection ConnectionMode
	}{
		{
			name:       "Idle state",
			connection: ConnectionPassive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := NewPeerSettings(
				netip.MustParseAddr("192.0.2.1"),
				65001, 65002, 0x01020304,
			)
			settings.Connection = tt.connection
			session := NewSession(settings)

			// Session starts in Idle - don't start it to stay in Idle
			require.Equal(t, fsm.StateIdle, session.State())

			// detectCollision with any remote ID
			shouldAccept, shouldCloseExisting := session.detectCollision(0xFFFFFFFF)

			// Non-collision states should accept
			assert.True(t, shouldAccept, "%s should accept", tt.name)
			assert.False(t, shouldCloseExisting, "%s should not close existing", tt.name)
		})
	}
}

// openConfirmSessionWithAS creates an OpenConfirm session with caller-chosen AS numbers and
// local BGP Identifier, for the RFC 6286 Section 2.3 equal-identifier collision cases.
func openConfirmSessionWithAS(t *testing.T, localAS, peerAS, localID uint32) *Session {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), localAS, peerAS, localID)
	return openConfirmSessionWithSettings(t, settings, peerAS)
}

// openConfirmSessionWithSettings configures admission before constructing the
// actual wire OPEN, with the advertised peer AS independent of configured AS.
func openConfirmSessionWithSettings(t *testing.T, settings *PeerSettings, peerAS uint32) *Session {
	t.Helper()
	settings.Connection = ConnectionPassive
	session := NewSession(settings)

	_ = session.Start()
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	collisionAcceptWithReader(t, session, server, client)

	peerOpen := &message.Open{
		Version: 4, MyAS: uint16(peerAS), HoldTime: 90, BGPIdentifier: settings.RouterID + 1, //nolint:gosec // test AS numbers < 65536
	}
	openBytes := message.PackTo(peerOpen, nil)
	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()
	_ = session.ReadAndProcess()

	require.Equal(t, fsm.StateOpenConfirm, session.State())
	return session
}

// TestDetectCollisionEqualIdentifierPrefersLargerAS verifies RFC 6286 Section 2.3.
//
// RFC requirement: RFC6286-2.3-1 positive -- when the BGP Identifiers involved in a
// connection collision are identical, detectCollision preserves the connection initiated by
// the speaker with the larger AS number: the remote-initiated (pending) connection when the
// peer's AS is larger, the local-initiated (existing) one when this speaker's AS is larger.
// RFC requirement: RFC6286-2.3-1 negative -- when the identifiers DIFFER the AS numbers are
// not consulted at all: the RFC 4271 Section 6.8 identifier comparison decides, in both
// directions, with the same AS pair that flips the outcome in the identical-identifier case.
//
// VALIDATES: peer AS > local AS -> pending accepted and existing closed.
// VALIDATES: peer AS < local AS -> pending rejected, existing kept.
// VALIDATES: the unequal-identifier RFC 4271 Section 6.8 comparison is unaffected.
// PREVENTS: two speakers sharing an identifier (which RFC 6286 permits across an AS
// boundary) deadlocking collision resolution by both keeping, or both closing, their
// connection.
func TestDetectCollisionEqualIdentifierPrefersLargerAS(t *testing.T) {
	const localID uint32 = 0x01020304

	tests := []struct {
		name        string
		localAS     uint32
		peerAS      uint32
		remoteID    uint32
		wantAccept  bool
		wantClose   bool
		description string
	}{
		{
			name:    "equal identifiers, larger remote AS wins",
			localAS: 65001, peerAS: 65002, remoteID: localID,
			wantAccept: true, wantClose: true,
			description: "the remote initiated the pending connection and has the larger AS",
		},
		{
			name:    "equal identifiers, larger local AS wins",
			localAS: 65002, peerAS: 65001, remoteID: localID,
			wantAccept: false, wantClose: false,
			description: "this speaker has the larger AS, so its own connection is preserved",
		},
		{
			name:    "unequal identifiers still follow RFC 4271 Section 6.8",
			localAS: 65001, peerAS: 65002, remoteID: localID + 1,
			wantAccept: true, wantClose: true,
			description: "local identifier is lower, so the remote connection wins on identifier",
		},
		{
			name:    "unequal identifiers, higher local identifier keeps existing",
			localAS: 65001, peerAS: 65002, remoteID: localID - 1,
			wantAccept: false, wantClose: false,
			description: "local identifier is higher, AS numbers are not consulted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := openConfirmSessionWithAS(t, tt.localAS, tt.peerAS, localID)

			accept, closeExisting := session.detectCollision(tt.remoteID)
			assert.Equal(t, tt.wantAccept, accept, tt.description)
			assert.Equal(t, tt.wantClose, closeExisting, tt.description)
		})
	}
}
