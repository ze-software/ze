package reactor

import (
	"context"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/exa-networks/zebgp/pkg/bgp/capability"
	"github.com/exa-networks/zebgp/pkg/bgp/fsm"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
)

// acceptWithReader handles net.Pipe's synchronous behavior by reading
// from client while Accept writes.
func acceptWithReader(t *testing.T, session *Session, server, client net.Conn) []byte {
	buf := make([]byte, 4096)
	var n int
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		n, _ = client.Read(buf)
	}()

	err := session.Accept(server)
	require.NoError(t, err)

	wg.Wait()
	return buf[:n]
}

// TestSessionCreation verifies Session initialization.
func TestSessionCreation(t *testing.T) {
	neighbor := NewNeighbor(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)

	session := NewSession(neighbor)

	require.NotNil(t, session)
	require.Equal(t, fsm.StateIdle, session.State())
	require.Nil(t, session.Conn())
	require.Nil(t, session.Negotiated())
}

// TestSessionPassiveMode verifies passive mode.
func TestSessionPassiveMode(t *testing.T) {
	neighbor := NewNeighbor(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	neighbor.Passive = true

	session := NewSession(neighbor)

	err := session.Start()
	require.NoError(t, err)
	require.Equal(t, fsm.StateActive, session.State())
}

// TestSessionActiveMode verifies active mode.
func TestSessionActiveMode(t *testing.T) {
	neighbor := NewNeighbor(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	neighbor.Passive = false

	session := NewSession(neighbor)

	err := session.Start()
	require.NoError(t, err)
	require.Equal(t, fsm.StateConnect, session.State())
}

// TestSessionAcceptConnection verifies accepting incoming TCP connection.
func TestSessionAcceptConnection(t *testing.T) {
	neighbor := NewNeighbor(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	neighbor.Passive = true

	session := NewSession(neighbor)
	_ = session.Start()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	_ = acceptWithReader(t, session, server, client)

	require.Equal(t, fsm.StateOpenSent, session.State())
	require.NotNil(t, session.Conn())
}

// TestSessionSendOpen verifies OPEN message is sent correctly.
func TestSessionSendOpen(t *testing.T) {
	neighbor := NewNeighbor(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	neighbor.HoldTime = 90 * time.Second
	neighbor.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
	}
	neighbor.Passive = true

	session := NewSession(neighbor)
	_ = session.Start()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	buf := acceptWithReader(t, session, server, client)
	require.Greater(t, len(buf), message.HeaderLen)

	// Parse header
	hdr, err := message.ParseHeader(buf[:message.HeaderLen])
	require.NoError(t, err)
	require.Equal(t, message.TypeOPEN, hdr.Type)

	// Parse OPEN
	open, err := message.UnpackOpen(buf[message.HeaderLen:hdr.Length])
	require.NoError(t, err)
	require.Equal(t, uint16(65001), open.MyAS)
	require.Equal(t, uint16(90), open.HoldTime)
	require.Equal(t, uint32(0x01020301), open.BGPIdentifier)
}

// TestSessionReceiveOpen verifies processing of received OPEN message.
func TestSessionReceiveOpen(t *testing.T) {
	neighbor := NewNeighbor(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	neighbor.Passive = true

	session := NewSession(neighbor)
	_ = session.Start()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// Accept (reads our OPEN)
	_ = acceptWithReader(t, session, server, client)
	require.Equal(t, fsm.StateOpenSent, session.State())

	// Send peer's OPEN and drain KEEPALIVE response in goroutine
	peerOpen := &message.Open{
		Version:       4,
		MyAS:          65002,
		HoldTime:      90,
		BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			2, 6, 65, 4, 0, 0, 0xFD, 0xEA, // ASN4
		},
	}
	openBytes, _ := peerOpen.Pack(nil)

	// Start goroutine to write OPEN and drain KEEPALIVE
	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateOpenConfirm, session.State())
	require.NotNil(t, session.Negotiated())
}

// TestSessionKeepaliveExchange verifies KEEPALIVE handling.
func TestSessionKeepaliveExchange(t *testing.T) {
	neighbor := NewNeighbor(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	neighbor.Passive = true

	session := NewSession(neighbor)
	_ = session.Start()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	_ = acceptWithReader(t, session, server, client)

	// Send peer's OPEN and drain KEEPALIVE
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
	}
	openBytes, _ := peerOpen.Pack(nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	// Send peer's KEEPALIVE (in goroutine since ReadAndProcess blocks)
	keepalive := message.NewKeepalive()
	keepaliveBytes, _ := keepalive.Pack(nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()

	err = session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateEstablished, session.State())
}

// TestSessionHoldTimerExpiry verifies dead peer detection.
func TestSessionHoldTimerExpiry(t *testing.T) {
	neighbor := NewNeighbor(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	neighbor.Passive = true
	neighbor.HoldTime = 50 * time.Millisecond

	session := NewSession(neighbor)
	_ = session.Start()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	_ = acceptWithReader(t, session, server, client)

	// Send peer's OPEN and drain KEEPALIVE
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 1, BGPIdentifier: 0x01020302,
	}
	openBytes, _ := peerOpen.Pack(nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()
	_ = session.ReadAndProcess()

	// Send KEEPALIVE to establish
	keepalive := message.NewKeepalive()
	keepaliveBytes, _ := keepalive.Pack(nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()
	_ = session.ReadAndProcess()

	require.Equal(t, fsm.StateEstablished, session.State())

	// Trigger hold timer expiry manually
	session.TriggerHoldTimerExpiry()
	require.Equal(t, fsm.StateIdle, session.State())
}

// TestSessionNotification verifies NOTIFICATION handling.
func TestSessionNotification(t *testing.T) {
	neighbor := NewNeighbor(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	neighbor.Passive = true

	session := NewSession(neighbor)
	_ = session.Start()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	_ = acceptWithReader(t, session, server, client)

	// Send NOTIFICATION (in goroutine)
	notif := &message.Notification{
		ErrorCode:    message.NotifyOpenMessage,
		ErrorSubcode: message.NotifyOpenUnsupportedVersion,
	}
	notifBytes, _ := notif.Pack(nil)

	go func() {
		_, _ = client.Write(notifBytes)
	}()

	err := session.ReadAndProcess()
	require.Error(t, err)
	require.Equal(t, fsm.StateIdle, session.State())
}

// TestSessionGracefulClose verifies clean shutdown.
func TestSessionGracefulClose(t *testing.T) {
	neighbor := NewNeighbor(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	neighbor.Passive = true

	session := NewSession(neighbor)
	_ = session.Start()

	client, server := net.Pipe()
	defer client.Close()

	_ = acceptWithReader(t, session, server, client)

	// Close session gracefully (sends NOTIFICATION)
	go func() {
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()

	err := session.Close()
	require.NoError(t, err)
	require.Equal(t, fsm.StateIdle, session.State())
}

// TestSessionConnectContext verifies context cancellation during connect.
func TestSessionConnectContext(t *testing.T) {
	neighbor := NewNeighbor(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	neighbor.Address = netip.MustParseAddr("10.255.255.1")
	neighbor.Port = 17900

	session := NewSession(neighbor)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := session.Connect(ctx)
	require.Error(t, err)
}

// TestSessionCapabilityNegotiation verifies capability intersection.
func TestSessionCapabilityNegotiation(t *testing.T) {
	neighbor := NewNeighbor(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	neighbor.Passive = true
	neighbor.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.RouteRefresh{},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(neighbor)
	_ = session.Start()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	_ = acceptWithReader(t, session, server, client)

	// Send peer's OPEN with only ASN4
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{2, 6, 65, 4, 0, 0, 0xFD, 0xEA},
	}
	openBytes, _ := peerOpen.Pack(nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()

	_ = session.ReadAndProcess()

	neg := session.Negotiated()
	require.NotNil(t, neg)
	require.True(t, neg.ASN4)
	require.False(t, neg.RouteRefresh)
}

// mockConn for testing edge cases
type mockConn struct {
	readData  []byte
	readErr   error
	writeData []byte
	writeErr  error
	closed    bool
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	if m.readErr != nil {
		return 0, m.readErr
	}
	if len(m.readData) == 0 {
		return 0, io.EOF
	}
	n = copy(b, m.readData)
	m.readData = m.readData[n:]
	return n, nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *mockConn) Close() error                       { m.closed = true; return nil }
func (m *mockConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (m *mockConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }
