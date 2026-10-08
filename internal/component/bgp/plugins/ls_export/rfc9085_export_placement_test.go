// Design: docs/architecture/wire/nlri-bgpls.md -- native snapshot consumption.
// Related: internal/plugins/isis/rfc9085_bgpls_object_placement_test.go -- native placement.
// Related: internal/plugins/ospf/rfc9085_bgpls_object_placement_test.go -- native placement.
package ls_export

import (
	"bytes"
	"context"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
)

// TestRFC9085NativeExportAttributePlacement checks the second half of the proof:
// replace/reconcile must pair each snapshot object's attributes with its own wire
// NLRI, not another origin, neighbor, prefix or topology. Native producer tests
// separately prove LSDB-to-snapshot placement; these inputs do not prove IGP decoding.
// RFC 9085 Section 2.2: "These TLVs should only be added to the BGP-LS Attribute associated
// with the Link NLRI that describes the link of the IGP node that is originating the
// corresponding IGP TLV/sub-TLV described below."
// RFC 9085 Section 2.3: "These TLVs should only be added to the BGP-LS Attribute associated
// with the Prefix NLRI that describes the prefix of the IGP node that is originating
// the corresponding IGP TLV/sub-TLV described below."
// RFC requirement: RFC9085-2.2-1 positive -- real replacement/reconciliation pairs 1099/1100 with complete emitted Link NLRI identities, including parallel links and exact local/remote ID, IPv4 and IPv6 interface descriptors; native derivation is proved separately.
// RFC requirement: RFC9085-2.2-1 negative -- differently attributed parallel links cannot exchange SID values or lose/reverse their descriptors; actual Node and Prefix NLRIs contain no link SID attributes.
// RFC requirement: RFC9085-2.3-1 positive -- real replacement/reconciliation pairs 1158/1159/1170 and IS-IS 1171 with the emitted IPv4/IPv6 Prefix NLRI origin, address, topology and OSPF route types 1/2/3/5, including competing classes of the same prefix.
// RFC requirement: RFC9085-2.3-1 negative -- prefix attributes cannot cross competing OSPF route classes or other origin/address/topology identities; actual Node and Link NLRIs contain no prefix SR attributes. Absent OSPF 1171/1174 are not claimed.
func TestRFC9085NativeExportAttributePlacement(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol linkstateevents.Protocol
		isis     bool
		ipv6     bool
	}{
		{"isis-l1-v4", linkstateevents.ISISLevel1, true, false},
		{"isis-l2-v4", linkstateevents.ISISLevel2, true, false},
		{"isis-l1-v6", linkstateevents.ISISLevel1, true, true},
		{"isis-l2-v6", linkstateevents.ISISLevel2, true, true},
		{"ospfv2", linkstateevents.OSPFv2, false, false},
		{"ospfv3-v4", linkstateevents.OSPFv3, false, false},
		{"ospfv3-v6", linkstateevents.OSPFv3, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: tc.protocol}, Generation: 1}
			for _, origin := range []byte{2, 3} {
				local := rfc9085ExportNode(origin, tc.isis)
				s.Nodes = append(s.Nodes, linkstateevents.Node{ID: local})
				for _, object := range []byte{4, 5} {
					for _, topology := range []uint16{0, 7} {
						marker := origin*20 + object + byte(topology)
						for linkMode := byte(0); linkMode < 3; linkMode++ {
							linkMarker := marker + linkMode*80
							flags := byte(0x60)
							if tc.isis {
								flags = 0x30
							}
							lan := append([]byte{flags, linkMarker + 1, 0, 0}, rfc9085ExportNode(4, tc.isis).RouterID...)
							lan = append(lan, 0, 0x3f, linkMarker)
							link := linkstateevents.Link{Local: local, Remote: rfc9085ExportNode(4, tc.isis),
								LocalID: uint32(object), RemoteID: uint32(origin) + 10, HasLinkIDs: true, Topologies: []uint16{topology},
								Attributes: []linkstateevents.TLV{
									{Type: 1099, Value: []byte{flags, linkMarker, 0, 0, 0, 0x3e, linkMarker}},
									{Type: 1100, Value: lan},
								}}
							if linkMode == 1 {
								link.LocalAddresses = []netip.Addr{netip.AddrFrom4([4]byte{10, origin, object, 1})}
								link.RemoteAddresses = []netip.Addr{netip.AddrFrom4([4]byte{10, origin, object, 2})}
							}
							if linkMode == 2 {
								link.LocalAddresses = []netip.Addr{netip.AddrFrom16([16]byte{0x20, 1, 0x0d, 0xb8, origin, object, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})}
								link.RemoteAddresses = []netip.Addr{netip.AddrFrom16([16]byte{0x20, 1, 0x0d, 0xb8, origin, object, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2})}
							}
							s.Links = append(s.Links, link)
						}
						for _, ranged := range []bool{false, true} {
							last := object
							attrs := []linkstateevents.TLV{{Type: 1158, Value: []byte{0x40, 1, 0, 0, 0, 0, 0, marker}}, {Type: 1170, Value: []byte{object}}}
							if tc.isis {
								source := []byte{192, 0, 2, origin}
								if tc.ipv6 {
									source = []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, origin}
								}
								attrs = append(attrs, linkstateevents.TLV{Type: 1171, Value: source})
							}
							if ranged {
								last += 10
								attrs = []linkstateevents.TLV{{Type: 1159, Value: []byte{0, 0, 0, object, 4, 0x86, 0, 8, 0x40, 1, 0, 0, 0, 0, 0, marker}}}
							}
							prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 0, last, 0}), 24)
							if tc.ipv6 {
								prefix = netip.PrefixFrom(netip.AddrFrom16([16]byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, last}), 64)
							}
							routes := []uint8{0}
							if !tc.isis && !ranged {
								routes = []uint8{1, 2, 3, 5}
							}
							for _, route := range routes {
								routeAttrs := attrs
								if route != 0 {
									routeAttrs = []linkstateevents.TLV{
										{Type: 1158, Value: []byte{0x40, 1, 0, 0, 0, 0, 0, marker + route*10}},
										{Type: 1170, Value: []byte{object}},
									}
								}
								s.Prefixes = append(s.Prefixes, linkstateevents.Prefix{Node: local,
									Prefix: prefix, Topology: topology, RouteType: route, Attributes: routeAttrs})
							}
						}
					}
				}
			}
			exporter, capture := exportFixture(t)
			// RFC 9085 Sections 2.2 and 2.3: use the real source-replacement and emission path.
			if err := exporter.replace(tc.name, s); err != nil {
				t.Fatal(err)
			}
			if err := exporter.reconcile(context.Background()); err != nil {
				t.Fatal(err)
			}
			wantCount := 42
			if !tc.isis {
				wantCount = 66
			}
			if len(capture.commands) != wantCount {
				t.Fatalf("announcements = %d, want %d", len(capture.commands), wantCount)
			}
			seen := make(map[string]bool)
			for _, command := range capture.commands {
				nlri := exportCommandBytes(t, command, "nlri")
				if len(nlri) < 13 {
					t.Fatalf("short NLRI %x", nlri)
				}
				if seen[string(nlri)] {
					t.Fatalf("duplicate NLRI %x", nlri)
				}
				seen[string(nlri)] = true
				if nlri[4] != byte(tc.protocol) {
					t.Fatalf("protocol = %d, want %d", nlri[4], tc.protocol)
				}
				local := rfc9085ExportOne(t, nlri[13:], 256)
				router := rfc9085ExportOne(t, local, 515)
				origin := router[len(router)-1]
				if origin != 2 && origin != 3 {
					t.Fatalf("unexpected origin %x", router)
				}
				if !bytes.Equal(router, rfc9085ExportNode(origin, tc.isis).RouterID) {
					t.Fatalf("corrupt origin %x", router)
				}
				want := make(map[uint16][]byte)
				kind := binary.BigEndian.Uint16(nlri[:2])
				if kind != 1 {
					topology := uint16(0)
					if found := exportTLVValues(t, nlri[13:], 263); len(found) != 0 {
						topology = binary.BigEndian.Uint16(rfc9085ExportOne(t, nlri[13:], 263))
					}
					if topology != 0 && topology != 7 {
						t.Fatalf("unexpected topology %d", topology)
					}
					if kind == 2 {
						remote := rfc9085ExportOne(t, rfc9085ExportOne(t, nlri[13:], 257), 515)
						wantRemote := []byte{4, 4, 4, 4}
						if tc.isis {
							wantRemote = []byte{0, 0, 0, 0, 0, 4}
						}
						if !bytes.Equal(remote, wantRemote) {
							t.Fatalf("wrong parallel-link remote %x, want %x", remote, wantRemote)
						}
						object, linkMode := rfc9085ExportLinkIdentity(t, nlri[13:], origin)
						marker := origin*20 + object + byte(topology) + linkMode*80
						want[1099] = []byte{0x60, marker, 0, 0, 0, 0x3e, marker}
						want[1100] = []byte{0x60, marker + 1, 0, 0, 4, 4, 4, 4, 0, 0x3f, marker}
						if tc.isis {
							want[1099][0] = 0x30
							want[1100] = []byte{0x30, marker + 1, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0x3f, marker}
						}
					} else if kind == 3 || kind == 4 {
						prefix := rfc9085ExportOne(t, nlri[13:], 265)
						wantHead := []byte{24, 192, 0}
						wantKind := uint16(3)
						if tc.ipv6 {
							wantHead = []byte{64, 0x20, 1, 0x0d, 0xb8, 0, 0, 0}
							wantKind = 4
						}
						if kind != wantKind || len(prefix) != len(wantHead)+1 {
							t.Fatalf("unexpected prefix type/length %d/%x", kind, prefix)
						}
						if !bytes.Equal(prefix[:len(wantHead)], wantHead) {
							t.Fatalf("unexpected prefix %x", prefix)
						}
						object := prefix[len(wantHead)]
						ranged := object >= 14
						if ranged {
							object -= 10
						}
						if object != 4 && object != 5 {
							t.Fatalf("unexpected prefix object %d", object)
						}
						marker := origin*20 + object + byte(topology)
						route := byte(0)
						routeDescriptors := exportTLVValues(t, nlri[13:], 264)
						if tc.isis || ranged {
							if len(routeDescriptors) != 0 {
								t.Fatalf("unexpected route-type descriptor %x", routeDescriptors)
							}
						} else {
							value := rfc9085ExportOne(t, nlri[13:], 264)
							if len(value) != 1 {
								t.Fatalf("route-type descriptor = %x, want one octet", value)
							}
							route = value[0]
							if route != 1 && route != 2 && route != 3 && route != 5 {
								t.Fatalf("unexpected OSPF route class %d", route)
							}
							marker += route * 10
						}
						if ranged {
							want[1159] = []byte{0, 0, 0, object, 4, 0x86, 0, 8, 0x40, 1, 0, 0, 0, 0, 0, marker}
						} else {
							want[1158] = []byte{0x40, 1, 0, 0, 0, 0, 0, marker}
							want[1170] = []byte{object}
							if tc.isis {
								want[1171] = []byte{192, 0, 2, origin}
								if tc.ipv6 {
									want[1171] = []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, origin}
								}
							}
						}
					} else {
						t.Fatalf("unexpected NLRI type %d", kind)
					}
				}
				attrs := exportCommandBytes(t, command, "attr")
				if len(attrs) < 11 {
					t.Fatalf("short attributes %x", attrs)
				}
				for _, typ := range []uint16{1099, 1100, 1172, 1158, 1159, 1170, 1171, 1174} {
					got := exportTLVValues(t, attrs[11:], typ)
					expected, present := want[typ]
					if !present {
						if len(got) != 0 {
							t.Fatalf("NLRI %x unexpectedly has TLV %d: %x", nlri, typ, got)
						}
						continue
					}
					if len(got) != 1 {
						t.Fatalf("NLRI %x TLV %d count = %d", nlri, typ, len(got))
					}
					if !bytes.Equal(got[0], expected) {
						t.Fatalf("NLRI %x TLV %d = %x, want %x", nlri, typ, got[0], expected)
					}
				}
			}
		})
	}
}

func rfc9085ExportNode(id byte, isis bool) linkstateevents.NodeID {
	if isis {
		return linkstateevents.NodeID{RouterID: []byte{0, 0, 0, 0, 0, id}}
	}
	return linkstateevents.NodeID{RouterID: []byte{id, id, id, id}, HasArea: true}
}

func rfc9085ExportOne(t *testing.T, wire []byte, typ uint16) []byte {
	t.Helper()
	values := exportTLVValues(t, wire, typ)
	if len(values) != 1 {
		t.Fatalf("TLV %d count = %d, want 1", typ, len(values))
	}
	return values[0]
}

// rfc9085ExportLinkIdentity compares complete descriptor bytes against independent
// literals, including absence of alternate descriptor families. The two allowed
// object IDs share endpoints and topology, but carry different SID attributes.
func rfc9085ExportLinkIdentity(t *testing.T, wire []byte, origin byte) (byte, byte) {
	t.Helper()
	for _, object := range []byte{4, 5} {
		for mode := byte(0); mode < 3; mode++ {
			want := map[uint16][]byte{258: {0, 0, 0, object, 0, 0, 0, origin + 10}}
			if mode == 1 {
				want = map[uint16][]byte{259: {10, origin, object, 1}, 260: {10, origin, object, 2}}
			}
			if mode == 2 {
				want = map[uint16][]byte{
					261: {0x20, 1, 0x0d, 0xb8, origin, object, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
					262: {0x20, 1, 0x0d, 0xb8, origin, object, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2},
				}
			}
			matches := true
			for _, typ := range []uint16{258, 259, 260, 261, 262} {
				values := exportTLVValues(t, wire, typ)
				expected, present := want[typ]
				if !present {
					if len(values) != 0 {
						matches = false
					}
					continue
				}
				if len(values) != 1 {
					matches = false
					continue
				}
				if !bytes.Equal(values[0], expected) {
					matches = false
				}
			}
			if matches {
				return object, mode
			}
		}
	}
	t.Fatalf("no complete expected parallel-link descriptor identity in %x", wire)
	return 0, 0
}
