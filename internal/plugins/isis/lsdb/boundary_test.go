// Design: plan/spec-isis-6-lsdb.md -- boundary tests for the numeric LSDB inputs.
//
// VALIDATES the spec "Boundary Tests" table at the LSDB layer:
//   - LSP sequence number 1..0xFFFFFFFF (1 first, 0 reserved, wrap triggers purge)
//   - Remaining lifetime 0..65535 (0 is the purge signal, valid)
//   - LSP fragment number 0..255
//   - IS-reachability metric (TLV 22, 24-bit) and IPv4 prefix metric (TLV 135,
//     32-bit) carried at their extremes without truncation
// The type-level range rejection (NewMetric > 24-bit) is owned by isis-1; here
// we confirm the LSDB store and origination handle the boundary values.

package lsdb

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

func TestISISLSDBSequenceBoundaries(t *testing.T) {
	d := New(nil)
	id := lspID(1, 0)

	// Sequence 1 (first valid) stores.
	first, fraw := buildLSP(t, packet.PDUTypeL2LSP, id, types.FirstSequenceNumber, 1000, nil)
	if r := d.Receive(Level2, first, fraw, false); !r.Stored {
		t.Fatalf("sequence 1 (first valid) not stored: %+v", r)
	}

	// Sequence 0xFFFFFFFF (last valid) is newer and stores.
	last, lraw := buildLSP(t, packet.PDUTypeL2LSP, id, types.MaxSequenceNumber, 1000, nil)
	if r := d.Receive(Level2, last, lraw, false); !r.Stored || r.Freshness != Newer {
		t.Fatalf("sequence 0xFFFFFFFF not accepted as newer: %+v", r)
	}
	if got := d.Lookup(Level2, id).Sequence(); got != types.MaxSequenceNumber {
		t.Errorf("stored sequence = %#x, want 0xFFFFFFFF", uint32(got))
	}
}

func TestISISLSDBLifetimeBoundaries(t *testing.T) {
	d := New(nil)
	id := lspID(2, 0)

	// Lifetime 65535 (max) stores and is not purged.
	maxLife, mraw := buildLSP(t, packet.PDUTypeL2LSP, id, 1, 65535, nil)
	d.Receive(Level2, maxLife, mraw, false)
	if e := d.Lookup(Level2, id); e == nil || e.IsPurged() {
		t.Fatalf("lifetime 65535 entry missing or wrongly purged")
	}
	if got := d.Lookup(Level2, id).Lifetime().Seconds(); got != 65535 {
		t.Errorf("stored lifetime = %d, want 65535", got)
	}

	// Lifetime 0 (the purge signal, a valid boundary) marks the entry purged.
	zero, zraw := buildLSP(t, packet.PDUTypeL2LSP, id, 2, 0, nil)
	d.Receive(Level2, zero, zraw, false)
	if e := d.Lookup(Level2, id); e == nil || !e.IsPurged() {
		t.Errorf("lifetime 0 not treated as a purge")
	}
}

func TestISISLSDBFragmentNumberBoundaries(t *testing.T) {
	d := New(nil)
	sys := testSys(3)

	// Store fragment 0 (min) and fragment 255 (max) for the same Source ID; both
	// are valid LSP numbers (ISO/IEC 10589: 0..255) and stored independently.
	for _, frag := range []byte{0, 255} {
		id := lspID(3, frag)
		lsp, raw := buildLSP(t, packet.PDUTypeL2LSP, id, 1, 1000, nil)
		d.Insert(Level2, lsp, raw)
	}
	if d.Lookup(Level2, lspID(3, 0)) == nil {
		t.Error("fragment 0 missing")
	}
	if e := d.Lookup(Level2, lspID(3, 255)); e == nil {
		t.Error("fragment 255 missing")
	} else if e.LSPID().LSPNumber() != 255 {
		t.Errorf("fragment number = %d, want 255", e.LSPID().LSPNumber())
	}
	_ = sys
}

func TestISISOriginateMetricBoundaries(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)

	// TLV 22 at the 24-bit metric maximum, TLV 135 at the 32-bit prefix-metric
	// maximum: both must round-trip through origination without truncation.
	maxIS, err := types.NewMetric(types.MaxMetric) // 16777215
	if err != nil {
		t.Fatalf("NewMetric(max): %v", err)
	}
	state := LevelState{
		Neighbors: []AdjacencyInfo{{Neighbor: types.NewSourceID(testSys(2), 0), Metric: maxIS}},
		Prefixes: []PrefixInfo{{
			Prefix: netip.MustParsePrefix("203.0.113.0/24"),
			Metric: types.NewPrefixMetric(types.MaxPrefixMetric), // 4294967295
		}},
	}
	o.Originate(Level2, node, state)
	lsp := decodeOwnFrag0(t, d, Level2, node.SystemID)

	for _, tl := range lsp.TLVs {
		switch tl.Type {
		case packet.TLVExtendedISReach:
			ext, err := packet.DecodeExtendedISReachTLV(tl.Value)
			if err != nil {
				t.Fatalf("decode TLV 22: %v", err)
			}
			if ext.Entries[0].Metric.Value() != types.MaxMetric {
				t.Errorf("TLV 22 metric = %d, want %d (24-bit max)", ext.Entries[0].Metric.Value(), types.MaxMetric)
			}
		case packet.TLVExtendedIPReach:
			ext, err := packet.DecodeExtendedIPReachTLV(tl.Value)
			if err != nil {
				t.Fatalf("decode TLV 135: %v", err)
			}
			if ext.Entries[0].Metric.Value() != types.MaxPrefixMetric {
				t.Errorf("TLV 135 metric = %d, want %d (32-bit max)", ext.Entries[0].Metric.Value(), types.MaxPrefixMetric)
			}
		}
	}
}
