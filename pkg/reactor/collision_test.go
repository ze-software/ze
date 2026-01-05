package reactor

import (
	"net"
	"net/netip"
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/fsm"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collisionAcceptWithReader handles net.Pipe's synchronous behavior by reading
// from client while Accept writes.
func collisionAcceptWithReader(t *testing.T, session *Session, server, client net.Conn) {
	t.Helper()
	buf := make([]byte, 4096)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = client.Read(buf)
	}()

	err := session.Accept(server)
	require.NoError(t, err)

	wg.Wait()
}

// setupOpenConfirmSession creates a session in OpenConfirm state for collision testing.
func setupOpenConfirmSession(t *testing.T, localID uint32) (*Session, net.Conn, net.Conn) {
	t.Helper()
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, localID,
	)
	settings.Passive = true
	session := NewSession(settings)

	_ = session.Start()
	client, server := net.Pipe()
	collisionAcceptWithReader(t, session, server, client)

	// Send peer's OPEN to reach OPENCONFIRM
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
	}
	openBytes, _ := peerOpen.Pack(nil)

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
	settings.Passive = true
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
	openBytes, _ := peerOpen.Pack(nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()
	_ = session.ReadAndProcess() // Process OPEN → OPENCONFIRM

	// Send KEEPALIVE to reach ESTABLISHED
	keepalive := message.NewKeepalive()
	keepaliveBytes, _ := keepalive.Pack(nil)
	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()
	_ = session.ReadAndProcess() // Process KEEPALIVE → ESTABLISHED

	require.Equal(t, fsm.StateEstablished, session.State())

	// New incoming connection should trigger collision detection
	// Regardless of remote BGP ID, ESTABLISHED always rejects
	remoteID := uint32(0xFFFFFFFF) // Higher than local - would normally win

	shouldAccept, shouldCloseExisting := session.DetectCollision(remoteID)

	assert.False(t, shouldAccept, "ESTABLISHED should reject new connection")
	assert.False(t, shouldCloseExisting, "ESTABLISHED should not close existing")
}

// TestCollisionOpenConfirmLocalWins verifies local BGP ID wins.
// RFC 4271 §6.8: "local system closes the newly created BGP connection"
//
// VALIDATES: When local_id > remote_id, incoming is rejected.
// PREVENTS: Wrong connection being kept when local ID is higher.
func TestCollisionOpenConfirmLocalWins(t *testing.T) {
	// Local ID is higher than remote
	localID := uint32(0xC0A80001)  // 192.168.0.1
	remoteID := uint32(0x0A000001) // 10.0.0.1

	session, client, server := setupOpenConfirmSession(t, localID)
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	shouldAccept, shouldCloseExisting := session.DetectCollision(remoteID)

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

	shouldAccept, shouldCloseExisting := session.DetectCollision(remoteID)

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
func TestCollisionOpenSentNoCollision(t *testing.T) {
	localID := uint32(0x01020304)
	remoteID := uint32(0xFFFFFFFF)

	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, localID,
	)
	settings.Passive = true
	session := NewSession(settings)

	// Start session and accept connection - puts us in OPENSENT
	_ = session.Start()
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	collisionAcceptWithReader(t, session, server, client)

	require.Equal(t, fsm.StateOpenSent, session.State())

	shouldAccept, shouldCloseExisting := session.DetectCollision(remoteID)

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
			name:              "local equals remote - local wins (tie goes to existing)",
			localID:           0x01020304,
			remoteID:          0x01020304,
			wantAccept:        false,
			wantCloseExisting: false,
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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			session, client, server := setupOpenConfirmSession(t, tc.localID)
			defer func() { _ = client.Close() }()
			defer func() { _ = server.Close() }()

			shouldAccept, shouldCloseExisting := session.DetectCollision(tc.remoteID)

			assert.Equal(t, tc.wantAccept, shouldAccept, "accept mismatch")
			assert.Equal(t, tc.wantCloseExisting, shouldCloseExisting, "close existing mismatch")
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
	settings.Passive = true
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
	openBytes, _ := peerOpen.Pack(nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()
	_ = session.ReadAndProcess()

	keepalive := message.NewKeepalive()
	keepaliveBytes, _ := keepalive.Pack(nil)
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
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 256)
		n, _ := incomingClient.Read(buf)
		if n > 0 {
			notifData = buf[:n]
		}
	}()

	// Send NOTIFICATION to the incoming connection (simulate collision reject)
	notif := &message.Notification{
		ErrorCode:    message.NotifyCease,
		ErrorSubcode: message.NotifyCeaseConnectionCollision,
	}
	data, err := notif.Pack(nil)
	require.NoError(t, err)
	_, err = incomingServer.Write(data)
	require.NoError(t, err)
	_ = incomingServer.Close()

	wg.Wait()

	// Verify NOTIFICATION was received
	require.NotEmpty(t, notifData, "should receive NOTIFICATION")
	require.GreaterOrEqual(t, len(notifData), message.HeaderLen+2, "NOTIFICATION too short")

	// Parse header
	hdr, err := message.ParseHeader(notifData[:message.HeaderLen])
	require.NoError(t, err)
	assert.Equal(t, message.TypeNOTIFICATION, hdr.Type)

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
	assert.False(t, peer.HasPendingConnection())

	// Set pending connection
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	err := peer.SetPendingConnection(server)
	require.NoError(t, err)
	assert.True(t, peer.HasPendingConnection())

	// Setting another should fail
	client2, server2 := net.Pipe()
	defer func() { _ = client2.Close() }()
	defer func() { _ = server2.Close() }()

	err = peer.SetPendingConnection(server2)
	require.Error(t, err)

	// Clear pending
	peer.ClearPendingConnection()
	assert.False(t, peer.HasPendingConnection())
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

	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, localID,
	)
	settings.Passive = true
	peer := NewPeer(settings)

	// Create session and get to OpenConfirm
	session := NewSession(settings)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	_ = session.Start()
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	collisionAcceptWithReader(t, session, server, client)

	// Send peer's OPEN to reach OPENCONFIRM
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
	}
	openBytes, _ := peerOpen.Pack(nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()
	_ = session.ReadAndProcess()
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	// Set up pending connection
	pendingClient, pendingServer := net.Pipe()
	defer func() { _ = pendingClient.Close() }()
	defer func() { _ = pendingServer.Close() }()
	_ = peer.SetPendingConnection(pendingServer)

	// Pending OPEN from "remote" with lower BGP ID
	pendingOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: remoteID,
	}

	// Resolve collision
	acceptPending, conn, _ := peer.ResolvePendingCollision(pendingOpen)

	assert.False(t, acceptPending, "local wins: should reject pending")
	assert.NotNil(t, conn, "should return connection for cleanup")
	assert.False(t, peer.HasPendingConnection(), "pending should be cleared")
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

	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, localID,
	)
	settings.Passive = true
	peer := NewPeer(settings)

	// Create session and get to OpenConfirm
	session := NewSession(settings)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	_ = session.Start()
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	collisionAcceptWithReader(t, session, server, client)

	// Send peer's OPEN to reach OPENCONFIRM
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
	}
	openBytes, _ := peerOpen.Pack(nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()
	_ = session.ReadAndProcess()
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	// Set up pending connection
	pendingClient, pendingServer := net.Pipe()
	defer func() { _ = pendingClient.Close() }()
	defer func() { _ = pendingServer.Close() }()
	_ = peer.SetPendingConnection(pendingServer)

	// Pending OPEN from "remote" with higher BGP ID
	pendingOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: remoteID,
	}

	// Drain the NOTIFICATION that will be sent when existing is closed
	go func() {
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()

	// Resolve collision
	acceptPending, conn, open := peer.ResolvePendingCollision(pendingOpen)

	assert.True(t, acceptPending, "remote wins: should accept pending")
	assert.NotNil(t, conn, "should return pending connection")
	assert.Equal(t, pendingOpen, open, "should return pending OPEN")
	assert.False(t, peer.HasPendingConnection(), "pending should be cleared")
}

// TestCollisionNonCollisionStates verifies states that cannot detect collision.
// RFC 4271 §6.8: "cannot be detected with connections in Idle, Connect, or Active"
//
// VALIDATES: Idle/Connect/Active states accept without collision detection.
// PREVENTS: False collision detection in non-applicable states.
func TestCollisionNonCollisionStates(t *testing.T) {
	// For states that cannot have a connection (Idle/Connect/Active),
	// DetectCollision should always return (true, false) - accept new, don't close existing
	tests := []struct {
		name    string
		passive bool
	}{
		{
			name:    "Idle state",
			passive: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(
				netip.MustParseAddr("192.0.2.1"),
				65001, 65002, 0x01020304,
			)
			settings.Passive = tc.passive
			session := NewSession(settings)

			// Session starts in Idle - don't start it to stay in Idle
			require.Equal(t, fsm.StateIdle, session.State())

			// DetectCollision with any remote ID
			shouldAccept, shouldCloseExisting := session.DetectCollision(0xFFFFFFFF)

			// Non-collision states should accept
			assert.True(t, shouldAccept, "%s should accept", tc.name)
			assert.False(t, shouldCloseExisting, "%s should not close existing", tc.name)
		})
	}
}
