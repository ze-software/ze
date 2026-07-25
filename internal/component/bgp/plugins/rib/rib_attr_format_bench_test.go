package rib

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// BenchmarkEnrichRouteCommunities measures the allocation cost of enriching
// a route map with 100 communities and marshaling to JSON.
//
// Before the wrapper change: per-element String() allocations + make([]string).
// After: lazy MarshalJSON with AppendText, no intermediate strings.
func BenchmarkEnrichRouteCommunities(b *testing.B) {
	communities := make([]attribute.Community, 100)
	for i := range communities {
		communities[i] = attribute.Community(uint32(i+1)<<16 | uint32(i+1))
	}

	rt := &Route{
		Communities: communities,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		routeMap := make(map[string]any)
		enrichRouteMapFromRoute(routeMap, rt)
		if _, err := json.Marshal(routeMap); err != nil {
			b.Fatal(err)
		}
	}
}
