//go:build !race

// Design: docs/architecture/bgp/egress-attribute-rules.md -- native policy admission allocation bounds.
// Related: filter_atomic_aggregate_test.go -- race-enabled semantic and ownership controls.
// Go's race instrumentation randomly drops sync.Pool returns; allocation
// oracles measure the normal producer while semantic tests retain race checks.
package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestAtomicAggregatePolicyWarmAllocations measures the warmed native-key
// admission path, including long opaque keys, without attributing pool creation
// or capacity growth to individual NLRI processing.
func TestAtomicAggregatePolicyWarmAllocations(t *testing.T) {
	fam := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	ctx := atomicPolicyContext(t, fam, false)
	a := atomicPolicyNativeFamilies(1)[fam]
	b := atomicPolicyNativeFamilies(2)[fam]
	input := atomicPolicyMP(fam, bytes.Join([][]byte{a, b}, nil), true)
	output := atomicPolicyMP(fam, b, true)
	allocations := testing.AllocsPerRun(100, func() {
		allowed, err := atomicAggregatePolicyAllowed(input, output, ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !allowed {
			t.Fatal("valid native subset refused")
		}
	})
	if allocations != 0 {
		t.Fatalf("warmed native subset admission allocates %.0f times", allocations)
	}
}

// TestAtomicAggregatePolicyLargeWarmAllocations retains a useful subset of a
// legal extended UPDATE with 4097 unique /32 routes. Repeated admission must
// reuse its warmed arena rather than discard it at a small retention threshold.
func TestAtomicAggregatePolicyLargeWarmAllocations(t *testing.T) {
	receive := bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{
		ASN4: true, ExtendedMessageRecv: true,
	}, bgpctx.DirectionRecv)
	ctx, err := bgpctx.Registry.Register(receive)
	if err != nil {
		t.Fatal(err)
	}
	const routes = 4097
	raw := make([]byte, routes*5)
	for i := range routes {
		off := i * 5
		raw[off], raw[off+1] = 32, 10
		raw[off+2], raw[off+3], raw[off+4] = byte(i>>16), byte(i>>8), byte(i)
	}
	input := atomicPolicyLegacy(raw, true)
	if len(input) <= 4096 {
		t.Fatal("large allocation fixture does not require extended messages")
	}
	if len(input) > maxUpdateBody {
		t.Fatal("large allocation fixture exceeds the extended message ceiling")
	}
	output := atomicPolicyLegacy(raw[len(raw)-5:], true)
	allocations := testing.AllocsPerRun(100, func() {
		allowed, err := atomicAggregatePolicyAllowed(input, output, ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !allowed {
			t.Fatal("valid large native subset refused")
		}
	})
	if allocations != 0 {
		t.Fatalf("warmed 4097-route native admission allocates %.0f times", allocations)
	}
}
