// Design: docs/architecture/api/update-syntax.md -- recognized raw UPDATE origination
// Related: session_aigp.go -- shared body/full-packet raw origination admission
// Related: rfc4271_forward_next_hop_test.go -- ordinary and relayed writer coverage

package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// TestRawOriginatedUpdateWithholdsPeerOwnNextHop observes the actual flushed
// UPDATEs from both raw APIs. The permitted twin changes only NEXT_HOP, so AS
// framing, export admission or a writer that refuses everything cannot pass.
// RFC requirement: RFC4271-5.1.3-1 positive -- recognized raw body and full-packet UPDATEs do not advertise their IPv4 NLRI using the recipient's own NEXT_HOP.
// RFC requirement: RFC4271-5.1.3-1 negative -- the same two raw API forms advertise the exact IPv4 prefix and literal third-party NEXT_HOP when only that address changes.
func TestRawOriginatedUpdateWithholdsPeerOwnNextHop(t *testing.T) {
	for _, fullPacket := range []bool{false, true} {
		name := "body"
		if fullPacket {
			name = "full-packet"
		}
		for _, forbidden := range []bool{true, false} {
			kind := "third-party"
			if forbidden {
				kind = "peer-own"
			}
			t.Run(name+"/"+kind, func(t *testing.T) {
				peer, conn := rawOriginatedNextHopPeer(t)
				nextHop := []byte{192, 0, 2, 9}
				if forbidden {
					nextHop = []byte{192, 0, 2, 2}
				}
				body := rawOriginatedNextHopBody(t, nil, nextHop, []byte{24, 203, 0, 113})
				rawOriginatedNextHopSend(t, peer, fullPacket, body)
				var announced []byte
				for _, output := range aigpSocketBodies(t, conn) {
					sections, err := wire.ParseUpdateSections(output)
					require.NoError(t, err)
					announced = append(announced, sections.NLRI(output)...)
					if len(sections.NLRI(output)) != 0 {
						_, _, actual, found := attribute.AttrFind(sections.Attrs(output), attribute.AttrNextHop)
						require.True(t, found, "a useful announcement carries NEXT_HOP")
						assert.Equal(t, []byte{192, 0, 2, 9}, actual, "the literal permitted address reaches the wire")
					}
				}
				if forbidden {
					assert.Empty(t, announced, "raw origination advertised the peer's own NEXT_HOP")
					return
				}
				assert.Equal(t, []byte{24, 203, 0, 113}, announced, "the permitted twin must actually announce the exact prefix")
			})
		}
	}
}

// TestRawOriginatedPeerOwnKeepsExplicitWithdrawals first advertises the route
// that a mixed raw UPDATE withdraws, then observes that exact owed withdrawal
// while its unrelated peer-own announcement is refused, through both APIs.
func TestRawOriginatedPeerOwnKeepsExplicitWithdrawals(t *testing.T) {
	for _, fullPacket := range []bool{false, true} {
		name := "body"
		if fullPacket {
			name = "full-packet"
		}
		t.Run(name, func(t *testing.T) {
			peer, conn := rawOriginatedNextHopPeer(t)
			owed := []byte{24, 198, 51, 100}
			body := rawOriginatedNextHopBody(t, nil, []byte{192, 0, 2, 9}, owed)
			rawOriginatedNextHopSend(t, peer, fullPacket, body)
			first := aigpSocketBodies(t, conn)
			require.Len(t, first, 1)
			sections, err := wire.ParseUpdateSections(first[0])
			require.NoError(t, err)
			require.Equal(t, owed, sections.NLRI(first[0]), "the withdrawal must be owed to this actual recipient")
			conn.reset()

			body = rawOriginatedNextHopBody(t, owed, []byte{192, 0, 2, 2}, []byte{24, 203, 0, 113})
			rawOriginatedNextHopSend(t, peer, fullPacket, body)
			var withdrawn []byte
			for _, output := range aigpSocketBodies(t, conn) {
				sections, err := wire.ParseUpdateSections(output)
				require.NoError(t, err)
				assert.Empty(t, sections.NLRI(output), "the peer-own announcement must not accompany the withdrawal")
				withdrawn = append(withdrawn, sections.Withdrawn(output)...)
			}
			assert.Contains(t, string(withdrawn), string(owed), "refusing one announcement must preserve the original withdrawal")
		})
	}
}

// TestRawOriginatedWithdrawalAndEORRemainUsable observes complete raw control
// bodies through both APIs; next-hop admission must leave non-announcements alone.
func TestRawOriginatedWithdrawalAndEORRemainUsable(t *testing.T) {
	for _, fullPacket := range []bool{false, true} {
		for _, body := range [][]byte{{0, 4, 24, 198, 51, 100, 0, 0}, {0, 0, 0, 0}} {
			peer, conn := rawOriginatedNextHopPeer(t)
			rawOriginatedNextHopSend(t, peer, fullPacket, body)
			assert.Equal(t, [][]byte{body}, aigpSocketBodies(t, conn), "raw withdrawal/EOR control must reach the wire unchanged")
		}
	}
}

// TestRawOriginatedPeerOwnPreservesHealthyFamily observes both raw carriers
// with different actual IPv4 and IPv6 recipients. Each body withdraws routes
// already advertised to that recipient and announces two different families;
// only the family whose active next hop is the peer's address is refused.
func TestRawOriginatedPeerOwnPreservesHealthyFamily(t *testing.T) {
	cases := []struct {
		name          string
		peerAddress   string
		localAddress  string
		legacyRefused bool
	}{
		{"legacy refused", "192.0.2.2", "192.0.2.1", true},
		{"MP refused", "2001:db8::2", "2001:db8::1", false},
	}
	for _, fullPacket := range []bool{false, true} {
		carrier := "body"
		if fullPacket {
			carrier = "full-packet"
		}
		for _, tc := range cases {
			t.Run(carrier+"/"+tc.name, func(t *testing.T) {
				peer, conn := rawOriginatedMixedNextHopPeer(t, tc.peerAddress, tc.localAddress)
				legacyHop := []byte{192, 0, 2, 9}
				mpHop := netip.MustParseAddr("2001:db8::9").AsSlice()
				owedLegacy := []byte{24, 198, 51, 100}
				owedMP := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 0x10}
				attrs := rawOriginatedMixedNextHopAttrs(legacyHop, mpHop, nil, owedMP)
				rawOriginatedNextHopSend(t, peer, fullPacket, makeUpdateBody(nil, attrs, owedLegacy))
				before := rawOriginatedMixedNextHopWire(t, conn)
				require.Equal(t, owedLegacy, before.legacyAnnounced, "the original IPv4 withdrawal must be owed")
				require.Equal(t, owedMP, before.mpAnnounced, "the original IPv6 withdrawal must be owed")
				require.Equal(t, legacyHop, before.legacyHop)
				require.Equal(t, mpHop, before.mpHop)
				conn.reset()

				legacyAnnounced := []byte{24, 203, 0, 113}
				mpAnnounced := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 0x20}
				wantLegacy, wantMP := legacyAnnounced, mpAnnounced
				if tc.legacyRefused {
					legacyHop = []byte{192, 0, 2, 2}
					wantLegacy = nil
				} else {
					mpHop = netip.MustParseAddr("2001:db8::2").AsSlice()
					wantMP = nil
				}
				attrs = rawOriginatedMixedNextHopAttrs(legacyHop, mpHop, owedMP, mpAnnounced)
				body := makeUpdateBody(owedLegacy, attrs, legacyAnnounced)
				rawOriginatedNextHopSend(t, peer, fullPacket, body)
				after := rawOriginatedMixedNextHopWire(t, conn)
				assert.Equal(t, owedLegacy, after.legacyWithdrawn, "preserve the exact original IPv4 withdrawal")
				assert.Equal(t, owedMP, after.mpWithdrawn, "preserve the exact original IPv6 withdrawal")
				assert.Equal(t, wantLegacy, after.legacyAnnounced, "only the permitted IPv4 route may be advertised")
				assert.Equal(t, wantMP, after.mpAnnounced, "only the permitted IPv6 route may be advertised")
				if tc.legacyRefused {
					assert.Equal(t, netip.MustParseAddr("2001:db8::9").AsSlice(), after.mpHop,
						"the healthy MP family retains its literal global NEXT_HOP")
				} else {
					assert.Equal(t, []byte{192, 0, 2, 9}, after.legacyHop,
						"the healthy legacy family retains its literal NEXT_HOP")
				}
			})
		}
	}
}

// TestRawOriginatedGenericWriterPreservesBorrowedFamilies uses writeMessage's
// own encoding into s.writeBuf, so final partition callbacks overwrite the
// original input unless the origin writer retains it. Both recipient families
// must keep the exact owed withdrawals and healthy sibling after that overwrite.
func TestRawOriginatedGenericWriterPreservesBorrowedFamilies(t *testing.T) {
	cases := []struct {
		name          string
		peerAddress   string
		localAddress  string
		legacyRefused bool
	}{
		{"legacy refused", "192.0.2.2", "192.0.2.1", true},
		{"MP refused", "2001:db8::2", "2001:db8::1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer, conn := rawOriginatedMixedNextHopPeer(t, tc.peerAddress, tc.localAddress)
			legacyHop := []byte{192, 0, 2, 9}
			mpHop := netip.MustParseAddr("2001:db8::9").AsSlice()
			owedLegacy := []byte{24, 198, 51, 100}
			owedMP := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 0x10}
			attrs := rawOriginatedMixedNextHopAttrs(legacyHop, mpHop, nil, owedMP)
			rawOriginatedGenericWrite(t, peer, conn, makeUpdateBody(nil, attrs, owedLegacy))
			before := rawOriginatedMixedNextHopWire(t, conn)
			require.Equal(t, owedLegacy, before.legacyAnnounced, "this connection must hold the IPv4 route before withdrawal")
			require.Equal(t, owedMP, before.mpAnnounced, "this connection must hold the IPv6 route before withdrawal")
			require.Equal(t, legacyHop, before.legacyHop)
			require.Equal(t, mpHop, before.mpHop)
			conn.reset()

			legacyAnnounced := []byte{24, 203, 0, 113}
			mpAnnounced := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 0x20}
			wantLegacy, wantMP := legacyAnnounced, mpAnnounced
			if tc.legacyRefused {
				legacyHop = []byte{192, 0, 2, 2}
				wantLegacy = nil
			} else {
				mpHop = netip.MustParseAddr("2001:db8::2").AsSlice()
				wantMP = nil
			}
			attrs = rawOriginatedMixedNextHopAttrs(legacyHop, mpHop, owedMP, mpAnnounced)
			body := makeUpdateBody(owedLegacy, attrs, legacyAnnounced)
			rawOriginatedGenericWrite(t, peer, conn, body)
			after := rawOriginatedMixedNextHopWire(t, conn)
			assert.Equal(t, owedLegacy, after.legacyWithdrawn, "callback output must not corrupt the original IPv4 withdrawal")
			assert.Equal(t, owedMP, after.mpWithdrawn, "callback output must not corrupt the original IPv6 withdrawal")
			assert.Equal(t, wantLegacy, after.legacyAnnounced, "retain only the permitted IPv4 prefix from the borrowed input")
			assert.Equal(t, wantMP, after.mpAnnounced, "retain only the permitted IPv6 prefix from the borrowed input")
			if tc.legacyRefused {
				assert.Equal(t, netip.MustParseAddr("2001:db8::9").AsSlice(), after.mpHop)
			} else {
				assert.Equal(t, []byte{192, 0, 2, 9}, after.legacyHop)
			}
		})
	}
}

// rawOriginatedGenericWrite enters the same writer used by session messages;
// writeMessageHeld encodes this UPDATE into its own output buffer before
// invoking writeOriginatedRawUpdate with raw=false. RFC 4271 Section 4.3.
func rawOriginatedGenericWrite(t *testing.T, peer *Peer, conn *recordingConn, body []byte) {
	t.Helper()
	sections, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	update := message.Update{
		WithdrawnRoutes: sections.Withdrawn(body),
		PathAttributes:  sections.Attrs(body),
		NLRI:            sections.NLRI(body),
	}
	require.NoError(t, peer.session.writeMessage(conn, &update))
}

// rawOriginatedMixedNextHopPeer negotiates both NLRI families with the actual
// recipient identity, without changing the existing single-family fixture.
func rawOriginatedMixedNextHopPeer(t *testing.T, peerAddress, localAddress string) (*Peer, *recordingConn) {
	t.Helper()
	peer, conn := newAnnouncePeer(t, peerAddress)
	peer.settings.LocalAddress = netip.MustParseAddr(localAddress)
	local := []capability.Capability{&capability.ASN4{ASN: 65000},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}}
	remote := []capability.Capability{&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}}
	negotiated := capability.Negotiate(local, remote, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001})
	peer.session.negotiated = negotiated
	peer.negotiated.Store(NewNegotiatedCapabilities(negotiated))
	peer.setEncodingContexts(negotiated)
	peer.session.egressFinalAccepts = func(filterapi.ExportKind) bool { return true }
	return peer, conn
}

// rawOriginatedMixedNextHopAttrs retains local AS_SEQUENCE [65000], ORIGIN IGP
// and both distinct next-hop fields. RFC 4760 Sections 3 and 4: each MP value
// begins with AFI(2), SAFI(1); MP_REACH then carries NH-length(1), NH, Reserved(1), NLRI.
func rawOriginatedMixedNextHopAttrs(legacyHop, mpHop, mpWithdrawn, mpAnnounced []byte) []byte {
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe8, 0x40, 3, 4}
	attrs = append(attrs, legacyHop...)
	if len(mpWithdrawn) != 0 {
		attrs = append(attrs, mixedAttr(15, mixedUnreachValue(2, 1, mpWithdrawn))...)
	}
	return append(attrs, mixedReach(1, mpHop, mpAnnounced)...)
}

// rawOriginatedMixedNextHopOutput aggregates complete flushed UPDATE sections;
// it cannot infer an advertisement from an ownership-table entry.
type rawOriginatedMixedNextHopOutput struct {
	legacyWithdrawn []byte
	legacyAnnounced []byte
	mpWithdrawn     []byte
	mpAnnounced     []byte
	legacyHop       []byte
	mpHop           []byte
}

func rawOriginatedMixedNextHopWire(t *testing.T, conn *recordingConn) rawOriginatedMixedNextHopOutput {
	t.Helper()
	var result rawOriginatedMixedNextHopOutput
	for _, body := range aigpSocketBodies(t, conn) {
		sections, err := wire.ParseUpdateSections(body)
		require.NoError(t, err)
		result.legacyWithdrawn = append(result.legacyWithdrawn, sections.Withdrawn(body)...)
		result.legacyAnnounced = append(result.legacyAnnounced, sections.NLRI(body)...)
		attrs := sections.Attrs(body)
		if len(sections.NLRI(body)) != 0 {
			_, _, value, found := attribute.AttrFind(attrs, attribute.AttrNextHop)
			require.True(t, found)
			result.legacyHop = append(result.legacyHop, value...)
		}
		if _, _, value, found := attribute.AttrFind(attrs, attribute.AttrMPUnreachNLRI); found {
			require.GreaterOrEqual(t, len(value), 3)
			require.Equal(t, []byte{0, 2, 1}, value[:3])
			result.mpWithdrawn = append(result.mpWithdrawn, value[3:]...)
		}
		if _, _, value, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI); found {
			require.GreaterOrEqual(t, len(value), 5)
			require.Equal(t, []byte{0, 2, 1, 16}, value[:4])
			require.GreaterOrEqual(t, len(value), 21)
			result.mpHop = append(result.mpHop, value[4:20]...)
			result.mpAnnounced = append(result.mpAnnounced, value[21:]...)
		}
	}
	return result
}

// rawOriginatedNextHopPeer uses a real negotiated AS identity and four-octet
// AS_PATH width. recordingConn makes SendRawMessage's flush a synchronous fence;
// no worker, sleep, concurrent assertion or transport timeout is needed.
func rawOriginatedNextHopPeer(t *testing.T) (*Peer, *recordingConn) {
	t.Helper()
	peer, conn := newAnnouncePeer(t, "192.0.2.2")
	peer.settings.LocalAddress = netip.MustParseAddr("192.0.2.1")
	local := []capability.Capability{&capability.ASN4{ASN: 65000},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}}
	remote := []capability.Capability{&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}}
	negotiated := capability.Negotiate(local, remote, capability.PeerIdentity{
		LocalASN: 65000, PeerASN: 65001,
	})
	peer.session.negotiated = negotiated
	peer.negotiated.Store(NewNegotiatedCapabilities(negotiated))
	peer.setEncodingContexts(negotiated)
	peer.session.egressFinalAccepts = func(filterapi.ExportKind) bool { return true }
	return peer, conn
}

// rawOriginatedNextHopBody supplies a valid classic IPv4 UPDATE, with local
// AS_SEQUENCE [65000] at the negotiated four-octet width and ORIGIN IGP.
// RFC 4271 Section 4.3: body offsets are Withdrawn-Length(0:2), Withdrawn,
// Attribute-Length(2+withdrawn:4+withdrawn), attributes, then IPv4 NLRI.
func rawOriginatedNextHopBody(t *testing.T, withdrawn, nextHop, announced []byte) []byte {
	t.Helper()
	require.Len(t, nextHop, 4)
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe8, 0x40, 3, 4}
	attrs = append(attrs, nextHop...)
	return makeUpdateBody(withdrawn, attrs, announced)
}

// rawOriginatedNextHopSend exercises the public Peer raw entry point and both
// Session dispatch branches. RFC 4271 Section 4.1: full packet offsets are
// Marker(0:16), Length(16:18), Type(18:19), then the UPDATE body.
func rawOriginatedNextHopSend(t *testing.T, peer *Peer, fullPacket bool, body []byte) {
	t.Helper()
	if !fullPacket {
		require.NoError(t, peer.SendRawMessage(uint8(msgtype.TypeUPDATE), body))
		return
	}
	packet := make([]byte, message.HeaderLen+len(body))
	copy(packet, message.Marker[:])
	binary.BigEndian.PutUint16(packet[16:18], uint16(len(packet)))
	packet[18] = byte(msgtype.TypeUPDATE)
	copy(packet[message.HeaderLen:], body)
	require.NoError(t, peer.SendRawMessage(0, packet))
}
