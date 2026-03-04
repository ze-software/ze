package reactor

import (
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
)

// VALIDATES: Session handler error paths (OPEN version/hold/malformed/caps, unknown type, ROUTE-REFRESH).
// PREVENTS: Silent acceptance of invalid messages, missing NOTIFICATION on protocol errors.

// newOpenSentSession creates a Session in OpenSent state with a connected net.Pipe.
// A drain goroutine reads data from the client side so handler writes don't block.
func newOpenSentSession(t *testing.T) *Session {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.HoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	err := session.Start()
	require.NoError(t, err)

	client, server := net.Pipe()

	_ = acceptWithReader(t, session, server, client)
	require.Equal(t, fsm.StateOpenSent, session.State())

	// Drain goroutine absorbs any NOTIFICATION/KEEPALIVE the handler sends.
	go func() {
		buf := make([]byte, 65536)
		for {
			if _, readErr := client.Read(buf); readErr != nil {
				return
			}
		}
	}()

	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})

	return session
}

// validOpenBody returns a minimal valid OPEN body (version 4, AS 65002, hold 90, ID 1.2.3.2, no opts).
func validOpenBody() []byte {
	return []byte{
		0x04,       // Version 4
		0xFD, 0xEA, // MyAS = 65002
		0x00, 0x5A, // HoldTime = 90
		0x01, 0x02, 0x03, 0x02, // BGP Identifier = 1.2.3.2
		0x00, // OptParamLen = 0
	}
}

// TestHandleOpen_InvalidVersion verifies OPEN with BGP version != 4.
// RFC 4271 Section 6.2: unsupported version sends NOTIFICATION.
func TestHandleOpen_InvalidVersion(t *testing.T) {
	s := newOpenSentSession(t)

	body := validOpenBody()
	body[0] = 0x03 // Version 3

	err := s.handleOpen(body)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsupportedVersion)
}

// TestHandleOpen_InvalidHoldTime verifies OPEN with hold time 1 or 2.
// RFC 4271 Section 6.2: hold time must be 0 or >= 3.
func TestHandleOpen_InvalidHoldTime(t *testing.T) {
	tests := []struct {
		name     string
		holdTime uint16
	}{
		{"hold_time_1", 1},
		{"hold_time_2", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newOpenSentSession(t)

			body := validOpenBody()
			body[3] = byte(tt.holdTime >> 8)
			body[4] = byte(tt.holdTime)

			err := s.handleOpen(body)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid hold time")
		})
	}
}

// TestHandleOpen_Malformed verifies OPEN with body too short to parse.
func TestHandleOpen_Malformed(t *testing.T) {
	s := newOpenSentSession(t)

	body := []byte{0x04, 0xFD} // Only 2 bytes, need at least 10

	err := s.handleOpen(body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unpack OPEN")
}

// TestHandleOpen_RequiredFamilyMissing verifies rejection when peer lacks required families.
// RFC 5492 Section 3: Unsupported Capability notification.
func TestHandleOpen_RequiredFamilyMissing(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.HoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}
	// Require IPv6 unicast — peer won't have it.
	settings.RequiredFamilies = []capability.Family{
		{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	_ = acceptWithReader(t, session, server, client)

	go func() {
		buf := make([]byte, 65536)
		for {
			if _, readErr := client.Read(buf); readErr != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})

	// Peer OPEN with only IPv4 unicast + ASN4 — missing required IPv6.
	body := []byte{
		0x04,       // Version 4
		0xFD, 0xEA, // MyAS = 65002
		0x00, 0x5A, // HoldTime = 90
		0x01, 0x02, 0x03, 0x02, // BGP ID
		0x10, // OptParamLen = 16
		// Capability param: ASN4 (code=65, len=4)
		0x02, 0x06, 0x41, 0x04, 0x00, 0x00, 0xFD, 0xEA,
		// Capability param: Multiprotocol IPv4/Unicast (code=1, len=4)
		0x02, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01,
	}

	err := session.handleOpen(body)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidState)
	assert.Contains(t, err.Error(), "required families not negotiated")
}

// TestHandleOpen_RequiredCapMissing verifies rejection when peer lacks required capability codes.
func TestHandleOpen_RequiredCapMissing(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.HoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
	}
	// Require Extended Message — peer won't have it.
	settings.RequiredCapabilities = []capability.Code{capability.CodeExtendedMessage}

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	_ = acceptWithReader(t, session, server, client)

	go func() {
		buf := make([]byte, 65536)
		for {
			if _, readErr := client.Read(buf); readErr != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})

	// Peer OPEN with only ASN4 — no Extended Message.
	body := []byte{
		0x04, 0xFD, 0xEA, 0x00, 0x5A, 0x01, 0x02, 0x03, 0x02,
		0x08,                                           // OptParamLen = 8
		0x02, 0x06, 0x41, 0x04, 0x00, 0x00, 0xFD, 0xEA, // ASN4
	}

	err := session.handleOpen(body)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidState)
	assert.Contains(t, err.Error(), "required capabilities not negotiated")
}

// TestHandleOpen_RefusedCapPresent verifies rejection when peer has a refused capability.
func TestHandleOpen_RefusedCapPresent(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.HoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.RouteRefresh{},
	}
	// Refuse Route Refresh.
	settings.RefusedCapabilities = []capability.Code{capability.CodeRouteRefresh}

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	_ = acceptWithReader(t, session, server, client)

	go func() {
		buf := make([]byte, 65536)
		for {
			if _, readErr := client.Read(buf); readErr != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})

	// Peer OPEN with ASN4 + Route Refresh.
	body := []byte{
		0x04, 0xFD, 0xEA, 0x00, 0x5A, 0x01, 0x02, 0x03, 0x02,
		0x0C,                                           // OptParamLen = 12 (ASN4=8 + RouteRefresh=4)
		0x02, 0x06, 0x41, 0x04, 0x00, 0x00, 0xFD, 0xEA, // ASN4
		0x02, 0x02, 0x02, 0x00, // Route Refresh (code=2, len=0)
	}

	err := session.handleOpen(body)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidState)
	assert.Contains(t, err.Error(), "refused capabilities present")
}

// TestHandleOpen_ValidatorRejects verifies rejection when openValidator returns error.
func TestHandleOpen_ValidatorRejects(t *testing.T) {
	s := newOpenSentSession(t)

	// Set a validator that always rejects.
	s.openValidator = func(peerAddr string, local, remote *message.Open) error {
		return errors.New("role mismatch")
	}

	err := s.handleOpen(validOpenBody())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open validation failed")
	assert.Contains(t, err.Error(), "role mismatch")
}

// TestHandleUnknownType verifies unknown message type sends NOTIFICATION and closes.
func TestHandleUnknownType(t *testing.T) {
	s := newOpenSentSession(t)

	err := s.handleUnknownType(message.MessageType(99))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidMessage)
	assert.Contains(t, err.Error(), "unknown type 99")
}

// TestHandleNotification_Malformed verifies too-short NOTIFICATION body.
func TestHandleNotification_Malformed(t *testing.T) {
	s := newOpenSentSession(t)

	body := []byte{0x06} // Only 1 byte, need at least 2

	err := s.handleNotification(body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unpack NOTIFICATION")
}

// TestHandleRouteRefresh_InvalidLength verifies ROUTE-REFRESH with wrong body length.
// RFC 7313 Section 5: body must be exactly 4 bytes.
func TestHandleRouteRefresh_InvalidLength(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{"too_short", []byte{0x00, 0x01, 0x00}},
		{"too_long", []byte{0x00, 0x01, 0x00, 0x01, 0xFF}},
		{"empty", []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newOpenSentSession(t)

			err := s.handleRouteRefresh(tt.body)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidMessage)
			assert.Contains(t, err.Error(), "ROUTE-REFRESH invalid length")
		})
	}
}

// TestHandleRouteRefresh_UnknownSubtype verifies subtypes > 2 are silently ignored.
// RFC 7313 Section 5: unknown subtypes MUST be ignored.
func TestHandleRouteRefresh_UnknownSubtype(t *testing.T) {
	s := newOpenSentSession(t)

	tests := []struct {
		name    string
		subtype byte
	}{
		{"subtype_3", 3},
		{"subtype_100", 100},
		{"subtype_255_reserved", 255},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// AFI=1 (IPv4), Subtype, SAFI=1 (Unicast)
			body := []byte{0x00, 0x01, tt.subtype, 0x01}

			err := s.handleRouteRefresh(body)
			assert.NoError(t, err)
		})
	}
}

// TestHandleUpdate_FamilyMismatchIgnoreMode verifies IgnoreFamilyMismatch mode.
// RFC 4760 Section 6: lenient mode logs but doesn't reject.
func TestHandleUpdate_FamilyMismatchIgnoreMode(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.HoldTime = 90 * time.Second
	settings.IgnoreFamilyMismatch = true
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	_ = acceptWithReader(t, session, server, client)

	// Exchange OPEN to get negotiated state.
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			0x02, 0x06, 0x41, 0x04, 0x00, 0x00, 0xFD, 0xEA, // ASN4
			0x02, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01, // IPv4/Unicast
		},
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		if _, writeErr := client.Write(openBytes); writeErr != nil {
			return
		}
		buf := make([]byte, 65536)
		for {
			if _, readErr := client.Read(buf); readErr != nil {
				return
			}
		}
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err)

	// Exchange KEEPALIVE to reach Established.
	keepalive := message.NewKeepalive()
	go func() {
		if _, writeErr := client.Write(message.PackTo(keepalive, nil)); writeErr != nil {
			return
		}
	}()
	err = session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateEstablished, session.State())

	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})

	// Build UPDATE with MP_REACH_NLRI for IPv6 (NOT negotiated).
	mpReach := []byte{
		0x00, 0x02, // AFI = 2 (IPv6)
		0x01, // SAFI = 1 (Unicast)
		0x10, // NH len = 16
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // NH = ::1
		0x00,                         // Reserved
		0x20, 0x20, 0x01, 0x0D, 0xB8, // 2001:db8::/32
	}

	attrFlags := byte(0x90) // Optional, Transitive, Extended Length
	attrCode := byte(0x0E)  // MP_REACH_NLRI
	attrLen := len(mpReach)
	pathAttrs := append([]byte{attrFlags, attrCode, byte(attrLen >> 8), byte(attrLen)}, mpReach...)

	// UPDATE body: withdrawn len (0) + attrs len + attrs
	updateBody := make([]byte, 4+len(pathAttrs))
	updateBody[2] = byte(len(pathAttrs) >> 8)
	updateBody[3] = byte(len(pathAttrs))
	copy(updateBody[4:], pathAttrs)

	wu := wireu.NewWireUpdate(updateBody, 0)
	err = session.handleUpdate(wu)
	assert.NoError(t, err, "IgnoreFamilyMismatch should accept non-negotiated family")
}

// TestShouldIgnoreFamily verifies per-family ignore configuration.
func TestShouldIgnoreFamily(t *testing.T) {
	s := &Session{
		settings: &PeerSettings{
			IgnoreFamilies: []capability.Family{
				{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
			},
		},
	}

	assert.True(t, s.shouldIgnoreFamily(capability.Family{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}))
	assert.False(t, s.shouldIgnoreFamily(capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}))
}
