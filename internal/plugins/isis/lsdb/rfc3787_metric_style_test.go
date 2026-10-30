// Design: docs/architecture/isis/isis-6-lsdb.md -- the metric style of the own and pseudo-node LSPs.
// Related: pseudonode_test.go -- pnMembers, decodePseudonodeFrag0
//
// VALIDATES: the Originator's metric style (RFC 3787 section 5, "If not all
// devices in the IS-IS domain support wide metrics, narrow metrics MUST continue
// to be used."): a narrow style lists a LAN's members in TLV 2 in the
// pseudo-node LSP, so a narrow-only router can cross the LAN; a run of TLV 2
// entries that overflows one TLV opens a second TLV 2 with its own Virtual Flag
// octet; SetMetricStyle refuses a value that is not a style.
// PREVENTS: a pseudo-node that only a wide-metric router can traverse, and a
// TLV 2 continuation whose first neighbor octet is read as the Virtual Flag.

package lsdb

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// narrowMembers decodes every TLV 2 of lsp into a member -> metric map.
func narrowMembers(t *testing.T, lsp packet.LSP) map[types.SourceID]uint8 {
	t.Helper()
	out := map[types.SourceID]uint8{}
	for _, tlv := range lsp.TLVs {
		if tlv.Type != packet.TLVISReachabilityNarrow {
			continue
		}
		decoded, err := packet.DecodeNarrowISReachTLV(tlv.Value)
		if err != nil {
			t.Fatalf("TLV 2 does not decode: %v", err)
		}
		if decoded.VirtualFlag != 0 {
			t.Fatalf("TLV 2 virtual flag = %d, want 0", decoded.VirtualFlag)
		}
		for _, e := range decoded.Entries {
			out[e.Neighbor] = e.DefaultMetricValue
		}
	}
	return out
}

// RFC requirement: RFC3787-5-1 positive -- under the narrow metric style the
// pseudo-node LSP lists every LAN member in TLV 2 at metric 0 and carries no
// TLV 22; under transition it lists them in both TLV 2 and TLV 22.
func TestRFC3787NarrowMetricStylePseudonodeListsMembersInTLV2(t *testing.T) {
	for _, style := range []MetricStyle{MetricStyleNarrow, MetricStyleTransition} {
		t.Run(style.String(), func(t *testing.T) {
			d := New(nil)
			o := NewOriginator(d, nil)
			if err := o.SetMetricStyle(style); err != nil {
				t.Fatal(err)
			}
			const pnid = 0x07
			o.OriginatePseudonode(Level1, PseudonodeInfo{
				SystemID: testSys(0x01), PseudonodeID: pnid,
				Members: pnMembers(0x01, 0x02, 0x03), MaxLifetime: 1200,
			})
			lsp := decodePseudonodeFrag0(t, d, Level1, testSys(0x01), pnid)
			narrow := narrowMembers(t, lsp)
			for _, m := range pnMembers(0x01, 0x02, 0x03) {
				metric, ok := narrow[types.NewSourceID(m, 0)]
				if !ok {
					t.Fatalf("pseudo-node TLV 2 omits member %s", m)
				}
				if metric != 0 {
					t.Fatalf("pseudo-node TLV 2 member %s metric = %d, want 0", m, metric)
				}
			}
			wide := pseudonodeMembers(t, lsp)
			if style == MetricStyleNarrow && len(wide) != 0 {
				t.Fatalf("narrow pseudo-node LSP carries %d TLV 22 members, want none", len(wide))
			}
			if style == MetricStyleTransition && len(wide) != 3 {
				t.Fatalf("transition pseudo-node LSP carries %d TLV 22 members, want 3", len(wide))
			}
		})
	}
}

// TestNarrowISReachOverflowRepeatsVirtualFlag: 30 neighbors exceed the 23
// entries one TLV 2 value holds ((255 - 1) / 11), so the packer opens a second
// TLV 2, and each TLV 2 starts with its own Virtual Flag octet: every neighbor
// decodes from either TLV, none is lost or misaligned.
func TestNarrowISReachOverflowRepeatsVirtualFlag(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	if err := o.SetMetricStyle(MetricStyleNarrow); err != nil {
		t.Fatal(err)
	}
	metric, err := types.NewMetric(7)
	if err != nil {
		t.Fatal(err)
	}
	var state LevelState
	for i := range 30 {
		state.Neighbors = append(state.Neighbors, AdjacencyInfo{
			Neighbor: types.NewSourceID(testSys(byte(0x10+i)), 0), Metric: metric,
		})
	}
	o.Originate(Level1, NodeInfo{SystemID: testSys(1), AdvertiseIPv4: true, MaxLifetime: 1200}, state)

	entry := d.Lookup(Level1, types.NewLSPID(types.NewSourceID(testSys(1), 0), 0))
	if entry == nil {
		t.Fatal("no own fragment zero")
	}
	lsp, err := entry.Decode()
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(lsp.TLVs)
	count := 0
	for _, tlv := range lsp.TLVs {
		if tlv.Type == packet.TLVISReachabilityNarrow {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("own LSP carries %d TLV 2, want 2 for 30 neighbors", count)
	}
	got := narrowMembers(t, lsp)
	if len(got) != 30 {
		t.Fatalf("decoded %d TLV 2 neighbors, want 30", len(got))
	}
	for _, n := range state.Neighbors {
		if got[n.Neighbor] != 7 {
			t.Fatalf("neighbor %s metric = %d, want 7", n.Neighbor, got[n.Neighbor])
		}
	}
}

// TestSetMetricStyleRefusesNonStyle: Unspecified and an out-of-range value are
// refused with ErrMetricStyle, and the Originator keeps the style it held.
func TestSetMetricStyleRefusesNonStyle(t *testing.T) {
	o := NewOriginator(New(nil), nil)
	for _, style := range []MetricStyle{MetricStyleUnspecified, MetricStyleWide + 1} {
		if err := o.SetMetricStyle(style); !errors.Is(err, ErrMetricStyle) {
			t.Fatalf("SetMetricStyle(%d) = %v, want ErrMetricStyle", style, err)
		}
	}
	if o.metricStyle != MetricStyleWide {
		t.Fatalf("style after refusals = %s, want the constructor's wide", o.metricStyle)
	}
}
