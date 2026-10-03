// Design: docs/architecture/wire/nlri-bgpls.md -- native origination contract
// RFC: rfc/short/rfc7752.md
// Related: export_state.go -- the announce command names the family
//
// VALIDATES: every Node, Link and Prefix NLRI the native producer originates is
// announced under the BGP-LS family, AFI 16388 / SAFI 71, and none under the
// VPN family (SAFI 72), which RFC 7752 Section 3.2 reserves for VPN
// information Ze does not originate.
// PREVENTS: non-VPN link-state information announced under another family.
package ls_export

import (
	"context"
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/core/linkstateevents"
)

// nativeAnnounceFamilies originates one IS-IS node, one link and one prefix and
// returns, for each announced NLRI type (1 Node, 2 Link, 3 IPv4 Prefix), the
// family token that follows "nlri" in its announce command.
func nativeAnnounceFamilies(t *testing.T) map[uint16]string {
	t.Helper()
	e, capture := exportFixture(t)
	node := nativeTestNode()
	remote := linkstateevents.NodeID{ASN: 65000, RouterID: []byte{1, 2, 3, 4, 5, 7}}
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1}, Generation: 1,
		Nodes: []linkstateevents.Node{{ID: node}},
		Links: []linkstateevents.Link{{Local: node, Remote: remote,
			LocalAddresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}},
		Prefixes: []linkstateevents.Prefix{{Node: node, Prefix: netip.MustParsePrefix("198.51.100.0/24")}}}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))

	families := map[uint16]string{}
	for _, command := range capture.commands {
		fields := strings.Fields(command)
		for i, field := range fields {
			if field != "nlri" {
				continue
			}
			require.Greater(t, len(fields), i+1)
			families[binary.BigEndian.Uint16(exportCommandBytes(t, command, "nlri"))] = fields[i+1]
		}
	}
	return families
}

// TestRFC7752NativeNLRIAnnouncedAsSAFI71 checks each object kind is announced,
// under the family the registry holds as (16388, 71).
//
// RFC requirement: RFC7752-3.2-5 positive -- the Node (type 1), Link (type 2) and IPv4 Prefix (type 3) NLRI the native producer originates are each announced under the family bgp-ls/bgp-ls, which is AFI 16388 / SAFI 71 (Section 3.2).
func TestRFC7752NativeNLRIAnnouncedAsSAFI71(t *testing.T) {
	require.Equal(t, uint16(16388), uint16(ls.BGPLSFamily.AFI))
	require.Equal(t, uint8(71), uint8(ls.BGPLSFamily.SAFI))
	families := nativeAnnounceFamilies(t)
	for _, kind := range []uint16{1, 2, 3} {
		require.Equal(t, ls.BGPLSFamily.String(), families[kind], "NLRI type %d", kind)
	}
}

// TestRFC7752NativeNLRINeverAnnouncedAsVPN checks no originated object, of any
// kind, is announced under the VPN family.
//
// RFC requirement: RFC7752-3.2-5 negative -- none of the Node, Link and Prefix NLRI the native producer originates is announced under bgp-ls/bgp-ls-vpn (AFI 16388 / SAFI 72) or any family other than (16388, 71) (Section 3.2).
func TestRFC7752NativeNLRINeverAnnouncedAsVPN(t *testing.T) {
	require.Equal(t, uint8(72), uint8(ls.BGPLSVPNFamily.SAFI))
	families := nativeAnnounceFamilies(t)
	require.Len(t, families, 3)
	for kind, family := range families {
		require.NotEqual(t, ls.BGPLSVPNFamily.String(), family, "NLRI type %d", kind)
		require.Equal(t, ls.BGPLSFamily.String(), family, "NLRI type %d", kind)
	}
}
