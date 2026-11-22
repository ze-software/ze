// Design: docs/architecture/mrt.md -- TABLE_DUMP_V2 ADD-PATH RIB snapshot writer
// RFC: rfc/short/rfc8050.md -- Section 4.1, RIB Entries with Path Identifier
// Related: dump.go -- Component.writeTableDumpV2

package mrt

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

// TestRFC8050DumpV2AddPathStoresPathID verifies that Component.writeTableDumpV2
// writes each route's Path Identifier into RIB_IPV4_UNICAST_ADDPATH entries
// when Config.AddPath is enabled, and omits the Path Identifier field in base
// RIB_IPV4_UNICAST entries when Config.AddPath is disabled.
//
// RFC 8050 Section 4.1: "This adds a field to the RIB Entries record to store
// the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH,
// RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and
// RIB_IPV6_MULTICAST_ADDPATH subtypes."
//
// VALIDATES: two paths for 203.0.113.0/24 (path IDs 7 and 42) written through
// writeTableDumpV2 with Config.AddPath=true decode with subtype
// TDV2RIBIPv4UnicastAP and distinct PathIDs 7 and 42; with Config.AddPath=false
// the record uses TDV2RIBIPv4Unicast and PathID 0.
// PREVENTS: writeTableDumpV2 discarding PathID and writing 0 for every entry.
//
// RFC requirement: RFC8050-4.1-1 positive -- writeTableDumpV2 with AddPath=true
// emits TDV2RIBIPv4UnicastAP (8), TDV2RIBIPv4MulticastAP (9),
// TDV2RIBIPv6UnicastAP (10), and TDV2RIBIPv6MulticastAP (11) records carrying
// the distinct Path Identifiers (7 and 42) supplied by OnRouteAddPath.
// RFC requirement: RFC8050-4.1-1 negative -- writeTableDumpV2 with AddPath=false
// emits the four base subtypes (2, 3, 4, 5) with no Path Identifier field on
// the wire (decoded PathID 0).
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
			run := func(addPath bool) ([]mrtfmt.Header, []*mrtfmt.RIBRecord) {
				c := New(Config{AddPath: addPath}, nil)
				path := filepath.Join(t.TempDir(), "rib.mrt")
				c.routes = mrtfmt.NewWriter(path)
				c.ribDumper = fakeRIBDumper{fn: func(v registry.RIBDumpVisitor) {
					idx := v.OnPeer("192.0.2.1", 65001, [4]byte{1, 2, 3, 4}, false)
					v.OnRouteAddPath(idx, tc.afi, tc.safi, 7, tc.prefixLen, tc.prefix, testWireASPath4())
					v.OnRouteAddPath(idx, tc.afi, tc.safi, 42, tc.prefixLen, tc.prefix, testWireASPath4())
				}}
				c.writeTableDumpV2()
				if err := c.routes.Close(); err != nil {
					t.Fatalf("close routes writer: %v", err)
				}
				var headers []mrtfmt.Header
				var ribs []*mrtfmt.RIBRecord
				h := &mrtfmt.Handler{
					OnHeader: func(hd mrtfmt.Header, _ uint32, _ []byte) error {
						headers = append(headers, hd)
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
				return headers, ribs
			}

			// Positive: ADD-PATH enabled stores distinct Path Identifiers 7 and 42.
			apHeaders, apRIBs := run(true)
			if len(apHeaders) != 3 || len(apRIBs) != 2 {
				t.Fatalf("addPath=true: got %d headers and %d RIB records, want 3 and 2", len(apHeaders), len(apRIBs))
			}
			if apHeaders[1].Subtype != tc.wantAPSub || apHeaders[2].Subtype != tc.wantAPSub {
				t.Fatalf("addPath=true: subtypes = (%d, %d), want %d",
					apHeaders[1].Subtype, apHeaders[2].Subtype, tc.wantAPSub)
			}
			if apRIBs[0].Entries[0].PathID != 7 || apRIBs[1].Entries[0].PathID != 42 {
				t.Fatalf("addPath=true: decoded PathIDs = (%d, %d), want (7, 42)",
					apRIBs[0].Entries[0].PathID, apRIBs[1].Entries[0].PathID)
			}

			// Negative: ADD-PATH disabled uses base subtype with no wire PathID.
			baseHeaders, baseRIBs := run(false)
			if len(baseHeaders) != 3 || len(baseRIBs) != 2 {
				t.Fatalf("addPath=false: got %d headers and %d RIB records, want 3 and 2", len(baseHeaders), len(baseRIBs))
			}
			if baseHeaders[1].Subtype != tc.wantBaseSub {
				t.Fatalf("addPath=false: subtype = %d, want %d", baseHeaders[1].Subtype, tc.wantBaseSub)
			}
			if baseRIBs[0].Entries[0].PathID != 0 || baseRIBs[1].Entries[0].PathID != 0 {
				t.Fatalf("addPath=false: decoded PathIDs = (%d, %d), want (0, 0)",
					baseRIBs[0].Entries[0].PathID, baseRIBs[1].Entries[0].PathID)
			}
		})
	}
}
