// Design: docs/architecture/wire/nlri-evpn.md -- explicit EVPN route origination
// RFC: rfc/short/rfc9136.md

package evpn

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// IP Prefix route bytes shared by the overlay-index cases: route type 05,
// Length 22 (34), RD 65000:7 (type 0: 0000 FDE8 00000007).
const (
	overlayRouteHead = "0522" + "0000FDE800000007"
	// Ethernet Tag 0, prefix length 24, prefix 10.1.2.0.
	overlayRoutePrefix = "00000000" + "18" + "0A010200"
	overlayESI         = "00:01:02:03:04:05:06:07:08:09"
	overlayESIHex      = "00010203040506070809"
	overlayZeroESIHex  = "00000000000000000000"
)

// TestRFC9136OverlayIndexESINonZeroOnlyAsIndex encodes one IP Prefix route per
// overlay-index selector, through the NLRI encoder every Ze producer of the
// family reaches (route command, native config, update text).
//
// VALIDATES: RFC 9136 Section 3.1 -- "The Ethernet Segment Identifier MUST be
// a non-zero 10-octet identifier if the ESI is used as an Overlay Index (see
// the definition of "Overlay Index" in Section 3.2).  It MUST be all bytes zero
// otherwise." overlay-index esi sends the configured non-zero ESI at octets
// 8-17; overlay-index none, gateway-ip and an omitted keyword send ten zero
// octets there.
// PREVENTS: a selector that is parsed and then dropped, or an ESI written
// as zero when it is the Overlay Index.
func TestRFC9136OverlayIndexESINonZeroOnlyAsIndex(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		// RFC requirement: RFC9136-3.1-4 positive -- sent: EncodeNLRIHex type5 with esi 00:01:02:03:04:05:06:07:08:09 and overlay-index esi emits the IP Prefix route with ESI 00010203040506070809 and a zero GW IP: 0522 0000FDE800000007 00010203040506070809 00000000 18 0A010200 00000000 000001.
		{"type5 rd 65000:7 esi " + overlayESI + " prefix 10.1.2.0/24 overlay-index esi",
			overlayRouteHead + overlayESIHex + overlayRoutePrefix + "00000000" + "000001"},
		// RFC requirement: RFC9136-3.1-4 positive -- sent: EncodeNLRIHex type5 with gateway 10.1.2.1 and overlay-index gateway-ip emits an all-zero ESI and GW IP 0A010201: 0522 0000FDE800000007 00000000000000000000 00000000 18 0A010200 0A010201 000001.
		{"type5 rd 65000:7 prefix 10.1.2.0/24 gateway 10.1.2.1 overlay-index gateway-ip",
			overlayRouteHead + overlayZeroESIHex + overlayRoutePrefix + "0A010201" + "000001"},
		// RFC requirement: RFC9136-3.1-6 positive -- sent: EncodeNLRIHex type5 with overlay-index none and label 100 emits an all-zero GW IP (00000000) after prefix 0A010200, as it does for overlay-index esi.
		// RFC requirement: RFC9136-3.1-4 positive -- sent: EncodeNLRIHex type5 with overlay-index none and label 100 emits an all-zero ESI and GW IP: 0522 0000FDE800000007 00000000000000000000 00000000 18 0A010200 00000000 000641.
		{"type5 rd 65000:7 prefix 10.1.2.0/24 label 100 overlay-index none",
			overlayRouteHead + overlayZeroESIHex + overlayRoutePrefix + "00000000" + "000641"},
		// RFC requirement: RFC9136-3.1-4 positive -- sent: EncodeNLRIHex type5 with no overlay-index and label 100 emits an all-zero ESI and GW IP, the same bytes as overlay-index none.
		{"type5 rd 65000:7 prefix 10.1.2.0/24 label 100",
			overlayRouteHead + overlayZeroESIHex + overlayRoutePrefix + "00000000" + "000641"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			encoded, err := EncodeNLRIHex("l2vpn/evpn", strings.Fields(tc.command))
			require.NoError(t, err)
			assert.Equal(t, tc.want, encoded)
		})
	}
}

// TestRFC9136OverlayIndexESIRefusedOtherwise asks the NLRI encoder for every
// ESI the selected Overlay Index does not allow, and for the GW IP rules of the
// same section.
//
// The GW IP rule is RFC 9136 Section 3.1: "The GW IP field MUST be all bytes
// zero if it is not used as an Overlay Index."
// VALIDATES: RFC 9136 Section 3.1 ESI rule on the send side: a non-zero ESI is
// refused unless overlay-index esi is named, and overlay-index esi is refused
// with an all-zero ESI. Each refusal names the field, the reason and the fix.
// PREVENTS: a non-zero ESI leaving Ze while it is not the Overlay Index, which
// a receiver would read as one (RFC 9136 Section 3.2).
func TestRFC9136OverlayIndexESIRefusedOtherwise(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		// RFC requirement: RFC9136-3.1-4 negative -- sent: EncodeNLRIHex type5 with esi 00:01:02:03:04:05:06:07:08:09 and no overlay-index is refused with "ip-prefix esi 00:01:02:03:04:05:06:07:08:09 is non-zero" and nothing is encoded.
		{"type5 rd 65000:7 esi " + overlayESI + " prefix 10.1.2.0/24",
			"ip-prefix esi 00:01:02:03:04:05:06:07:08:09 is non-zero, which makes the ESI the route's Overlay Index (RFC 9136 Section 3.2), but no overlay-index was named; add overlay-index esi, or remove esi (RFC 9136 Section 3.1)"},
		// RFC requirement: RFC9136-3.1-4 negative -- sent: EncodeNLRIHex type5 with a non-zero esi and overlay-index none is refused with "overlay-index none needs an all-zero ESI".
		{"type5 rd 65000:7 esi " + overlayESI + " prefix 10.1.2.0/24 overlay-index none",
			"overlay-index none needs an all-zero ESI, but esi is 00:01:02:03:04:05:06:07:08:09 (RFC 9136 Section 3.1); remove esi, or set overlay-index esi"},
		// RFC requirement: RFC9136-3.1-4 negative -- sent: EncodeNLRIHex type5 with a non-zero esi, gateway 10.1.2.1 and overlay-index gateway-ip is refused with "overlay-index gateway-ip needs an all-zero ESI".
		{"type5 rd 65000:7 esi " + overlayESI + " prefix 10.1.2.0/24 gateway 10.1.2.1 overlay-index gateway-ip",
			"overlay-index gateway-ip needs an all-zero ESI, but esi is 00:01:02:03:04:05:06:07:08:09 (RFC 9136 Section 3.1); remove esi, or set overlay-index esi and remove gateway"},
		// RFC requirement: RFC9136-3.1-4 negative -- sent: EncodeNLRIHex type5 with overlay-index esi and no esi (all zero) is refused with "overlay-index esi needs a non-zero ESI".
		{"type5 rd 65000:7 prefix 10.1.2.0/24 overlay-index esi",
			"overlay-index esi needs a non-zero ESI, but esi is all zero (RFC 9136 Section 3.1); set esi to the Ethernet Segment the prefix resolves through, or remove overlay-index"},
		// RFC requirement: RFC9136-3.1-6 negative -- sent: EncodeNLRIHex type5 with gateway 10.1.2.1 and overlay-index esi is refused with "overlay-index esi needs an all-zero GW IP".
		{"type5 rd 65000:7 esi " + overlayESI + " prefix 10.1.2.0/24 gateway 10.1.2.1 overlay-index esi",
			"overlay-index esi needs an all-zero GW IP, but gateway is 10.1.2.1 (RFC 9136 Section 3.1)"},
		// RFC requirement: RFC9136-3.1-6 negative -- sent: EncodeNLRIHex type5 with gateway 10.1.2.1 and no overlay-index is refused with "ip-prefix gateway 10.1.2.1 is non-zero".
		{"type5 rd 65000:7 prefix 10.1.2.0/24 gateway 10.1.2.1",
			"ip-prefix gateway 10.1.2.1 is non-zero, which makes the GW IP the route's Overlay Index (RFC 9136 Section 3.2), but no overlay-index was named; add overlay-index gateway-ip, or remove gateway"},
		// RFC requirement: RFC9136-3.1-6 negative -- sent: EncodeNLRIHex type5 with gateway 10.1.2.1 and overlay-index none is refused with "overlay-index none needs an all-zero GW IP".
		{"type5 rd 65000:7 prefix 10.1.2.0/24 gateway 10.1.2.1 overlay-index none",
			"overlay-index none needs an all-zero GW IP, but gateway is 10.1.2.1"},
		{"type5 rd 65000:7 prefix 10.1.2.0/24 gateway 0.0.0.0 overlay-index gateway-ip",
			"overlay-index gateway-ip needs a non-zero GW IP, but gateway is absent or zero"},
		{"type5 rd 65000:7 prefix 10.1.2.0/24 overlay-index next-hop",
			`invalid overlay-index "next-hop": use none, esi or gateway-ip`},
		{"type2 rd 65000:7 mac 00:11:22:33:44:55 overlay-index none",
			"overlay-index applies only to a type5 (ip-prefix) route; remove it"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			encoded, err := EncodeNLRIHex("l2vpn/evpn", strings.Fields(tc.command))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.Empty(t, encoded)
		})
	}
}

// TestRFC9136OverlayIndexRouteCommandAndConfig drives the route-command and
// native-config producers, which parse their own keywords before the NLRI is
// built, so the selector must survive that parser too.
//
// VALIDATES: overlay-index esi reaches the UPDATE's NLRI with the ESI at
// octets 8-17 through EncodeRoute; the config parser refuses a non-zero ESI
// without the keyword and a selector on a non-IP-Prefix route.
// PREVENTS: the keyword working through one producer and not the other.
func TestRFC9136OverlayIndexRouteCommandAndConfig(t *testing.T) {
	route := "ip-prefix rd 65000:7 esi " + overlayESI + " prefix 10.1.2.0/24 next-hop 192.0.2.1"
	_, nlri, err := EncodeRoute(route+" overlay-index esi", "l2vpn/evpn", 65000, true, true, false)
	require.NoError(t, err)
	require.Len(t, nlri, 36)
	assert.Equal(t, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, nlri[10:20], "ESI at octets 8-17 after type and Length")

	_, _, err = EncodeRoute(route, "l2vpn/evpn", 65000, true, true, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "add overlay-index esi, or remove esi")

	parser := registry.ConfigRouteParserByFamily("l2vpn/evpn")
	require.NotNil(t, parser)
	_, err = parser(registry.ConfigRouteRequest{Content: strings.Fields(route)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "add overlay-index esi, or remove esi")
	routeMAC := "mac-ip rd 65000:7 mac 00:11:22:33:44:55 next-hop 192.0.2.1 overlay-index none"
	_, err = parser(registry.ConfigRouteRequest{Content: strings.Fields(routeMAC)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overlay-index applies only to an ip-prefix route; remove it")
}
