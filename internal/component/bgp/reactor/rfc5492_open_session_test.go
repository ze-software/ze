package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// newCapabilityOpenSession returns a passive OpenSent session advertising
// caps and requiring required, with the recording connection and the octet
// count its own OPEN took.
func newCapabilityOpenSession(t *testing.T, caps []capability.Capability, required []capability.Code) (*Session, *recordingConn, int) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.Capabilities = caps
	settings.RequiredCapabilities = required
	session := NewSession(settings)
	require.NoError(t, session.Start())
	conn := &recordingConn{}
	require.NoError(t, session.Accept(conn))
	require.Equal(t, fsm.StateOpenSent, session.State())
	t.Cleanup(func() {
		require.NoError(t, session.Stop())
		session.closeConn()
	})
	return session, conn, len(conn.written())
}

// openBodyWithParameters returns the peer's OPEN body carrying one
// Capabilities optional parameter (type 2) for each entry of params, each
// holding that entry's capability TLV octets.
func openBodyWithParameters(params ...[]byte) []byte {
	body := validOpenBody()
	optionalOctets := 0
	for _, caps := range params {
		body = append(body, 2, byte(len(caps)))
		body = append(body, caps...)
		optionalOctets += 2 + len(caps)
	}
	body[9] = byte(optionalOctets)
	return body
}

// requireOnlyKeepalive asserts the octets ze wrote after its own OPEN are
// exactly one KEEPALIVE: the OPEN was accepted and no NOTIFICATION left.
func requireOnlyKeepalive(t *testing.T, written []byte) {
	t.Helper()
	messages := splitMessages(t, written)
	require.Len(t, messages, 1, "acceptance writes exactly one message")
	require.Equal(t, byte(msgtype.TypeKEEPALIVE), messages[0][18])
}

// requireUnsupportedCapabilityData asserts the octets ze wrote after its own
// OPEN are one NOTIFICATION 2/7 whose Data is exactly data.
func requireUnsupportedCapabilityData(t *testing.T, written, data []byte) {
	t.Helper()
	messages := splitMessages(t, written)
	require.Len(t, messages, 1, "refusal writes exactly one message")
	notification := messages[0]
	require.Equal(t, byte(msgtype.TypeNOTIFICATION), notification[18])
	require.Equal(t, byte(message.NotifyOpenMessage), notification[message.HeaderLen])
	require.Equal(t, message.NotifyOpenUnsupportedCapability, notification[message.HeaderLen+1])
	require.Equal(t, data, notification[message.HeaderLen+2:])
}

var (
	familyIPv4Unicast = capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}
	familyIPv6Unicast = capability.Family{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}
)

// Goal: prove an OPEN carrying capabilities ze does not recognize is accepted
// at the session, with those capabilities ignored.
// Method: an OpenSent session advertising IPv4 unicast and Route Refresh (the
// latter required) takes a peer OPEN through handleOpen that interleaves the
// unrecognized codes 253 (empty) and 254 (two octets) with the known ones.
//
// VALIDATES: handleOpen advances to OpenConfirm, writes only a KEEPALIVE, and
// negotiates the known capabilities around the unrecognized ones.
// PREVENTS: an unrecognized capability refused, or taking a known one with it.
//
// RFC requirement: RFC5492-3-2 positive -- a peer OPEN carrying the unrecognized codes 253 and 254 between Multiprotocol IPv4 unicast and the required Route Refresh is accepted: OpenConfirm, exactly one KEEPALIVE written, Route Refresh and IPv4 unicast negotiated.
func TestRFC5492UnrecognizedCapabilitiesIgnoredAtTheSession(t *testing.T) {
	s, conn, sent := newCapabilityOpenSession(t,
		[]capability.Capability{capIPv4(), &capability.RouteRefresh{}},
		[]capability.Code{capability.CodeRouteRefresh})

	require.NoError(t, s.handleOpen(openBodyWithParameters([]byte{
		253, 0, // unrecognized, empty
		1, 4, 0, 1, 0, 1, // Multiprotocol IPv4 unicast
		254, 2, 0xab, 0xcd, // unrecognized, two octets
		2, 0, // Route Refresh
	})))
	require.Equal(t, fsm.StateOpenConfirm, s.State())
	requireOnlyKeepalive(t, conn.written()[sent:])
	require.True(t, s.Negotiated().RouteRefresh)
	require.True(t, s.Negotiated().SupportsFamily(familyIPv4Unicast))
}

// Goal: prove an unrecognized capability has no effect on negotiation, even
// when its value octets are exactly those of a capability ze knows.
// Method: ze advertises IPv4 and IPv6 unicast; the peer advertises IPv4
// unicast and an unrecognized code 254 whose value is 00 02 00 01, the
// Multiprotocol value for IPv6 unicast.
//
// VALIDATES: the OPEN is accepted and IPv6 unicast is NOT negotiated.
// PREVENTS: an unrecognized capability acted on rather than ignored.
//
// RFC requirement: RFC5492-3-2 negative -- an unrecognized code 254 whose value is the Multiprotocol IPv6 unicast value does not negotiate IPv6 unicast: ze advertising IPv4 and IPv6 unicast, the peer sending IPv4 unicast plus that TLV, the session reaches OpenConfirm with exactly one KEEPALIVE, IPv4 unicast negotiated and IPv6 unicast not.
func TestRFC5492UnrecognizedCapabilityNeverReadAsKnown(t *testing.T) {
	s, conn, sent := newCapabilityOpenSession(t,
		[]capability.Capability{capIPv4(), capIPv6()}, nil)

	require.NoError(t, s.handleOpen(openBodyWithParameters([]byte{
		1, 4, 0, 1, 0, 1, // Multiprotocol IPv4 unicast
		254, 4, 0, 2, 0, 1, // unrecognized, value of Multiprotocol IPv6 unicast
	})))
	require.Equal(t, fsm.StateOpenConfirm, s.State())
	requireOnlyKeepalive(t, conn.written()[sent:])
	require.True(t, s.Negotiated().SupportsFamily(familyIPv4Unicast))
	require.False(t, s.Negotiated().SupportsFamily(familyIPv6Unicast),
		"an unrecognized capability is ignored, never read as Multiprotocol")
}

// Goal: prove an OPEN repeating identical capability instances is accepted
// at the session and negotiated as if each were sent once.
// Method: ze requires Route Refresh and advertises IPv4 and IPv6 unicast; the
// peer OPEN repeats Route Refresh three times and each Multiprotocol TLV.
//
// VALIDATES: OpenConfirm, one KEEPALIVE, Route Refresh and both families
// negotiated.
// PREVENTS: a session-level dedup or refusal of repeated instances, which the
// parser-level test cannot see.
//
// RFC requirement: RFC5492-4-1 positive -- a peer OPEN carrying Route Refresh three times, Multiprotocol IPv4 unicast twice and Multiprotocol IPv6 unicast twice goes through handleOpen to OpenConfirm with exactly one KEEPALIVE written, Route Refresh and both families negotiated.
func TestRFC5492RepeatedCapabilityInstancesAcceptedAtTheSession(t *testing.T) {
	s, conn, sent := newCapabilityOpenSession(t,
		[]capability.Capability{capIPv4(), capIPv6(), &capability.RouteRefresh{}},
		[]capability.Code{capability.CodeRouteRefresh})

	require.NoError(t, s.handleOpen(openBodyWithParameters([]byte{
		2, 0,
		1, 4, 0, 1, 0, 1,
		2, 0,
		1, 4, 0, 2, 0, 1,
		1, 4, 0, 1, 0, 1,
		1, 4, 0, 2, 0, 1,
		2, 0,
	})))
	require.Equal(t, fsm.StateOpenConfirm, s.State())
	requireOnlyKeepalive(t, conn.written()[sent:])
	require.True(t, s.Negotiated().RouteRefresh)
	require.True(t, s.Negotiated().SupportsFamily(familyIPv4Unicast))
	require.True(t, s.Negotiated().SupportsFamily(familyIPv6Unicast))
}

// Goal: prove an OPEN whose capabilities are spread over several Capabilities
// optional parameters is accepted, every parameter contributing.
// Method: ze requires Route Refresh and Extended Message; the peer sends them
// in two separate parameters, and a third parameter holds an unrecognized TLV.
//
// VALIDATES: OpenConfirm, one KEEPALIVE, both capabilities negotiated.
// PREVENTS: only the first Capabilities parameter being read at the session.
//
// RFC requirement: RFC5492-4-2 positive -- a peer OPEN with three Capabilities optional parameters (Route Refresh; Extended Message; unrecognized 254) goes through handleOpen to OpenConfirm with exactly one KEEPALIVE written, Route Refresh and Extended Message both negotiated.
func TestRFC5492CapabilitiesSpreadOverSeveralParameters(t *testing.T) {
	s, conn, sent := newTwoRequiredOpenSession(t)

	require.NoError(t, s.handleOpen(openBodyWithParameters(
		[]byte{2, 0},
		[]byte{6, 0},
		[]byte{254, 2, 0xab, 0xcd},
	)))
	require.Equal(t, fsm.StateOpenConfirm, s.State())
	requireOnlyKeepalive(t, conn.written()[sent:])
	require.True(t, s.Negotiated().RouteRefresh)
	require.True(t, s.Negotiated().ExtendedMessageRecv)
	require.True(t, s.Negotiated().ExtendedMessageSend)
}

// Goal: prove an OPEN spread over several Capabilities parameters that lacks a
// required capability is refused, and that the refusal names only what is
// missing from ALL the parameters.
// Method: ze requires Route Refresh and Extended Message; the peer sends one
// of them in the first or the second parameter and an unrecognized TLV in the
// other.
//
// VALIDATES: NOTIFICATION 2/7 whose Data is exactly the capability absent
// from every parameter.
// PREVENTS: a parameter after the first being dropped (its capability would
// then be listed as missing), or a missing capability accepted.
//
// RFC requirement: RFC5492-4-2 negative -- with two Capabilities parameters, Route Refresh in the first and unrecognized 254 in the second draws NOTIFICATION 2/7 with Data exactly Extended Message (6 0); unrecognized 254 in the first and Extended Message in the second draws Data exactly Route Refresh (2 0); both refusals end in Idle.
func TestRFC5492EveryCapabilitiesParameterReadBeforeRefusing(t *testing.T) {
	s, conn, sent := newTwoRequiredOpenSession(t)
	require.ErrorIs(t, s.handleOpen(openBodyWithParameters(
		[]byte{2, 0},
		[]byte{254, 2, 0xab, 0xcd},
	)), ErrInvalidState)
	require.Equal(t, fsm.StateIdle, s.State())
	requireUnsupportedCapabilityData(t, conn.written()[sent:], []byte{6, 0})

	s, conn, sent = newTwoRequiredOpenSession(t)
	require.ErrorIs(t, s.handleOpen(openBodyWithParameters(
		[]byte{254, 2, 0xab, 0xcd},
		[]byte{6, 0},
	)), ErrInvalidState)
	require.Equal(t, fsm.StateIdle, s.State())
	requireUnsupportedCapabilityData(t, conn.written()[sent:], []byte{2, 0})
}

// Goal: preserve complete causing capability tuples on both OPEN processing rails.
// Method: compare emitted NOTIFICATION Data with literal local or peer OPEN TLVs,
// including different values on the two sides and repeated variable-length instances.
// RFC 5492 Section 5: "Each such capability is encoded in the same way as it
// would be encoded in the OPEN message."
// MUTATION: encoding only capability codes loses the ASN4/GR/ADD-PATH values.
// RFC requirement: RFC5492-5-1 positive -- both OPEN paths emit exact code, length and original local-required or peer-refused values, including ASN4, repeated Graceful Restart, ADD-PATH directions and variable-length hostname.
// RFC requirement: RFC5492-5-1 negative -- exact NOTIFICATION Data excludes satisfied or noncausing capabilities and does not substitute the opposite side's values or strip value-bearing capabilities to zero length.
func TestUnsupportedCapabilityNotificationPreservesOpenTuples(t *testing.T) {
	for _, rail := range []string{"handleOpen", "processOpen"} {
		for _, tc := range []struct {
			name     string
			local    []byte
			peer     []byte
			required []capability.Code
			refused  []capability.Code
			want     []byte
		}{
			{
				name:     "missing-asn4-local-value",
				required: []capability.Code{capability.CodeASN4},
				want:     []byte{65, 4, 0, 0, 0xfd, 0xe9},
			},
			{
				name:    "refused-asn4-peer-value",
				peer:    []byte{65, 4, 0, 0, 0xfd, 0xea},
				refused: []capability.Code{capability.CodeASN4},
				want:    []byte{65, 4, 0, 0, 0xfd, 0xea},
			},
			{
				name:     "missing-mixed-values-only-causing-set",
				local:    []byte{64, 6, 0, 120, 0, 1, 1, 0x80, 2, 0, 6, 0},
				peer:     []byte{6, 0},
				required: []capability.Code{capability.CodeASN4, capability.CodeGracefulRestart, capability.CodeRouteRefresh, capability.CodeExtendedMessage},
				want:     []byte{65, 4, 0, 0, 0xfd, 0xe9, 64, 6, 0, 120, 0, 1, 1, 0x80, 2, 0},
			},
			{
				name:    "refused-variable-values-and-repeated-instances",
				local:   []byte{64, 2, 0, 30, 2, 0},
				peer:    []byte{64, 6, 0, 90, 0, 1, 1, 0x80, 2, 0, 64, 6, 0, 120, 0, 2, 1, 0},
				refused: []capability.Code{capability.CodeGracefulRestart},
				want:    []byte{64, 6, 0, 90, 0, 1, 1, 0x80, 64, 6, 0, 120, 0, 2, 1, 0},
			},
			{
				name:     "missing-addpath-preserves-all-local-directions",
				local:    []byte{69, 8, 0, 1, 1, 2, 0, 2, 1, 1},
				required: []capability.Code{capability.CodeAddPath},
				want:     []byte{69, 8, 0, 1, 1, 2, 0, 2, 1, 1},
			},
			{
				name:    "refused-addpath-preserves-peer-directions",
				local:   []byte{69, 4, 0, 1, 1, 2},
				peer:    []byte{69, 8, 0, 1, 1, 1, 0, 2, 1, 3},
				refused: []capability.Code{capability.CodeAddPath},
				want:    []byte{69, 8, 0, 1, 1, 1, 0, 2, 1, 3},
			},
			{
				name:    "refused-hostname-retains-variable-value",
				peer:    []byte{73, 5, 1, 'r', 2, 'e', 'x'},
				refused: []capability.Code{capability.CodeFQDN},
				want:    []byte{73, 5, 1, 'r', 2, 'e', 'x'},
			},
		} {
			t.Run(rail+"/"+tc.name, func(t *testing.T) {
				caps, err := capability.Parse(tc.local)
				require.NoError(t, err)
				s, conn, sent := newCapabilityOpenSession(t, append([]capability.Capability{capIPv4()}, caps...), tc.required)
				s.settings.RefusedCapabilities = tc.refused
				body := validOpenBody()
				if len(tc.peer) > 0 {
					body = openBodyWithParameters(tc.peer)
				}
				if rail == "handleOpen" {
					err = s.handleOpen(body)
				} else {
					var open *message.Open
					open, err = message.UnpackOpen(body)
					require.NoError(t, err)
					err = s.processOpen(open)
				}
				require.ErrorIs(t, err, ErrInvalidState)
				require.Equal(t, fsm.StateIdle, s.State())
				requireUnsupportedCapabilityData(t, conn.written()[sent:], tc.want)
			})
		}
	}
}

// Goal: identify the rejected ADD-PATH family, not an unrelated Multiprotocol
// capability or the negotiated (direction-reversed) value.
// Method: drive both OPEN rails with local send/peer receive and a second family;
// compare the complete refusal Data with the causing side's family tuple.
// RFC 5492 Section 5: "The Data field in the NOTIFICATION message MUST list the
// set of capabilities that causes the speaker to send the message."
// MUTATION: using the Multiprotocol tuple builder emits code 1 instead of 69.
// RFC requirement: RFC5492-5-1 positive -- both OPEN paths preserve the causing ADD-PATH family's original direction and repeated capability grouping, including an exact 3771-byte NOTIFICATION from a legal extended OPEN.
// RFC requirement: RFC5492-5-1 negative -- exact NOTIFICATION Data excludes the noncausing family and cannot substitute Multiprotocol tuples, reverse directions or inflate repeated family entries into separate oversized capability tuples.
func TestUnsupportedAddPathNotificationPreservesFamilyTuples(t *testing.T) {
	for _, rail := range []string{"handleOpen", "processOpen"} {
		for _, refused := range []bool{false, true} {
			name := "required"
			if refused {
				name = "refused"
			}
			t.Run(rail+"/"+name, func(t *testing.T) {
				caps, err := capability.Parse([]byte{69, 8, 0, 1, 1, 2, 0, 2, 1, 3})
				require.NoError(t, err)
				s, conn, sent := newCapabilityOpenSession(t, append([]capability.Capability{capIPv4(), capIPv6()}, caps...), nil)
				body := openBodyWithParameters([]byte{1, 4, 0, 1, 0, 1, 1, 4, 0, 2, 0, 1})
				want := []byte{69, 4, 0, 1, 1, 2}
				if refused {
					s.settings.RefusedAddPathFamilies = []capability.Family{familyIPv4Unicast}
					body = openBodyWithParameters([]byte{1, 4, 0, 1, 0, 1, 1, 4, 0, 2, 0, 1, 69, 8, 0, 1, 1, 1, 0, 2, 1, 3})
					want = []byte{69, 4, 0, 1, 1, 1}
				} else {
					s.settings.RequiredAddPathFamilies = []capability.Family{familyIPv4Unicast}
				}
				if rail == "handleOpen" {
					err = s.handleOpen(body)
				} else {
					var open *message.Open
					open, err = message.UnpackOpen(body)
					require.NoError(t, err)
					err = s.processOpen(open)
				}
				require.ErrorIs(t, err, ErrInvalidState)
				require.Equal(t, fsm.StateIdle, s.State())
				requireUnsupportedCapabilityData(t, conn.written()[sent:], want)
			})
		}
		for _, refused := range []bool{false, true} {
			name := "required-repeated-family-grouping"
			mode := byte(2)
			if refused {
				name = "refused-repeated-family-grouping"
				mode = 1
			}
			t.Run(rail+"/"+name, func(t *testing.T) {
				// Fifteen legal 254-octet ADD-PATH capabilities fit in one OPEN.
				// Expanding each of their 63 family entries into a separate TLV
				// would inflate the rejection past the unnegotiated 4096 limit.
				var tuples []byte
				var selected []byte
				for range 15 {
					tuples = append(tuples, 69, 252)
					selected = append(selected, 69, 248)
					for range 62 {
						tuples = append(tuples, 0, 1, 1, mode)
						selected = append(selected, 0, 1, 1, mode)
					}
					tuples = append(tuples, 0, 2, 1, 3) // Noncausing family.
				}
				caps, err := capability.Parse(tuples)
				require.NoError(t, err)
				local := []capability.Capability{capIPv4()}
				peer := &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x05060708}
				if refused {
					local = append(local, &capability.AddPath{Families: []capability.AddPathFamily{
						{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathSend},
					}})
					// RFC 9072 extended parameter framing, length 3810 (0x0ee2).
					peer.OptionalParams = append([]byte{2, 0x0e, 0xe2}, tuples...)
					peer.ExtendedParams = true
				} else {
					local = append(local, caps...)
				}
				s, conn, sent := newCapabilityOpenSession(t, local, nil)
				if refused {
					s.settings.RefusedAddPathFamilies = []capability.Family{familyIPv4Unicast}
				} else {
					s.settings.RequiredAddPathFamilies = []capability.Family{familyIPv4Unicast}
				}
				wire := message.PackTo(peer, nil)
				require.LessOrEqual(t, len(wire), 4096, "legal OPEN, no Extended Message negotiation")
				require.NotPanics(t, func() {
					if rail == "handleOpen" {
						err = s.handleOpen(wire[message.HeaderLen:])
					} else {
						var open *message.Open
						open, err = message.UnpackOpen(wire[message.HeaderLen:])
						require.NoError(t, err)
						err = s.processOpen(open)
					}
				})
				require.ErrorIs(t, err, ErrInvalidState)
				require.Equal(t, fsm.StateIdle, s.State())
				// Exact complete NOTIFICATION: length 3771 (0x0ebb), type 3,
				// OPEN Message Error / Unsupported Capability, grouped causing TLVs.
				want := append([]byte{
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0x0e, 0xbb, 3, 2, 7,
				}, selected...)
				require.Equal(t, want, conn.written()[sent:])
			})
		}
	}
}
