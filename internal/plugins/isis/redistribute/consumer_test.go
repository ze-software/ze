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

// acceptAll is an export chain that accepts every route and sets its metric.
type acceptAll struct{ metric uint32 }

func (c acceptAll) Evaluate(ribpolicy.Route) ribpolicy.Verdict {
	return ribpolicy.Verdict{Action: ribpolicy.ActionAccept, Sets: ribpolicy.Sets{Metric: c.metric, HasMetric: true}}
}

// TestISISExportReloadRejudges proves a reload re-judges the routes already
// handed to IS-IS: a chain that widens advertises the route it rejected before,
// a changed set metric reaches the prefixes already in the LSP, a chain that
// narrows withdraws them, and a route the orchestrator withdrew never returns.
// Method: install a chain, inject, install another and call Rejudge, as the
// config apply does, then read the fake LSP injector.
func TestISISExportReloadRejudges(t *testing.T) {
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
	c.InjectRoute(ctx, v4, exportEntry("203.0.113.1/32"))
	c.WithdrawRoute(ctx, v4, "203.0.113.1/32")

	SetExportChain(acceptAll{metric: 30})
	c.Rejudge(ctx)
	got := inj.snapshot(lsdb.Level2)
	if len(got) != 2 {
		t.Fatalf("TLV 135 entries after the chain widened = %v, want 192.0.2.1/32 and 198.51.100.0/24", got)
	}
	for _, info := range got {
		if info.Prefix == netip.MustParsePrefix("203.0.113.1/32") {
			t.Fatal("the route the orchestrator withdrew came back on re-judge")
		}
		if info.Metric.Value() != 30 {
			t.Fatalf("TLV 135 %s metric = %d, want the new chain's set metric 30", info.Prefix, info.Metric.Value())
		}
	}
	gotV6 := inj.snapshotV6(lsdb.Level2)
	if len(gotV6) != 1 || gotV6[0].Metric.Value() != 30 {
		t.Fatalf("TLV 236 entries after the chain widened = %v, want 2001:db8::1/128 at metric 30", gotV6)
	}

	SetExportChain(rejectAll{})
	c.Rejudge(ctx)
	if got := inj.snapshot(lsdb.Level2); len(got) != 0 {
		t.Fatalf("TLV 135 entries after the chain narrowed = %v, want none", got)
	}
	if got := inj.snapshotV6(lsdb.Level2); len(got) != 0 {
		t.Fatalf("TLV 236 entries after the chain narrowed = %v, want none", got)
	}

	// The routes the narrowed chain withdrew are still offered: widening again
	// advertises them, so a reject is never a forgotten route.
	SetExportChain(nil)
	c.Rejudge(ctx)
	if got := inj.snapshot(lsdb.Level2); len(got) != 2 {
		t.Fatalf("TLV 135 entries after the chain was removed = %v, want both offered routes", got)
	}
}

// acceptKeep is an export chain that accepts every route and sets nothing.
type acceptKeep struct{}

func (acceptKeep) Evaluate(ribpolicy.Route) ribpolicy.Verdict {
	return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
}

// TestISISExportKeepsImportMetric proves the metric precedence at the
// consumer: the redistribute import chain's `set metric` (RouteEntry.HasMetric)
// is the prefix metric when no export chain is configured and when the export
// chain sets none, and an export chain's `set metric` replaces it. Method:
// inject one entry carrying an import metric under each chain shape and read
// the TLV 135 and TLV 236 metrics the fake LSP injector holds.
func TestISISExportKeepsImportMetric(t *testing.T) {
	t.Cleanup(func() { SetExportChain(nil) })
	v4 := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	v6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}
	cases := []struct {
		name  string
		chain ribpolicy.Chain
		want  uint32
	}{
		{"no export chain", nil, 30},
		{"export chain sets no metric", acceptKeep{}, 30},
		{"export chain sets metric", acceptAll{metric: 10}, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			SetExportChain(tc.chain)
			inj := newFakeInjector(lsdb.Level2)
			c := NewConsumer(inj)
			ctx := context.Background()

			entry := exportEntry("198.51.100.0/24")
			entry.Metric, entry.HasMetric = 30, true
			c.InjectRoute(ctx, v4, entry)
			entryV6 := exportEntry("2001:db8:1::/64")
			entryV6.Metric, entryV6.HasMetric = 30, true
			c.InjectRoute(ctx, v6, entryV6)

			got := inj.snapshot(lsdb.Level2)
			if len(got) != 1 {
				t.Fatalf("TLV 135 entries = %v, want 198.51.100.0/24", got)
			}
			if got[0].Metric.Value() != tc.want {
				t.Fatalf("TLV 135 metric = %d, want %d", got[0].Metric.Value(), tc.want)
			}
			gotV6 := inj.snapshotV6(lsdb.Level2)
			if len(gotV6) != 1 {
				t.Fatalf("TLV 236 entries = %v, want 2001:db8:1::/64", gotV6)
			}
			if gotV6[0].Metric.Value() != tc.want {
				t.Fatalf("TLV 236 metric = %d, want %d", gotV6[0].Metric.Value(), tc.want)
			}
		})
	}
}
