package reactor

import (
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// rfc-test-change-approved: 2026-07-22 Thomas approved the msgtype/routeaction
// package rename (spec-feature-gate-10-bgp). MessageType/Type* moved to
// internal/core/bgp/msgtype and the route-action enum to
// internal/core/bgp/routeaction so MRT, sysrib and the FIB backends keep
// compiling when the BGP engine is compiled out (//go:build ze_bgp). Every hunk
// in this file is a package-qualifier requalification: no assertion was added,
// removed, reworded, weakened or re-tagged, verified by normalising the diff
// under the renaming and confirming the add/delete multisets cancel.

// VALIDATES: Session handler error paths (OPEN version/hold/malformed/caps, unknown type, ROUTE-REFRESH).
// PREVENTS: Silent acceptance of invalid messages, missing NOTIFICATION on protocol errors.

// newOpenSentSession creates a Session in OpenSent state with a connected net.Pipe.
// A drain goroutine reads data from the client side so handler writes don't block.
func newOpenSentSession(t *testing.T) *Session {
	t.Helper()

	session, client := newOpenSentSessionWithClient(t)
	go func() {
		buf := make([]byte, 65536)
		for {
			if _, readErr := client.Read(buf); readErr != nil {
				return
			}
		}
	}()
	return session
}

func newOpenSentSessionWithClient(t *testing.T) (*Session, net.Conn) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
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

	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})

	return session, client
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

// TestOpenRejectsMalformedKnownCapability verifies OPEN capability parsing rejects
// known zero-length capabilities with non-zero payload lengths.
//
// VALIDATES: AC-1 malformed known capabilities reject OPEN before negotiation.
//
// PREVENTS: Session establishment with malformed Route Refresh capability data.
//
// RFC requirement: RFC2918-2-1 negative -- a Route Refresh capability whose Length
// is not 0 (here 1) is rejected: parseZeroLengthCapability (capability.go) returns
// ErrInvalidLength, so the OPEN is refused with an Unsupported Capability
// NOTIFICATION instead of establishing. Proves the Length-0 constraint is enforced,
// not merely emitted.
//
// RFC requirement: RFC5492-3-1 positive -- when the Unsupported Capability NOTIFICATION
// is sent, its Data field carries the offending capability TLV {CodeRouteRefresh, 0x01,
// 0x00}; buildUnsupportedCapabilityData/ErrorData place the capability that caused the
// message into the NOTIFICATION (internal/component/bgp/reactor/session_validation.go:396,
// session_handlers.go:190-197).
// RFC requirement: RFC5492-3-3 negative -- the session IS torn down here, but only because
// the capability is a KNOWN one with a malformed length (ErrInvalidLength), not because it
// is unsupported; this bounds the MUST-NOT-terminate rule to reject malformed input only.
// RFC requirement: RFC5492-3-4 negative -- an Unsupported Capability NOTIFICATION IS
// generated here for a malformed known capability, showing the notification path is reached
// only on malformed input, never for a merely unrecognized capability.
// RFC requirement: RFC5492-5-2 negative -- rejection with an Unsupported Capability
// NOTIFICATION occurs only for a malformed known capability, distinguishing it from a
// not-understood capability, which MUST be ignored rather than rejected.
// Untagged for RFC4271-6.2-3: that requirement is recorded {gap} in rfc/short/rfc4271.md
// because an OPEN whose body fails to decode returns from handleOpen with no NOTIFICATION
// at all (internal/component/bgp/reactor/session_handlers.go:43-47). This case reaches only
// the capability-validation rail (:185-199), which does emit Error Code 2, so it cannot
// stand as coverage of "ALL OPEN errors".
func TestOpenRejectsMalformedKnownCapability(t *testing.T) {
	s, client := newOpenSentSessionWithClient(t)

	body := []byte{
		0x04,       // Version 4
		0xFD, 0xEA, // MyAS = 65002
		0x00, 0x5A, // HoldTime = 90
		0x01, 0x02, 0x03, 0x02, // BGP Identifier = 1.2.3.2
		0x05,       // OptParamLen = 5
		0x02, 0x03, // Optional Parameter type=Capabilities, len=3
		byte(capability.CodeRouteRefresh), 0x01, 0x00, // Route Refresh must be length 0
	}

	notifCh := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := client.Read(buf)
		notifCh <- append([]byte(nil), buf[:n]...)
	}()

	err := s.handleOpen(body)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidMessage)
	assert.Nil(t, s.negotiated)
	assert.NotEqual(t, fsm.StateEstablished, s.State())

	var notif []byte
	select {
	case notif = <-notifCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OPEN NOTIFICATION")
	}
	require.GreaterOrEqual(t, len(notif), message.HeaderLen+5)
	assert.Equal(t, byte(msgtype.TypeNOTIFICATION), notif[18])
	assert.Equal(t, byte(message.NotifyOpenMessage), notif[message.HeaderLen])
	assert.Equal(t, message.NotifyOpenUnsupportedCapability, notif[message.HeaderLen+1])
	assert.Equal(t, []byte{byte(capability.CodeRouteRefresh), 0x01, 0x00}, notif[message.HeaderLen+2:])
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
	settings.ReceiveHoldTime = 90 * time.Second
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
	settings.ReceiveHoldTime = 90 * time.Second
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
	settings.ReceiveHoldTime = 90 * time.Second
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

	err := s.handleUnknownType(msgtype.MessageType(99))
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
//
// RFC requirement: RFC2918-3-2 negative -- on receive, a ROUTE-REFRESH body whose
// length is not exactly 4 (too short, too long, or empty) is rejected by
// validateRouteRefreshLength (session_handlers.go) with ErrInvalidMessage before any
// <AFI, SAFI> is acted upon.
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
//
// RFC requirement: RFC7313-4-3 negative -- handleRouteRefresh examines the Message
// Subtype field, and for a value other than 0/1/2 (here 3, 100, 255) it takes no
// BoRR/EoRR/normal action, returning without error rather than processing the message.
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

// TestHandleRouteRefresh_NoCapability verifies ROUTE-REFRESH is ignored when
// Route Refresh capability was not negotiated.
// RFC 2918 Section 3: receiver SHOULD advertise capability.
func TestHandleRouteRefresh_NoCapability(t *testing.T) {
	s := newOpenSentSession(t)

	// Set negotiated without RouteRefresh
	s.negotiated = capability.Negotiate(
		[]capability.Capability{
			&capability.ASN4{ASN: 65001},
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		},
		[]capability.Capability{
			&capability.ASN4{ASN: 65002},
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		},
		65001, 65002,
	)

	// Valid ROUTE-REFRESH body: AFI=1 (IPv4), Subtype=0 (normal), SAFI=1 (Unicast)
	body := []byte{0x00, 0x01, 0x00, 0x01}

	err := s.handleRouteRefresh(body)
	assert.NoError(t, err) // Silently ignored, no error
}

// TestHandleRouteRefresh_NonNegotiatedFamily verifies ROUTE-REFRESH for a non-negotiated
// address family is ignored.
// RFC 2918 Section 4: SHOULD ignore for AFI/SAFI not advertised.
//
// RFC requirement: RFC2918-4-2 positive -- a ROUTE-REFRESH for an <AFI, SAFI> the
// speaker did not advertise (IPv6 unicast, with only IPv4 unicast negotiated) is
// ignored: handleRouteRefresh takes the !SupportsFamily branch (session_handlers.go)
// and returns without acting on the request.
func TestHandleRouteRefresh_NonNegotiatedFamily(t *testing.T) {
	s := newOpenSentSession(t)

	// Negotiate with IPv4 unicast and RouteRefresh
	s.negotiated = capability.Negotiate(
		[]capability.Capability{
			&capability.ASN4{ASN: 65001},
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
			&capability.RouteRefresh{},
		},
		[]capability.Capability{
			&capability.ASN4{ASN: 65002},
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
			&capability.RouteRefresh{},
		},
		65001, 65002,
	)

	// Request ROUTE-REFRESH for IPv6 unicast (not negotiated)
	body := []byte{0x00, 0x02, 0x00, 0x01} // AFI=2 (IPv6), Subtype=0, SAFI=1

	err := s.handleRouteRefresh(body)
	assert.NoError(t, err) // Silently ignored
}

// VALIDATES: a second OPEN arriving on an ALREADY-ESTABLISHED connection is
// refused with NOTIFICATION Cease and the connection closed, instead of being
// processed as if the session were still negotiating.
// PREVENTS: silent mid-session re-negotiation. handleOpen had no state gate, so
// an OPEN on a live session re-ran the whole path: it overwrote s.peerOpen and
// called negotiateWith, letting a peer change the agreed capability set (here
// AddPath, but equally families or ASN4) on an established peering. The second
// assertion is the load-bearing one -- checking only that a NOTIFICATION was
// sent would still pass if the capabilities had been swapped first.
//
// RFC 4271 Section 8.2.2 excludes BGPOpen (Event 19) from the FSM-Error branch
// in both Established ("Events 9, 12-13, 20-22") and OpenConfirm ("Events 9,
// 12-13, 20, 27-28"), routing it through collision detection, whose termination
// action is a NOTIFICATION with a Cease. Hence Cease (6), not FSM Error (5).
func TestSecondOpenOnEstablishedSessionIsRefused(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	_ = acceptWithReader(t, session, server, client)

	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			0x02, 0x06, 0x41, 0x04, 0x00, 0x00, 0xFD, 0xEA, // ASN4
			0x02, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01, // IPv4/Unicast
		},
	}
	// Everything ze writes is captured rather than discarded: the NOTIFICATION
	// code is part of what this test proves, and a plain drain would swallow it.
	fromZe := make(chan []byte, 16)
	go func() {
		defer close(fromZe)
		buf := make([]byte, 65536)
		for {
			n, readErr := client.Read(buf)
			if readErr != nil {
				return
			}
			msg := make([]byte, n)
			copy(msg, buf[:n])
			fromZe <- msg
		}
	}()
	go func() {
		if _, writeErr := client.Write(message.PackTo(peerOpen, nil)); writeErr != nil {
			return
		}
	}()
	require.NoError(t, session.ReadAndProcess())

	go func() {
		if _, writeErr := client.Write(message.PackTo(message.NewKeepalive(), nil)); writeErr != nil {
			return
		}
	}()
	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, session.State())

	session.mu.RLock()
	negotiatedBefore := session.negotiated
	session.mu.RUnlock()
	require.NotNil(t, negotiatedBefore, "precondition: capabilities negotiated at establishment")

	// A second OPEN on the SAME connection, now advertising AddPath, which the
	// first OPEN did not carry. If handleOpen re-processes it, the negotiated
	// set silently changes underneath an established session.
	secondOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			0x02, 0x06, 0x41, 0x04, 0x00, 0x00, 0xFD, 0xEA, // ASN4
			0x02, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01, // IPv4/Unicast
			0x02, 0x06, 0x45, 0x04, 0x00, 0x01, 0x01, 0x03, // AddPath IPv4/Unicast send+recv
		},
	}
	go func() {
		if _, writeErr := client.Write(message.PackTo(secondOpen, nil)); writeErr != nil {
			return
		}
	}()
	err := session.ReadAndProcess()

	// Checked BEFORE the error assertions on purpose. The FSM does eventually
	// reject the event either way, so asserting the error first would abort the
	// test on a fatal and hide whether the capability set had already been
	// rebuilt by then -- which is the actual defect, and the thing that stayed
	// invisible before this gate existed.
	session.mu.RLock()
	negotiatedAfter := session.negotiated
	session.mu.RUnlock()
	assert.Same(t, negotiatedBefore, negotiatedAfter,
		"negotiated capabilities must not be rebuilt by an OPEN on an established session")

	// The FSM MUST leave Established. Section 8.2.2's action list for
	// terminating on Event 19 says "changes its state to Idle", and in this
	// reactor that transition is what drives the whole peer-closed cascade:
	// peer_run.go's `from == fsm.StateEstablished` branch owns stopBFDClient,
	// raiseSessionDropped and notifyPeerClosed, and notifyPeerClosed is the only
	// producer of the SessionStateDown that makes adj_rib_in clear peerUp and
	// drop the peer's stored routes. Closing the socket without the transition
	// leaves a dead peer marked up whose routes are replayed on reconnect --
	// which is exactly what the first version of this fix did.
	assert.NotEqual(t, fsm.StateEstablished, session.State(),
		"session must leave Established so the peer-closed cascade runs")

	require.Error(t, err, "a second OPEN on an established session must be refused")
	require.ErrorIs(t, err, ErrInvalidState)

	// Cease (6), per Section 8.2.2's Event 19 termination action. Asserted on
	// the WIRE: without this the comment argues for Cease over FSM Error while
	// nothing stops the code sending either.
	var notification []byte
	for msg := range fromZe {
		if len(msg) >= message.HeaderLen+2 && msg[message.HeaderLen-1] == byte(msgtype.TypeNOTIFICATION) {
			notification = msg
			break
		}
	}
	require.NotNil(t, notification, "expected a NOTIFICATION on the wire")
	assert.Equal(t, uint8(message.NotifyCease), notification[message.HeaderLen],
		"RFC 4271 Section 8.2.2 terminates Event 19 with a Cease, not an FSM Error")

	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})
}

// VALIDATES: AC-4a's OpenConfirm half -- a second OPEN arriving after the peer's
// first OPEN but BEFORE the confirming KEEPALIVE is refused with Cease, exactly
// as in Established.
// PREVENTS: a gate that reads correct but only covers one of the two states it
// names. handleOpen's guard tests Established||OpenConfirm; before this test
// only the Established arm was exercised, so dropping the OpenConfirm arm would
// not have turned anything red.
//
// RFC 4271 Section 8.2.2 treats the two states the same way for BGPOpen (Event
// 19): OpenConfirm's any-other-event branch is scoped to "Events 9, 12-13, 20,
// 27-28" and Established's to "Events 9, 12-13, 20-22", so 19 is excluded from
// both and routed through collision detection, terminating with a Cease.
func TestSecondOpenInOpenConfirmIsRefused(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	_ = acceptWithReader(t, session, server, client)
	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})

	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			0x02, 0x06, 0x41, 0x04, 0x00, 0x00, 0xFD, 0xEA, // ASN4
			0x02, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01, // IPv4/Unicast
		},
	}
	fromZe := make(chan []byte, 16)
	go func() {
		defer close(fromZe)
		buf := make([]byte, 65536)
		for {
			n, readErr := client.Read(buf)
			if readErr != nil {
				return
			}
			msg := make([]byte, n)
			copy(msg, buf[:n])
			fromZe <- msg
		}
	}()
	go func() {
		if _, writeErr := client.Write(message.PackTo(peerOpen, nil)); writeErr != nil {
			return
		}
	}()
	require.NoError(t, session.ReadAndProcess())

	// No KEEPALIVE yet, so the FSM sits in OpenConfirm -- the state this test
	// exists for. handleOpen fires EventBGPOpen at its end, which is what moved
	// us out of OpenSent.
	require.Equal(t, fsm.StateOpenConfirm, session.State(),
		"precondition: a first OPEN with no KEEPALIVE leaves the session in OpenConfirm")

	go func() {
		if _, writeErr := client.Write(message.PackTo(peerOpen, nil)); writeErr != nil {
			return
		}
	}()
	err := session.ReadAndProcess()

	assert.NotEqual(t, fsm.StateOpenConfirm, session.State(),
		"session must leave OpenConfirm rather than silently re-negotiating")
	require.Error(t, err, "a second OPEN in OpenConfirm must be refused")
	require.ErrorIs(t, err, ErrInvalidState)

	var notification []byte
	for msg := range fromZe {
		if len(msg) >= message.HeaderLen+2 && msg[message.HeaderLen-1] == byte(msgtype.TypeNOTIFICATION) {
			notification = msg
			break
		}
	}
	require.NotNil(t, notification, "expected a NOTIFICATION on the wire")
	assert.Equal(t, uint8(message.NotifyCease), notification[message.HeaderLen],
		"RFC 4271 Section 8.2.2 terminates Event 19 with a Cease in OpenConfirm too")
}

// TestHandleUpdate_FamilyMismatchIgnoreMode verifies IgnoreFamilyMismatch mode.
// RFC 4760 Section 6: lenient mode logs but doesn't reject.
func TestHandleUpdate_FamilyMismatchIgnoreMode(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
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

// TestHandleNotificationShutdownMessage verifies RFC 8203 shutdown communication
// is processed without error when present in a Cease/AdminShutdown NOTIFICATION.
//
// VALIDATES: handleNotification correctly parses shutdown message and returns
// an error wrapping ErrNotificationRecv.
//
// PREVENTS: Crash or incorrect error when peer sends shutdown with message.
func TestHandleNotificationShutdownMessage(t *testing.T) {
	s := newOpenSentSession(t)

	// Cease (6) / Admin Shutdown (2) with 11-byte message "maintenance"
	body := []byte{0x06, 0x02, 0x0B, 'm', 'a', 'i', 'n', 't', 'e', 'n', 'a', 'n', 'c', 'e'}

	err := s.handleNotification(body)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotificationRecv)
}

// TestHandleNotificationInvalidShutdownUTF8 verifies handleNotification does not
// crash when the shutdown communication contains invalid UTF-8 bytes.
//
// RFC 9003 Section 2: message MUST be UTF-8 encoded. Invalid UTF-8 should be
// logged as a warning but must not prevent normal NOTIFICATION processing.
//
// VALIDATES: Invalid UTF-8 in shutdown data does not panic or prevent error return.
//
// PREVENTS: Crash on malformed shutdown communication from misbehaving peer.
func TestHandleNotificationInvalidShutdownUTF8(t *testing.T) {
	s := newOpenSentSession(t)

	// Cease (6) / Admin Shutdown (2) with 3-byte claimed length, invalid UTF-8
	body := []byte{0x06, 0x02, 0x03, 0xFF, 0xFE, 0xFD}

	err := s.handleNotification(body)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotificationRecv)
}

// TestHandleNotificationCeaseNoMessage verifies a Cease/Unspecific NOTIFICATION
// with no data is handled cleanly.
//
// VALIDATES: Cease with subcode 0 (Unspecific) and no data returns ErrNotificationRecv.
//
// PREVENTS: Error path diverging for minimal Cease notifications.
func TestHandleNotificationCeaseNoMessage(t *testing.T) {
	s := newOpenSentSession(t)

	// Cease (6) / Unspecific (0), no data
	body := []byte{0x06, 0x00}

	err := s.handleNotification(body)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotificationRecv)
}
