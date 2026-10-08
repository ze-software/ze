// Design: docs/architecture/wire/nlri-bgpls.md -- native OSPF placement.
// Related: bgpls_export.go -- native LSA translation.
package ospf

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	v3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	v3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// TestRFC9085OSPFLinkPlacement compares each origin/neighbor/topology's SID
// bytes after real OSPFv2 and OSPFv3 LSDB replay, with two links per origin.
// RFC 9085 Section 2.2: "These TLVs should only be added to the BGP-LS Attribute associated
// with the Link NLRI that describes the link of the IGP node that is originating the
// corresponding IGP TLV/sub-TLV described below."
// RFC requirement: RFC9085-2.2-1 positive -- native OSPFv2/v3 LSDB replay places exact 1099/1100 values on each originating router's complete link identity, including two parallel links with shared endpoints and topology.
// RFC requirement: RFC9085-2.2-1 negative -- OSPF parallel links cannot exchange or merge SID values, lose their interface addresses/IDs, or leak link SID attributes onto actual Node/Prefix controls.
func TestRFC9085OSPFLinkPlacement(t *testing.T) {
	for _, v3 := range []bool{false, true} {
		name := "v2"
		if v3 {
			name = "v3"
		}
		t.Run(name, func(t *testing.T) {
			s := rfc9085OSPFPlacementSnapshot(t, v3, v3types.LSTypeEInterAreaPrefix)
			wantCount := 8
			if v3 {
				wantCount = 4
			}
			if len(s.Links) != wantCount {
				t.Fatalf("links = %d, want %d", len(s.Links), wantCount)
			}
			seen := make(map[[3]byte]bool)
			for _, link := range s.Links {
				if len(link.Topologies) != 1 || len(link.Local.RouterID) != 4 || len(link.Remote.RouterID) != 4 {
					t.Fatalf("link identity: %+v", link)
				}
				origin, topology := link.Local.RouterID[0], byte(link.Topologies[0])
				if link.Topologies[0] != uint16(topology) {
					t.Fatalf("truncated topology identity %d", link.Topologies[0])
				}
				object := byte(link.LocalID)
				if v3 {
					if !link.HasLinkIDs || link.LocalID != uint32(object) || link.RemoteID != uint32(origin) {
						t.Fatalf("wrong v3 interface identifiers %+v", link)
					}
					if len(link.LocalAddresses) != 0 || len(link.RemoteAddresses) != 0 {
						t.Fatalf("unexpected v3 interface addresses %+v", link)
					}
				} else {
					if len(link.LocalAddresses) != 1 || len(link.RemoteAddresses) != 0 || link.HasLinkIDs {
						t.Fatalf("wrong v2 interface descriptors %+v", link)
					}
					address := link.LocalAddresses[0].As4()
					object = address[2]
					if address != [4]byte{10, origin, object, 1} {
						t.Fatalf("wrong v2 interface address %v", address)
					}
				}
				if !bytes.Equal(link.Local.RouterID, []byte{origin, origin, origin, origin}) {
					t.Fatalf("wrong origin %x", link.Local.RouterID)
				}
				if !bytes.Equal(link.Remote.RouterID, []byte{4, 4, 4, 4}) {
					t.Fatalf("wrong parallel-link neighbor %x", link.Remote.RouterID)
				}
				key := [3]byte{origin, object, topology}
				if seen[key] || (origin != 2 && origin != 3) || (object != 4 && object != 5) || (topology != 0 && topology != 7) {
					t.Fatalf("unexpected or duplicate link %+v", link)
				}
				seen[key] = true
				marker := origin*20 + object + topology
				rfc9085OSPFPlacementAttributes(t, link.Attributes, map[uint16][]byte{
					1099: {0x60, marker, 0, 0, 0, 0x5d, marker},
					1100: {0x60, marker + 1, 0, 0, 4, 4, 4, 4, 0, 0x5e, marker},
				}, true)
			}
			if len(s.Nodes) < 2 || len(s.Prefixes) < 4 {
				t.Fatal("missing node/prefix controls")
			}
			for _, node := range s.Nodes {
				rfc9085OSPFPlacementAttributes(t, node.Attributes, nil, true)
			}
			for _, prefix := range s.Prefixes {
				rfc9085OSPFPlacementAttributes(t, prefix.Attributes, nil, true)
			}
		})
	}
}

// TestRFC9085OSPFPrefixPlacement checks two prefixes plus two ranges per origin,
// including all four OSPFv3 Extended Prefix LSA classes and OSPFv2 MT-ID selection.
// RFC 9085 Section 2.3: "These TLVs should only be added to the BGP-LS Attribute associated
// with the Prefix NLRI that describes the prefix of the IGP node that is originating
// the corresponding IGP TLV/sub-TLV described below."
// RFC requirement: RFC9085-2.3-1 positive -- native OSPFv2/v3 LSDB replay places 1158/1159/1170 on their own origin, prefix, topology and route class with exact values, including base-v3 flags; nodes/links carry none. Absent OSPF 1171/1174 are not claimed.
// RFC requirement: RFC9085-2.3-1 negative -- native OSPF prefix attributes cannot cross origin, prefix, topology or route-class identities, and actual Node/Link controls contain no prefix-class SR attributes.
func TestRFC9085OSPFPrefixPlacement(t *testing.T) {
	for _, tc := range []struct {
		name  string
		v3    bool
		typ   v3types.LSType
		route uint8
	}{
		{"v2", false, 0, 1},
		{"v3-intra", true, v3types.LSTypeEIntraAreaPrefix, 1},
		{"v3-inter", true, v3types.LSTypeEInterAreaPrefix, 2},
		{"v3-external", true, v3types.LSTypeEASExternal, 3},
		{"v3-nssa", true, v3types.LSTypeEType7, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := rfc9085OSPFPlacementSnapshot(t, tc.v3, tc.typ)
			wantCount := 16
			if tc.v3 {
				wantCount = 8
			}
			if len(s.Prefixes) != wantCount {
				t.Fatalf("prefix count = %d, want %d: %+v", len(s.Prefixes), wantCount, s.Prefixes)
			}
			seen := make(map[[4]byte]bool)
			for _, prefix := range s.Prefixes {
				origin := prefix.Node.RouterID[0]
				address := prefix.Prefix.Addr().AsSlice()
				object := address[2]
				if tc.v3 {
					object = address[7]
				}
				isRange := object >= 14
				if isRange {
					object -= 10
				}
				topology := byte(prefix.Topology)
				key := [4]byte{origin, address[len(address)-1], object, topology}
				if isRange {
					key[1] = 1
				}
				if seen[key] || (origin != 2 && origin != 3) || (object != 4 && object != 5) || (topology != 0 && topology != 7) {
					t.Fatalf("unexpected or duplicate prefix %+v", prefix)
				}
				seen[key] = true
				prefixObject := object
				if isRange {
					prefixObject += 10
				}
				wantAddress := []byte{192, 0, prefixObject, 0}
				wantBits := 24
				if tc.v3 {
					wantAddress = []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, prefixObject, 0, 0, 0, 0, 0, 0, 0, 0}
					wantBits = 64
				}
				if !bytes.Equal(address, wantAddress) || prefix.Prefix.Bits() != wantBits {
					t.Fatalf("prefix identity = %s, want %x/%d", prefix.Prefix, wantAddress, wantBits)
				}
				marker := origin*20 + object + topology
				want := map[uint16][]byte{1158: {0x40, 1, 0, 0, 0, 0, 0, marker}, 1170: {object}}
				route := tc.route
				if isRange {
					route = 0
					want = map[uint16][]byte{1159: {0, 0, 0, object, 4, 0x86, 0, 8, 0x40, 1, 0, 0, 0, 0, 0, marker}}
				}
				if prefix.RouteType != route {
					t.Fatalf("prefix %s route type = %d, want %d", prefix.Prefix, prefix.RouteType, route)
				}
				rfc9085OSPFPlacementAttributes(t, prefix.Attributes, want, false)
			}
			if len(s.Nodes) < 2 || len(s.Links) < 4 {
				t.Fatal("missing node/link controls")
			}
			for _, node := range s.Nodes {
				rfc9085OSPFPlacementAttributes(t, node.Attributes, nil, false)
			}
			for _, link := range s.Links {
				rfc9085OSPFPlacementAttributes(t, link.Attributes, nil, false)
			}
		})
	}
	t.Run("v3-base-flags", rfc9085OSPFBasePrefixPlacement)
}

// rfc9085OSPFPlacementSnapshot installs wire-native LSAs and uses synchronous replay.
func rfc9085OSPFPlacementSnapshot(t *testing.T, v3 bool, prefixType v3types.LSType) linkstateevents.Snapshot {
	t.Helper()
	e, bus, events := bgplsTestSource(t, v3)
	for _, origin := range []byte{2, 3} {
		if !v3 {
			// Router LSAs supply the actual Node NLRI controls. Extended Link
			// and Prefix opaque LSAs alone do not advertise Node attributes.
			router := packet.LSA{Header: packet.LSAHeader{
				Type: types.LSTypeRouter, LinkStateID: types.LinkStateID{origin, origin, origin, origin},
				AdvertisingRouter: types.RouterID{origin, origin, origin, origin}, Sequence: types.InitialSequenceNumber,
			}, Router: &packet.RouterLSA{Links: []packet.RouterLink{
				{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{4, 4, 4, 4}, LinkData: [4]byte{10, origin, 4, 1}, Metric: 10},
				{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{4, 4, 4, 4}, LinkData: [4]byte{10, origin, 5, 1}, Metric: 10},
			}}}
			if !e.lsdb.Install(types.BackboneArea, router) {
				t.Fatal("install placement router")
			}
		}
		for _, object := range []byte{4, 5} {
			if v3 {
				rfc9085OSPFPlacementV3(t, e, origin, object, prefixType)
				continue
			}
			var links []packet.ExtSubTLV
			var prefixes []packet.ExtSubTLV
			var ranges []packet.ExtPrefixRangeTLV
			for _, topology := range []byte{0, 7} {
				marker := origin*20 + object + topology
				links = append(links,
					packet.ExtSubTLV{Type: sr.V4TypeAdjSID, Value: []byte{0x60, 0, topology, marker, 0, 0x5d, marker}},
					packet.ExtSubTLV{Type: sr.V4TypeLANAdjSID, Value: []byte{0x60, 0, topology, marker + 1, 4, 4, 4, 4, 0, 0x5e, marker}})
				sid := []byte{0x40, 0, topology, 1, 0, 0, 0, marker}
				prefixes = append(prefixes, packet.ExtSubTLV{Type: sr.V4TypePrefixSID, Value: sid})
				rangeValue := []byte{24, 0, 0, object, 0, 0, 0, 0, 192, 0, object + 10, 0, 0, 2, 0, 8}
				ranges = append(ranges, packet.ExtPrefixRangeTLV{Value: append(rangeValue, sid...)})
			}
			rfc9085OSPFPlacementOpaque(t, e, origin, object, packet.ExtLinkOpaqueType, packet.EncodeExtLinkLSA(packet.ExtLinkTLV{
				LinkType: 1, LinkID: [4]byte{4, 4, 4, 4}, LinkData: [4]byte{10, origin, object, 1}, SubTLVs: links,
			}))
			rfc9085OSPFPlacementOpaque(t, e, origin, object, packet.ExtPrefixOpaqueType, packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{
				Prefixes: []packet.ExtPrefixTLV{{RouteType: packet.ExtRouteTypeIntraArea, Flags: object, PrefixLength: 24,
					AddressPrefix: [4]byte{192, 0, object, 0}, SubTLVs: prefixes}}, Ranges: ranges,
			}))
		}
	}
	// RFC 9085 Sections 2.2 and 2.3: observe the actual native publisher boundary.
	return bgplsReplaySnapshot(t, bus, events)
}

func rfc9085OSPFPlacementOpaque(t *testing.T, e *engine, origin, object, typ byte, body []byte) {
	t.Helper()
	lsa := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeOpaqueArea,
		LinkStateID: types.LinkStateID{typ, 0, 0, object}, AdvertisingRouter: types.RouterID{origin, origin, origin, origin},
		Sequence: types.InitialSequenceNumber}, Body: body}
	if !e.lsdb.Install(types.BackboneArea, lsa) {
		t.Fatal("install placement LSA")
	}
}

func rfc9085OSPFPlacementV3(t *testing.T, e *engine, origin, object byte, prefixType v3types.LSType) {
	t.Helper()
	marker := origin*20 + object
	link := []byte{1, 0, 0, 10, 0, 0, 0, object, 0, 0, 0, origin, 4, 4, 4, 4}
	link = v3packet.AppendSubTLVs(link, []v3packet.ExtendedTLV{
		{Type: sr.V6TypeAdjSID, Value: []byte{0x60, marker, 0, 0, 0, 0x5d, marker}},
		{Type: sr.V6TypeLANAdjSID, Value: []byte{0x60, marker + 1, 0, 0, 4, 4, 4, 4, 0, 0x5e, marker}},
	})
	body := append([]byte{0, 0, 0, 0}, v3packet.EncodeExtendedLSABody(v3packet.ExtendedLSA{TLVs: []v3packet.ExtendedTLV{{Type: 1, Value: link}}})...)
	router := v3types.RouterID{origin, origin, origin, origin}
	if !e.lsdb.Install(types.BackboneArea, bgplsNativeV3(t, router, v3types.LSTypeERouter, uint32(object), body, types.InitialSequenceNumber, false)) {
		t.Fatal("install extended router")
	}
	prefix := []byte{0, 0, 0, 10, 64, object, 0, 0, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, object}
	prefix = v3packet.AppendSubTLVs(prefix, []v3packet.ExtendedTLV{{Type: sr.V6TypePrefixSID, Value: []byte{0x40, 1, 0, 0, 0, 0, 0, marker}}})
	typ := uint16(3)
	//exhaustive:ignore // The fixture varies only the four extended prefix LSA classes.
	switch prefixType {
	case v3types.LSTypeEIntraAreaPrefix:
		typ = 6
	case v3types.LSTypeEASExternal, v3types.LSTypeEType7:
		typ = 5
	}
	address := []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, object + 10}
	body = v3packet.EncodeExtendedLSABody(v3packet.ExtendedLSA{TLVs: []v3packet.ExtendedTLV{
		{Type: typ, Value: prefix},
		{Type: sr.V6TypeExtPrefixRange, Value: sr.EncodeExtPrefixRangeValueV6(64, address, uint16(object), sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Algorithm: 1, Index: uint32(marker)})},
	}})
	if prefixType == v3types.LSTypeEIntraAreaPrefix {
		header := make([]byte, 12)
		binary.BigEndian.PutUint16(header[2:4], uint16(v3types.LSTypeERouter))
		copy(header[8:12], router[:])
		body = append(header, body...)
	}
	if !e.lsdb.Install(types.BackboneArea, bgplsNativeV3(t, router, prefixType, uint32(object), body, types.InitialSequenceNumber, false)) {
		t.Fatal("install extended prefix")
	}
}

// rfc9085OSPFPlacementAttributes checks the whole applicable attribute class.
func rfc9085OSPFPlacementAttributes(t *testing.T, attrs []linkstateevents.TLV, want map[uint16][]byte, link bool) {
	t.Helper()
	types := []uint16{1158, 1159, 1170, 1171, 1174}
	if link {
		types = []uint16{1099, 1100, 1172}
	}
	for _, typ := range types {
		var got [][]byte
		for _, attr := range attrs {
			if attr.Type == typ {
				got = append(got, attr.Value)
			}
		}
		expected, present := want[typ]
		if !present {
			if len(got) != 0 {
				t.Fatalf("unexpected TLV %d = %x", typ, got)
			}
			continue
		}
		if len(got) != 1 {
			t.Fatalf("TLV %d count = %d, want 1", typ, len(got))
		}
		if !bytes.Equal(got[0], expected) {
			t.Fatalf("TLV %d = %x, want %x", typ, got[0], expected)
		}
	}
}

// rfc9085OSPFBasePrefixPlacement covers the non-extended v3Prefix callers. A
// same-address prefix from another origin must retain its own options byte.
func rfc9085OSPFBasePrefixPlacement(t *testing.T) {
	for _, tc := range []struct {
		name  string
		typ   v3types.LSType
		route uint8
	}{
		{"intra", v3types.LSTypeIntraAreaPrefix, 1},
		{"inter", v3types.LSTypeInterAreaPrefix, 2},
		{"external", v3types.LSTypeASExternal, 3},
		{"nssa", v3types.LSTypeNSSA, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, bus, events := bgplsTestSource(t, true)
			for _, origin := range []byte{2, 3} {
				for _, object := range []byte{4, 5} {
					router := v3types.RouterID{origin, origin, origin, origin}
					body := []byte{0, 0, 0, 10, 64, origin*16 + object, 0, 0, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, object}
					if tc.typ == v3types.LSTypeIntraAreaPrefix {
						body = []byte{0, 1, 0x20, 1, 0, 0, 0, 0, origin, origin, origin, origin,
							64, origin*16 + object, 0, 10, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, object}
					}
					if !e.lsdb.Install(types.BackboneArea, bgplsNativeV3(t, router, tc.typ, uint32(object), body, types.InitialSequenceNumber, false)) {
						t.Fatal("install base prefix")
					}
				}
			}
			// RFC 9085 Section 2.3.2: base PrefixOptions is native Prefix Attribute Flags.
			s := bgplsReplaySnapshot(t, bus, events)
			if len(s.Prefixes) != 4 {
				t.Fatalf("base prefixes = %d, want 4", len(s.Prefixes))
			}
			seen := make(map[[2]byte]bool)
			for _, prefix := range s.Prefixes {
				origin := prefix.Node.RouterID[0]
				address := prefix.Prefix.Addr().As16()
				object := address[7]
				key := [2]byte{origin, object}
				if seen[key] || (origin != 2 && origin != 3) || (object != 4 && object != 5) {
					t.Fatalf("unexpected base identity %+v", prefix)
				}
				seen[key] = true
				if prefix.RouteType != tc.route {
					t.Fatalf("base route type = %d, want %d", prefix.RouteType, tc.route)
				}
				rfc9085OSPFPlacementAttributes(t, prefix.Attributes, map[uint16][]byte{1170: {origin*16 + object}}, false)
			}
			for _, node := range s.Nodes {
				rfc9085OSPFPlacementAttributes(t, node.Attributes, nil, false)
			}
		})
	}
}
