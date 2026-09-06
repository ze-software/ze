// Design: docs/architecture/update-building.md -- what a withdrawal carries
// Overview: reactor_api_batch.go -- buildBatchWithdrawUpdate and planBatchAttrs
// Related: reactor_batch_test.go -- the announce rail's own build tests

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// vpnWithdrawNLRI is one opaque MPLS-VPN NLRI. MP_UNREACH_NLRI copies it
// verbatim, so its bytes only have to be recognizable in the assertion.
var vpnWithdrawNLRI = []byte{0x70, 0x00, 0x01, 0x81, 0x00, 0x00, 0x00, 0x64, 0x00, 0x00, 0x00, 0x64, 0x01, 0x04, 0x00}

// targetTenTenExtCommunity is `extended-community [target:10:10]` on the wire:
// optional transitive (0xC0), type code 16, eight octets of value.
var targetTenTenExtCommunity = []byte{0xC0, 0x10, 0x08, 0x00, 0x02, 0x00, 0x0A, 0x00, 0x00, 0x00, 0x0A}

// withdrawBatchFor builds the batch a `send bgp <sel> update text <attrs> nlri
// <family> del <nlri>` command reaches the reactor with.
func withdrawBatchFor(t *testing.T, fam family.Family, raw, wire []byte, nextHop string) bgptypes.NLRIBatch {
	t.Helper()
	route, err := nlri.NewWireNLRI(fam, raw, false)
	require.NoError(t, err)
	batch := bgptypes.NLRIBatch{Family: fam, NLRIs: []nlri.NLRI{route}}
	if wire != nil {
		batch.Wire = attribute.NewAttributesWire(wire, 0)
	}
	if nextHop != "" {
		batch.NextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr(nextHop))
	}
	return batch
}

// TestWithdrawCarriesTheAttributesTheCallerNamed drives buildBatchWithdrawUpdate
// with the batch `send bgp * update text extended-community [target:10:10]
// next-hop 10.10.6.3 nlri ipv4/mpls-vpn del <route>` produces, and asserts the
// whole Path Attributes field byte for byte.
//
// VALIDATES: an operator's attribute block reaches the wire on a withdrawal, and
// so do the RFC 4271 Section 4.3 well-known mandatory attributes beside it.
// PREVENTS: the silent discard this rail performed until 2026-09-06, where the
// command was acknowledged and the peer received a bare MP_UNREACH_NLRI, which a
// caller cannot tell from having asked for no attributes.
func TestWithdrawCarriesTheAttributesTheCallerNamed(t *testing.T) {
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	batch := withdrawBatchFor(t, fam, vpnWithdrawNLRI, targetTenTenExtCommunity, "10.10.6.3")

	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update := adapter.buildBatchWithdrawUpdate(attrBuf, nlriBuf, batch,
		announceFacts{isIBGP: true, asn4: true, nextHop: netip.MustParseAddr("10.10.6.3")})
	require.NotNil(t, update)

	// Ascending type-code order (RFC 4271 Section 5), which announceAttrs.emit
	// owns: ORIGIN, an AS_PATH empty toward an internal peer, NEXT_HOP 10.10.6.3,
	// LOCAL_PREF 100, MP_UNREACH_NLRI over AFI 1 SAFI 128, and the caller's own
	// extended community last.
	want := []byte{0x40, 0x01, 0x01, 0x00}
	want = append(want,
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0x0A, 0x0A, 0x06, 0x03,
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64,
		0x80, 0x0F, byte(3+len(vpnWithdrawNLRI)), 0x00, 0x01, 0x80)
	want = append(want, vpnWithdrawNLRI...)
	want = append(want, targetTenTenExtCommunity...)

	assert.Equal(t, want, update.PathAttributes)
	assert.Empty(t, update.WithdrawnRoutes, "a non-unicast withdrawal names its routes inside MP_UNREACH_NLRI")
	assert.Empty(t, update.NLRI, "a withdrawal advertises no route")
}

// TestWithdrawOfAUnicastPrefixCarriesNoAttributes drives the same builder with an
// IPv6 unicast batch that names the same attributes, and asserts the bare
// MP_UNREACH_NLRI RFC 4760 Section 4 permits.
//
// VALIDATES: unicast is the seam. RFC 4271 Section 4.3 gives the Withdrawn Routes
// field no attributes, and the IPv6 unicast withdrawal is that withdrawal in the
// RFC 4760 encoding, so the AFI cannot decide what the command means.
// PREVENTS: an attribute block on the withdrawal api-ipv6.ci pins as bare.
func TestWithdrawOfAUnicastPrefixCarriesNoAttributes(t *testing.T) {
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
	raw := []byte{0x40, 0xFC, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00} // fc00:1::/64
	batch := withdrawBatchFor(t, family.IPv6Unicast, raw, targetTenTenExtCommunity, "2001::11")

	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update := adapter.buildBatchWithdrawUpdate(attrBuf, nlriBuf, batch,
		announceFacts{isIBGP: true, asn4: true})
	require.NotNil(t, update)

	want := []byte{0x80, 0x0F, byte(3 + len(raw)), 0x00, 0x02, 0x01}
	want = append(want, raw...)
	assert.Equal(t, want, update.PathAttributes)
}

// TestWithdrawOfAnIPv4UnicastPrefixCarriesNoAttributes is the same assertion on
// the other half of the seam: the Withdrawn Routes field of RFC 4271 Section 4.3.
//
// VALIDATES: naming attributes on an IPv4 unicast withdrawal changes nothing.
// PREVENTS: the two unicast AFIs disagreeing about one command, which is what the
// rule in buildBatchWithdrawUpdate exists to stop.
func TestWithdrawOfAnIPv4UnicastPrefixCarriesNoAttributes(t *testing.T) {
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
	raw := []byte{0x18, 0x0A, 0x00, 0x01} // 10.0.1.0/24
	batch := withdrawBatchFor(t, family.IPv4Unicast, raw, targetTenTenExtCommunity, "10.0.1.254")

	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update := adapter.buildBatchWithdrawUpdate(attrBuf, nlriBuf, batch,
		announceFacts{isIBGP: true, asn4: true, nextHop: netip.MustParseAddr("10.0.1.254")})
	require.NotNil(t, update)

	assert.Equal(t, raw, update.WithdrawnRoutes)
	assert.Empty(t, update.PathAttributes)
}

// TestWithdrawSkipsAnUnspecifiedNextHop drives the builder with the
// `next-hop 0.0.0.0` an operator writes to say a withdrawal names no next hop.
//
// VALIDATES: RFC 4271 Section 6.3 -- "Syntactic correctness means that the
// NEXT_HOP attribute represents a valid IP host address" -- so 0.0.0.0 is not
// written as one.
// PREVENTS: an Invalid NEXT_HOP Attribute NOTIFICATION, which RFC 4271 Section
// 6.3 makes the receiver's answer and which resets the session.
func TestWithdrawSkipsAnUnspecifiedNextHop(t *testing.T) {
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMUP}
	batch := withdrawBatchFor(t, fam, vpnWithdrawNLRI, targetTenTenExtCommunity, "0.0.0.0")

	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update := adapter.buildBatchWithdrawUpdate(attrBuf, nlriBuf, batch,
		announceFacts{isIBGP: true, asn4: true, nextHop: netip.MustParseAddr("0.0.0.0")})
	require.NotNil(t, update)

	_, _, _, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
	assert.False(t, found, "0.0.0.0 is not a valid IP host address, so it is not written as a NEXT_HOP")
	_, _, _, found = attribute.AttrFind(update.PathAttributes, attribute.AttrOrigin)
	assert.True(t, found, "the withdrawal still carries the well-known mandatory ORIGIN")
}

// TestAnnounceRestatesAnIPv4NextHopWhereTheFamilyCarriesOne drives the announce
// rail for the two answers family.Family.LegacyNextHop gives.
//
// VALIDATES: the API rail and the config rail put the same attributes on the wire
// for one route. message.(*UpdateBuilder).BuildVPN adds NEXT_HOP beside
// MP_REACH_NLRI for an IPv4 next hop and 42 `conf-*` fixtures pin it; this rail
// sent MP_REACH_NLRI alone until 2026-09-06.
// PREVENTS: the same route reaching the wire as two different byte strings
// depending on whether an operator configured it or announced it.
func TestAnnounceRestatesAnIPv4NextHopWhereTheFamilyCarriesOne(t *testing.T) {
	cases := []struct {
		name string
		fam  family.Family
		want bool
	}{
		{"mpls-vpn carries it", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}, true},
		{"mcast-vpn carries it", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMVPN}, true},
		{"flowspec does not", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, false},
		{"vpls does not", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIVPLS}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
			batch := withdrawBatchFor(t, tc.fam, vpnWithdrawNLRI, targetTenTenExtCommunity, "10.10.6.3")

			attrBuf := make([]byte, message.MaxMsgLen)
			nlriBuf := make([]byte, message.MaxMsgLen)
			update, err := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch,
				announceFacts{isIBGP: true, asn4: true, nextHop: netip.MustParseAddr("10.10.6.3")})
			require.NoError(t, err)
			require.NotNil(t, update)

			_, _, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
			assert.Equal(t, tc.want, found)
			if tc.want {
				assert.Equal(t, []byte{0x0A, 0x0A, 0x06, 0x03}, value)
			}
		})
	}
}
