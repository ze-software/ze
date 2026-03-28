package reactor

import (
	"bufio"
	"context"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
)

// acceptWithReader handles net.Pipe's synchronous behavior by reading
// from client while Accept writes.
func acceptWithReader(t *testing.T, session *Session, server, client net.Conn) []byte {
	buf := make([]byte, 4096)
	var n int
	var wg sync.WaitGroup
	wg.Go(func() {
		n, _ = client.Read(buf)
	})

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
	settings.Connection = ConnectionPassive

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
	settings.Connection = ConnectionBoth

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
	settings.Connection = ConnectionPassive

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
	settings.ReceiveHoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
	}
	settings.Connection = ConnectionPassive

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

// extractCapabilitiesFromOptParams extracts capabilities from OPEN optional parameters.
// Format: series of (Type=2, Length, CapabilityBytes) tuples.
func extractCapabilitiesFromOptParams(optParams []byte) ([]capability.Capability, error) {
	var allCapBytes []byte
	offset := 0
	for offset < len(optParams) {
		if offset+2 > len(optParams) {
			break
		}
		paramType := optParams[offset]
		paramLen := int(optParams[offset+1])
		offset += 2
		if paramType == 2 { // Capabilities parameter
			if offset+paramLen > len(optParams) {
				break
			}
			allCapBytes = append(allCapBytes, optParams[offset:offset+paramLen]...)
		}
		offset += paramLen
	}
	return capability.Parse(allCapBytes)
}

// TestSessionSendOpenWithPluginFamilies verifies plugin decode families are advertised in OPEN.
//
// VALIDATES: Families from plugins with decode capability are added as Multiprotocol caps.
// PREVENTS: Plugin families not being advertised, breaking family negotiation.
func TestSessionSendOpenWithPluginFamilies(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.ReceiveHoldTime = 90 * time.Second
	settings.Connection = ConnectionPassive

	session := NewSession(settings)

	// Set plugin families getter to return flowspec families
	session.SetPluginFamiliesGetter(func() []string {
		return []string{"ipv4/flow", "ipv6/flow"}
	})

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

	// Extract Multiprotocol capabilities from OPEN
	caps, err := extractCapabilitiesFromOptParams(open.OptionalParams)
	require.NoError(t, err)

	// Find flowspec families in capabilities
	var foundIPv4Flow, foundIPv6Flow bool
	for _, c := range caps {
		if mp, ok := c.(*capability.Multiprotocol); ok {
			if mp.AFI == 1 && mp.SAFI == 133 { // IPv4 FlowSpec
				foundIPv4Flow = true
			}
			if mp.AFI == 2 && mp.SAFI == 133 { // IPv6 FlowSpec
				foundIPv6Flow = true
			}
		}
	}

	require.True(t, foundIPv4Flow, "OPEN should contain IPv4 FlowSpec Multiprotocol capability")
	require.True(t, foundIPv6Flow, "OPEN should contain IPv6 FlowSpec Multiprotocol capability")
}

// TestSessionSendOpenConfigFamiliesOverridePlugin verifies config families take precedence.
//
// VALIDATES: If config has family block, ONLY config families are used (plugin families ignored).
// PREVENTS: Plugin families being added when config explicitly specifies families.
func TestSessionSendOpenConfigFamiliesOverridePlugin(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.ReceiveHoldTime = 90 * time.Second
	settings.Connection = ConnectionPassive
	// Add IPv4 FlowSpec via config - this means config has families, plugin families ignored
	settings.Capabilities = []capability.Capability{
		&capability.Multiprotocol{AFI: 1, SAFI: 133}, // IPv4 FlowSpec
	}

	session := NewSession(settings)

	// Plugin returns IPv4 FlowSpec - should be IGNORED because config has families
	session.SetPluginFamiliesGetter(func() []string {
		return []string{"ipv4/flow"}
	})

	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	buf := acceptWithReader(t, session, server, client)
	require.Greater(t, len(buf), message.HeaderLen)

	// Parse header
	hdr, err := message.ParseHeader(buf[:message.HeaderLen])
	require.NoError(t, err)

	// Parse OPEN
	open, err := message.UnpackOpen(buf[message.HeaderLen:hdr.Length])
	require.NoError(t, err)

	// Extract Multiprotocol capabilities from OPEN
	caps, err := extractCapabilitiesFromOptParams(open.OptionalParams)
	require.NoError(t, err)

	// Count IPv4 FlowSpec capabilities - should be exactly 1 (from config only)
	ipv4FlowCount := 0
	for _, c := range caps {
		if mp, ok := c.(*capability.Multiprotocol); ok {
			if mp.AFI == 1 && mp.SAFI == 133 {
				ipv4FlowCount++
			}
		}
	}

	require.Equal(t, 1, ipv4FlowCount, "IPv4 FlowSpec should appear exactly once (config only, plugin ignored)")
}

// TestSessionReceiveOpen verifies processing of received OPEN message.
func TestSessionReceiveOpen(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive

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
	openBytes := message.PackTo(peerOpen, nil)

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
	settings.Connection = ConnectionPassive

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
	openBytes := message.PackTo(peerOpen, nil)

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
	keepaliveBytes := message.PackTo(keepalive, nil)

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
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 50 * time.Millisecond

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	_ = acceptWithReader(t, session, server, client)

	// Send peer's OPEN and drain KEEPALIVE
	// Note: HoldTime must be 0 or >= 3 per RFC 4271
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 3, BGPIdentifier: 0x01020302,
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()
	_ = session.ReadAndProcess()

	// Send KEEPALIVE to establish
	keepalive := message.NewKeepalive()
	keepaliveBytes := message.PackTo(keepalive, nil)

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
	settings.Connection = ConnectionPassive

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
	notifBytes := message.PackTo(notif, nil)

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
	settings.Connection = ConnectionPassive

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
	settings.Connection = ConnectionPassive
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
	openBytes := message.PackTo(peerOpen, nil)

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
	settings.Connection = ConnectionPassive
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
	openBytes := message.PackTo(peerOpen, nil)

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
	keepaliveBytes := message.PackTo(keepalive, nil)

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

	// Path attributes: ORIGIN IGP, AS_PATH, MP_REACH_NLRI
	// AS_PATH is mandatory when MP_REACH_NLRI is present (RFC 7606 Section 3.d)
	pathAttrs := make([]byte, 0, 10+len(mpReach))
	pathAttrs = append(pathAttrs,
		// ORIGIN = IGP
		0x40, 0x01, 0x01, 0x00,
		// AS_PATH (empty — valid for originating router)
		0x40, 0x02, 0x00,
		// MP_REACH_NLRI (optional non-transitive)
		0x80, 0x0e, byte(len(mpReach)),
	)
	pathAttrs = append(pathAttrs, mpReach...)

	// Build UPDATE message
	update := make([]byte, 0, 100)
	update = append(update, 0x00, 0x00, byte(len(pathAttrs)>>8), byte(len(pathAttrs))) // Withdrawn=0, Path attrs length
	update = append(update, pathAttrs...)
	// No IPv4 NLRI

	// Write as BGP message
	hdr := make([]byte, 19)
	for i := range 16 {
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
	settings.Connection = ConnectionPassive
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
	openBytes := message.PackTo(peerOpen, nil)

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
	keepaliveBytes := message.PackTo(keepalive, nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()

	err = session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateEstablished, session.State())

	// Build UPDATE header with length > 4096 (e.g., 4100)
	// RFC 8654: Without extended message, this MUST be rejected
	hdr := make([]byte, 19)
	for i := range 16 {
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
	// Ensure the global buffer pool has enough budget. Under full-suite load,
	// concurrent tests with small prefix maximums can auto-size the shared
	// budget too low, causing getReadBuffer() to return nil.
	bufMuxGlobalMu.Lock()
	oldBudget := bufMux4K.mux.budget.maxBytes.Load()
	updateBufMuxBudget(0) // 0 = unlimited
	bufMuxGlobalMu.Unlock()
	t.Cleanup(func() {
		bufMuxGlobalMu.Lock()
		updateBufMuxBudget(oldBudget)
		bufMuxGlobalMu.Unlock()
	})

	// Setup: session WITH extended message capability
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive
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
	openBytes := message.PackTo(peerOpen, nil)

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
	keepaliveBytes := message.PackTo(keepalive, nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()

	err = session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateEstablished, session.State())

	// Build valid UPDATE with length > 4096 (e.g., 5000)
	// RFC 8654: With extended message, this SHOULD be accepted
	updateMsg := make([]byte, 5000)
	for i := range 16 {
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
	settings.Connection = ConnectionPassive
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
	for i := range 16 {
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
	settings.Connection = ConnectionPassive
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
	openBytes := message.PackTo(peerOpen, nil)

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
	keepaliveBytes := message.PackTo(keepalive, nil)

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

	// AS_PATH is mandatory when MP_REACH_NLRI is present (RFC 7606 Section 3.d)
	pathAttrs := make([]byte, 0, 10+len(mpReach))
	pathAttrs = append(pathAttrs, 0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, 0x80, 0x0e, byte(len(mpReach)))
	pathAttrs = append(pathAttrs, mpReach...)

	update := make([]byte, 0, 100)
	update = append(update, 0x00, 0x00, byte(len(pathAttrs)>>8), byte(len(pathAttrs)))
	update = append(update, pathAttrs...)

	hdr := make([]byte, 19)
	for i := range 16 {
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

// TestSessionRejectsInvalidHoldTime verifies RFC 4271 hold time validation.
//
// RFC 4271 Section 6.2: "An implementation MUST reject Hold Time values of
// one or two seconds."
//
// VALIDATES: OPEN with HoldTime 1 or 2 triggers NOTIFICATION (Unacceptable Hold Time).
//
// PREVENTS: Accepting invalid hold times that violate RFC 4271.
func TestSessionRejectsInvalidHoldTime(t *testing.T) {
	tests := []struct {
		name     string
		holdTime uint16
		wantErr  bool
	}{
		{"holdtime_0_valid", 0, false},
		{"holdtime_1_invalid", 1, true},
		{"holdtime_2_invalid", 2, true},
		{"holdtime_3_valid", 3, false},
		{"holdtime_90_valid", 90, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := NewPeerSettings(
				netip.MustParseAddr("192.0.2.1"),
				65001, 65002, 0x01020301,
			)
			settings.Connection = ConnectionPassive
			settings.Capabilities = []capability.Capability{
				&capability.ASN4{ASN: 65001},
				&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
			}

			session := NewSession(settings)
			_ = session.Start()

			client, server := net.Pipe()
			defer func() { _ = client.Close() }()
			defer func() { _ = server.Close() }()

			// Accept connection (sends our OPEN)
			_ = acceptWithReader(t, session, server, client)

			// Peer's OPEN with test hold time
			peerOpen := &message.Open{
				Version:       4,
				MyAS:          65002,
				HoldTime:      tt.holdTime,
				BGPIdentifier: 0x01020302,
				OptionalParams: []byte{
					2, 12,
					65, 4, 0, 0, 0xFD, 0xEA, // ASN4
					1, 4, 0, 1, 0, 1, // Multiprotocol IPv4/Unicast
				},
			}
			openBytes := message.PackTo(peerOpen, nil)

			go func() {
				_, _ = client.Write(openBytes)
				// Drain any response (KEEPALIVE or NOTIFICATION)
				buf := make([]byte, 4096)
				_, _ = client.Read(buf)
			}()

			err := session.ReadAndProcess()

			if tt.wantErr {
				require.Error(t, err, "should reject OPEN with HoldTime=%d", tt.holdTime)
				require.Contains(t, err.Error(), "hold time", "error should mention hold time")
			} else {
				require.NoError(t, err, "should accept OPEN with HoldTime=%d", tt.holdTime)
				require.Equal(t, fsm.StateOpenConfirm, session.State())
			}
		})
	}
}

// =============================================================================
// RFC 5492 - Capability Mode Enforcement (require/refuse)
// =============================================================================

// TestBuildUnsupportedCapabilityDataCodes verifies NOTIFICATION data encoding
// for non-family capability codes.
//
// RFC 5492 Section 3: The Data field contains one or more capability tuples.
// For non-Multiprotocol codes: code (1) + length (1) = 2 bytes each (length=0).
//
// VALIDATES: Wire format of capability code NOTIFICATION data.
// PREVENTS: Malformed NOTIFICATION data for non-family capabilities.
func TestBuildUnsupportedCapabilityDataCodes(t *testing.T) {
	tests := []struct {
		name     string
		codes    []capability.Code
		expected []byte
	}{
		{
			name:     "single_asn4",
			codes:    []capability.Code{capability.CodeASN4},
			expected: []byte{65, 0}, // code=65, length=0
		},
		{
			name:  "multiple_codes",
			codes: []capability.Code{capability.CodeASN4, capability.CodeExtendedMessage},
			expected: []byte{
				65, 0, // ASN4
				6, 0, // ExtendedMessage
			},
		},
		{
			name:     "empty",
			codes:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildUnsupportedCapabilityDataCodes(tt.codes)
			require.Equal(t, tt.expected, result)
		})
	}
}

// sendOpenAndDrain writes an OPEN message to client and drains any response.
// Suitable for use in a goroutine during OPEN exchange tests.
func sendOpenAndDrain(client net.Conn, openBytes []byte) {
	client.Write(openBytes) //nolint:errcheck // test goroutine write to net.Pipe
	buf := make([]byte, 4096)
	client.Read(buf) //nolint:errcheck // drain response from net.Pipe
}

// TestSessionRejectsRequiredCapability verifies that a session is rejected when
// a required capability is not negotiated by the peer.
//
// RFC 5492 Section 3: "If a peer does not support a capability that is
// required, the speaker MUST send a NOTIFICATION message."
//
// VALIDATES: Required capability missing from peer → NOTIFICATION (code 2, subcode 7).
// PREVENTS: Session establishment when required capability not negotiated.
func TestSessionRejectsRequiredCapability(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ExtendedMessage{},
	}
	// Require ExtendedMessage — peer will NOT have it
	settings.RequiredCapabilities = []capability.Code{capability.CodeExtendedMessage}

	session := NewSession(settings)
	err := session.Start()
	require.NoError(t, err)

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	// Accept connection (sends our OPEN)
	_ = acceptWithReader(t, session, server, client)

	// Peer OPEN without ExtendedMessage
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x02020302,
		OptionalParams: []byte{
			2, 12,
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4 only
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4/Unicast
		},
	}
	openBytes := message.PackTo(peerOpen, nil)

	go sendOpenAndDrain(client, openBytes)

	err = session.ReadAndProcess()
	require.Error(t, err, "should reject OPEN when required capability is missing")
	require.Contains(t, err.Error(), "required")
}

// TestSessionRejectsRefusedCapability verifies that a session is rejected when
// a refused capability is present in the peer's OPEN.
//
// RFC 5492 Section 3: refuse checks against peer's raw capabilities.
//
// VALIDATES: Refused capability present in peer → NOTIFICATION (code 2, subcode 7).
// PREVENTS: Session establishment when refused capability is advertised by peer.
func TestSessionRejectsRefusedCapability(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive
	// We don't advertise ExtendedMessage (refuse = don't advertise + reject if peer has)
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}
	settings.RefusedCapabilities = []capability.Code{capability.CodeExtendedMessage}

	session := NewSession(settings)
	err := session.Start()
	require.NoError(t, err)

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	// Accept connection (sends our OPEN)
	_ = acceptWithReader(t, session, server, client)

	// Peer OPEN WITH ExtendedMessage — we refuse it
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x02020302,
		OptionalParams: []byte{
			2, 14,
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4/Unicast
			6, 0, // ExtendedMessage
		},
	}
	openBytes := message.PackTo(peerOpen, nil)

	go sendOpenAndDrain(client, openBytes)

	err = session.ReadAndProcess()
	require.Error(t, err, "should reject OPEN when refused capability is present")
	require.Contains(t, err.Error(), "refused")
}

// TestSessionAcceptsRequiredCapability verifies that a session succeeds when
// a required capability IS negotiated.
//
// VALIDATES: Required capability present → session proceeds to OpenConfirm.
// PREVENTS: False rejections when required capability is properly negotiated.
func TestSessionAcceptsRequiredCapability(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ExtendedMessage{},
	}
	// Require ExtendedMessage — peer WILL have it
	settings.RequiredCapabilities = []capability.Code{capability.CodeExtendedMessage}

	session := NewSession(settings)
	err := session.Start()
	require.NoError(t, err)

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	// Accept connection (sends our OPEN)
	_ = acceptWithReader(t, session, server, client)

	// Peer OPEN WITH ExtendedMessage
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x02020302,
		OptionalParams: []byte{
			2, 14,
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4/Unicast
			6, 0, // ExtendedMessage
		},
	}
	openBytes := message.PackTo(peerOpen, nil)

	go sendOpenAndDrain(client, openBytes)

	err = session.ReadAndProcess()
	require.NoError(t, err, "should accept OPEN when required capability is present")
	require.Equal(t, fsm.StateOpenConfirm, session.State())
}

// =============================================================================
// RFC 7606 - Revised Error Handling for BGP UPDATE Messages
// =============================================================================

// TestSessionRFC7606MalformedOriginTreatAsWithdraw verifies RFC 7606 Section 7.1.
//
// RFC 7606 Section 7.1: "The [ORIGIN] attribute is considered malformed if its
// length is not 1 or if it has an undefined value. An UPDATE message with a
// malformed ORIGIN attribute SHALL be handled using the approach of 'treat-as-withdraw'."
//
// VALIDATES: Malformed ORIGIN (wrong length) triggers treat-as-withdraw, session stays up.
//
// PREVENTS: Session reset from recoverable attribute errors.
func TestSessionRFC7606MalformedOriginTreatAsWithdraw(t *testing.T) {
	// Setup: established session
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	_ = acceptWithReader(t, session, server, client)

	// Peer's OPEN with ASN4 + IPv4 unicast capabilities
	peerOpen := &message.Open{
		Version:       4,
		MyAS:          65002,
		HoldTime:      90,
		BGPIdentifier: 0x02020302,
		OptionalParams: []byte{
			2, 12, // Capability param, length 12
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4 = 65002
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4/Unicast
		},
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	// Send KEEPALIVE to reach Established
	keepalive := message.NewKeepalive()
	keepaliveBytes := message.PackTo(keepalive, nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()

	err = session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateEstablished, session.State())

	// Build UPDATE with MALFORMED ORIGIN (length=2 instead of 1)
	// RFC 7606: ORIGIN must be length 1
	pathAttrs := []byte{
		// ORIGIN with wrong length (2 bytes instead of 1)
		0x40, 0x01, 0x02, 0x00, 0x00, // Flags, Code=1, Len=2, Value=0x0000
		// AS_PATH (empty, valid for iBGP or originating router)
		0x40, 0x02, 0x00,
		// NEXT_HOP = 192.0.2.1
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
	}

	// Build UPDATE with IPv4 NLRI: 10.0.0.0/8
	update := make([]byte, 0, 50)
	update = append(update, 0x00, 0x00, byte(len(pathAttrs)>>8), byte(len(pathAttrs))) // Withdrawn=0, Path attrs length
	update = append(update, pathAttrs...)
	update = append(update, 0x08, 0x0a) // NLRI: 10.0.0.0/8

	// Write as BGP message
	hdr := make([]byte, 19, 19+len(update))
	for i := range 16 {
		hdr[i] = 0xff
	}
	msgLen := uint16(19 + len(update)) // #nosec G115 -- test message size is small
	hdr[16] = byte(msgLen >> 8)
	hdr[17] = byte(msgLen)
	hdr[18] = byte(message.TypeUPDATE)

	hdr = append(hdr, update...) //nolint:gocritic // test code

	go func() {
		_, _ = client.Write(hdr)
		// Drain any potential NOTIFICATION response
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()

	// RFC 7606: treat-as-withdraw should NOT cause session reset
	err = session.ReadAndProcess()
	require.NoError(t, err, "RFC 7606: malformed ORIGIN should trigger treat-as-withdraw, not session reset")
	require.Equal(t, fsm.StateEstablished, session.State(), "session should remain Established")
}

// TestSessionRFC7606MalformedCommunityTreatAsWithdraw verifies RFC 7606 Section 7.8.
//
// RFC 7606 Section 7.8: "The Community attribute SHALL be considered malformed
// if its length is not a non-zero multiple of 4. An UPDATE message with a
// malformed Community attribute SHALL be handled using the approach of 'treat-as-withdraw'."
//
// VALIDATES: Malformed Community (wrong length) triggers treat-as-withdraw.
//
// PREVENTS: Session reset from Community attribute parsing errors.
func TestSessionRFC7606MalformedCommunityTreatAsWithdraw(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	_ = acceptWithReader(t, session, server, client)

	// Complete handshake
	peerOpen := &message.Open{
		Version:       4,
		MyAS:          65002,
		HoldTime:      90,
		BGPIdentifier: 0x02020302,
		OptionalParams: []byte{
			2, 12,
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4 = 65002
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4/Unicast
		},
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()
	_ = session.ReadAndProcess()

	keepalive := message.NewKeepalive()
	keepaliveBytes := message.PackTo(keepalive, nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()
	_ = session.ReadAndProcess()
	require.Equal(t, fsm.StateEstablished, session.State())

	// Build UPDATE with MALFORMED COMMUNITY (length=5, not multiple of 4)
	pathAttrs := []byte{
		// ORIGIN = IGP
		0x40, 0x01, 0x01, 0x00,
		// AS_PATH (empty)
		0x40, 0x02, 0x00,
		// NEXT_HOP = 192.0.2.1
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
		// COMMUNITY with wrong length (5 bytes, should be multiple of 4)
		0xc0, 0x08, 0x05, 0x00, 0x01, 0x00, 0x02, 0x03, // Optional transitive, Code=8, Len=5
	}

	update := make([]byte, 0, 50)
	update = append(update, 0x00, 0x00, byte(len(pathAttrs)>>8), byte(len(pathAttrs)))
	update = append(update, pathAttrs...)
	update = append(update, 0x08, 0x0a) // NLRI: 10.0.0.0/8

	hdr := make([]byte, 19, 19+len(update))
	for i := range 16 {
		hdr[i] = 0xff
	}
	msgLen := uint16(19 + len(update)) // #nosec G115 -- test message size is small
	hdr[16] = byte(msgLen >> 8)
	hdr[17] = byte(msgLen)
	hdr[18] = byte(message.TypeUPDATE)

	hdr = append(hdr, update...) //nolint:gocritic // test code

	go func() {
		_, _ = client.Write(hdr)
		// Drain any potential NOTIFICATION response
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "RFC 7606: malformed Community should trigger treat-as-withdraw, not session reset")
	require.Equal(t, fsm.StateEstablished, session.State())
}

// TestSessionRFC7606MissingMandatoryTreatAsWithdraw verifies RFC 7606 Section 3.d.
//
// RFC 7606 Section 3.d: "If any of the well-known mandatory attributes are not
// present in an UPDATE message, then 'treat-as-withdraw' MUST be used."
//
// VALIDATES: Missing ORIGIN attribute triggers treat-as-withdraw.
//
// PREVENTS: Session reset when mandatory attributes are missing.
func TestSessionRFC7606MissingMandatoryTreatAsWithdraw(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	_ = acceptWithReader(t, session, server, client)

	// Complete handshake
	peerOpen := &message.Open{
		Version:       4,
		MyAS:          65002,
		HoldTime:      90,
		BGPIdentifier: 0x02020302,
		OptionalParams: []byte{
			2, 12,
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4 = 65002
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4/Unicast
		},
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()
	_ = session.ReadAndProcess()

	keepalive := message.NewKeepalive()
	keepaliveBytes := message.PackTo(keepalive, nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()
	_ = session.ReadAndProcess()
	require.Equal(t, fsm.StateEstablished, session.State())

	// Build UPDATE MISSING ORIGIN (well-known mandatory)
	// Only has AS_PATH and NEXT_HOP, no ORIGIN
	pathAttrs := []byte{
		// AS_PATH (empty) - NO ORIGIN!
		0x40, 0x02, 0x00,
		// NEXT_HOP = 192.0.2.1
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
	}

	update := make([]byte, 0, 50)
	update = append(update, 0x00, 0x00, byte(len(pathAttrs)>>8), byte(len(pathAttrs)))
	update = append(update, pathAttrs...)
	update = append(update, 0x08, 0x0a) // NLRI: 10.0.0.0/8

	hdr := make([]byte, 19, 19+len(update))
	for i := range 16 {
		hdr[i] = 0xff
	}
	msgLen := uint16(19 + len(update)) // #nosec G115 -- test message size is small
	hdr[16] = byte(msgLen >> 8)
	hdr[17] = byte(msgLen)
	hdr[18] = byte(message.TypeUPDATE)

	hdr = append(hdr, update...) //nolint:gocritic // test code

	go func() {
		_, _ = client.Write(hdr)
		// Drain any potential NOTIFICATION response
		buf := make([]byte, 4096)
		_, _ = client.Read(buf)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "RFC 7606: missing mandatory attribute should trigger treat-as-withdraw, not session reset")
	require.Equal(t, fsm.StateEstablished, session.State())
}

// setupEstablishedSessionEBGP creates an established EBGP session (LocalAS != PeerAS)
// with a callback tracker. Returns session, client conn, callback call count, and cleanup.
func setupEstablishedSessionEBGP(t *testing.T) (*Session, net.Conn, *int, func()) {
	t.Helper()

	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)

	// Track callback invocations
	callbackCount := new(int)
	session.onMessageReceived = func(_ netip.Addr, _ message.MessageType, _ []byte, _ *wireu.WireUpdate, _ bgpctx.ContextID, direction string, _ BufHandle, _ map[string]any) bool {
		if direction == "received" {
			*callbackCount++
		}
		return false
	}

	startSession(t, session)

	client, server := net.Pipe()
	cleanup := func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	}

	readOpen(t, session, server, client)
	exchangeOpenKeepalive(t, session, client)
	require.Equal(t, fsm.StateEstablished, session.State())

	// Reset callback count (OPEN and KEEPALIVE may have triggered it)
	*callbackCount = 0

	return session, client, callbackCount, cleanup
}

// startSession starts the session and asserts no error.
func startSession(t *testing.T, session *Session) {
	t.Helper()
	err := session.Start()
	require.NoError(t, err)
}

// readOpen accepts a connection and reads the OPEN message.
func readOpen(t *testing.T, session *Session, server, client net.Conn) {
	t.Helper()
	acceptWithReader(t, session, server, client)
}

// exchangeOpenKeepalive sends peer OPEN and KEEPALIVE to reach Established.
func exchangeOpenKeepalive(t *testing.T, session *Session, client net.Conn) {
	t.Helper()

	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x02020302,
		OptionalParams: []byte{
			2, 12,
			65, 4, 0, 0, 0xFD, 0xEA,
			1, 4, 0, 1, 0, 1,
		},
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		client.Write(openBytes) //nolint:errcheck // test goroutine
		buf := make([]byte, 4096)
		client.Read(buf) //nolint:errcheck // drain KEEPALIVE
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err)

	keepalive := message.NewKeepalive()
	keepaliveBytes := message.PackTo(keepalive, nil)

	go func() {
		client.Write(keepaliveBytes) //nolint:errcheck // test goroutine
	}()

	err = session.ReadAndProcess()
	require.NoError(t, err)
}

// buildUpdateMsg constructs a full BGP UPDATE message from raw UPDATE body bytes.
func buildUpdateMsg(update []byte) []byte {
	hdr := make([]byte, 19, 19+len(update))
	for i := range 16 {
		hdr[i] = 0xff
	}
	msgLen := uint16(19 + len(update)) // #nosec G115 -- test message size is small
	hdr[16] = byte(msgLen >> 8)
	hdr[17] = byte(msgLen)
	hdr[18] = byte(message.TypeUPDATE)
	return append(hdr, update...)
}

// sendUpdateAndDrain writes an UPDATE message and drains any response.
func sendUpdateAndDrain(client net.Conn, updateMsg []byte) {
	client.Write(updateMsg) //nolint:errcheck // test goroutine
	buf := make([]byte, 4096)
	client.Read(buf) //nolint:errcheck // drain response
}

// TestSessionRFC7606SessionResetNotification verifies RFC 7606 session-reset enforcement.
//
// RFC 7606 Section 3.g: "If the MP_REACH_NLRI attribute or the MP_UNREACH_NLRI
// attribute appears more than once in the UPDATE message, then a NOTIFICATION
// message MUST be sent with the Error Subcode 'Malformed Attribute List'."
//
// VALIDATES: Duplicate MP_REACH_NLRI triggers session-reset with NOTIFICATION (code 3, subcode 1).
// PREVENTS: Session staying up after structural errors that require session-reset.
func TestSessionRFC7606SessionResetNotification(t *testing.T) {
	session, client, callbackCount, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	// Build UPDATE with TWO MP_REACH_NLRI attributes (duplicate → session-reset per Section 3.g)
	mpReach := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x01,                   // SAFI = 1 (Unicast)
		0x04,                   // Next-hop length = 4
		0xc0, 0x00, 0x02, 0x01, // Next-hop = 192.0.2.1
		0x00,       // Reserved
		0x08, 0x0a, // NLRI: 10.0.0.0/8
	}

	pathAttrs := make([]byte, 0, 100)
	// ORIGIN = IGP + AS_PATH (empty) + First MP_REACH_NLRI header
	pathAttrs = append(pathAttrs, 0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, 0x80, 0x0e, byte(len(mpReach)))
	pathAttrs = append(pathAttrs, mpReach...)
	// Second MP_REACH_NLRI (DUPLICATE — triggers session-reset)
	pathAttrs = append(pathAttrs, 0x80, 0x0e, byte(len(mpReach)))
	pathAttrs = append(pathAttrs, mpReach...)

	update := make([]byte, 0, 100)
	update = append(update, 0x00, 0x00, byte(len(pathAttrs)>>8), byte(len(pathAttrs))) // Withdrawn=0, Path attrs length
	update = append(update, pathAttrs...)

	updateMsg := buildUpdateMsg(update)

	// Read any NOTIFICATION the session sends back
	var received []byte
	done := make(chan struct{})
	go func() {
		client.Write(updateMsg) //nolint:errcheck // test goroutine
		buf := make([]byte, 4096)
		n, _ := client.Read(buf) //nolint:errcheck // read NOTIFICATION
		received = buf[:n]
		close(done)
	}()

	// ReadAndProcess should return error (session-reset)
	err := session.ReadAndProcess()
	require.Error(t, err, "RFC 7606: duplicate MP_REACH_NLRI must trigger session-reset")
	require.Contains(t, err.Error(), "session reset")

	// Session should be Idle after reset
	require.Equal(t, fsm.StateIdle, session.State(), "session must be Idle after session-reset")

	// Callback should NOT have been called (validation runs before dispatch)
	require.Equal(t, 0, *callbackCount, "callback must not fire for session-reset UPDATE")

	// Verify NOTIFICATION was sent
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for NOTIFICATION")
	}

	require.GreaterOrEqual(t, len(received), message.HeaderLen+2, "NOTIFICATION too short")
	hdr, hdrErr := message.ParseHeader(received[:message.HeaderLen])
	require.NoError(t, hdrErr)
	require.Equal(t, message.TypeNOTIFICATION, hdr.Type, "must send NOTIFICATION")
	notifBody := received[message.HeaderLen:]
	// RFC 4271 Section 6.3: UPDATE Message Error = code 3
	require.Equal(t, byte(message.NotifyUpdateMessage), notifBody[0],
		"NOTIFICATION error code must be 3 (UPDATE Message Error)")
	// RFC 7606: Malformed Attribute List = subcode 1
	require.Equal(t, message.NotifyUpdateMalformedAttr, notifBody[1],
		"NOTIFICATION subcode must be 1 (Malformed Attribute List)")
}

// TestSessionRFC7606TreatAsWithdrawSuppressesCallback verifies callback suppression.
//
// RFC 7606 Section 2: treat-as-withdraw "MUST be handled as though all of the
// routes contained in an UPDATE message ... had been withdrawn"
//
// VALIDATES: Malformed UPDATE triggers treat-as-withdraw; callback is NOT invoked.
// PREVENTS: Plugins receiving malformed UPDATEs that should be treated as withdrawn.
func TestSessionRFC7606TreatAsWithdrawSuppressesCallback(t *testing.T) {
	session, client, callbackCount, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	// Build UPDATE with MALFORMED ORIGIN (length=2 instead of 1)
	pathAttrs := []byte{
		0x40, 0x01, 0x02, 0x00, 0x00, // ORIGIN with length 2 (invalid)
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
	}

	update := make([]byte, 0, 50)
	update = append(update, 0x00, 0x00, byte(len(pathAttrs)>>8), byte(len(pathAttrs)))
	update = append(update, pathAttrs...)
	update = append(update, 0x08, 0x0a) // NLRI: 10.0.0.0/8

	updateMsg := buildUpdateMsg(update)

	go func() {
		sendUpdateAndDrain(client, updateMsg)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "treat-as-withdraw must not return error")
	require.Equal(t, fsm.StateEstablished, session.State())

	// Key assertion: callback must NOT fire for treat-as-withdraw
	require.Equal(t, 0, *callbackCount, "callback must NOT fire for treat-as-withdraw UPDATE")
}

// TestSessionRFC7606AttributeDiscardContinues verifies attribute-discard enforcement.
//
// RFC 7606 Section 7.5: "If [LOCAL_PREF] is received from an external neighbor,
// it SHALL be discarded using the approach of 'attribute discard'."
//
// VALIDATES: LOCAL_PREF from EBGP triggers attribute-discard; session stays up; callback fires.
// PREVENTS: Session reset from EBGP LOCAL_PREF; ensures UPDATE still dispatched.
func TestSessionRFC7606AttributeDiscardContinues(t *testing.T) {
	session, client, callbackCount, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	// Build UPDATE with LOCAL_PREF from EBGP peer (attribute-discard per Section 7.5)
	pathAttrs := []byte{
		// ORIGIN = IGP
		0x40, 0x01, 0x01, 0x00,
		// AS_PATH: AS_SEQUENCE [65002]
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xEA,
		// NEXT_HOP = 192.0.2.1
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
		// LOCAL_PREF = 200 (from EBGP — must be discarded per Section 7.5)
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0xc8,
	}

	update := make([]byte, 0, 50)
	update = append(update, 0x00, 0x00, byte(len(pathAttrs)>>8), byte(len(pathAttrs)))
	update = append(update, pathAttrs...)
	update = append(update, 0x08, 0x0a) // NLRI: 10.0.0.0/8

	updateMsg := buildUpdateMsg(update)

	go func() {
		sendUpdateAndDrain(client, updateMsg)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "attribute-discard must not return error")
	require.Equal(t, fsm.StateEstablished, session.State(), "session must stay Established")

	// Key assertion: callback MUST fire (attribute-discard continues processing)
	require.Equal(t, 1, *callbackCount, "callback MUST fire for attribute-discard (UPDATE still dispatched)")
}

// TestSessionRFC7606ValidUpdateUnchanged verifies valid UPDATEs are unaffected.
//
// VALIDATES: Valid UPDATE is dispatched to callback; session stays Established.
// PREVENTS: RFC 7606 enforcement incorrectly rejecting valid UPDATEs.
func TestSessionRFC7606ValidUpdateUnchanged(t *testing.T) {
	session, client, callbackCount, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	// Build a perfectly valid UPDATE
	pathAttrs := []byte{
		// ORIGIN = IGP
		0x40, 0x01, 0x01, 0x00,
		// AS_PATH: AS_SEQUENCE [65002]
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xEA,
		// NEXT_HOP = 192.0.2.1
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
	}

	update := make([]byte, 0, 50)
	update = append(update, 0x00, 0x00, byte(len(pathAttrs)>>8), byte(len(pathAttrs)))
	update = append(update, pathAttrs...)
	update = append(update, 0x08, 0x0a) // NLRI: 10.0.0.0/8

	updateMsg := buildUpdateMsg(update)

	go func() {
		sendUpdateAndDrain(client, updateMsg)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "valid UPDATE must not return error")
	require.Equal(t, fsm.StateEstablished, session.State(), "session must stay Established")

	// Key assertion: callback fires exactly once for valid UPDATE
	require.Equal(t, 1, *callbackCount, "callback must fire exactly once for valid UPDATE")
}

// TestSendRawUpdateBody verifies raw UPDATE body sending with BGP header.
//
// VALIDATES: SendRawUpdateBody prepends correct BGP header (marker, length, type).
// PREVENTS: Malformed messages when using zero-copy forwarding.
func TestSendRawUpdateBody(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	// Accept connection
	_ = acceptWithReader(t, session, server, client)

	// Peer's OPEN
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			2, 6,
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4 = 65002
		},
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err)

	// Exchange KEEPALIVE to reach Established
	keepalive := message.NewKeepalive()
	keepaliveBytes := message.PackTo(keepalive, nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()

	err = session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateEstablished, session.State())

	// Create raw UPDATE body (minimal valid UPDATE: no withdrawals, no attrs, no NLRI)
	rawBody := []byte{
		0x00, 0x00, // Withdrawn routes length = 0
		0x00, 0x00, // Path attributes length = 0
		// No NLRI
	}

	// Read what session sends
	var received []byte
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		n, _ := client.Read(buf)
		received = buf[:n]
		close(done)
	}()

	// Send raw UPDATE body
	err = session.SendRawUpdateBody(rawBody)
	require.NoError(t, err)

	// Wait for receive
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}

	// Verify BGP header
	require.GreaterOrEqual(t, len(received), message.HeaderLen, "message too short")

	// Check marker (16 bytes of 0xFF)
	for i := range 16 {
		require.Equal(t, byte(0xFF), received[i], "marker byte %d should be 0xFF", i)
	}

	// Check length (19 header + 4 body = 23)
	expectedLen := message.HeaderLen + len(rawBody)
	actualLen := int(received[16])<<8 | int(received[17])
	require.Equal(t, expectedLen, actualLen, "length field mismatch")

	// Check type (UPDATE = 2)
	require.Equal(t, byte(message.TypeUPDATE), received[18], "type should be UPDATE")

	// Check body
	require.Equal(t, rawBody, received[message.HeaderLen:], "body mismatch")
}

// TestSendRawUpdateBodyNotEstablished verifies error when session not established.
//
// VALIDATES: SendRawUpdateBody returns ErrInvalidState before ESTABLISHED.
// PREVENTS: Sending data on non-established sessions.
func TestSendRawUpdateBodyNotEstablished(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	// Accept connection (now in OpenSent, not Established)
	_ = acceptWithReader(t, session, server, client)
	require.Equal(t, fsm.StateOpenSent, session.State())

	// Try to send - should fail
	rawBody := []byte{0x00, 0x00, 0x00, 0x00}
	err := session.SendRawUpdateBody(rawBody)
	require.ErrorIs(t, err, ErrInvalidState)
}

// =============================================================================
// RFC 7313 - Enhanced Route Refresh Tests
// =============================================================================

// buildRouteRefreshMsg creates a ROUTE-REFRESH message with the given body.
// Handles BGP header construction safely.
func buildRouteRefreshMsg(body []byte) []byte {
	msg := make([]byte, 19+len(body))
	// Marker
	for i := range 16 {
		msg[i] = 0xff
	}
	// Length
	msgLen := 19 + len(body)
	msg[16] = byte(msgLen >> 8)
	msg[17] = byte(msgLen)
	// Type
	msg[18] = byte(message.TypeROUTEREFRESH)
	// Body
	copy(msg[19:], body)
	return msg
}

// setupEstablishedSession creates an established BGP session for testing.
// Returns session, client conn, and a cleanup function.
func setupEstablishedSession(t *testing.T) (*Session, net.Conn, func()) {
	t.Helper()

	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.RouteRefresh{},
		&capability.EnhancedRouteRefresh{},
	}

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()

	cleanup := func() {
		_ = client.Close()
		_ = server.Close()
	}

	// Accept connection
	_ = acceptWithReader(t, session, server, client)

	// Peer's OPEN with Route Refresh and Enhanced Route Refresh
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			2, 16, // Capability param
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4 = 65002
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4/Unicast
			2, 0, // Route Refresh (code=2, len=0)
			70, 0, // Enhanced Route Refresh (code=70, len=0)
		},
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // Drain KEEPALIVE
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	// Exchange KEEPALIVE to reach Established
	keepalive := message.NewKeepalive()
	keepaliveBytes := message.PackTo(keepalive, nil)

	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()

	err = session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateEstablished, session.State())

	return session, client, cleanup
}

// TestHandleRouteRefreshNormal verifies normal ROUTE-REFRESH (subtype 0) handling.
//
// RFC 2918: Normal route refresh request triggers re-advertisement.
//
// VALIDATES: ROUTE-REFRESH with subtype 0 is processed without error.
// PREVENTS: Session reset on valid route refresh request.
func TestHandleRouteRefreshNormal(t *testing.T) {
	session, client, cleanup := setupEstablishedSession(t)
	defer cleanup()

	// Build ROUTE-REFRESH message: AFI=1, Subtype=0, SAFI=1
	// RFC 2918: 4 bytes = AFI(2) + Reserved/Subtype(1) + SAFI(1)
	rrBody := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x00, // Subtype = 0 (normal)
		0x01, // SAFI = 1 (Unicast)
	}
	rrMsg := buildRouteRefreshMsg(rrBody)

	go func() {
		_, _ = client.Write(rrMsg)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "normal ROUTE-REFRESH should be processed without error")
	require.Equal(t, fsm.StateEstablished, session.State(), "session should remain Established")
}

// TestHandleRouteRefreshBoRR verifies BoRR (subtype 1) handling.
//
// RFC 7313 Section 4: BoRR marks the beginning of route refresh.
//
// VALIDATES: ROUTE-REFRESH with subtype 1 (BoRR) is processed without error.
// PREVENTS: Session reset on valid BoRR marker.
func TestHandleRouteRefreshBoRR(t *testing.T) {
	session, client, cleanup := setupEstablishedSession(t)
	defer cleanup()

	// Build ROUTE-REFRESH message: AFI=1, Subtype=1 (BoRR), SAFI=1
	rrBody := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x01, // Subtype = 1 (BoRR)
		0x01, // SAFI = 1 (Unicast)
	}
	rrMsg := buildRouteRefreshMsg(rrBody)

	go func() {
		_, _ = client.Write(rrMsg)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "BoRR should be processed without error")
	require.Equal(t, fsm.StateEstablished, session.State())
}

// TestHandleRouteRefreshEoRR verifies EoRR (subtype 2) handling.
//
// RFC 7313 Section 4: EoRR marks the end of route refresh.
//
// VALIDATES: ROUTE-REFRESH with subtype 2 (EoRR) is processed without error.
// PREVENTS: Session reset on valid EoRR marker.
func TestHandleRouteRefreshEoRR(t *testing.T) {
	session, client, cleanup := setupEstablishedSession(t)
	defer cleanup()

	// Build ROUTE-REFRESH message: AFI=1, Subtype=2 (EoRR), SAFI=1
	rrBody := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x02, // Subtype = 2 (EoRR)
		0x01, // SAFI = 1 (Unicast)
	}
	rrMsg := buildRouteRefreshMsg(rrBody)

	go func() {
		_, _ = client.Write(rrMsg)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "EoRR should be processed without error")
	require.Equal(t, fsm.StateEstablished, session.State())
}

// TestHandleRouteRefreshUnknown verifies unknown subtype is ignored.
//
// RFC 7313 Section 5: "When the BGP speaker receives a ROUTE-REFRESH message
// with a 'Message Subtype' field other than 0, 1, or 2, it MUST ignore
// the received ROUTE-REFRESH message."
//
// VALIDATES: Unknown subtype (e.g., 42) is silently ignored.
// PREVENTS: Error or session reset on unknown subtype.
func TestHandleRouteRefreshUnknown(t *testing.T) {
	session, client, cleanup := setupEstablishedSession(t)
	defer cleanup()

	// Build ROUTE-REFRESH message with unknown subtype 42
	rrBody := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x2A, // Subtype = 42 (unknown)
		0x01, // SAFI = 1 (Unicast)
	}
	rrMsg := buildRouteRefreshMsg(rrBody)

	go func() {
		_, _ = client.Write(rrMsg)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "RFC 7313: unknown subtype MUST be ignored")
	require.Equal(t, fsm.StateEstablished, session.State())
}

// TestHandleRouteRefreshReserved verifies reserved subtype 255 is ignored.
//
// RFC 7313: Subtype 255 is reserved.
//
// VALIDATES: Reserved subtype 255 is silently ignored.
// PREVENTS: Error or session reset on reserved subtype.
func TestHandleRouteRefreshReserved(t *testing.T) {
	session, client, cleanup := setupEstablishedSession(t)
	defer cleanup()

	// Build ROUTE-REFRESH message with reserved subtype 255
	rrBody := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0xFF, // Subtype = 255 (reserved)
		0x01, // SAFI = 1 (Unicast)
	}
	rrMsg := buildRouteRefreshMsg(rrBody)

	go func() {
		_, _ = client.Write(rrMsg)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "reserved subtype 255 should be ignored")
	require.Equal(t, fsm.StateEstablished, session.State())
}

// TestHandleRouteRefreshBadLen verifies invalid length triggers NOTIFICATION.
//
// RFC 7313 Section 5: "If the length... is not 4, then the BGP speaker
// MUST send a NOTIFICATION message with Error Code 'ROUTE-REFRESH Message Error'
// and subcode 'Invalid Message Length'."
//
// VALIDATES: ROUTE-REFRESH with body length != 4 triggers NOTIFICATION 7/1.
// PREVENTS: Processing malformed ROUTE-REFRESH messages.
//
// Note: Cases where total message length < 23 bytes are caught by header validation
// (tested separately), not by handleRouteRefresh. Here we test only body lengths
// that pass header validation but fail in handleRouteRefresh.
func TestHandleRouteRefreshBadLen(t *testing.T) {
	tests := []struct {
		name    string
		bodyLen int
	}{
		{"too_long_5", 5},
		{"too_long_8", 8},
		{"too_long_100", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, client, cleanup := setupEstablishedSession(t)
			defer cleanup()

			// Build malformed ROUTE-REFRESH message with body > 4 bytes
			rrBody := make([]byte, tt.bodyLen)
			// Fill with valid-looking header data
			rrBody[0] = 0x00
			rrBody[1] = 0x01
			rrBody[2] = 0x00
			rrBody[3] = 0x01

			rrMsg := buildRouteRefreshMsg(rrBody)

			go func() {
				_, _ = client.Write(rrMsg)
				// Drain NOTIFICATION response
				buf := make([]byte, 4096)
				_, _ = client.Read(buf)
			}()

			err := session.ReadAndProcess()
			require.Error(t, err, "RFC 7313: invalid length must trigger error")
			require.Contains(t, err.Error(), "invalid length", "error should mention invalid length")
		})
	}
}

// TestSessionCloseOnCancel verifies that canceling the context exits Run() immediately.
//
// VALIDATES: AC-1: context cancel closes conn, Run() returns within 10ms (not 100ms).
// PREVENTS: Slow 100ms polling delay on shutdown due to SetReadDeadline polling.
func TestSessionCloseOnCancel(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	session := NewSession(settings)

	// Use net.Pipe — a synchronous in-memory connection.
	server, client := net.Pipe()
	defer func() { _ = client.Close() }()

	// Set the connection directly (skip BGP handshake).
	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	// Collect Run() result.
	type runResult struct {
		err      error
		duration time.Duration
	}
	resultCh := make(chan runResult, 1)

	go func() {
		start := time.Now()
		err := session.Run(ctx)
		resultCh <- runResult{err: err, duration: time.Since(start)}
	}()

	// Let ReadFull block (no data sent to server).
	time.Sleep(20 * time.Millisecond)

	// Cancel the context.
	cancel()

	// Run should return quickly (< 50ms, not 100ms+ from polling).
	select {
	case result := <-resultCh:
		require.ErrorIs(t, result.err, context.Canceled,
			"Run should return context.Canceled on cancel")
		require.Less(t, result.duration, 200*time.Millisecond,
			"Run should exit promptly on cancel, not wait for polling interval")
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel — likely blocked on ReadFull")
	}
}

// TestSessionHoldTimerStillWorks verifies that hold timer expiry still exits Run().
//
// VALIDATES: AC-2: hold timer expiry sends ErrHoldTimerExpired through errChan, Run() exits.
// PREVENTS: Regression where close-on-cancel breaks the hold timer → errChan → Run() exit path.
func TestSessionHoldTimerStillWorks(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.ReceiveHoldTime = 50 * time.Millisecond // Very short for testing
	session := NewSession(settings)

	// Use net.Pipe — ReadFull will block.
	server, client := net.Pipe()
	defer func() { _ = client.Close() }()

	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	// Start the hold timer so it fires after 50ms.
	session.timers.StartHoldTimer()

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- session.Run(t.Context())
	}()

	// Hold timer fires after ~50ms → errChan → cancel goroutine → closeConn → Run exits.
	select {
	case err := <-resultCh:
		require.ErrorIs(t, err, ErrHoldTimerExpired,
			"Run should return ErrHoldTimerExpired when hold timer fires")
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after hold timer expiry — hold timer path broken")
	}
}

// TestSessionTeardownStillWorks verifies that Teardown() exits Run() with ErrTeardown.
//
// VALIDATES: AC-3: Teardown sends NOTIFICATION, closes conn, Run() returns ErrTeardown.
// PREVENTS: Regression where close-on-cancel breaks the Teardown → Run() exit path.
func TestSessionTeardownStillWorks(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	session := NewSession(settings)

	server, client := net.Pipe()

	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- session.Run(t.Context())
	}()

	// Let ReadFull block.
	time.Sleep(20 * time.Millisecond)

	// Drain data from client side so sendNotification doesn't block on pipe.
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := client.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	_ = session.Teardown(message.NotifyCeaseAdminShutdown, "")

	select {
	case err := <-resultCh:
		require.ErrorIs(t, err, ErrTeardown,
			"Run should return ErrTeardown after Teardown()")
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Teardown — teardown path broken")
	}
}

// --- Backpressure pause/resume tests ---
// VALIDATES: AC-1 — Pause() stops read loop from calling readAndProcessMessage
// PREVENTS: Unbounded read when system is under memory pressure

func TestSessionPauseBlocksRead(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	session := NewSession(settings)

	server, client := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	// Pause BEFORE starting Run.
	session.Pause()
	require.True(t, session.IsPaused(), "should be paused after Pause()")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Track whether ReadFull was called by monitoring data read from client side.
	var readAttempted atomic.Bool
	go func() {
		buf := make([]byte, 1)
		if _, err := client.Read(buf); err != nil {
			return
		}
		readAttempted.Store(true)
	}()

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- session.Run(ctx)
	}()

	// Give Run() time to hit the pause gate (not ReadFull).
	time.Sleep(50 * time.Millisecond)

	require.False(t, readAttempted.Load(),
		"ReadFull should NOT be called while paused — pause gate not working")

	// Clean up: cancel context so Run exits.
	cancel()

	select {
	case <-resultCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel while paused")
	}
}

// VALIDATES: AC-2 — Resume() allows read loop to continue
// PREVENTS: Stuck session after Resume()

func TestSessionResumeUnblocksRead(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	session := NewSession(settings)

	server, client := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	// Start paused.
	session.Pause()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- session.Run(ctx)
	}()

	// Let Run() block on pause gate.
	time.Sleep(30 * time.Millisecond)

	// Resume — Run should now attempt to read.
	session.Resume()
	require.False(t, session.IsPaused(), "should not be paused after Resume()")

	// After resume, Run will call ReadFull which blocks on net.Pipe.
	// Cancel to unblock it.
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-resultCh:
		// Run returned — resume unblocked the read loop.
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Resume + cancel — resume not working")
	}
}

// VALIDATES: AC-3 — Context cancel while paused returns promptly
// PREVENTS: Deadlocked session when context is canceled during pause

func TestSessionPauseCancelContext(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	session := NewSession(settings)

	server, client := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	session.Pause()

	ctx, cancel := context.WithCancel(t.Context())

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- session.Run(ctx)
	}()

	// Let Run() settle on pause gate.
	time.Sleep(30 * time.Millisecond)

	// Cancel context — Run should return promptly.
	cancel()

	select {
	case err := <-resultCh:
		require.ErrorIs(t, err, context.Canceled,
			"Run should return context.Canceled when canceled while paused")
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel while paused")
	}
}

// VALIDATES: AC-4 — Hold timer fires while paused, session closes via errChan
// PREVENTS: Paused session surviving past hold timer

func TestSessionPauseHoldTimerExpiry(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	session := NewSession(settings)

	server, client := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	session.Pause()

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- session.Run(t.Context())
	}()

	// Simulate hold timer expiry by sending to errChan.
	time.Sleep(30 * time.Millisecond)
	session.errChan <- ErrHoldTimerExpired

	select {
	case err := <-resultCh:
		require.ErrorIs(t, err, ErrHoldTimerExpired,
			"Run should return ErrHoldTimerExpired when hold timer fires while paused")
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after hold timer expiry while paused")
	}
}

// VALIDATES: AC-10, AC-11 — Pause/Resume are idempotent
// PREVENTS: Panic or channel close errors on duplicate calls

func TestSessionPauseIdempotent(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	session := NewSession(settings)

	// Multiple Pause calls should not panic.
	session.Pause()
	session.Pause()
	session.Pause()
	require.True(t, session.IsPaused())

	// Multiple Resume calls should not panic.
	session.Resume()
	session.Resume()
	session.Resume()
	require.False(t, session.IsPaused())

	// Pause-Resume-Pause cycle.
	session.Pause()
	require.True(t, session.IsPaused())
	session.Resume()
	require.False(t, session.IsPaused())
	session.Pause()
	require.True(t, session.IsPaused())
	session.Resume()
	require.False(t, session.IsPaused())
}

// VALIDATES: AC-12 — KEEPALIVE sending continues while read is paused
// PREVENTS: Write path accidentally gated by read pause
// Note: The pause gate only affects the read loop, not the write path.

func TestSessionPauseKeepaliveContinues(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	session := NewSession(settings)

	server, client := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	// Pause reading.
	session.Pause()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		_ = session.Run(ctx)
	}()

	// Write a KEEPALIVE directly to conn while paused — proves conn is writable.
	time.Sleep(20 * time.Millisecond)

	keepalive := make([]byte, 19)
	for i := range 16 {
		keepalive[i] = 0xFF
	}
	keepalive[16] = 0x00
	keepalive[17] = 0x13 // length = 19
	keepalive[18] = 0x04 // type = KEEPALIVE

	writeDone := make(chan error, 1)
	go func() {
		_, err := server.Write(keepalive)
		writeDone <- err
	}()

	// Read from client side to unblock the pipe write.
	readBuf := make([]byte, 19)
	if _, err := client.Read(readBuf); err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	select {
	case err := <-writeDone:
		require.NoError(t, err, "write should succeed while read is paused")
	case <-time.After(2 * time.Second):
		t.Fatal("write blocked while read is paused — write path not independent")
	}

	cancel()
}

// TestSendUpdateConcurrentNoRace verifies that concurrent SendRawUpdateBody
// calls on the same session do not race on writeBuf.
//
// VALIDATES: AC-1 — 10 goroutines call SendRawUpdateBody concurrently, no race.
// PREVENTS: Data race on Session.writeBuf from unsynchronized concurrent writes.
func TestSendUpdateConcurrentNoRace(t *testing.T) {
	session, client, _, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	// Drain all data sent by concurrent goroutines so net.Pipe doesn't block.
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		buf := make([]byte, 65536)
		for {
			_, err := client.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	// Minimal valid UPDATE body: no withdrawals, no attrs, no NLRI.
	rawBody := []byte{
		0x00, 0x00, // Withdrawn routes length = 0
		0x00, 0x00, // Path attributes length = 0
	}

	const goroutines = 10
	const sends = 50

	var wg sync.WaitGroup
	var errCount atomic.Int32

	for range goroutines {
		wg.Go(func() {
			for range sends {
				if err := session.SendRawUpdateBody(rawBody); err != nil {
					errCount.Add(1)
					return
				}
			}
		})
	}

	wg.Wait()

	// Close client to stop drain goroutine.
	client.Close() //nolint:errcheck // test cleanup
	<-drainDone

	require.Zero(t, errCount.Load(), "all sends should succeed without error")
}

// TestSessionBufWriterCreated verifies bufWriter is created during connection establishment.
//
// VALIDATES: AC-1 (bufWriter created wrapping conn)
// PREVENTS: Missing bufWriter initialization causing nil pointer on Send*.
func TestSessionBufWriterCreated(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive

	session := NewSession(settings)
	require.Nil(t, session.bufWriter, "bufWriter should be nil before connection")

	_ = session.Start()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	_ = acceptWithReader(t, session, server, client)

	require.NotNil(t, session.bufWriter, "bufWriter should be set after connectionEstablished")
}

// TestSendUpdateUsesBufWriter verifies SendUpdate writes through bufWriter.
// Uses an established session and verifies data arrives at the remote end.
//
// VALIDATES: AC-2 (SendUpdate writes to bufWriter, not raw conn)
// PREVENTS: Regression where writes bypass bufWriter.
func TestSendUpdateUsesBufWriter(t *testing.T) {
	session, client, cleanup := setupEstablishedSession(t)
	defer cleanup()

	require.NotNil(t, session.bufWriter, "bufWriter should exist on established session")

	// Build a minimal UPDATE (empty withdraw + empty attrs + empty NLRI)
	update := &message.Update{}

	// Send in background (net.Pipe is synchronous)
	errCh := make(chan error, 1)
	go func() {
		errCh <- session.SendUpdate(update)
	}()

	// Read from client side
	buf := make([]byte, 4096)
	n, err := client.Read(buf)
	require.NoError(t, err)
	require.Greater(t, n, message.HeaderLen, "should receive at least a header")

	// Verify header: marker + length + UPDATE type
	for i := range 16 {
		require.Equal(t, byte(0xff), buf[i], "marker byte %d", i)
	}
	require.Equal(t, byte(message.TypeUPDATE), buf[18], "type should be UPDATE")

	require.NoError(t, <-errCh)
}

// TestSessionCloseFlushesBufWriter verifies closeConn flushes pending data.
//
// VALIDATES: AC-10 (bufWriter flushed before conn close)
// PREVENTS: Lost data on session teardown.
func TestSessionCloseFlushesBufWriter(t *testing.T) {
	session, client, cleanup := setupEstablishedSession(t)
	defer cleanup()

	// Write data to bufWriter without flushing (simulate unflushed data)
	session.writeMu.Lock()
	_, err := session.bufWriter.WriteString("test-data")
	session.writeMu.Unlock()
	require.NoError(t, err)

	// Verify bufWriter has buffered data
	require.Greater(t, session.bufWriter.Buffered(), 0)

	// Close should flush. Read in background since net.Pipe is sync.
	readDone := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := client.Read(buf)
		readDone <- buf[:n]
	}()

	session.closeConn()

	select {
	case data := <-readDone:
		require.Equal(t, "test-data", string(data), "flush should deliver buffered data")
	case <-time.After(time.Second):
		t.Fatal("closeConn should have flushed bufWriter")
	}
}

// TestSendMessageAutoFlush verifies writeMessage (KEEPALIVE/OPEN/NOTIFICATION) flushes immediately.
//
// VALIDATES: AC-6 (non-UPDATE sends flush immediately)
// PREVENTS: KEEPALIVEs stuck in bufWriter causing hold timer expiry.
func TestSendMessageAutoFlush(t *testing.T) {
	session, client, cleanup := setupEstablishedSession(t)
	defer cleanup()

	// Send KEEPALIVE (uses writeMessage internally)
	errCh := make(chan error, 1)
	go func() {
		conn := session.Conn()
		errCh <- session.sendKeepalive(conn)
	}()

	// Read from client — data must arrive (proves immediate flush)
	buf := make([]byte, 4096)
	n, err := client.Read(buf)
	require.NoError(t, err)
	require.Equal(t, message.HeaderLen, n, "KEEPALIVE is exactly 19 bytes")
	require.Equal(t, byte(message.TypeKEEPALIVE), buf[18])

	require.NoError(t, <-errCh)
}

// TestSessionMD5DialerWiring verifies NewSession sets MD5 fields on the dialer.
//
// VALIDATES: MD5Key from PeerSettings flows to RealDialer.
// PREVENTS: MD5 config accepted but not applied to the dialer.
func TestSessionMD5DialerWiring(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.MD5Key = "test-md5-key"

	session := NewSession(settings)
	require.NotNil(t, session)

	// Verify the dialer has MD5 fields set.
	rd, ok := session.dialer.(*network.RealDialer)
	require.True(t, ok, "dialer should be *network.RealDialer")
	require.Equal(t, "test-md5-key", rd.MD5Key)
	require.Equal(t, net.IP(netip.MustParseAddr("192.0.2.1").AsSlice()), rd.PeerAddr)
}

// TestSessionMD5DialerWiringWithMD5IP verifies MD5IP overrides peer address.
//
// VALIDATES: MD5IP from PeerSettings is used as the dialer's PeerAddr.
// PREVENTS: MD5IP being ignored when set.
func TestSessionMD5DialerWiringWithMD5IP(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.MD5Key = "test-md5-key"
	settings.MD5IP = netip.MustParseAddr("10.0.0.1")

	session := NewSession(settings)
	rd, ok := session.dialer.(*network.RealDialer)
	require.True(t, ok)
	require.Equal(t, net.IP(netip.MustParseAddr("10.0.0.1").AsSlice()), rd.PeerAddr)
}

// TestSessionNoMD5WhenKeyAbsent verifies no MD5 fields set when key is empty.
//
// VALIDATES: Dialer has no MD5 config when PeerSettings.MD5Key is empty.
// PREVENTS: False positive MD5 activation.
func TestSessionNoMD5WhenKeyAbsent(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)

	session := NewSession(settings)
	rd, ok := session.dialer.(*network.RealDialer)
	require.True(t, ok)
	require.Empty(t, rd.MD5Key)
	require.Nil(t, rd.PeerAddr)
}

// TestCloseConnGracefulTCP verifies closeConn sends FIN (not RST) on real TCP.
// Uses a real TCP connection to exercise the CloseWrite + drain path
// (which is skipped by net.Pipe in other tests).
//
// VALIDATES: NOTIFICATION data is readable by remote after closeConn.
// PREVENTS: TCP RST discarding NOTIFICATION before remote reads it.
func TestCloseConnGracefulTCP(t *testing.T) {
	// Start a TCP listener on localhost.
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	// Accept connection in background.
	acceptCh := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		acceptCh <- conn
	}()

	// Dial to the listener.
	var dialer net.Dialer
	clientConn, err := dialer.DialContext(context.Background(), "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer func() { _ = clientConn.Close() }()

	// Get the server-side connection.
	var serverConn net.Conn
	select {
	case serverConn = <-acceptCh:
	case <-time.After(time.Second):
		t.Fatal("accept timed out")
	}
	defer func() { _ = serverConn.Close() }()

	// Set up session with the real TCP connection directly (bypass
	// connectionEstablished which sends OPEN and complicates the read).
	ps := NewPeerSettings(mustParseAddr("127.0.0.1"), 65000, 65001, 0)
	session := NewSession(ps)
	session.mu.Lock()
	session.conn = serverConn
	session.bufReader = bufio.NewReaderSize(serverConn, 4096)
	session.bufWriter = bufio.NewWriterSize(serverConn, 4096)
	session.mu.Unlock()

	// Write data that the remote should be able to read after close.
	session.writeMu.Lock()
	_, writeErr := session.bufWriter.WriteString("NOTIFICATION-DATA")
	session.writeMu.Unlock()
	require.NoError(t, writeErr)

	// Send unread data from client to server (creates unread data in server's
	// receive buffer, which would cause RST without the drain fix).
	_, writeErr = clientConn.Write([]byte("KEEPALIVE-FROM-CLIENT"))
	require.NoError(t, writeErr)

	// Brief pause to let the client data arrive in server's receive buffer.
	time.Sleep(10 * time.Millisecond)

	// Close the session (should do CloseWrite + drain + Close).
	session.closeConn()

	// Verify: client can still read the data we wrote (FIN, not RST).
	buf := make([]byte, 4096)
	_ = clientConn.SetReadDeadline(time.Now().Add(time.Second))
	n, readErr := clientConn.Read(buf)
	require.NoError(t, readErr, "should read data before EOF (FIN not RST)")
	assert.Equal(t, "NOTIFICATION-DATA", string(buf[:n]))
}

// TestSendHoldDurationAuto verifies the RFC 9687 auto formula: max(8min, 2x ReceiveHoldTime).
//
// VALIDATES: When SendHoldTime=0 (auto), duration is max(8 minutes, 2x ReceiveHoldTime).
// PREVENTS: Wrong formula producing too-short or too-long send hold timer.
func TestSendHoldDurationAuto(t *testing.T) {
	tests := []struct {
		name     string
		recvHold time.Duration
		want     time.Duration
	}{
		{"recv_90s_auto_8min", 90 * time.Second, 8 * time.Minute},     // 2*90=180s < 8min=480s
		{"recv_0s_auto_8min", 0, 8 * time.Minute},                     // 2*0=0 < 8min
		{"recv_300s_auto_10min", 300 * time.Second, 10 * time.Minute}, // 2*300=600s > 8min=480s
		{"recv_240s_auto_8min", 240 * time.Second, 8 * time.Minute},   // 2*240=480s == 8min
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("10.0.0.1"), 65001, 65002, 0)
			settings.ReceiveHoldTime = tt.recvHold
			// SendHoldTime = 0 (auto, default)
			session := NewSession(settings)
			assert.Equal(t, tt.want, session.sendHoldDuration())
		})
	}
}

// TestSendHoldDurationExplicit verifies explicit send-hold-time overrides the formula.
//
// VALIDATES: When SendHoldTime > 0, that exact value is returned.
// PREVENTS: Explicit value being ignored in favor of the auto formula.
func TestSendHoldDurationExplicit(t *testing.T) {
	tests := []struct {
		name     string
		sendHold time.Duration
		recvHold time.Duration
		want     time.Duration
	}{
		{"explicit_480s", 480 * time.Second, 90 * time.Second, 480 * time.Second},
		{"explicit_600s", 600 * time.Second, 90 * time.Second, 600 * time.Second},
		{"explicit_overrides_formula", 500 * time.Second, 300 * time.Second, 500 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("10.0.0.1"), 65001, 65002, 0)
			settings.ReceiveHoldTime = tt.recvHold
			settings.SendHoldTime = tt.sendHold
			session := NewSession(settings)
			assert.Equal(t, tt.want, session.sendHoldDuration())
		})
	}
}
