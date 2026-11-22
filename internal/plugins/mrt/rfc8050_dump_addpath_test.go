// Design: docs/architecture/mrt.md -- TABLE_DUMP_V2 ADD-PATH RIB snapshot writer
// RFC: rfc/short/rfc8050.md -- Section 4.1, RIB Entries with Path Identifier
// Related: dump.go -- Component.writeTableDumpV2

package mrt

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

// TestRFC8050DumpV2AddPathStoresPathID verifies that Component.writeTableDumpV2,
// on the default configuration, writes a route the RIB hands to OnRouteAddPath
// (a family stored with ADD-PATH) under the ADD-PATH subtype with its Path
// Identifier in the RIB Entry, and a route the RIB hands to OnRoute (a family
// stored without ADD-PATH) under the base subtype with no Path Identifier field.
//
// RFC 8050 Section 4.1: "This adds a field to the RIB Entries record to store
// the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH,
// RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and
// RIB_IPV6_MULTICAST_ADDPATH subtypes."
//
// VALIDATES: two paths for one prefix (path IDs 7 and 42) given to
// OnRouteAddPath decode with the ADD-PATH subtype and PathIDs 7 and 42, and
// each record body is exactly 4 octets longer than the base layout; the same
// prefix given to OnRoute is written under the base subtype at the base length.
// PREVENTS: the subtype following an operator switch instead of the route, so
// that the default configuration dropped every Path Identifier.
//
// RFC requirement: RFC8050-4.1-1 positive -- on the default configuration a
// route given to OnRouteAddPath is written as TDV2RIBIPv4UnicastAP (8),
// TDV2RIBIPv4MulticastAP (9), TDV2RIBIPv6UnicastAP (10) or
// TDV2RIBIPv6MulticastAP (11) whose RIB Entry carries the Path Identifier (7
// and 42), the record body 4 octets longer than the base layout.
// RFC requirement: RFC8050-4.1-1 negative -- on the default configuration a
// route given to OnRoute is written under the base subtype (2, 3, 4, 5) whose
// record body has the base length, with no Path Identifier field.
func TestRFC8050DumpV2AddPathStoresPathID(t *testing.T) {
	t.Parallel()

	families := []struct {
		name        string
		afi, safi   uint16
		prefixLen   uint8
		prefix      []byte
		wantAPSub   uint16
		wantBaseSub uint16
	}{
		{"ipv4-unicast", mrtfmt.AFIIPv4, 1, 24, []byte{203, 0, 113}, mrtfmt.TDV2RIBIPv4UnicastAP, mrtfmt.TDV2RIBIPv4Unicast},
		{"ipv4-multicast", mrtfmt.AFIIPv4, 2, 24, []byte{233, 252, 0}, mrtfmt.TDV2RIBIPv4MulticastAP, mrtfmt.TDV2RIBIPv4Multicast},
		{"ipv6-unicast", mrtfmt.AFIIPv6, 1, 48, []byte{0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}, mrtfmt.TDV2RIBIPv6UnicastAP, mrtfmt.TDV2RIBIPv6Unicast},
		{"ipv6-multicast", mrtfmt.AFIIPv6, 2, 48, []byte{0xff, 0x3e, 0x00, 0x00, 0x00, 0x01}, mrtfmt.TDV2RIBIPv6MulticastAP, mrtfmt.TDV2RIBIPv6Multicast},
	}

	for _, tc := range families {
		t.Run(tc.name, func(t *testing.T) {
			attrs := testWireASPath4()
			// seq(4) + prefix length(1) + prefix + entry count(2) + peer
			// index(2) + originated time(4) + attribute length(2) + attributes
			baseLen := 4 + 1 + len(tc.prefix) + 2 + 2 + 4 + 2 + len(attrs)

			run := func(addPath bool) ([]mrtfmt.Header, [][]byte, []*mrtfmt.RIBRecord) {
				c := New(Config{}, nil)
				path := filepath.Join(t.TempDir(), "rib.mrt")
				c.routes = mrtfmt.NewWriter(path)
				c.ribDumper = fakeRIBDumper{fn: func(v registry.RIBDumpVisitor) {
					idx := v.OnPeer("192.0.2.1", 65001, [4]byte{1, 2, 3, 4}, false)
					if addPath {
						v.OnRouteAddPath(idx, tc.afi, tc.safi, 7, tc.prefixLen, tc.prefix, attrs)
						v.OnRouteAddPath(idx, tc.afi, tc.safi, 42, tc.prefixLen, tc.prefix, attrs)
						return
					}
					v.OnRoute(idx, tc.afi, tc.safi, tc.prefixLen, tc.prefix, attrs)
					v.OnRoute(idx, tc.afi, tc.safi, tc.prefixLen, tc.prefix, attrs)
				}}
				c.writeTableDumpV2()
				if err := c.routes.Close(); err != nil {
					t.Fatalf("close routes writer: %v", err)
				}
				var headers []mrtfmt.Header
				var bodies [][]byte
				var ribs []*mrtfmt.RIBRecord
				h := &mrtfmt.Handler{
					OnHeader: func(hd mrtfmt.Header, _ uint32, data []byte) error {
						headers = append(headers, hd)
						bodies = append(bodies, bytes.Clone(data))
						return nil
					},
					OnRIB: func(_ mrtfmt.Header, r *mrtfmt.RIBRecord) error {
						ribs = append(ribs, r)
						return nil
					},
				}
				if err := mrtfmt.ReadFile(path, h); err != nil {
					t.Fatalf("read back dump: %v", err)
				}
				return headers, bodies, ribs
			}

			// Positive: an ADD-PATH route keeps its Path Identifier on the default config.
			apHeaders, apBodies, apRIBs := run(true)
			if len(apHeaders) != 3 || len(apRIBs) != 2 {
				t.Fatalf("OnRouteAddPath: got %d headers and %d RIB records, want 3 and 2", len(apHeaders), len(apRIBs))
			}
			if apHeaders[1].Subtype != tc.wantAPSub || apHeaders[2].Subtype != tc.wantAPSub {
				t.Fatalf("OnRouteAddPath: subtypes = (%d, %d), want %d",
					apHeaders[1].Subtype, apHeaders[2].Subtype, tc.wantAPSub)
			}
			if len(apBodies[1]) != baseLen+4 || len(apBodies[2]) != baseLen+4 {
				t.Fatalf("OnRouteAddPath: record bodies = (%d, %d) octets, want %d (base layout plus the Path Identifier)",
					len(apBodies[1]), len(apBodies[2]), baseLen+4)
			}
			if apRIBs[0].Entries[0].PathID != 7 || apRIBs[1].Entries[0].PathID != 42 {
				t.Fatalf("OnRouteAddPath: decoded PathIDs = (%d, %d), want (7, 42)",
					apRIBs[0].Entries[0].PathID, apRIBs[1].Entries[0].PathID)
			}

			// Negative: a route stored without ADD-PATH carries no Path Identifier field.
			baseHeaders, baseBodies, baseRIBs := run(false)
			if len(baseHeaders) != 3 || len(baseRIBs) != 2 {
				t.Fatalf("OnRoute: got %d headers and %d RIB records, want 3 and 2", len(baseHeaders), len(baseRIBs))
			}
			if baseHeaders[1].Subtype != tc.wantBaseSub || baseHeaders[2].Subtype != tc.wantBaseSub {
				t.Fatalf("OnRoute: subtypes = (%d, %d), want %d",
					baseHeaders[1].Subtype, baseHeaders[2].Subtype, tc.wantBaseSub)
			}
			if len(baseBodies[1]) != baseLen || len(baseBodies[2]) != baseLen {
				t.Fatalf("OnRoute: record bodies = (%d, %d) octets, want %d (no Path Identifier field)",
					len(baseBodies[1]), len(baseBodies[2]), baseLen)
			}
		})
	}
}
