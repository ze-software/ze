package reactor

import (
	"context"
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
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)

	session := NewSession(settings)

	require.NotNil(t, session)
	require.Equal(t, fsm.StateIdle, session.State())
	require.Nil(t, session.Conn())
	require.Nil(t, session.Negotiated())
}

// TestSessionPassiveMode verifies passive mode.
func TestSessionPassiveMode(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true

	session := NewSession(settings)

	err := session.Start()
	require.NoError(t, err)
	require.Equal(t, fsm.StateActive, session.State())
}

// TestSessionActiveMode verifies active mode.
func TestSessionActiveMode(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = false

	session := NewSession(settings)

	err := session.Start()
	require.NoError(t, err)
	require.Equal(t, fsm.StateConnect, session.State())
}

// TestSessionAcceptConnection verifies accepting incoming TCP connection.
func TestSessionAcceptConnection(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	_ = acceptWithReader(t, session, server, client)

	require.Equal(t, fsm.StateOpenSent, session.State())
	require.NotNil(t, session.Conn())
}

// TestSessionSendOpen verifies OPEN message is sent correctly.
func TestSessionSendOpen(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.HoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
	}
	settings.Passive = true

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

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
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

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
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

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
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true
	settings.HoldTime = 50 * time.Millisecond

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

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
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

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
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()

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
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Address = netip.MustParseAddr("10.255.255.1")
	settings.Port = 17900

	session := NewSession(settings)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := session.Connect(ctx)
	require.Error(t, err)
}

// TestSessionCapabilityNegotiation verifies capability intersection.
func TestSessionCapabilityNegotiation(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.RouteRefresh{},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

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

// TestSessionFamilyValidation verifies non-negotiated AFI/SAFI rejection.
//
// RFC 4760 Section 6: "If a BGP speaker receives an UPDATE with
// MP_REACH_NLRI or MP_UNREACH_NLRI where the AFI/SAFI do not match
// those negotiated in OPEN, the speaker MAY treat this as an error."
//
// VALIDATES: UPDATE with non-negotiated family returns error (default).
//
// PREVENTS: Processing NLRI for address families peer didn't negotiate.
func TestSessionFamilyValidation(t *testing.T) {
	// Setup: session with only IPv4 unicast negotiated
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	// Accept connection
	_ = acceptWithReader(t, session, server, client)

	// Peer's OPEN with same IPv4 unicast capability
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			2, 12, // Capability param, length 12
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4 = 65002 (code=65, len=4, data=4 bytes)
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4/Unicast (code=1, len=4, AFI=1, res=0, SAFI=1)
		},
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

	// Verify only IPv4 unicast is negotiated
	neg := session.Negotiated()
	require.True(t, neg.SupportsFamily(capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}))
	require.False(t, neg.SupportsFamily(capability.Family{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}))

	// Exchange KEEPALIVE to reach Established
	keepalive := message.NewKeepalive()
	keepaliveBytes, _ := keepalive.Pack(nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()

	err = session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateEstablished, session.State())

	// Build UPDATE with MP_REACH_NLRI for IPv6 unicast (NOT negotiated)
	// MP_REACH_NLRI: AFI=2 (IPv6), SAFI=1 (Unicast), NH=::1, NLRI=2001:db8::/32
	mpReach := []byte{
		0x00, 0x02, // AFI = 2 (IPv6)
		0x01, // SAFI = 1 (Unicast)
		0x10, // Next-hop length = 16
		// Next-hop: ::1
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, // Reserved
		// NLRI: 2001:db8::/32
		0x20, // Prefix length = 32
		0x20, 0x01, 0x0d, 0xb8,
	}

	// Path attributes: ORIGIN IGP (required), MP_REACH_NLRI
	pathAttrs := []byte{
		// ORIGIN = IGP
		0x40, 0x01, 0x01, 0x00,
		// MP_REACH_NLRI (optional non-transitive)
		0x80, 0x0e, byte(len(mpReach)),
	}
	pathAttrs = append(pathAttrs, mpReach...)

	// Build UPDATE message
	update := make([]byte, 0, 100)
	update = append(update, 0x00, 0x00)                                    // Withdrawn routes length = 0
	update = append(update, byte(len(pathAttrs)>>8), byte(len(pathAttrs))) // Path attrs length
	update = append(update, pathAttrs...)
	// No IPv4 NLRI

	// Pack as BGP message
	hdr := make([]byte, 19)
	for i := 0; i < 16; i++ {
		hdr[i] = 0xff // Marker
	}
	msgLen := uint16(19 + len(update)) // #nosec G115 -- test message size is small
	hdr[16] = byte(msgLen >> 8)
	hdr[17] = byte(msgLen)
	hdr[18] = byte(message.TypeUPDATE)

	updateMsg := hdr
	updateMsg = append(updateMsg, update...)

	go func() {
		_, _ = client.Write(updateMsg)
	}()

	// This should return an error because IPv6 unicast is not negotiated
	err = session.ReadAndProcess()
	require.Error(t, err, "should reject UPDATE with non-negotiated AFI/SAFI")
	require.Contains(t, err.Error(), "family", "error should mention family mismatch")
}

// TestSessionExtendedMessageValidation verifies RFC 8654 extended message validation.
//
// RFC 8654 Section 4: "The BGP Extended Message Capability applies to all
// messages except for OPEN and KEEPALIVE messages."
// RFC 8654 Section 5: "A BGP speaker that has not advertised the BGP Extended
// Message Capability... MUST NOT accept a BGP Extended Message."
//
// VALIDATES: UPDATE >4096 bytes rejected without extended message capability.
//
// PREVENTS: Buffer overflow from oversized messages without negotiation.
func TestSessionExtendedMessageValidation(t *testing.T) {
	// Setup: session WITHOUT extended message capability
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		// NOTE: No ExtendedMessage capability
	}

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	// Accept connection
	_ = acceptWithReader(t, session, server, client)

	// Peer's OPEN without extended message capability
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			2, 12,
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4/Unicast
		},
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

	// Verify extended message is NOT negotiated
	neg := session.Negotiated()
	require.False(t, neg.ExtendedMessage, "extended message should not be negotiated")

	// Exchange KEEPALIVE to reach Established
	keepalive := message.NewKeepalive()
	keepaliveBytes, _ := keepalive.Pack(nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()

	err = session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateEstablished, session.State())

	// Build UPDATE header with length > 4096 (e.g., 4100)
	// RFC 8654: Without extended message, this MUST be rejected
	hdr := make([]byte, 19)
	for i := 0; i < 16; i++ {
		hdr[i] = 0xff // Marker
	}
	// Length = 4100 (> 4096 max without extended message)
	hdr[16] = 0x10 // 4100 >> 8 = 16
	hdr[17] = 0x04 // 4100 & 0xFF = 4
	hdr[18] = byte(message.TypeUPDATE)

	go func() {
		_, _ = client.Write(hdr)
		// Drain any NOTIFICATION response
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()

	// This should return an error because message > 4096 without extended message
	err = session.ReadAndProcess()
	require.Error(t, err, "should reject UPDATE >4096 without extended message capability")
}

// TestSessionExtendedMessageAccepted verifies RFC 8654 extended message acceptance.
//
// RFC 8654 Section 4: "A BGP speaker MAY send BGP Extended Messages to a peer
// only if the BGP Extended Message Capability was received from that peer."
//
// VALIDATES: UPDATE >4096 bytes accepted WITH extended message capability.
//
// PREVENTS: Rejection of valid large UPDATE messages when capability is negotiated.
func TestSessionExtendedMessageAccepted(t *testing.T) {
	// Setup: session WITH extended message capability
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ExtendedMessage{}, // Enable extended message
	}

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	// Accept connection
	_ = acceptWithReader(t, session, server, client)

	// Peer's OPEN WITH extended message capability
	// Capability code 6 (ExtendedMessage), length 0
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			2, 14, // Capability param, length 14
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4/Unicast
			6, 0, // ExtendedMessage capability (code=6, len=0)
		},
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

	// Verify extended message IS negotiated
	neg := session.Negotiated()
	require.True(t, neg.ExtendedMessage, "extended message should be negotiated")

	// Exchange KEEPALIVE to reach Established
	keepalive := message.NewKeepalive()
	keepaliveBytes, _ := keepalive.Pack(nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()

	err = session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateEstablished, session.State())

	// Build valid UPDATE with length > 4096 (e.g., 5000)
	// RFC 8654: With extended message, this SHOULD be accepted
	updateMsg := make([]byte, 5000)
	for i := 0; i < 16; i++ {
		updateMsg[i] = 0xff // Marker
	}
	// Length = 5000 (> 4096, allowed with extended message)
	updateMsg[16] = 0x13 // 5000 >> 8 = 19
	updateMsg[17] = 0x88 // 5000 & 0xFF = 136
	updateMsg[18] = byte(message.TypeUPDATE)
	// Body starts at offset 19
	updateMsg[19] = 0x00 // Withdrawn routes length high
	updateMsg[20] = 0x00 // Withdrawn routes length low
	updateMsg[21] = 0x00 // Path attrs length high
	updateMsg[22] = 0x00 // Path attrs length low
	// Rest is padding (no NLRI)

	go func() {
		_, _ = client.Write(updateMsg)
	}()

	// This should NOT return an error because extended message is negotiated
	err = session.ReadAndProcess()
	require.NoError(t, err, "should accept UPDATE >4096 with extended message capability")
	require.Equal(t, fsm.StateEstablished, session.State(), "session should remain Established")
}

// TestSessionOpenAlwaysBounded verifies OPEN messages are always bounded.
//
// RFC 8654 Section 4: "The BGP Extended Message Capability applies to all
// messages except for OPEN and KEEPALIVE messages."
//
// VALIDATES: OPEN >4096 rejected even if extended message would be negotiated.
//
// PREVENTS: Oversized OPEN messages bypassing RFC 4271 limits.
func TestSessionOpenAlwaysBounded(t *testing.T) {
	// Setup: session that would accept extended message
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.ExtendedMessage{},
	}

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	// Accept connection (sends our OPEN)
	_ = acceptWithReader(t, session, server, client)

	// Build OPEN header with length > 4096 (e.g., 4100)
	// Even though peer might support extended message, OPEN is always bounded
	hdr := make([]byte, 19)
	for i := 0; i < 16; i++ {
		hdr[i] = 0xff // Marker
	}
	// Length = 4100 (> 4096 - invalid for OPEN regardless of extended message)
	hdr[16] = 0x10 // 4100 >> 8 = 16
	hdr[17] = 0x04 // 4100 & 0xFF = 4
	hdr[18] = byte(message.TypeOPEN)

	go func() {
		_, _ = client.Write(hdr)
		// Drain any NOTIFICATION response
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()

	// This should return an error - OPEN is always bounded to 4096
	err := session.ReadAndProcess()
	require.Error(t, err, "should reject OPEN >4096 even before extended message negotiation")
}

// TestSessionFamilyValidationIgnoreMismatch verifies ignore-mismatch mode.
//
// RFC 4760 Section 6: speaker MAY treat non-negotiated AFI/SAFI as error.
// With ignore-mismatch enabled, we log a warning but don't return error.
//
// VALIDATES: ignore-mismatch=true skips non-negotiated NLRI without error.
//
// PREVENTS: Connection drops with buggy peers that send extra families.
func TestSessionFamilyValidationIgnoreMismatch(t *testing.T) {
	// Setup: session with ignore-mismatch enabled
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Passive = true
	settings.IgnoreFamilyMismatch = true // Enable lenient mode
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	// Accept connection
	_ = acceptWithReader(t, session, server, client)

	// Peer's OPEN with IPv4 unicast capability
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			2, 12,
			65, 4, 0, 0, 0xFD, 0xEA,
			1, 4, 0, 1, 0, 1,
		},
	}
	openBytes, _ := peerOpen.Pack(nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	// Exchange KEEPALIVE to reach Established
	keepalive := message.NewKeepalive()
	keepaliveBytes, _ := keepalive.Pack(nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()

	err = session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateEstablished, session.State())

	// Build same UPDATE with non-negotiated IPv6 unicast
	mpReach := []byte{
		0x00, 0x02, 0x01, 0x10,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x20, 0x20, 0x01, 0x0d, 0xb8,
	}

	pathAttrs := []byte{0x40, 0x01, 0x01, 0x00, 0x80, 0x0e, byte(len(mpReach))}
	pathAttrs = append(pathAttrs, mpReach...)

	update := make([]byte, 0, 100)
	update = append(update, 0x00, 0x00)
	update = append(update, byte(len(pathAttrs)>>8), byte(len(pathAttrs)))
	update = append(update, pathAttrs...)

	hdr := make([]byte, 19)
	for i := 0; i < 16; i++ {
		hdr[i] = 0xff
	}
	msgLen := uint16(19 + len(update)) // #nosec G115 -- test message size is small
	hdr[16] = byte(msgLen >> 8)
	hdr[17] = byte(msgLen)
	hdr[18] = byte(message.TypeUPDATE)

	updateMsg := hdr
	updateMsg = append(updateMsg, update...)

	go func() {
		_, _ = client.Write(updateMsg)
	}()

	// With ignore-mismatch, this should NOT return an error
	err = session.ReadAndProcess()
	require.NoError(t, err, "ignore-mismatch should skip non-negotiated family without error")
	require.Equal(t, fsm.StateEstablished, session.State(), "session should remain Established")
}
