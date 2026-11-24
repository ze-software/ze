// VALIDATES: steady-state replace of already-held prefixes allocates 0 bytes
// and 0 allocations per operation in dummyFIB.applyBatch (AC-15, R-4).
// PREVENTS: hidden string formatting, slice cloning, or map growth entering
// the best-change hot path.
package fibdummy

import (
	"net/netip"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
)

func BenchmarkHoldReplace(b *testing.B) {
	prev := metricsPtr.Load()
	b.Cleanup(func() { metricsPtr.Store(prev) })
	SetMetricsRegistry(metrics.NopRegistry{})

	const (
		octetsHi   = 4
		octetsLo   = 250
		routeCount = octetsHi * octetsLo
	)
	f := newDummyFIB(time.Now().UTC())

	nhInit := netip.MustParseAddr("192.0.2.1")
	nhNext := netip.MustParseAddr("192.0.2.2")

	installEntries := make([]sysribevents.BestChangeEntry, 0, routeCount)
	replaceEntries := make([]sysribevents.BestChangeEntry, 0, routeCount)
	for hi := range byte(octetsHi) {
		for lo := range byte(octetsLo) {
			addr := netip.AddrFrom4([4]byte{10, hi, lo, 0})
			pfx := netip.PrefixFrom(addr, 24)
			installEntries = append(installEntries, sysribevents.BestChangeEntry{
				Action:   routeaction.Add,
				Prefix:   pfx,
				NextHop:  nhInit,
				Protocol: "bgp",
				TableID:  254,
			})
			replaceEntries = append(replaceEntries, sysribevents.BestChangeEntry{
				Action:   routeaction.Update,
				Prefix:   pfx,
				NextHop:  nhNext,
				Protocol: "bgp",
				TableID:  254,
			})
		}
	}

	f.applyBatch(&sysribevents.BestChangeBatch{
		Family:  family.IPv4Unicast,
		Changes: installEntries,
	})

	replaceBatch := &sysribevents.BestChangeBatch{
		Family:  family.IPv4Unicast,
		Changes: replaceEntries,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		f.applyBatch(replaceBatch)
	}
}
