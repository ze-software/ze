// Design: docs/architecture/route-selection.md -- next-hop resolution allocation budget
package igpcost

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// BenchmarkNextHopResolution compares the same real selected routing table and
// terminal cost through ordinary and prospective resolution. Setup, protocol
// registration and route insertion are outside the measured loops. The lookup
// result is checked so neither benchmark can time an unresolved or eliminated path.
func BenchmarkNextHopResolution(b *testing.B) {
	loc := locrib.NewRIB()
	source := redistevents.RegisterProtocol("prospective-bench")
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), locrib.Path{
		Source: source, Interface: "bench0", Metric: 11,
	})
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("10.251.0.0/24"), locrib.Path{
		Source: source, NextHop: netip.MustParseAddr("198.51.100.1"), MetricRecursive: true,
	})
	prefix := netip.MustParsePrefix("10.252.0.0/24")
	for _, route := range []struct {
		name string
		hop  netip.Addr
	}{
		{"direct", netip.MustParseAddr("198.51.100.1")},
		{"recursive", netip.MustParseAddr("10.251.0.1")},
	} {
		b.Run(route.name+"/ordinary", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				distance := Resolve(loc, route.hop)
				if !distance.Resolved {
					b.Fatal("the benchmark's existing next hop did not resolve")
				}
				if distance.Cost != 11 {
					b.Fatalf("terminal cost is %d, want 11", distance.Cost)
				}
			}
		})
		b.Run(route.name+"/prospective", func(b *testing.B) {
			offered := locrib.Path{Source: source, Instance: 1, NextHop: route.hop, IsBGP: true}
			b.ReportAllocs()
			for b.Loop() {
				distance := ResolveProspective(loc, route.hop, family.IPv4Unicast, prefix, offered)
				if !distance.Resolved {
					b.Fatal("the independently resolved next hop was excluded by the preview")
				}
				if distance.Cost != 11 {
					b.Fatalf("terminal cost is %d, want 11", distance.Cost)
				}
			}
		})
	}
}
