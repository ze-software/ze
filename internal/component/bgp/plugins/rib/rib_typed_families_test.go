// RFC: rfc/short/rfc7606.md -- Section 5.4 typed NLRI
// Related: rib_test.go -- TestHandleReceived_PoolStorage_StoresEVPN, the same shape for EVPN
//
// RFC 7606 Section 5.4 needs each NLRI in a typed family carved out before its route type
// can be judged, and that carve is nlrisplit. Registering the MCAST-VPN and BGP-MUP
// splitters for it also flipped nlrisplit.Supported for those families, and that predicate
// is what gates RIB installation (rib.go insertPoolNLRIs, rib_structured.go
// handleReceivedStructured). So the two families started reaching the RIB, where they
// previously reached nothing at all.
//
// That is a behavior change beyond the discard, and it is the kind that ships unnoticed
// because no test asserted the OLD behavior either. These tests state the new one.

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: an MCAST-VPN and a BGP-MUP NLRI now reach the opaque-map FamilyRIB through the
// nlrisplit registry, exactly as an EVPN one does.
// PREVENTS: the Section 5.4 splitter registration changing RIB behavior with nothing
// asserting the result. It also pins the framing: the hex below is carved by the family's
// own splitter, so a splitter reading the length octet at the wrong offset stores the wrong
// number of routes rather than failing loudly.
func TestHandleReceived_PoolStorage_StoresTypedFamilies(t *testing.T) {
	cases := []struct {
		name string
		fam  family.Family
		// One NLRI, then a second, so a splitter that mis-frames stores 1 or 0 rather than 2.
		rawNLRI string
		want    int
	}{
		{
			// RFC 6514 Section 4: [route-type:1][length:1][body]. Types 1 and 5, 3-octet bodies.
			name:    "mcast-vpn",
			fam:     family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMVPN},
			rawNLRI: "0103deadbe" + "0503c0ffee",
			want:    2,
		},
		{
			// draft-ietf-bess-mup-safi Section 3.1: [arch:1][route-type:2][length:1][body].
			// Architecture 1, route types 1 and 4, 3-octet bodies.
			name:    "bgp-mup",
			fam:     family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMUP},
			rawNLRI: "01000103deadbe" + "01000403c0ffee",
			want:    2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRIBManager(t)
			peerJSON := mustMarshal(t, map[string]any{
				"local":  map[string]any{"address": "10.0.0.2", "as": uint32(65002)},
				"remote": map[string]any{"address": "10.0.0.1", "as": uint32(65001)},
			})

			event := &Event{
				Message:       &MessageInfo{Type: rpc.EventKindUpdate, ID: 411},
				Peer:          peerJSON,
				RawAttributes: "40010100",
				RawNLRI:       map[family.Family]string{tc.fam: tc.rawNLRI},
				FamilyOps: map[family.Family][]FamilyOperation{
					tc.fam: {{Action: routeaction.Add, NLRIs: []any{"route-a", "route-b"}}},
				},
			}

			r.handleReceived(event)

			peerRIB := r.bgpPeers[netip.MustParseAddr("10.0.0.1")]
			require.NotNil(t, peerRIB, "a PeerRIB must be created for %s", tc.fam)
			assert.Equal(t, tc.want, peerRIB.Len(),
				"%s: both NLRIs must be carved and stored", tc.fam)
		})
	}
}
