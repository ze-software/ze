package lg

import (
	"fmt"
	"testing"
)

// benchRIB builds a Ze route-list payload (the shape extractRoutes consumes)
// with n routes, mirroring a large RIB. Communities/as-path are populated so
// the transform exercises its per-route allocation path realistically.
func benchRIB(n int) map[string]any {
	routes := make([]any, n)
	for i := range routes {
		routes[i] = map[string]any{
			"prefix":           fmt.Sprintf("10.%d.%d.0/24", i/256, i%256),
			"next-hop":         "10.0.0.1",
			"origin":           "igp",
			"as-path":          []any{float64(65001), float64(65002), float64(65003)},
			"local-preference": float64(100),
			"med":              float64(0),
			"peer-address":     "10.0.0.1",
			"community":        []any{"65000:100", "65001:200"},
			"large-community":  []any{"65000:0:100"},
			"best":             true,
		}
	}
	return map[string]any{"routes": routes}
}

// BenchmarkRoutesTableTransform measures the routes/table transform at 100k
// routes -- the per-request cost the looking glass pays because the full RIB is
// materialized server-side (spec A-5 / AC-12). Baseline recorded in the learned
// summary.
func BenchmarkRoutesTableTransform(b *testing.B) {
	ze := benchRIB(100_000)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		bw := transformRoutes(ze, "")
		if len(bw) == 0 {
			b.Fatal("empty transform")
		}
	}
}

// BenchmarkRoutesPeerTransformPaginated measures the routes/peer transform plus
// a paginated slice at 100k routes -- the cost when a client requests one page.
func BenchmarkRoutesPeerTransformPaginated(b *testing.B) {
	ze := benchRIB(100_000)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		bw := transformRoutes(ze, "peer1")
		paginateRoutes(bw, 100, 0)
	}
}
