package isisredistribute

import (
	"context"
	"net/netip"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
)

// loopbackChain is an export chain that accepts only host routes, /32 and
// /128, and sets their metric: the shape `filters [ loopbacks set-metric ];
// final accept;` compiles to when loopbacks rejects every other length.
type loopbackChain struct{ metric uint32 }

func (c loopbackChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict {
	if r.Prefix.Bits() != r.Prefix.Addr().BitLen() {
		return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
	}
	return ribpolicy.Verdict{Action: ribpolicy.ActionAccept, Sets: ribpolicy.Sets{Metric: c.metric, HasMetric: true}}
}

// exportEntry builds the entry the orchestrator hands the consumer, with the
// typed route view the export chain reads.
func exportEntry(prefix string) configredist.RouteEntry {
	p := netip.MustParsePrefix(prefix)
	return configredist.RouteEntry{Prefix: prefix, Source: "connected", Route: ribpolicy.Route{Prefix: p, HasTag: true}}
}

// TestISISExportChain proves AC-12 at the consumer: with an export chain that
// accepts only loopback host routes, redistributing connected routes leaves only
// the /32 and /128 in the LSP, at the metric the chain set, and a route the
// chain now rejects is withdrawn from the LSP. Method: a fake LSP injector
// records the TLV 135 and TLV 236 entries the consumer writes.
func TestISISExportChain(t *testing.T) {
	SetExportChain(loopbackChain{metric: 10})
	t.Cleanup(func() { SetExportChain(nil) })

	inj := newFakeInjector(lsdb.Level2)
	c := NewConsumer(inj)
	ctx := context.Background()
	v4 := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	v6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}

	c.InjectRoute(ctx, v4, exportEntry("192.0.2.1/32"))
	c.InjectRoute(ctx, v4, exportEntry("198.51.100.0/24"))
	c.InjectRoute(ctx, v6, exportEntry("2001:db8::1/128"))
	c.InjectRoute(ctx, v6, exportEntry("2001:db8:1::/64"))

	got := inj.snapshot(lsdb.Level2)
	if len(got) != 1 {
		t.Fatalf("TLV 135 entries = %v, want only 192.0.2.1/32", got)
	}
	if got[0].Prefix != netip.MustParsePrefix("192.0.2.1/32") {
		t.Fatalf("TLV 135 prefix = %s, want 192.0.2.1/32", got[0].Prefix)
	}
	if got[0].Metric.Value() != 10 {
		t.Fatalf("TLV 135 metric = %d, want the chain's set metric 10", got[0].Metric.Value())
	}
	gotV6 := inj.snapshotV6(lsdb.Level2)
	if len(gotV6) != 1 {
		t.Fatalf("TLV 236 entries = %v, want only 2001:db8::1/128", gotV6)
	}
	if gotV6[0].Metric.Value() != 10 {
		t.Fatalf("TLV 236 metric = %d, want the chain's set metric 10", gotV6[0].Metric.Value())
	}

	// The chain narrows: the loopback it advertised is now rejected and leaves the LSP.
	SetExportChain(rejectAll{})
	c.InjectRoute(ctx, v4, exportEntry("192.0.2.1/32"))
	if got := inj.snapshot(lsdb.Level2); len(got) != 0 {
		t.Fatalf("TLV 135 entries after the chain rejected the loopback = %v, want none", got)
	}
}

// rejectAll is an export chain whose final rejects every route.
type rejectAll struct{}

func (rejectAll) Evaluate(ribpolicy.Route) ribpolicy.Verdict {
	return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
}

// TestISISExportNoChainDefaultMetric proves the guard on the other side: with
// no export chain every route enters the LSP at DefaultRedistMetric.
func TestISISExportNoChainDefaultMetric(t *testing.T) {
	SetExportChain(nil)
	inj := newFakeInjector(lsdb.Level2)
	c := NewConsumer(inj)
	c.InjectRoute(context.Background(), family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}, exportEntry("198.51.100.0/24"))
	got := inj.snapshot(lsdb.Level2)
	if len(got) != 1 {
		t.Fatalf("TLV 135 entries = %v, want 198.51.100.0/24", got)
	}
	if got[0].Metric.Value() != DefaultRedistMetric {
		t.Fatalf("TLV 135 metric = %d, want DefaultRedistMetric %d", got[0].Metric.Value(), DefaultRedistMetric)
	}
}
