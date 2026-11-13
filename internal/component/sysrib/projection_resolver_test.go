// Design: docs/architecture/core-design.md -- recursive next-hop reader.
// Related: projection.go -- completed external selected view.

package sysrib

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The same selected routes resolve identically through direct and projected LPM.
func TestSelectedProjectionResolverParity(t *testing.T) {
	cases := []struct{ name, root, recursive, nextHop, terminal string }{
		{"IPv4", "192.0.2.9", "192.0.2.0/24", "198.51.100.1", "198.51.100.0/24"},
		{"IPv6", "2001:db8:1::9", "2001:db8:1::/64", "2001:db8:2::1", "2001:db8:2::/64"},
		{"IPv4 over IPv6", "192.0.2.9", "192.0.2.0/24", "2001:db8:2::1", "2001:db8:2::/64"},
		{"IPv6 over IPv4", "2001:db8:1::9", "2001:db8:1::/64", "198.51.100.1", "198.51.100.0/24"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projection := projectionFixture(t)
			local := locrib.NewRIB()
			recursive := projectionEntry(tc.recursive)
			recursive.Path.NextHop = netip.MustParseAddr(tc.nextHop)
			recursive.Path.MetricRecursive = true
			recursive.Path.Metric = 100
			terminal := projectionEntry(tc.terminal)
			terminal.Path.Metric = 5
			terminal.Path.Interface = "terminal0"
			for _, entry := range []*rpc.RouteViewEntry{&recursive, &terminal} {
				var value selectedProjectionPath
				if err := projectionPath(&entry.Path, &value); err != nil {
					t.Fatal(err)
				}
				local.Insert(familyForPrefix(entry.Prefix), entry.Prefix, value.path)
			}
			if _, err := projection.Apply(projectionPage(3, rpc.RouteViewPhaseSnapshot, true, recursive, terminal)); err != nil {
				t.Fatal(err)
			}
			root := netip.MustParseAddr(tc.root)
			dependent := netip.MustParsePrefix("203.0.113.0/24")
			resolvers := []*nhResolver{newNHResolver(local), newNHResolver(projection)}
			for _, resolver := range resolvers {
				reader := resolver.rib
				got := resolver.Resolve(root)
				want := ResolvedNH{DirectNH: terminal.Prefix.Addr().Next(), Interface: "terminal0", Metric: 105, Resolved: true}
				if got != want {
					t.Fatalf("%T resolution: got %+v, want %+v", reader, got, want)
				}
				metric := resolver.IGPMetric(root)
				if !metric.Resolved || !metric.Recursive || metric.Cost != 5 {
					t.Fatalf("%T metric: %+v", reader, metric)
				}
				resolver.Track(root, dependent)
				if !slices.Contains(resolver.CoveredNHs(terminal.Prefix), root) {
					t.Fatalf("%T missed terminal dependency", reader)
				}
				if !slices.Contains(resolver.Dependents(root), dependent) {
					t.Fatalf("%T missed dependent prefix", reader)
				}
			}
			withdraw := terminal
			withdraw.Action = rpc.RouteViewActionWithdraw
			withdraw.Path = rpc.RouteViewPath{}
			local.Remove(familyForPrefix(terminal.Prefix), terminal.Prefix, testProjectionProtocol(t), 0)
			if _, err := projection.Apply(projectionPage(4, rpc.RouteViewPhaseDelta, true, withdraw)); err != nil {
				t.Fatal(err)
			}
			for _, resolver := range resolvers {
				reader := resolver.rib
				if resolver.Resolve(root).Resolved {
					t.Fatalf("%T retained lost cover", reader)
				}
				if resolver.IGPMetric(root).Resolved {
					t.Fatalf("%T retained lost metric", reader)
				}
				if !slices.Contains(resolver.CoveredNHs(terminal.Prefix), root) {
					t.Fatalf("%T lost withdrawal dependency", reader)
				}
			}
			var restored selectedProjectionPath
			if err := projectionPath(&terminal.Path, &restored); err != nil {
				t.Fatal(err)
			}
			local.Insert(familyForPrefix(terminal.Prefix), terminal.Prefix, restored.path)
			if _, err := projection.Apply(projectionPage(5, rpc.RouteViewPhaseDelta, true, terminal)); err != nil {
				t.Fatal(err)
			}
			for _, resolver := range resolvers {
				if !resolver.Resolve(root).Resolved {
					t.Fatal("cover recovery needs a producer reannouncement")
				}
			}
		})
	}
}

func testProjectionProtocol(t *testing.T) redistevents.ProtocolID {
	t.Helper()
	id, known := redistevents.ProtocolIDOf("projection-test")
	if !known {
		t.Fatal("fixture protocol not registered")
	}
	return id
}

// A missing or incomplete view must never prove a next hop reachable.
func TestSelectedProjectionMissingReader(t *testing.T) {
	var local *locrib.RIB
	var missing *selectedProjection
	for _, tc := range []struct {
		name   string
		reader igpcost.Reader
	}{
		{"nil", nil}, {"typed nil authority", local}, {"typed nil projection", missing}, {"incomplete projection", projectionFixture(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("missing reader panicked: %v", recovered)
				}
			}()
			resolver := newNHResolver(tc.reader)
			if resolver.Resolve(netip.MustParseAddr("192.0.2.1")).Resolved {
				t.Fatalf("%T proved a next hop without a table", tc.reader)
			}
			if resolver.IGPMetric(netip.MustParseAddr("192.0.2.1")).Resolved {
				t.Fatalf("%T proved an IGP distance without a table", tc.reader)
			}
		})
	}
}

// A reset cannot consume an identity from a different pinned snapshot.
func TestSelectedProjectionRejectsForeignResetSnapshot(t *testing.T) {
	p := projectionFixture(t)
	if _, err := p.Apply(projectionPage(2, rpc.RouteViewPhaseSnapshot, true, projectionEntry("192.0.2.0/24"))); err != nil {
		t.Fatal(err)
	}
	reset := projectionPage(2, rpc.RouteViewPhaseReset, false)
	reset.Snapshot = "foreign"
	if _, err := p.Apply(reset); err == nil {
		t.Fatal("foreign reset snapshot accepted")
	}
	if !p.Synchronized() {
		t.Fatal("foreign reset destroyed current table")
	}
}

// A fenced reset may carry a newer applied barrier, but never an older frontier.
func TestSelectedProjectionAcceptsResetBarrier(t *testing.T) {
	p := projectionFixture(t)
	if _, err := p.Apply(projectionPage(2, rpc.RouteViewPhaseSnapshot, true, projectionEntry("192.0.2.0/24"))); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Apply(projectionPage(1, rpc.RouteViewPhaseReset, false)); err == nil {
		t.Fatal("regressed reset cursor accepted")
	}
	if !p.Synchronized() {
		t.Fatal("regressed reset destroyed current table")
	}
	if _, err := p.Apply(projectionPage(10, rpc.RouteViewPhaseReset, false)); err != nil {
		t.Fatalf("newer fenced reset refused: %v", err)
	}
	if p.Synchronized() {
		t.Fatal("fenced reset left the view synchronized")
	}
	if _, _, found := p.LPM(familyForPrefix(netip.MustParsePrefix("192.0.2.0/24")), netip.MustParseAddr("192.0.2.1")); found {
		t.Fatal("fenced reset retained reachability")
	}
}
