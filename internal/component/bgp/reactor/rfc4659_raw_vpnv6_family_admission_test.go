// Design: docs/architecture/api/update-syntax.md -- Raw Passthrough Commands family admission
// RFC: rfc/short/rfc4659.md -- Section 3.4 (labeled IPv6 VPN NLRI requires negotiated AFI 2 / SAFI 128)
// RFC naming: untagged -- AC-27 raw UPDATE VPNv6 family admission regression; RFC4659-3.4-1 whole-row proof stays on the tagged managed units and interop scenario.
// Related: session_write.go, session_ownership.go, rfc4659_vpnv6_negotiated_send_test.go

package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
)

func rawVPNv6TestUpdateBody(t *testing.T) []byte {
	t.Helper()
	// ORIGIN=IGP, AS_PATH={65000} (4-byte), MP_REACH_NLRI(AFI=2, SAFI=128, NH=RD0+2001:db8::1, NLRI=2001:db8:1::/48)
	mpReach, err := hex.DecodeString("800e2f" + rfc4659VPNv6MPReach)
	require.NoError(t, err)
	attrs := append(firstASAttrs(4, 65000), mpReach...)
	return makeUpdateBody(nil, attrs, nil)
}

func rawIPv4ControlBody() []byte {
	return makeUpdateBody(nil, firstASAttrs(4, 65000), ownershipLiveNLRI([]netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}))
}

func rawFullUpdatePacket(body []byte) []byte {
	total := message.HeaderLen + len(body)
	pkt := make([]byte, total)
	copy(pkt[:message.MarkerLen], message.Marker[:])
	pkt[16] = byte(total >> 8)
	pkt[17] = byte(total)
	pkt[18] = byte(msgtype.TypeUPDATE)
	copy(pkt[message.HeaderLen:], body)
	return pkt
}

// TestRawVPNv6RequiresNegotiatedFamily verifies AC-27: explicit raw UPDATE
// writes (both message-body and full-packet forms via Session.SendRawMessage)
// emit labeled VPNv6 NLRI only when AFI 2 / SAFI 128 was negotiated, withhold
// unnegotiated VPNv6 on an IPv4-only session, and keep ordinary IPv4 output
// working on that same session.
//
// RFC 4659 Section 3.4: "In order for two PEs to exchange labeled IPv6 VPN
// NLRIs, they MUST use BGP Capabilities Negotiation to ensure that they both
// are capable of properly processing such NLRIs."
func TestRawVPNv6RequiresNegotiatedFamily(t *testing.T) {
	vpnv6Body := rawVPNv6TestUpdateBody(t)
	ipv4Body := rawIPv4ControlBody()

	for _, fullPacket := range []bool{false, true} {
		mode := "body"
		if fullPacket {
			mode = "full-packet"
		}
		sendRaw := func(t *testing.T, s *Session, body []byte) {
			t.Helper()
			if fullPacket {
				require.NoError(t, s.SendRawMessage(0, rawFullUpdatePacket(body)))
				return
			}
			require.NoError(t, s.SendRawMessage(uint8(msgtype.TypeUPDATE), body))
		}

		t.Run(mode+"/negotiated-vpnv6-emits", func(t *testing.T) {
			peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast, rfc4659VPNv6Family)
			negotiateForwardTestFamilies(peer, family.IPv4Unicast, rfc4659VPNv6Family)

			sendRaw(t, peer.session, vpnv6Body)

			frames := rfc4659UpdatesNamingVPNv6(t, conn.written())
			require.Len(t, frames, 1, "negotiated VPNv6 raw UPDATE must reach the wire")
			assert.Contains(t, frames[0], "800e2f"+rfc4659VPNv6MPReach)
		})

		t.Run(mode+"/ipv4-only-withholds-vpnv6-and-keeps-ipv4", func(t *testing.T) {
			peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
			negotiateForwardTestFamilies(peer, family.IPv4Unicast)

			sendRaw(t, peer.session, vpnv6Body)
			assert.Empty(t, rfc4659UpdatesNamingVPNv6(t, conn.written()),
				"raw VPNv6 UPDATE must not be emitted on an IPv4-only session")

			sendRaw(t, peer.session, ipv4Body)
			frames := bgpFrames(t, conn.written())
			require.Len(t, frames, 1, "ordinary IPv4 raw UPDATE must still succeed on the same session")
			assert.Equal(t, byte(msgtype.TypeUPDATE), frames[0][18])
			assert.Empty(t, rfc4659UpdatesNamingVPNv6(t, conn.written()))
		})

		t.Run(mode+"/mixed-raw-strips-unnegotiated-vpnv6-keeps-ipv4", func(t *testing.T) {
			peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
			negotiateForwardTestFamilies(peer, family.IPv4Unicast)

			mpReach, err := hex.DecodeString("800e2f" + rfc4659VPNv6MPReach)
			require.NoError(t, err)
			mixedAttrs := append(firstASAttrs(4, 65000), mpReach...)
			mixedBody := makeUpdateBody(nil, mixedAttrs, ownershipLiveNLRI([]netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}))

			sendRaw(t, peer.session, mixedBody)
			assert.Empty(t, rfc4659UpdatesNamingVPNv6(t, conn.written()),
				"unnegotiated VPNv6 MP_REACH_NLRI must be stripped from a mixed raw UPDATE")
			frames := bgpFrames(t, conn.written())
			require.Len(t, frames, 1, "negotiated IPv4 NLRI in the same mixed raw UPDATE must still be emitted")
			assert.Contains(t, hex.EncodeToString(frames[0]), hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")})))
		})
	}
}

// TestGenericVPNv6FamilyAdmissionControl confirms the non-raw writeMessage
// control path applies the same family admission gate for the exact same route
// and capability sets.
func TestGenericVPNv6FamilyAdmissionControl(t *testing.T) {
	mpReach, err := hex.DecodeString("800e2f" + rfc4659VPNv6MPReach)
	require.NoError(t, err)
	vpnv6Update := &message.Update{
		PathAttributes: append(firstASAttrs(4, 65000), mpReach...),
	}
	ipv4Update := &message.Update{
		PathAttributes: firstASAttrs(4, 65000),
		NLRI:           ownershipLiveNLRI([]netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}),
	}

	t.Run("negotiated-vpnv6-emits", func(t *testing.T) {
		peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast, rfc4659VPNv6Family)
		negotiateForwardTestFamilies(peer, family.IPv4Unicast, rfc4659VPNv6Family)

		require.NoError(t, peer.session.writeMessage(conn, vpnv6Update))
		frames := rfc4659UpdatesNamingVPNv6(t, conn.written())
		require.Len(t, frames, 1)
		assert.Contains(t, frames[0], "800e2f"+rfc4659VPNv6MPReach)
	})

	t.Run("ipv4-only-withholds-vpnv6-and-keeps-ipv4", func(t *testing.T) {
		peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
		negotiateForwardTestFamilies(peer, family.IPv4Unicast)

		require.NoError(t, peer.session.writeMessage(conn, vpnv6Update))
		assert.Empty(t, rfc4659UpdatesNamingVPNv6(t, conn.written()))

		require.NoError(t, peer.session.writeMessage(conn, ipv4Update))
		frames := bgpFrames(t, conn.written())
		require.Len(t, frames, 1)
		assert.Empty(t, rfc4659UpdatesNamingVPNv6(t, conn.written()))
	})
}
