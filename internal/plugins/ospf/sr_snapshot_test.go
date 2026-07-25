// VALIDATES: spec-ospf-ext-5 AC-17 -- `show ospf segment-routing` /
// `show ospf ipv6 segment-routing` render the configured SRGB/SRLB, this node's
// Prefix-SIDs and Adj-SIDs, and the advertised algorithm.
// PREVENTS: a show command that omits configured SR state.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestSRSnapshot(t *testing.T) {
	srTestReset(t)
	router := types.RouterID{10, 0, 0, 1}
	srWire.set(router, sr.SRConfig{
		Enabled:  true,
		SRGB:     []sr.LabelRange{{Base: 16000, Size: 8000}},
		SRLB:     []sr.LabelRange{{Base: 40000, Size: 1000}},
		Prefixes: []sr.PrefixSIDConfig{{Prefix: netip.MustParsePrefix("10.0.0.1/32"), Index: 1, NodeSID: true}},
	})
	srWire.setAdj(router, [4]byte{10, 0, 12, 1}, sr.AdjSID{Flags: sr.AdjSIDFlags{V: true, L: true}, Label: 40001})

	e := &engine{cfg: ospfConfig{RouterID: router}}
	snap := e.srSnapshot(interfaceFamilyIPv4)
	if !snap.Enabled {
		t.Fatalf("snapshot must report enabled")
	}
	if len(snap.SRGB) != 1 || snap.SRGB[0].LowerBound != 16000 || snap.SRGB[0].UpperBound != 23999 {
		t.Fatalf("SRGB view = %+v", snap.SRGB)
	}
	if len(snap.SRLB) != 1 || snap.SRLB[0].LowerBound != 40000 {
		t.Fatalf("SRLB view = %+v", snap.SRLB)
	}
	if len(snap.PrefixSIDs) != 1 || snap.PrefixSIDs[0].Index != 1 {
		t.Fatalf("prefix-SID view = %+v", snap.PrefixSIDs)
	}
	if len(snap.AdjSIDs) != 1 || snap.AdjSIDs[0].Label != 40001 {
		t.Fatalf("adj-SID view = %+v", snap.AdjSIDs)
	}
	if len(snap.Algorithms) != 1 || snap.Algorithms[0] != 0 {
		t.Fatalf("algorithm view = %+v", snap.Algorithms)
	}
}

func TestSRSnapshotDisabled(t *testing.T) {
	srTestReset(t)
	e := &engine{cfg: ospfConfig{RouterID: types.RouterID{10, 0, 0, 9}}}
	snap := e.srSnapshot(interfaceFamilyIPv4)
	if snap.Enabled {
		t.Fatalf("unconfigured router must report SR disabled")
	}
}
