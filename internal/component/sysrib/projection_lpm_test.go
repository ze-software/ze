// Design: docs/architecture/rib/unified-locrib.md -- selectable LPM parity.
// Related: projection_resolver_test.go -- recursive cover loss and recovery.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type projectionHiddenChain struct{ prefix netip.Prefix }

func (chain projectionHiddenChain) Evaluate(route ribpolicy.Route) ribpolicy.Verdict {
	if route.Prefix == chain.prefix {
		return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
	}
	return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
}

// An authority-only hidden prefix cannot suppress the visible broader cover.
func TestSelectedProjectionHiddenCoverParity(t *testing.T) {
	for _, tc := range []struct{ cover, hidden, address string }{
		{"192.0.2.0/24", "192.0.2.0/25", "192.0.2.1"},
		{"2001:db8::/32", "2001:db8:1::/64", "2001:db8:1::1"},
	} {
		t.Run(tc.cover, func(t *testing.T) {
			projection := projectionFixture(t)
			hidden := netip.MustParsePrefix(tc.hidden)
			prior := ribpolicy.Current()
			t.Cleanup(func() { ribpolicy.Set(prior) })
			policy, err := ribpolicy.NewSnapshot(map[redistevents.ProtocolID]ribpolicy.Chain{testProjectionProtocol(t): projectionHiddenChain{prefix: hidden}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			ribpolicy.Set(policy)
			local := locrib.NewRIB()
			cover := projectionEntry(tc.cover)
			var value selectedProjectionPath
			if err := projectionPath(&cover.Path, &value); err != nil {
				t.Fatal(err)
			}
			local.Insert(familyForPrefix(cover.Prefix), cover.Prefix, value.path)
			local.Insert(familyForPrefix(hidden), hidden, value.path)
			inspected := false
			local.Inspect(familyForPrefix(hidden), hidden, func(group locrib.PathGroup) {
				inspected = true
				if len(group.Paths) != 1 {
					t.Fatal("hidden candidate not retained")
				}
				if !group.Paths[0].Hidden {
					t.Fatal("narrow candidate was not hidden")
				}
			})
			if !inspected {
				t.Fatal("hidden candidate not inspectable")
			}
			if _, err := projection.Apply(projectionPage(2, rpc.RouteViewPhaseSnapshot, true, cover)); err != nil {
				t.Fatal(err)
			}
			address := netip.MustParseAddr(tc.address)
			for _, reader := range []igpcost.Reader{local, projection} {
				_, prefix, found := reader.LPM(familyForAddr(address), address)
				if !found || prefix != cover.Prefix {
					t.Fatalf("%T selected %s, found %v", reader, prefix, found)
				}
				if !newNHResolver(reader).Resolve(address).Resolved {
					t.Fatalf("%T hidden candidate suppressed broader reachability", reader)
				}
			}
		})
	}
}

// A protocol-proved adjacency keeps its gateway, device and on-link state.
func TestSelectedProjectionOnLinkParity(t *testing.T) {
	projection := projectionFixture(t)
	entry := projectionEntry("192.0.2.0/24")
	entry.Path.NextHop = netip.MustParseAddr("2001:db8::1")
	entry.Path.Interface = "adjacency0"
	entry.Path.OnLink = true
	var value selectedProjectionPath
	if err := projectionPath(&entry.Path, &value); err != nil {
		t.Fatal(err)
	}
	local := locrib.NewRIB()
	local.Insert(familyForPrefix(entry.Prefix), entry.Prefix, value.path)
	if _, err := projection.Apply(projectionPage(2, rpc.RouteViewPhaseSnapshot, true, entry)); err != nil {
		t.Fatal(err)
	}
	want := ResolvedNH{DirectNH: entry.Path.NextHop, Interface: "adjacency0", OnLink: true, Metric: 7, Resolved: true}
	for _, reader := range []igpcost.Reader{local, projection} {
		if got := newNHResolver(reader).Resolve(netip.MustParseAddr("192.0.2.1")); got != want {
			t.Fatalf("%T got %+v, want %+v", reader, got, want)
		}
	}
}

// Dependent resolution starts only after all additions and removals in a page.
func TestSelectedProjectionWholeBatchBeforeResolution(t *testing.T) {
	projection := projectionFixture(t)
	root := projectionEntry("192.0.2.0/24")
	root.Path.NextHop = netip.MustParseAddr("198.51.100.1")
	root.Path.Metric = 100
	old := projectionEntry("198.51.100.0/24")
	if _, err := projection.Apply(projectionPage(3, rpc.RouteViewPhaseSnapshot, true, root, old)); err != nil {
		t.Fatal(err)
	}
	resolver := newNHResolver(projection)
	root.Path.NextHop = netip.MustParseAddr("203.0.113.1")
	root.Action = rpc.RouteViewActionUpdate
	terminal := projectionEntry("203.0.113.0/24")
	terminal.Path.Interface = "new0"
	terminal.Path.Metric = 11
	old.Action = rpc.RouteViewActionWithdraw
	old.Path = rpc.RouteViewPath{}
	keys, err := projection.Apply(projectionPage(6, rpc.RouteViewPhaseDelta, true, root, terminal, old))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 {
		t.Fatalf("changed keys: %v", keys)
	}
	want := ResolvedNH{DirectNH: terminal.Prefix.Addr().Next(), Interface: "new0", Metric: 111, Resolved: true}
	if got := resolver.Resolve(netip.MustParseAddr("192.0.2.1")); got != want {
		t.Fatalf("batch result %+v, want %+v", got, want)
	}
	if _, _, found := projection.LPM(familyForPrefix(old.Prefix), old.Prefix.Addr()); found {
		t.Fatal("batch retained old cover")
	}
}
