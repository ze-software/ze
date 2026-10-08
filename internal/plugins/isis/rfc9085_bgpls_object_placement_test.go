// Design: docs/architecture/wire/nlri-bgpls.md -- native IS-IS placement.
// Related: bgpls_export.go -- LSDB-to-snapshot translation.
package isis

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// TestRFC9085ISISLinkPlacement distinguishes two links at each of two origins,
// in both standard and nondefault topology reachability. No BGP-LS TLVs are inputs.
// RFC 9085 Section 2.2: "These TLVs should only be added to the BGP-LS Attribute associated
// with the Link NLRI that describes the link of the IGP node that is originating the
// corresponding IGP TLV/sub-TLV described below."
// RFC requirement: RFC9085-2.2-1 positive -- native IS-IS TLV 22/222 sub-TLV 31/32 produce exact 1099/1100 values on their own origin, complete link identifiers and topology, including parallel links with shared endpoints.
// RFC requirement: RFC9085-2.2-1 negative -- parallel-link SID values cannot be swapped, merged or assigned to a different link descriptor; real Node and Prefix controls carry no link SID attributes.
func TestRFC9085ISISLinkPlacement(t *testing.T) {
	eng := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(eng.shutdown)
	for _, origin := range []byte{2, 3} {
		var tlvs []packet.TLV
		for _, topology := range []byte{0, 7} {
			var links []byte
			for _, object := range []byte{4, 5} {
				marker := origin*20 + object + topology
				subs := []byte{4, 8, 0, 0, 0, object, 0, 0, 0, object + 10,
					31, 5, 0x30, marker, 0, 0x3e, marker,
					32, 11, 0x30, marker + 1, 0, 0, 0, 0, 0, 4, 0, 0x3f, marker}
				links = append(links, 0, 0, 0, 0, 0, 4, 0, 0, 0, 9, byte(len(subs)))
				links = append(links, subs...)
			}
			typ := uint8(22)
			if topology != 0 {
				typ = 222
				links = append([]byte{0, topology}, links...)
			}
			tlvs = append(tlvs, packet.TLV{Type: typ, Value: links})
		}
		tlvs = append(tlvs, packet.TLV{Type: 135, Value: []byte{0, 0, 0, 10, 24, 192, 0, 2}})
		bgplsStoreLSP(t, eng, lsdb.Level1, types.LSPID{0, 0, 0, 0, 0, origin}, 1, 1200, tlvs)
	}
	var builder bgplsBuilder
	// RFC 9085 Section 2.2: translate native link sub-TLVs with their origin and neighbor.
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	s := &builder.snapshot
	if len(s.Links) != 8 {
		t.Fatalf("links = %d, want 8", len(s.Links))
	}
	if len(s.Nodes) != 2 || len(s.Prefixes) != 2 {
		t.Fatalf("control nodes/prefixes = %d/%d, want 2/2", len(s.Nodes), len(s.Prefixes))
	}
	seen := make(map[[3]byte]bool)
	for _, link := range s.Links {
		if len(link.Local.RouterID) != 6 || len(link.Remote.RouterID) != 6 || len(link.Topologies) != 1 {
			t.Fatalf("unexpected link identity: %+v", link)
		}
		origin, object, topology := link.Local.RouterID[5], byte(link.LocalID), byte(link.Topologies[0])
		if link.Topologies[0] != uint16(topology) {
			t.Fatalf("truncated topology identity %d", link.Topologies[0])
		}
		if !bytes.Equal(link.Local.RouterID, []byte{0, 0, 0, 0, 0, origin}) {
			t.Fatalf("corrupt local router %x", link.Local.RouterID)
		}
		if !bytes.Equal(link.Remote.RouterID, []byte{0, 0, 0, 0, 0, 4}) {
			t.Fatalf("wrong parallel-link neighbor %x", link.Remote.RouterID)
		}
		if !link.HasLinkIDs || link.LocalID != uint32(object) || link.RemoteID != uint32(object)+10 {
			t.Fatalf("wrong parallel-link identifiers %+v", link)
		}
		key := [3]byte{origin, object, topology}
		if seen[key] || (origin != 2 && origin != 3) || (object != 4 && object != 5) || (topology != 0 && topology != 7) {
			t.Fatalf("unexpected or duplicate link %v", key)
		}
		seen[key] = true
		marker := origin*20 + object + topology
		rfc9085ISISPlacementAttributes(t, link.Attributes, map[uint16][]byte{
			1099: {0x30, marker, 0, 0, 0, 0x3e, marker},
			1100: {0x30, marker + 1, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0x3f, marker},
		})
	}
	for _, node := range s.Nodes {
		rfc9085ISISPlacementAttributes(t, node.Attributes, nil)
	}
	for _, prefix := range s.Prefixes {
		rfc9085ISISPlacementAttributes(t, prefix.Attributes, nil)
	}
}

// TestRFC9085ISISPrefixPlacement uses repeated prefix identities at distinct
// origins and topologies, with two prefixes per native TLV and distinct values.
// RFC 9085 Section 2.3: "These TLVs should only be added to the BGP-LS Attribute associated
// with the Prefix NLRI that describes the prefix of the IGP node that is originating
// the corresponding IGP TLV/sub-TLV described below."
// RFC requirement: RFC9085-2.3-1 positive -- native IS-IS IPv4/IPv6 reachability, binding and narrow-external inputs attach 1158/1159/1170/1171 to their own origin, prefix and topology with exact values; nodes and links carry none. This does not claim absent OSPF source-identifier features.
// RFC requirement: RFC9085-2.3-1 negative -- distinct native IS-IS prefix values cannot be swapped between origins, addresses or topologies; actual Node and Link controls contain no prefix-class SR attributes.
func TestRFC9085ISISPrefixPlacement(t *testing.T) {
	for _, mode := range []string{"ipv4", "ipv6", "range4", "range6", "narrow-external"} {
		t.Run(mode, func(t *testing.T) {
			eng := newEngine(transport.New(&fakeBackend{}))
			t.Cleanup(eng.shutdown)
			for _, origin := range []byte{2, 3} {
				var tlvs []packet.TLV
				for _, topology := range []byte{0, 7} {
					if mode == "narrow-external" && topology != 0 {
						continue
					}
					for _, object := range []byte{4, 5} {
						typ, value := rfc9085ISISPlacementPrefix(mode, origin, object, topology)
						tlvs = append(tlvs, packet.TLV{Type: typ, Value: value})
					}
				}
				tlvs = append(tlvs, packet.TLV{Type: 22, Value: []byte{0, 0, 0, 0, 0, 4, 0, 0, 0, 9, 0}})
				bgplsStoreLSP(t, eng, lsdb.Level1, types.LSPID{0, 0, 0, 0, 0, origin}, 1, 1200, tlvs)
			}
			var builder bgplsBuilder
			// RFC 9085 Section 2.3: translate native reachability and binding identities.
			builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
			if len(builder.snapshot.Nodes) != 2 || len(builder.snapshot.Links) != 2 {
				t.Fatal("missing node/link controls")
			}
			wantCount := 8
			if mode == "narrow-external" {
				wantCount = 4
			}
			if len(builder.snapshot.Prefixes) != wantCount {
				t.Fatalf("prefix count = %d, want %d", len(builder.snapshot.Prefixes), wantCount)
			}
			seen := make(map[[3]byte]bool)
			for _, prefix := range builder.snapshot.Prefixes {
				origin := prefix.Node.RouterID[5]
				address := prefix.Prefix.Addr().AsSlice()
				object := address[2]
				if prefix.Prefix.Addr().Is6() {
					object = address[7]
				}
				topology := byte(prefix.Topology)
				key := [3]byte{origin, object, topology}
				if seen[key] || (origin != 2 && origin != 3) || (object != 4 && object != 5) || (topology != 0 && topology != 7) {
					t.Fatalf("unexpected or duplicate prefix %+v", prefix)
				}
				seen[key] = true
				wantPrefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 0, object, 0}), 24)
				if mode == "ipv6" || mode == "range6" {
					wantPrefix = netip.PrefixFrom(netip.AddrFrom16([16]byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, object}), 64)
				}
				if prefix.Prefix != wantPrefix {
					t.Fatalf("prefix = %s, want %s", prefix.Prefix, wantPrefix)
				}
				marker := origin*20 + object + topology
				want := map[uint16][]byte{1158: {0x40, 0, 0, 0, 0, 0, 0, marker}, 1170: {object << 4}, 1171: {192, 0, 2, origin}}
				if mode == "ipv6" {
					want[1171] = []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, origin}
				}
				if mode == "range4" || mode == "range6" {
					flags := byte(0)
					if mode == "range6" {
						flags = 0x80
					}
					want = map[uint16][]byte{1159: {flags, 0, 0, object, 4, 0x86, 0, 8, 0x40, 0, 0, 0, 0, 0, 0, marker}}
				}
				if mode == "narrow-external" {
					want = map[uint16][]byte{1170: {0x80}}
				}
				rfc9085ISISPlacementAttributes(t, prefix.Attributes, want)
			}
			for _, node := range builder.snapshot.Nodes {
				rfc9085ISISPlacementAttributes(t, node.Attributes, nil)
			}
			for _, link := range builder.snapshot.Links {
				rfc9085ISISPlacementAttributes(t, link.Attributes, nil)
			}
		})
	}
}

// rfc9085ISISPlacementPrefix constructs native reachability, not BGP-LS values.
func rfc9085ISISPlacementPrefix(mode string, origin, object, topology byte) (uint8, []byte) {
	marker := origin*20 + object + topology
	address := []byte{192, 0, object}
	bits, typ := byte(24), uint8(135)
	if mode == "ipv6" || mode == "range6" {
		address = []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, object}
		bits, typ = 64, 236
	}
	subs := []byte{3, 6, 0x40, 0, 0, 0, 0, marker, 4, 1, object << 4, 11, 4, 192, 0, 2, origin}
	if mode == "ipv6" {
		subs = []byte{3, 6, 0x40, 0, 0, 0, 0, marker, 4, 1, object << 4, 12, 16,
			0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, origin}
	}
	value := []byte{0, 0, 0, 10, 0x40 | bits}
	if mode == "ipv6" {
		value = []byte{0, 0, 0, 10, 0x20, bits}
	}
	value = append(value, address...)
	value = append(value, byte(len(subs)))
	value = append(value, subs...)
	if mode == "range4" || mode == "range6" {
		flags := byte(0)
		if mode == "range6" {
			flags = 0x80
		}
		typ = 149
		value = append([]byte{flags, 0, 0, object, bits}, address...)
		value = append(value, 3, 6, 0x40, 0, 0, 0, 0, marker)
	}
	if mode == "narrow-external" {
		return 130, []byte{10, 0x80, 0x80, 0x80, 192, 0, object, 0, 255, 255, 255, 0}
	}
	if topology != 0 {
		switch typ {
		case 135:
			typ = 235
		case 236:
			typ = 237
		case 149:
			typ = 150
		}
		value = append([]byte{0, topology}, value...)
	}
	return typ, value
}

// rfc9085ISISPlacementAttributes checks all table-2/4 attributes, including absence
// and duplicate counts; an attribute moved to another object's list cannot pass.
func rfc9085ISISPlacementAttributes(t *testing.T, attrs []linkstateevents.TLV, want map[uint16][]byte) {
	t.Helper()
	for _, typ := range []uint16{1099, 1100, 1172, 1158, 1159, 1170, 1171, 1174} {
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
