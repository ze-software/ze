package bridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTranslateRTC checks ExaBGP 6.0's `announce ipv4 rtc` forms
// (grammar/tree/announce.py RTC_VALUES): the membership words reach ze as the
// `mp ipv4/rtc` section the rtc plugin encodes, and the next hop as `nhop`.
// Method: exact command text for the announce and withdraw of both forms.
func TestTranslateRTC(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{
			name: "membership",
			line: "neighbor 127.0.0.1 announce ipv4 rtc origin-as 65000 route-target 65000:1 next-hop 192.0.2.1",
			want: "send bgp 127.0.0.1 update text nhop 192.0.2.1 mp ipv4/rtc add origin-as 65000 route-target 65000:1",
		},
		{
			name: "default membership",
			line: "neighbor 127.0.0.1 announce ipv4 rtc default next-hop self",
			want: "send bgp 127.0.0.1 update text nhop self mp ipv4/rtc add default",
		},
		{
			name: "membership withdrawn",
			line: "neighbor 127.0.0.1 withdraw ipv4 rtc origin-as 65000 route-target 65000:1",
			want: "send bgp 127.0.0.1 update text mp ipv4/rtc del origin-as 65000 route-target 65000:1",
		},
		{
			name: "default withdrawn",
			line: "neighbor 127.0.0.1 withdraw ipv4 rtc default",
			want: "send bgp 127.0.0.1 update text mp ipv4/rtc del default",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			translation, err := TranslateLine(tc.line)
			require.NoError(t, err)
			assert.Equal(t, tc.want, onlyCommand(translation))
		})
	}
}

// TestTranslateMPLSVPNFamily checks ExaBGP 6.0's `announce ipv4 mpls-vpn`
// (CHANGELOG 6.0.0: "announce ipv4 mpls-vpn ... send the route as SAFI 128, as
// announce route ... rd ... label does"). Method: the family form and the
// family-less form of one route translate to the same command.
func TestTranslateMPLSVPNFamily(t *testing.T) {
	familyForm, err := TranslateLine("neighbor 127.0.0.1 announce ipv4 mpls-vpn 10.0.0.0/24 next-hop 1.2.3.4 rd 100:100 label 10")
	require.NoError(t, err)
	routeForm, err := TranslateLine("neighbor 127.0.0.1 announce route 10.0.0.0/24 next-hop 1.2.3.4 rd 100:100 label 10")
	require.NoError(t, err)
	assert.Contains(t, onlyCommand(familyForm), "mp ipv4/mpls-vpn add")
	assert.Equal(t, onlyCommand(routeForm), onlyCommand(familyForm))

	withdrawn, err := TranslateLine("neighbor 127.0.0.1 withdraw ipv4 mpls-vpn 10.0.0.0/24 rd 100:100 label 10")
	require.NoError(t, err)
	assert.Contains(t, onlyCommand(withdrawn), "mp ipv4/mpls-vpn del 10.0.0.0/24")
}
