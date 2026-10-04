// Design: server_inventory.go -- pooled scratch for route-server inventory extraction.

package rs

import (
	"encoding/hex"
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestInventoryGroupedExtractionAllocations measures real grouped extraction
// against its parser cost. Each retained wire hex needs one owned string; CIDR
// scratch must not add an allocation per route. Record checks in every measured
// extraction prevent dropping routes from satisfying the allocation bound.
func TestInventoryGroupedExtractionAllocations(t *testing.T) {
	const count = 16
	for _, tc := range []struct {
		name string
		fam  family.Family
		wire []byte
		cidr netip.Prefix
	}{
		{"labeled", family.Family{AFI: 1, SAFI: 4}, []byte{32, 0, 6, 65, 10}, netip.MustParsePrefix("10.0.0.0/8")},
		{"bgpls", family.Family{AFI: 16388, SAFI: 71}, []byte{0, 99, 0, 4, 0xde, 0xad, 0xbe, 0xef}, netip.Prefix{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := []byte{byte(tc.fam.AFI >> 8), byte(tc.fam.AFI), byte(tc.fam.SAFI), 4, 192, 0, 2, 1, 0}
			for range count {
				value = append(value, tc.wire...)
			}
			attrs := append([]byte{0x80, 14, byte(len(value))}, value...)
			body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
			ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			if err != nil {
				t.Fatal(err)
			}
			wu := wireu.NewWireUpdate(body, ctxID)
			attrsWire, err := wu.Attrs()
			if err != nil {
				t.Fatal(err)
			}
			msg := &bgptypes.RawMessage{WireUpdate: wu, AttrsWire: attrsWire}
			mp, err := wu.MPReach()
			if err != nil {
				t.Fatal(err)
			}
			parseAllocs := testing.AllocsPerRun(100, func() {
				nlris, err := mp.NLRIs(false)
				if err != nil {
					t.Fatal(err)
				}
				if len(nlris) != count {
					t.Fatalf("parsed %d routes, want %d", len(nlris), count)
				}
			})
			wantHex := hex.EncodeToString(tc.wire)
			allocs := testing.AllocsPerRun(100, func() {
				records := extractWireNLRIRecords(msg)
				if records == nil {
					t.Fatal("no inventory extracted")
				}
				defer returnNLRIRecords(records)
				if len(records.records) != count {
					t.Fatalf("extracted %d routes, want %d", len(records.records), count)
				}
				for _, rec := range records.records {
					if rec.fam != tc.fam || rec.action != actionAdd || !rec.wireForm || rec.addPath {
						t.Fatalf("wrong inventory identity: %+v", rec)
					}
					if rec.nlriStr != wantHex || rec.prefix != tc.cidr || rec.cidrKeyed != tc.cidr.IsValid() || rec.pathID != 0 {
						t.Fatalf("lost wire bytes or CIDR identity: %+v", rec)
					}
				}
			})
			// Allow two fixed extraction allocations, not a scratch per NLRI.
			wantMax := parseAllocs + count + 2
			if allocs > wantMax {
				t.Fatalf("grouped extraction allocated %.0f times, want at most %.0f (parser %.0f + %d owned hex strings + 2 fixed)", allocs, wantMax, parseAllocs, count)
			}
		})
	}
}
