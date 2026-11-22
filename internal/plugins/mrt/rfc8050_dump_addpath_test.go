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
// emits TDV2RIBIPv4UnicastAP records carrying the distinct Path Identifiers
// (7 and 42) supplied by OnRouteAddPath for the same prefix and peer.
// RFC requirement: RFC8050-4.1-1 negative -- writeTableDumpV2 with AddPath=false
// emits base TDV2RIBIPv4Unicast records with no Path Identifier field on the
// wire (decoded PathID 0).
func TestRFC8050DumpV2AddPathStoresPathID(t *testing.T) {
	t.Parallel()

	run := func(addPath bool) ([]mrtfmt.Header, []*mrtfmt.RIBRecord) {
		c := New(Config{AddPath: addPath}, nil)
		path := filepath.Join(t.TempDir(), "rib.mrt")
		c.routes = mrtfmt.NewWriter(path)
		c.ribDumper = fakeRIBDumper{fn: func(v registry.RIBDumpVisitor) {
			idx := v.OnPeer("192.0.2.1", 65001, [4]byte{1, 2, 3, 4}, false)
			v.OnRouteAddPath(idx, mrtfmt.AFIIPv4, 1, 7, 24, []byte{203, 0, 113}, testWireASPath4())
			v.OnRouteAddPath(idx, mrtfmt.AFIIPv4, 1, 42, 24, []byte{203, 0, 113}, testWireASPath4())
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
	if apHeaders[1].Subtype != mrtfmt.TDV2RIBIPv4UnicastAP || apHeaders[2].Subtype != mrtfmt.TDV2RIBIPv4UnicastAP {
		t.Fatalf("addPath=true: subtypes = (%d, %d), want TDV2RIBIPv4UnicastAP (%d)",
			apHeaders[1].Subtype, apHeaders[2].Subtype, mrtfmt.TDV2RIBIPv4UnicastAP)
	}
	if apRIBs[0].Entries[0].PathID != 7 || apRIBs[1].Entries[0].PathID != 42 {
		t.Fatalf("addPath=true: decoded PathIDs = (%d, %d), want (7, 42)",
			apRIBs[0].Entries[0].PathID, apRIBs[1].Entries[0].PathID)
	}

	// Negative: ADD-PATH disabled uses base TDV2RIBIPv4Unicast with no wire PathID.
	baseHeaders, baseRIBs := run(false)
	if len(baseHeaders) != 3 || len(baseRIBs) != 2 {
		t.Fatalf("addPath=false: got %d headers and %d RIB records, want 3 and 2", len(baseHeaders), len(baseRIBs))
	}
	if baseHeaders[1].Subtype != mrtfmt.TDV2RIBIPv4Unicast {
		t.Fatalf("addPath=false: subtype = %d, want TDV2RIBIPv4Unicast (%d)",
			baseHeaders[1].Subtype, mrtfmt.TDV2RIBIPv4Unicast)
	}
	if baseRIBs[0].Entries[0].PathID != 0 || baseRIBs[1].Entries[0].PathID != 0 {
		t.Fatalf("addPath=false: decoded PathIDs = (%d, %d), want (0, 0)",
			baseRIBs[0].Entries[0].PathID, baseRIBs[1].Entries[0].PathID)
	}
}
