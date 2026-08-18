package reactor

import (
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// VALIDATES: Capability negotiation (hold time, extended message, family intersection).
// PREVENTS: Hold time negotiation bugs, missing buffer resize for extended messages.

// newNegotiateSession creates a minimal Session with localOpen and peerOpen set for negotiation tests.
func newNegotiateSession(localHold, peerHold time.Duration) *Session {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.ReceiveHoldTime = localHold

	session := NewSession(settings)

	// Set local and peer OPENs so negotiateWith doesn't return early.
	session.localOpen = &message.Open{
		Version: 4, MyAS: 65001, HoldTime: uint16(localHold / time.Second),
		BGPIdentifier: 0x01020301,
	}
	session.peerOpen = &message.Open{
		Version: 4, MyAS: 65002, HoldTime: uint16(peerHold / time.Second),
		BGPIdentifier: 0x01020302, ASN4: 65002,
	}

	return session
}

// TestNegotiateWith_HoldTimeMinOfBoth verifies hold time is min(local, peer).
// RFC 4271 Section 4.2: "the smaller of its configured Hold Time and the Hold Time received".
// RFC requirement: RFC4271-4.2-2 positive -- the negotiated Hold Time is the smaller of the
// configured and the received value, whichever side it comes from
// (internal/component/bgp/reactor/session_negotiate.go:47-54).
func TestNegotiateWith_HoldTimeMinOfBoth(t *testing.T) {
	tests := []struct {
		name     string
		local    time.Duration
		peer     time.Duration
		expected uint16
	}{
		{"local_smaller", 60 * time.Second, 90 * time.Second, 60},
		{"peer_smaller", 90 * time.Second, 30 * time.Second, 30},
		{"equal", 45 * time.Second, 45 * time.Second, 45},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newNegotiateSession(tt.local, tt.peer)
			s.negotiateWith(nil, nil)

			neg := s.Negotiated()
			require.NotNil(t, neg)
			assert.Equal(t, tt.expected, neg.HoldTime)
		})
	}
}

// TestNegotiateWith_HoldTimeZero verifies zero hold time from either side.
// RFC 4271 Section 4.2: "if the negotiated value is zero, no keepalive messages".
// RFC requirement: RFC4271-4.2-2 negative -- the smaller-of-the-two rule is not applied blindly:
// a zero proposed by either side yields zero rather than the other side's non-zero value
// (internal/component/bgp/reactor/session_negotiate.go:50-53).
func TestNegotiateWith_HoldTimeZero(t *testing.T) {
	tests := []struct {
		name  string
		local time.Duration
		peer  time.Duration
	}{
		{"local_zero", 0, 90 * time.Second},
		{"peer_zero", 90 * time.Second, 0},
		{"both_zero", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newNegotiateSession(tt.local, tt.peer)
			s.negotiateWith(nil, nil)

			neg := s.Negotiated()
			require.NotNil(t, neg)
			assert.Equal(t, uint16(0), neg.HoldTime)
		})
	}
}

// TestNegotiateWith_HoldTimeFloorAt3 verifies the floor at 3 seconds.
// RFC 4271 Section 4.2: hold time value MUST be either zero or at least 3 seconds.
func TestNegotiateWith_HoldTimeFloorAt3(t *testing.T) {
	// Both sides have a very low hold time (but > 0) — floor applies.
	s := newNegotiateSession(1*time.Second, 2*time.Second)
	s.negotiateWith(nil, nil)

	neg := s.Negotiated()
	require.NotNil(t, neg)
	assert.Equal(t, uint16(3), neg.HoldTime, "hold time should be floored to 3s")
}

// TestNegotiateWith_ExtendedMessage verifies extended message resizes write buffer.
// RFC 8654: both sides must support for negotiation.
func TestNegotiateWith_ExtendedMessage(t *testing.T) {
	s := newNegotiateSession(90*time.Second, 90*time.Second)

	localCaps := []capability.Capability{
		&capability.ExtendedMessage{},
	}
	peerCaps := []capability.Capability{
		&capability.ExtendedMessage{},
	}

	s.negotiateWith(localCaps, peerCaps)

	neg := s.Negotiated()
	require.NotNil(t, neg)
	assert.True(t, neg.ExtendedMessage)
	assert.True(t, s.extendedMessage)
}

// TestNegotiateWith_NilOpens verifies early return when OPENs not set.
func TestNegotiateWith_NilOpens(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	session := NewSession(settings)

	// Neither localOpen nor peerOpen set — should return without panicking.
	session.negotiateWith(nil, nil)
	assert.Nil(t, session.Negotiated())
}

// TestNegotiateWith_FamilyIntersection verifies only common families are negotiated.
func TestNegotiateWith_FamilyIntersection(t *testing.T) {
	s := newNegotiateSession(90*time.Second, 90*time.Second)

	localCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}
	peerCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		// Peer does NOT have IPv6 unicast.
	}

	s.negotiateWith(localCaps, peerCaps)

	neg := s.Negotiated()
	require.NotNil(t, neg)
	assert.True(t, neg.SupportsFamily(capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}))
	assert.False(t, neg.SupportsFamily(capability.Family{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}))
}

// TestOpenAdvertisesVPNv6Capability verifies the OPEN advertises the VPNv6
// Multiprotocol capability when the ipv6/mpls-vpn family is configured.
//
// VALIDATES: A configured ipv6/mpls-vpn family produces a Multiprotocol capability
// (AFI=2/SAFI=128) that is carried in the OPEN's optional parameters.
// PREVENTS: A configured VPNv6 family that is never advertised, so the peer cannot
// negotiate it.
//
// RFC requirement: RFC4659-3.4-1 positive -- when the ipv6/mpls-vpn family is configured, the OPEN advertises the Multiprotocol capability for AFI=2/SAFI=128 so VPN-IPv6 is exchanged via BGP capability negotiation.
func TestOpenAdvertisesVPNv6Capability(t *testing.T) {
	tree := map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "10.0.0.1"},
			"local":  map[string]any{"ip": "auto"},
		},
		"session": map[string]any{
			"asn": map[string]any{"remote": "65001"},
			"family": map[string]any{
				"ipv6/mpls-vpn": map[string]any{
					"mode":   "enable",
					"prefix": map[string]any{"maximum": "100000"},
				},
			},
		},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)

	// sendOpen encodes settings.Capabilities into the OPEN's optional params.
	open := &message.Open{Version: 4, OptionalParams: buildOptionalParams(ps.Capabilities)}
	caps, err := capability.ParseFromOptionalParams(open.OptionalParams)
	require.NoError(t, err)

	found := false
	for _, c := range caps {
		if mp, ok := c.(*capability.Multiprotocol); ok &&
			mp.AFI == capability.AFIIPv6 && mp.SAFI == capability.SAFIVPN {
			found = true
			break
		}
	}
	assert.True(t, found, "OPEN must advertise Multiprotocol AFI=2/SAFI=128 for configured ipv6/mpls-vpn")
}

// TestNegotiateWith_VPNv6NotActiveWithoutPeerCapability verifies the VPNv6 family
// is not active when the peer did not advertise the AFI=2/SAFI=128 capability.
//
// VALIDATES: negotiateWith intersects local and peer capabilities, so VPNv6 is not
// negotiated when the peer omits the Multiprotocol capability for AFI=2/SAFI=128.
// PREVENTS: Treating VPN-IPv6 as active without the peer having negotiated it.
//
// RFC requirement: RFC4659-3.4-1 negative -- VPN-IPv6 is not negotiated (family inactive) when the peer does not advertise the Multiprotocol capability for AFI=2/SAFI=128.
func TestNegotiateWith_VPNv6NotActiveWithoutPeerCapability(t *testing.T) {
	s := newNegotiateSession(90*time.Second, 90*time.Second)

	localCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIVPN},
	}
	// Peer advertises only IPv4 unicast: no VPNv6 capability.
	peerCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	s.negotiateWith(localCaps, peerCaps)

	neg := s.Negotiated()
	require.NotNil(t, neg)
	assert.False(t,
		neg.SupportsFamily(capability.Family{AFI: capability.AFIIPv6, SAFI: capability.SAFIVPN}),
		"VPNv6 must not be active when the peer did not negotiate AFI=2/SAFI=128")
}

// TestBuildOptionalParams_Empty verifies nil return for no capabilities.
func TestBuildOptionalParams_Empty(t *testing.T) {
	result := buildOptionalParams(nil)
	assert.Nil(t, result)
}

// TestBuildOptionalParams_SingleCap verifies correct TLV encoding for one capability.
func TestBuildOptionalParams_SingleCap(t *testing.T) {
	caps := []capability.Capability{
		&capability.ASN4{ASN: 65001},
	}

	result := buildOptionalParams(caps)
	require.NotNil(t, result)

	// Param type=2, param length=6 (ASN4: code=65, len=4, data=4 bytes)
	assert.Equal(t, byte(2), result[0], "param type")
	assert.Equal(t, byte(6), result[1], "param length")
	assert.Equal(t, byte(65), result[2], "cap code = ASN4")
	assert.Equal(t, byte(4), result[3], "cap length")
}

// TestBuildOptionalParams_MultipleCaps verifies bundled encoding (RFC 5492 §4).
// All capabilities are packed in a single type-2 parameter.
func TestBuildOptionalParams_MultipleCaps(t *testing.T) {
	caps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: 65001},
	}

	result := buildOptionalParams(caps)
	require.NotNil(t, result)

	// Single type-2 param wrapping both capabilities.
	// MP: code=1, len=4, data=4 bytes (6 total)
	// ASN4: code=65, len=4, data=4 bytes (6 total)
	// Param: type=2, len=12, then 12 bytes of capability TLVs.
	assert.Equal(t, byte(2), result[0], "param type")
	assert.Equal(t, byte(12), result[1], "param length = 6+6")
	assert.Equal(t, byte(1), result[2], "first cap code = Multiprotocol")
	assert.Equal(t, byte(65), result[8], "second cap code = ASN4")
}

// newNoFamilySession returns a Session whose settings declare no capability at
// all, which is what a peer block with no `family` section produces.
func newNoFamilySession(t *testing.T) (*Session, *PeerSettings) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	return NewSession(settings), settings
}

// negotiateAgainstMirror negotiates the given OPEN against a peer that echoes it,
// which is what ze-peer does on the wire and what a silent RFC 4271 speaker
// produces when ze itself declares no Multiprotocol capability.
func negotiateAgainstMirror(t *testing.T, open *message.Open) *NegotiatedCapabilities {
	t.Helper()
	caps, err := capability.ParseFromOptionalParams(open.OptionalParams)
	require.NoError(t, err)
	return NewNegotiatedCapabilities(capability.Negotiate(caps, caps, 65001, 65002))
}

// TestBuildOpenNoFamilyNegotiatesImplicitIPv4Unicast covers the peer this spec
// exists for: no `family` block in the config and no plugin declaring decode
// families. The OPEN carries ASN4 and nothing else, exactly as before, and the
// negotiated family set is ipv4/unicast, so the End-of-RIB loop in
// sendInitialRoutes has a family to send a marker for.
//
// VALIDATES: AC-1 -- a peer with no family block and no plugin decode families
// negotiates ipv4/unicast rather than nothing.
// PREVENTS: the fix being applied in buildOpen, which would add a Multiprotocol
// capability to an OPEN that carried none and change the bytes ze puts on the
// wire for every such peer.
func TestBuildOpenNoFamilyNegotiatesImplicitIPv4Unicast(t *testing.T) {
	s, settings := newNoFamilySession(t)

	open := s.buildOpen(settings, settings.Capabilities)

	// One type-2 optional parameter carrying one capability: ASN4 (code 0x41,
	// length 4) with local AS 65001 = 0x0000FDE9. No Multiprotocol capability.
	assert.Equal(t, "020641040000FDE9", strings.ToUpper(hex.EncodeToString(open.OptionalParams)),
		"the OPEN of a peer with no family block must carry ASN4 alone, unchanged by this fix")

	nc := negotiateAgainstMirror(t, open)
	require.NotNil(t, nc)
	assert.Equal(t, []family.Family{family.IPv4Unicast}, nc.Families(),
		"a session where neither side declares a Multiprotocol capability exchanges "+
			"IPv4 unicast (RFC 4271) and is owed its End-of-RIB marker (RFC 4724 Section 4)")
}

// TestBuildOpenConfigFamiliesUnchanged pins AC-2: a peer that DOES declare a
// family keeps its wire and its negotiated set byte for byte.
//
// VALIDATES: AC-2 -- the implicit family never fires for a side that advertised a
// Multiprotocol capability, so the declared set is the negotiated set.
// PREVENTS: the default being added to the advertised set rather than substituted
// for an empty one, which would negotiate ipv4/unicast for an ipv6-only peer.
func TestBuildOpenConfigFamiliesUnchanged(t *testing.T) {
	s, settings := newNoFamilySession(t)
	settings.Capabilities = []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}

	open := s.buildOpen(settings, settings.Capabilities)

	// Multiprotocol ipv6/unicast (code 01, length 4, AFI 0x0002, reserved 00,
	// SAFI 01) then ASN4, bundled in one type-2 optional parameter of 12 bytes.
	assert.Equal(t, "020C010400020001"+"41040000FDE9",
		strings.ToUpper(hex.EncodeToString(open.OptionalParams)),
		"a declared family produces the same OPEN bytes as before the implicit-family fix")

	nc := negotiateAgainstMirror(t, open)
	require.NotNil(t, nc)
	assert.Equal(t, []family.Family{family.IPv6Unicast}, nc.Families(),
		"the declared family is the negotiated family: no implicit ipv4/unicast is added")
}

// TestBuildOpenPluginFamiliesUnchanged pins AC-3: when the config declares no
// family but a plugin declares decode families, the plugin families still fill
// the gap and the implicit default does not fire.
//
// VALIDATES: AC-3 -- plugin decode families are advertised and negotiated exactly
// as before.
// PREVENTS: the default overriding a plugin-supplied family set, which would make
// a plugin-only peer negotiate ipv4/unicast instead of what the plugin decodes.
func TestBuildOpenPluginFamiliesUnchanged(t *testing.T) {
	s, settings := newNoFamilySession(t)
	s.SetPluginFamiliesGetter(func() []string { return []string{"ipv6/unicast"} })

	open := s.buildOpen(settings, settings.Capabilities)

	assert.Equal(t, "020C010400020001"+"41040000FDE9",
		strings.ToUpper(hex.EncodeToString(open.OptionalParams)),
		"a plugin decode family is advertised exactly as a configured one is")

	nc := negotiateAgainstMirror(t, open)
	require.NotNil(t, nc)
	assert.Equal(t, []family.Family{family.IPv6Unicast}, nc.Families(),
		"the plugin family is the negotiated family: no implicit ipv4/unicast is added")
}
