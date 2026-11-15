//go:build !race

// Design: docs/architecture/bgp/egress-attribute-rules.md -- bounded source retirement allocation.
// Related: filter_atomic_subset_test.go -- race-enabled semantic and ownership controls.
package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestAtomicAggregateSubsetNoRemovalAllocations keeps native identities while
// changing attributes, so the full warmed comparison returns no owned body.
func TestAtomicAggregateSubsetNoRemovalAllocations(t *testing.T) {
	fam := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	ctx := atomicPolicyContext(t, fam, false)
	a, b := atomicPolicyNativeFamilies(1)[fam], atomicPolicyNativeFamilies(2)[fam]
	input := atomicPolicyMP(fam, bytes.Join([][]byte{a, b}, nil), true)
	attrs := append(bytes.Clone(input[4:]), 0x40, 1, 1, 2)
	output := makeUpdateBody(nil, attrs, nil)
	allocations := testing.AllocsPerRun(100, func() {
		bodies, err := atomicAggregatePolicyRemovals(input, output, ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(bodies) != 0 {
			t.Fatal("attribute-only policy retired a native identity")
		}
	})
	if allocations != 0 {
		t.Fatalf("warmed attribute-only retirement check allocates %.0f times", allocations)
	}
}

// TestAtomicAggregateSubsetOwnedAllocationBound contrasts two routes with a
// legal extended 4097-route input. One owned body and its result slice require
// two allocations in either case, independent of the number of removed paths.
func TestAtomicAggregateSubsetOwnedAllocationBound(t *testing.T) {
	receive := bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{
		ASN4: true, ExtendedMessageRecv: true,
	}, bgpctx.DirectionRecv)
	ctx, err := bgpctx.Registry.Register(receive)
	if err != nil {
		t.Fatal(err)
	}
	for _, routes := range []int{2, 4097} {
		raw := make([]byte, routes*5)
		for i := range routes {
			off := i * 5
			raw[off], raw[off+1] = 32, 10
			raw[off+2], raw[off+3], raw[off+4] = byte(i>>16), byte(i>>8), byte(i)
		}
		input := atomicPolicyLegacy(raw, true)
		output := atomicPolicyLegacy(raw[len(raw)-5:], true)
		allocations := testing.AllocsPerRun(100, func() {
			bodies, err := atomicAggregatePolicyRemovals(input, output, ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(bodies) != 1 {
				t.Fatal("source subset did not produce exactly one owned retirement body")
			}
		})
		if allocations != 2 {
			t.Fatalf("%d source routes allocate %.0f times, want one owned body and its result slice", routes, allocations)
		}
	}
}
