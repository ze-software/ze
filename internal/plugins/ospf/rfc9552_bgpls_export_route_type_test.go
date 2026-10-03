// Design: docs/architecture/wire/nlri-bgpls.md -- OSPF route type correlation.
package ospf

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestRFC9552ExtendedPrefixResolvesRouteType exercises the live LSDB snapshot
// publisher. An opaque marker identifies the extended prefix, so a correctly
// typed base prefix alone cannot satisfy the assertion.
// RFC requirement: RFC9552-5.2.3.1-1 positive -- Extended Prefix route type zero
// is resolved from matching Router, Summary, AS External and NSSA LSAs; external
// and NSSA types distinguish Type 1 from Type 2 even when the Extended Prefix
// only signals the external class. The published prefix carries the exact type.
func TestRFC9552ExtendedPrefixResolvesRouteType(t *testing.T) {
	for _, tc := range []struct {
		name   string
		base   types.LSType
		native uint8
		type2  bool
		want   uint8
	}{
		{name: "unsignaled-intra", base: types.LSTypeRouter, want: 1},
		{name: "unsignaled-inter", base: types.LSTypeSummaryNetwork, want: 2},
		{name: "unsignaled-external1", base: types.LSTypeASExternal, want: 3},
		{name: "unsignaled-external2", base: types.LSTypeASExternal, type2: true, want: 4},
		{name: "unsignaled-nssa1", base: types.LSTypeNSSA, want: 5},
		{name: "unsignaled-nssa2", base: types.LSTypeNSSA, type2: true, want: 6},
		{name: "external1", base: types.LSTypeASExternal, native: packet.ExtRouteTypeASExternal, want: 3},
		{name: "external2", base: types.LSTypeASExternal, native: packet.ExtRouteTypeASExternal, type2: true, want: 4},
		{name: "nssa1", base: types.LSTypeNSSA, native: packet.ExtRouteTypeNSSAExternal, want: 5},
		{name: "nssa2", base: types.LSTypeNSSA, native: packet.ExtRouteTypeNSSAExternal, type2: true, want: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, bus, events := bgplsTestSource(t, false)
			nssa := types.AreaID{0, 0, 0, 1}
			e.setConfig(ospfConfig{present: true, RouterID: types.RouterID{1, 1, 1, 1}, Areas: []areaConfig{
				{AreaID: types.BackboneArea}, {AreaID: nssa, AreaType: types.AreaTypeNSSA},
			}})
			base := packet.LSA{Header: packet.LSAHeader{Type: tc.base,
				LinkStateID: types.LinkStateID{192, 0, 2, 0}, AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: types.InitialSequenceNumber}}
			area := types.BackboneArea
			opaqueType := types.LSTypeOpaqueArea
			switch tc.base {
			case types.LSTypeRouter:
				base = bgplsTestRouter(10, types.InitialSequenceNumber)
			case types.LSTypeSummaryNetwork:
				base.Summary = &packet.SummaryLSA{NetworkMask: [4]byte{255, 255, 255, 0}, Metric: 10}
			case types.LSTypeASExternal, types.LSTypeNSSA:
				base.External = &packet.ExternalLSA{NetworkMask: [4]byte{255, 255, 255, 0}, ExternalType2: tc.type2, Metric: 10}
				opaqueType = types.LSTypeOpaqueAS
				if tc.base == types.LSTypeNSSA {
					area = nssa
				}
			default:
				t.Fatal("unsupported fixture LSA type")
			}
			if !e.lsdb.Install(area, base) {
				t.Fatal("install base LSA")
			}
			extended := packet.LSA{Header: packet.LSAHeader{Type: opaqueType,
				LinkStateID: types.LinkStateID{packet.ExtPrefixOpaqueType, 0, 0, 1}, AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: types.InitialSequenceNumber},
				Body: packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{{
					RouteType: tc.native, PrefixLength: 24, AddressPrefix: [4]byte{192, 0, 2, 0},
					SubTLVs: []packet.ExtSubTLV{{Type: 65003, Value: []byte{1, 2, 3, 4}}},
				}}})}
			if !e.lsdb.Install(types.BackboneArea, extended) {
				t.Fatal("install extended prefix")
			}
			// RFC 9552 Section 5.2.3.1: inspect the publisher's correlated result.
			snapshot := bgplsReplaySnapshot(t, bus, events)
			var found int
			for _, prefix := range snapshot.Prefixes {
				if prefix.Prefix != netip.MustParsePrefix("192.0.2.0/24") {
					continue
				}
				for _, opaque := range prefix.Opaque {
					if opaque.Source != linkstateevents.OSPFv2ExtendedPrefix {
						continue
					}
					if !bytes.Equal(prefix.Node.RouterID, []byte{2, 2, 2, 2}) || prefix.RouteType != tc.want {
						t.Fatalf("extended prefix identity/type = %+v, want router 2.2.2.2 type %d", prefix, tc.want)
					}
					found++
				}
			}
			if found != 1 {
				t.Fatalf("extended prefix advertisements = %d, want exactly one", found)
			}
		})
	}
}
