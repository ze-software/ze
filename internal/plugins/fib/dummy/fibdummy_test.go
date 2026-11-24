// VALIDATES: fib-dummy held route table, per-table keying, install/replace/remove
// counters, Prometheus metrics updates, invalid prefix counting, JSON envelope
// delivery for forked mode, and concurrent read/write safety (AC-2, AC-14, A-3, R-6).
// PREVENTS: lost route updates, table collisions, silently held invalid prefixes,
// metric drift from counters, or data races between bus ingestion and show reads.
package fibdummy

import (
	"encoding/json"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type trackingGauge struct {
	mu  sync.Mutex
	val float64
}

func (g *trackingGauge) Set(v float64) {
	g.mu.Lock()
	g.val = v
	g.mu.Unlock()
}

func (g *trackingGauge) Inc()          { g.Add(1) }
func (g *trackingGauge) Dec()          { g.Add(-1) }
func (g *trackingGauge) Add(v float64) { g.mu.Lock(); g.val += v; g.mu.Unlock() }
func (g *trackingGauge) value() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.val
}

type trackingCounter struct {
	mu  sync.Mutex
	val float64
}

func (c *trackingCounter) Inc()          { c.Add(1) }
func (c *trackingCounter) Add(v float64) { c.mu.Lock(); c.val += v; c.mu.Unlock() }
func (c *trackingCounter) value() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.val
}

type trackingMetricsRegistry struct {
	metrics.NopRegistry
	gauges   map[string]*trackingGauge
	counters map[string]*trackingCounter
}

func newTrackingMetricsRegistry() *trackingMetricsRegistry {
	return &trackingMetricsRegistry{
		gauges:   make(map[string]*trackingGauge),
		counters: make(map[string]*trackingCounter),
	}
}

func (r *trackingMetricsRegistry) Gauge(name, _ string) metrics.Gauge {
	g := &trackingGauge{}
	r.gauges[name] = g
	return g
}

func (r *trackingMetricsRegistry) Counter(name, _ string) metrics.Counter {
	c := &trackingCounter{}
	r.counters[name] = c
	return c
}

func TestHoldInstallReplaceRemove(t *testing.T) {
	prev := metricsPtr.Load()
	t.Cleanup(func() { metricsPtr.Store(prev) })

	reg := newTrackingMetricsRegistry()
	SetMetricsRegistry(reg)

	since := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	f := newDummyFIB(since)

	// Install two prefixes.
	f.applyBatch(&sysribevents.BestChangeBatch{
		Family: family.IPv4Unicast,
		Changes: []sysribevents.BestChangeEntry{
			{
				Action:   routeaction.Add,
				Prefix:   netip.MustParsePrefix("198.51.100.0/24"),
				NextHop:  netip.MustParseAddr("192.0.2.1"),
				Protocol: "bgp",
				TableID:  254,
			},
			{
				Action:   routeaction.Add,
				Prefix:   netip.MustParsePrefix("203.0.113.0/24"),
				NextHop:  netip.MustParseAddr("192.0.2.2"),
				Protocol: "static",
				TableID:  254,
			},
		},
	})

	rows := f.routeRows()
	require.Len(t, rows, 2)
	assert.Equal(t, routeRow{
		Table:      254,
		Prefix:     "198.51.100.0/24",
		Protocol:   "bgp",
		NextHop:    "192.0.2.1",
		Paths:      1,
		Programmed: "no",
	}, rows[0])
	assert.Equal(t, routeRow{
		Table:      254,
		Prefix:     "203.0.113.0/24",
		Protocol:   "static",
		NextHop:    "192.0.2.2",
		Paths:      1,
		Programmed: "no",
	}, rows[1])

	snap := f.counters()
	assert.Equal(t, forwardingNone, snap.Forwarding)
	assert.Equal(t, 2, snap.RoutesHeld)
	assert.Equal(t, uint64(2), snap.Installs)
	assert.Equal(t, uint64(0), snap.Replaces)
	assert.Equal(t, uint64(0), snap.Removals)
	assert.Equal(t, since, snap.Since)
	assert.Equal(t, float64(2), reg.gauges["ze_fibdummy_routes_held"].value())
	assert.Equal(t, float64(2), reg.counters["ze_fibdummy_route_installs_total"].value())

	// Replace one prefix with a new next-hop and an ECMP sibling.
	f.applyBatch(&sysribevents.BestChangeBatch{
		Family: family.IPv4Unicast,
		Changes: []sysribevents.BestChangeEntry{
			{
				Action:   routeaction.Update,
				Prefix:   netip.MustParsePrefix("198.51.100.0/24"),
				NextHop:  netip.MustParseAddr("192.0.2.9"),
				Protocol: "bgp",
				TableID:  254,
				ECMPPaths: []sysribevents.ECMPPath{
					{NextHop: netip.MustParseAddr("192.0.2.10")},
				},
			},
		},
	})

	rows = f.routeRows()
	require.Len(t, rows, 2)
	assert.Equal(t, "192.0.2.9", rows[0].NextHop)
	assert.Equal(t, 2, rows[0].Paths)
	assert.Equal(t, uint64(1), f.counters().Replaces)
	assert.Equal(t, float64(1), reg.counters["ze_fibdummy_route_replaces_total"].value())

	// Remove the other prefix via the forked JSON envelope path.
	removeBatch := &sysribevents.BestChangeBatch{
		Family: family.IPv4Unicast,
		Changes: []sysribevents.BestChangeEntry{
			{
				Action:  routeaction.Withdraw,
				Prefix:  netip.MustParsePrefix("203.0.113.0/24"),
				TableID: 254,
			},
		},
	}
	payload, err := json.Marshal(removeBatch)
	require.NoError(t, err)
	envBytes, err := json.Marshal(rpc.EventEnvelope{
		Namespace: sysribevents.Namespace,
		Event:     sysribevents.EventBestChange,
		Payload:   payload,
	})
	require.NoError(t, err)
	require.NoError(t, f.handleEvent(string(envBytes)))

	rows = f.routeRows()
	require.Len(t, rows, 1)
	assert.Equal(t, "198.51.100.0/24", rows[0].Prefix)

	snap = f.counters()
	assert.Equal(t, 1, snap.RoutesHeld)
	assert.Equal(t, uint64(2), snap.Installs)
	assert.Equal(t, uint64(1), snap.Replaces)
	assert.Equal(t, uint64(1), snap.Removals)
	assert.Equal(t, float64(1), reg.gauges["ze_fibdummy_routes_held"].value())
	assert.Equal(t, float64(1), reg.counters["ze_fibdummy_route_removals_total"].value())
}

func TestHoldKeysByTable(t *testing.T) {
	f := newDummyFIB(time.Now().UTC())
	pfx := netip.MustParsePrefix("10.0.0.0/24")

	f.applyBatch(&sysribevents.BestChangeBatch{
		Family: family.IPv4Unicast,
		Changes: []sysribevents.BestChangeEntry{
			{
				Action:   routeaction.Add,
				Prefix:   pfx,
				NextHop:  netip.MustParseAddr("192.0.2.1"),
				Protocol: "static",
				TableID:  0,
			},
			{
				Action:   routeaction.Add,
				Prefix:   pfx,
				NextHop:  netip.MustParseAddr("192.0.2.2"),
				Protocol: "bgp",
				TableID:  100,
			},
		},
	})

	rows := f.routeRows()
	require.Len(t, rows, 2)
	assert.Equal(t, uint32(0), rows[0].Table)
	assert.Equal(t, "192.0.2.1", rows[0].NextHop)
	assert.Equal(t, uint32(100), rows[1].Table)
	assert.Equal(t, "192.0.2.2", rows[1].NextHop)
	assert.Equal(t, 2, f.counters().RoutesHeld)

	// Removing from table 100 leaves table 0 untouched.
	f.applyBatch(&sysribevents.BestChangeBatch{
		Family: family.IPv4Unicast,
		Changes: []sysribevents.BestChangeEntry{
			{
				Action:  routeaction.Withdraw,
				Prefix:  pfx,
				TableID: 100,
			},
		},
	})

	rows = f.routeRows()
	require.Len(t, rows, 1)
	assert.Equal(t, uint32(0), rows[0].Table)
	assert.Equal(t, 1, f.counters().RoutesHeld)
}

func TestHoldInvalidChangeIsCounted(t *testing.T) {
	f := newDummyFIB(time.Now().UTC())
	var callbackCalls atomic.Int64
	f.onInvalidChange = func(_ string) {
		callbackCalls.Add(1)
	}

	f.applyBatch(&sysribevents.BestChangeBatch{
		Family: family.IPv4Unicast,
		Changes: []sysribevents.BestChangeEntry{
			{
				Action:   routeaction.Add,
				Prefix:   netip.Prefix{}, // invalid
				NextHop:  netip.MustParseAddr("192.0.2.1"),
				Protocol: "bgp",
			},
			{
				Action:   routeaction.Add,
				Prefix:   netip.MustParsePrefix("198.51.100.0/24"),
				NextHop:  netip.MustParseAddr("192.0.2.1"),
				Protocol: "bgp",
			},
		},
	})

	assert.Equal(t, uint64(1), f.counters().InvalidChanges)
	assert.Equal(t, 1, f.counters().RoutesHeld)
	assert.Equal(t, int64(1), callbackCalls.Load())
	require.Len(t, f.routeRows(), 1)
	assert.Equal(t, "198.51.100.0/24", f.routeRows()[0].Prefix)

	// A second invalid change increments the counter but does not re-fire the hook.
	f.applyBatch(&sysribevents.BestChangeBatch{
		Family: family.IPv4Unicast,
		Changes: []sysribevents.BestChangeEntry{
			{
				Action:   routeaction.Update,
				Prefix:   netip.Prefix{},
				Protocol: "bgp",
			},
		},
	})
	assert.Equal(t, uint64(2), f.counters().InvalidChanges)
	assert.Equal(t, int64(1), callbackCalls.Load())
}

func TestHoldConcurrentReader(t *testing.T) {
	f := newDummyFIB(time.Now().UTC())
	prefixes := []netip.Prefix{
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("2001:db8:1::/48"),
	}
	nhA := netip.MustParseAddr("192.0.2.1")
	nhB := netip.MustParseAddr("192.0.2.2")

	const iterations = 200
	var wg sync.WaitGroup

	// Writers apply installs, replaces, removes, and invalid changes concurrently.
	wg.Go(func() {
		for i := range iterations {
			p := prefixes[i%len(prefixes)]
			f.applyBatch(&sysribevents.BestChangeBatch{
				Family: family.IPv4Unicast,
				Changes: []sysribevents.BestChangeEntry{
					{Action: routeaction.Add, Prefix: p, NextHop: nhA, Protocol: "bgp"},
					{Action: routeaction.Update, Prefix: p, NextHop: nhB, Protocol: "bgp"},
				},
			})
		}
	})
	wg.Go(func() {
		for i := range iterations {
			p := prefixes[i%len(prefixes)]
			f.applyBatch(&sysribevents.BestChangeBatch{
				Family: family.IPv4Unicast,
				Changes: []sysribevents.BestChangeEntry{
					{Action: routeaction.Withdraw, Prefix: p},
					{Action: routeaction.Add, Prefix: netip.Prefix{}},
				},
			})
		}
	})

	// Readers snapshot routeRows and counters concurrently.
	for range 2 {
		wg.Go(func() {
			for range iterations {
				rows := f.routeRows()
				for _, r := range rows {
					if r.Programmed != programmedNo {
						t.Errorf("row %s had programmed=%q, want %q", r.Prefix, r.Programmed, programmedNo)
					}
				}
				snap := f.counters()
				if snap.Forwarding != forwardingNone {
					t.Errorf("counters forwarding=%q, want %q", snap.Forwarding, forwardingNone)
				}
			}
		})
	}

	wg.Wait()
}
