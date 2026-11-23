// Design: docs/architecture/bgp/egress-attribute-rules.md -- originated MED on Established eBGP announce builders.
// Related: forward_med_test.go -- forward-rail MED provenance and Section 5.1.4 suppression.
// Related: reactor_wire.go -- writeAnnounceUpdate direct announce writer.
// Related: reactor_api_batch.go -- planBatchAttrs batch announce planner.
// Related: peer_static_wire.go -- deliverStaticRouteDelta static route delta sender via UpdateBuilder.BuildUnicast.
// RFC: rfc/short/rfc4271.md -- Section 5.1.4 permits locally originated MULTI_EXIT_DISC toward external peers.
package reactor

import (
	"encoding/binary"
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// assertOriginatedEBGPMEDOnWire verifies that the Established eBGP peer
// connection received one UPDATE per expected prefix carrying its exact
// locally originated MULTI_EXIT_DISC metric.
func assertOriginatedEBGPMEDOnWire(t *testing.T, wireBytes []byte, wantPrefix netip.Prefix, wantMED uint32) {
	t.Helper()
	bodies := updateBodies(t, wireBytes)
	require.NotEmpty(t, bodies, "Established eBGP peer must receive an UPDATE")

	matched := false
	for _, body := range bodies {
		prefixes := updatePrefixes(t, body, false)
		if !slices.Contains(prefixes, wantPrefix) {
			continue
		}
		med := payloadMED(body)
		require.True(t, med.present, "originated eBGP UPDATE for %s must carry MULTI_EXIT_DISC", wantPrefix)
		require.Len(t, med.raw, 4, "MULTI_EXIT_DISC must be 4 octets")
		require.Equal(t, wantMED, binary.BigEndian.Uint32(med.raw),
			"originated eBGP UPDATE for %s must carry exact MED %d", wantPrefix, wantMED)
		matched = true
	}
	require.True(t, matched, "Established eBGP peer must receive prefix %s", wantPrefix)
}

// VALIDATES: locally originated routes with MULTI_EXIT_DISC announced after
// Established via Peer.SendAnnounce -> Session.SendAnnounce ->
// writeAnnounceUpdate reach an eBGP peer carrying their exact MED (including 0).
// PREVENTS: a blanket eBGP MED strip in writeAnnounceUpdate dropping Ze's own
// outbound metric on post-Established direct announces.
//
// RFC requirement: RFC4271-5.1.4-1 negative -- the Section 5.1.4 prohibition
// covers only a received MULTI_EXIT_DISC relayed to another AS; announcing a
// locally originated route with MED (42 and 0) to an already-Established eBGP
// peer through writeAnnounceUpdate writes attribute 4 with that exact metric
// onto the peer's wire.
func TestRFC4271OriginatedMEDEstablishedWriteAnnounceUpdate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prefix string
		med    uint32
	}{
		{"med-42", "198.19.220.0/24", 42},
		{"med-zero", "198.19.221.0/24", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, conn := newGroupUpdatesPeer(t, "192.0.2.51", "false")
			require.False(t, peer.IsIBGP(), "target session must be eBGP")

			prefix := netip.MustParsePrefix(tc.prefix)
			wireAttrs := slices.Concat(
				makeAttr(0x40, byte(attribute.AttrOrigin), []byte{byte(attribute.OriginIGP)}),
				makeAttr(0x80, byte(attribute.AttrMED), binary.BigEndian.AppendUint32(nil, tc.med)),
			)
			route := bgptypes.RouteSpec{
				Prefix:  prefix,
				NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
				Wire:    attribute.NewAttributesWire(wireAttrs, bgpctx.APIContextID),
			}
			require.NoError(t, peer.SendAnnounce(route, 65000))
			assertOriginatedEBGPMEDOnWire(t, conn.written(), prefix, tc.med)
		})
	}
}

// VALIDATES: locally originated static routes with MULTI_EXIT_DISC delivered
// after Established via Peer.deliverStaticRouteDelta -> sendStaticRoutes ->
// buildStaticRouteUpdateNew -> UpdateBuilder.BuildUnicast reach an eBGP peer
// carrying their exact MED (including 0).
// PREVENTS: a blanket eBGP MED strip in UpdateBuilder.BuildUnicast dropping
// Ze's own outbound metric on post-Established static route updates.
//
// RFC requirement: RFC4271-5.1.4-1 negative -- the Section 5.1.4 prohibition
// covers only a received MULTI_EXIT_DISC relayed to another AS; announcing a
// locally originated static route with MED (42 and 0) to an already-Established
// eBGP peer through UpdateBuilder.BuildUnicast writes attribute 4 with that
// exact metric onto the peer's wire.
func TestRFC4271OriginatedMEDEstablishedBuildUnicast(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prefix string
		med    uint32
	}{
		{"med-42", "198.19.222.0/24", 42},
		{"med-zero", "198.19.223.0/24", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, conn := newGroupUpdatesPeer(t, "192.0.2.52", "false")
			require.False(t, peer.IsIBGP(), "target session must be eBGP")
			peer.staticWire = staticWireSet{session: peer.currentSession()}

			prefix := netip.MustParsePrefix(tc.prefix)
			wanted := []StaticRoute{
				{
					Prefix:  prefix,
					NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
					Origin:  uint8(attribute.OriginIGP),
					HasMED:  true,
					MED:     tc.med,
				},
			}
			peer.deliverStaticRouteDelta(wanted)
			assertOriginatedEBGPMEDOnWire(t, conn.written(), prefix, tc.med)
		})
	}
}

// VALIDATES: locally originated batch routes with MULTI_EXIT_DISC announced
// after Established via reactorAPIAdapter.AnnounceNLRIBatch ->
// announceBatchToPeers -> buildBatchAnnounceUpdate -> planBatchAttrs reach an
// eBGP peer carrying their exact MED (including 0).
// PREVENTS: a blanket eBGP MED strip in planBatchAttrs dropping Ze's own
// outbound metric on post-Established API/plugin batch announces.
//
// RFC requirement: RFC4271-5.1.4-1 negative -- the Section 5.1.4 prohibition
// covers only a received MULTI_EXIT_DISC relayed to another AS; announcing a
// locally originated batch route with MED (42 and 0) to an already-Established
// eBGP peer through planBatchAttrs writes attribute 4 with that exact metric
// onto the peer's wire.
func TestRFC4271OriginatedMEDEstablishedPlanBatchAttrs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prefix string
		med    uint32
	}{
		{"med-42", "198.19.224.0/24", 42},
		{"med-zero", "198.19.225.0/24", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, conn := newGroupUpdatesPeer(t, "192.0.2.53", "false")
			require.False(t, peer.IsIBGP(), "target session must be eBGP")
			adapter := groupUpdatesReactor([]*Peer{peer}, false)

			prefix := netip.MustParsePrefix(tc.prefix)
			batch := bgptypes.NLRIBatch{
				Family:  family.IPv4Unicast,
				NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, prefix, 0, false)},
				NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
				Attrs:   attribute.NewBuilder().SetOrigin(uint8(attribute.OriginIGP)).SetMED(tc.med),
			}
			require.NoError(t, adapter.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()))
			assertOriginatedEBGPMEDOnWire(t, conn.written(), prefix, tc.med)
		})
	}
}
