package reactor

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// TestRFC7311RecursiveAIGPChangeReachesWire changes the received AIGP of a
// recursive route in the actual Loc-RIB, without manually invoking replay.
// RFC 7311 Section 3.4.3: "Any change due to (a) in any of these values
// MUST trigger a new AIGP computation for that route."
// RFC requirement: RFC7311-3.4.3-7 positive -- changing a recursive next hop's received AIGP triggers a new wire advertisement without a new source UPDATE.
// RFC requirement: RFC7311-3.4.3-7 negative -- after a saturated advertisement, a finite recursive metric is recomputed from the original received value, not the previous advertisement.
// RFC requirement: RFC7311-3.4.3-2 positive -- both recursive accumulation and the subsequent source-metric addition saturate at the uint64 maximum on the wire.
// RFC requirement: RFC7311-3.4.3-2 negative -- finite recursive sums remain exact rather than being clamped or accumulated from prior sent metrics.
func TestRFC7311RecursiveAIGPChangeReachesWire(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	loc := locrib.Default()
	protocol := redistevents.RegisterProtocol("aigp-readvertise-test")
	terminal := netip.MustParsePrefix("198.18.233.3/32")
	loc.Insert(family.IPv4Unicast, terminal, locrib.Path{Source: protocol, Metric: 7})
	t.Cleanup(func() { loc.Remove(family.IPv4Unicast, terminal, protocol, 0) })
	recursive := locrib.Path{Source: protocol, IsBGP: true, AIGPPresent: true, AIGP: 200, NextHop: terminal.Addr()}
	prefix := netip.PrefixFrom(f.source.Settings().Address, 32)
	loc.Insert(family.IPv4Unicast, prefix, recursive)
	id := f.receive(t, f.body(t, 100))
	f.forward(t, id)
	f.waitBatch(t)
	assertMetric := func(want uint64) {
		t.Helper()
		bodies := aigpSocketBodies(t, f.conn)
		if len(bodies) == 0 {
			t.Fatal("no AIGP advertisement")
		}
		got, present := aigpReceivedMetric(t, bodies[len(bodies)-1])
		if !present || got != want {
			t.Fatalf("wire metric = %d present=%v, want %d", got, present, want)
		}
	}
	assertMetric(307)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { defer close(stopped); f.r.runAIGPAdvertisements(ctx) }()
	t.Cleanup(func() { cancel(); <-stopped })
	for _, metric := range []uint64{^uint64(0) - 5, 400, ^uint64(0) - 50, 400} {
		recursive.AIGP = metric
		loc.Insert(family.IPv4Unicast, prefix, recursive)
		f.waitBatch(t)
		want := ^uint64(0)
		if metric == 400 {
			want = 507
		}
		assertMetric(want)
	}
}
