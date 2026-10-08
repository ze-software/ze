// VALIDATES: the Loc-RIB owns administrative distance. It ranks every path at
// the distance `rib { distance { } }` declares for its protocol, re-ranks the
// paths already installed when the declaration changes (Reselect), and lets a
// route's own override beat the declared distance for that route alone.
// PREVENTS: a reload that changes a distance leaving the installed winner in
// place until the losing protocol happens to re-send its route.

package locrib

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribdistance "github.com/ze-software/ze/internal/core/rib/distance"
)

// declare publishes a distance table on the seam for the rest of the test, the
// way sysrib's publishDistances does on configure.
func declare(t *testing.T, table map[string]uint8) {
	t.Helper()
	ribdistance.Set(func(protocol string) (uint8, bool) {
		d, ok := table[protocol]
		return d, ok
	})
	t.Cleanup(func() { ribdistance.Set(nil) })
}

// changeLog records the changes a RIB dispatches.
type changeLog struct {
	mu      sync.Mutex
	changes []Change
}

func (l *changeLog) record(c Change) { //nolint:gocritic // hugeParam: the ChangeHandler signature passes Change by value
	l.mu.Lock()
	l.changes = append(l.changes, c)
	l.mu.Unlock()
}

func (l *changeLog) reset() {
	l.mu.Lock()
	l.changes = nil
	l.mu.Unlock()
}

func (l *changeLog) last(t *testing.T) Change {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.changes) == 0 {
		t.Fatal("no change was dispatched")
	}
	return l.changes[len(l.changes)-1]
}

// contest is one prefix held by a static route and by a route from another
// protocol, each with its own next-hop.
type contest struct {
	rib    *RIB
	log    *changeLog
	prefix netip.Prefix
	static redistevents.ProtocolID
	other  redistevents.ProtocolID
}

func newContest(t *testing.T, other, static Path) *contest {
	t.Helper()
	c := &contest{
		rib:    NewRIB(),
		log:    &changeLog{},
		prefix: netip.MustParsePrefix("10.0.0.0/8"),
		static: static.Source,
		other:  other.Source,
	}
	t.Cleanup(c.rib.OnChange(c.log.record))
	c.rib.Insert(family.IPv4Unicast, c.prefix, static)
	c.rib.Insert(family.IPv4Unicast, c.prefix, other)
	return c
}

func (c *contest) winner(t *testing.T) redistevents.ProtocolID {
	t.Helper()
	best, ok := c.rib.Best(family.IPv4Unicast, c.prefix)
	if !ok {
		t.Fatal("the contested prefix has no best path")
	}
	return best.Source
}

func staticRoute() Path {
	return Path{
		Source:  redistevents.RegisterProtocol("static"),
		NextHop: netip.MustParseAddr("192.0.2.1"),
	}
}

// TestReloadReranksStaticAgainstBGP is the owner decision's acceptance: with
// `static 5` the static route beats eBGP at 20; a reload to `static 250` must
// hand the prefix to BGP for the route ALREADY installed, and tell the FIB.
func TestReloadReranksStaticAgainstBGP(t *testing.T) {
	declare(t, map[string]uint8{"static": 5, "ebgp": 20})
	bgp := Path{
		Source:  redistevents.RegisterProtocol("bgp"),
		NextHop: netip.MustParseAddr("198.51.100.1"),
		IsBGP:   true,
		IsEBGP:  true,
	}
	c := newContest(t, bgp, staticRoute())
	if got := c.winner(t); got != c.static {
		t.Fatalf("static 5 vs ebgp 20: winner %v, want static", got)
	}

	c.log.reset()
	declare(t, map[string]uint8{"static": 250, "ebgp": 20})
	c.rib.Reselect()

	if got := c.winner(t); got != c.other {
		t.Fatalf("after the reload to static 250: winner %v, want bgp", got)
	}
	change := c.log.last(t)
	if change.Kind != ChangeUpdate || change.Best.Source != c.other || change.Prefix != c.prefix {
		t.Fatalf("dispatched %+v, want an update naming bgp as the new best of %s", change, c.prefix)
	}
	if change.Best.AdminDistance != 20 {
		t.Errorf("the new best carries distance %d, want the declared 20", change.Best.AdminDistance)
	}
}

// TestReselectIsQuietWhenNothingMoved proves Reselect dispatches only for a
// prefix whose ranking changed, so a reload that touches no distance sends the
// FIB nothing.
func TestReselectIsQuietWhenNothingMoved(t *testing.T) {
	declare(t, map[string]uint8{"static": 5, "ebgp": 20})
	bgp := Path{Source: redistevents.RegisterProtocol("bgp"), NextHop: netip.MustParseAddr("198.51.100.1"), IsBGP: true, IsEBGP: true}
	c := newContest(t, bgp, staticRoute())

	c.log.reset()
	c.rib.Reselect()

	c.log.mu.Lock()
	defer c.log.mu.Unlock()
	if len(c.log.changes) != 0 {
		t.Fatalf("Reselect with an unchanged declaration dispatched %+v", c.log.changes)
	}
}

// TestStaticOwnDistanceOverridesTheDeclared is the per-route override: a static
// route that names its own distance ranks at it, whatever `static` declares,
// and a reload of the declared static distance leaves it where it is.
func TestStaticOwnDistanceOverridesTheDeclared(t *testing.T) {
	declare(t, map[string]uint8{"static": 250, "ebgp": 20})
	own := staticRoute()
	own.DistanceOverride, own.HasDistanceOverride = 3, true
	bgp := Path{Source: redistevents.RegisterProtocol("bgp"), NextHop: netip.MustParseAddr("198.51.100.1"), IsBGP: true, IsEBGP: true}
	c := newContest(t, bgp, own)

	if got := c.winner(t); got != c.static {
		t.Fatalf("own distance 3 vs ebgp 20 under static 250: winner %v, want static", got)
	}
	best, _ := c.rib.Best(family.IPv4Unicast, c.prefix)
	if best.AdminDistance != 3 {
		t.Errorf("the static best ranks at %d, want its own 3", best.AdminDistance)
	}

	declare(t, map[string]uint8{"static": 1, "ebgp": 2})
	c.rib.Reselect()
	if got := c.winner(t); got != c.other {
		t.Fatalf("own distance 3 vs ebgp 2: winner %v, want bgp, because the override does not follow static", got)
	}
}

// TestEachProtocolDistanceChangeReranks covers every other declared protocol:
// raising its distance above the static route's hands the prefix to static,
// and lowering it hands the prefix back, each by Reselect alone.
func TestEachProtocolDistanceChangeReranks(t *testing.T) {
	cases := []struct {
		leaf string
		path Path
	}{
		{"ebgp", Path{Source: redistevents.RegisterProtocol("bgp"), IsBGP: true, IsEBGP: true}},
		{"ibgp", Path{Source: redistevents.RegisterProtocol("bgp"), IsBGP: true}},
		{"ospf", Path{Source: redistevents.RegisterProtocol("ospf")}},
		{"isis", Path{Source: redistevents.RegisterProtocol("isis")}},
	}
	for _, tc := range cases {
		t.Run(tc.leaf, func(t *testing.T) {
			declare(t, map[string]uint8{"static": 100, tc.leaf: 50})
			other := tc.path
			other.NextHop = netip.MustParseAddr("198.51.100.1")
			c := newContest(t, other, staticRoute())
			if got := c.winner(t); got != c.other {
				t.Fatalf("%s 50 vs static 100: winner %v, want %s", tc.leaf, got, tc.leaf)
			}

			declare(t, map[string]uint8{"static": 100, tc.leaf: 150})
			c.rib.Reselect()
			if got := c.winner(t); got != c.static {
				t.Fatalf("%s 150 vs static 100: winner %v, want static", tc.leaf, got)
			}

			declare(t, map[string]uint8{"static": 100, tc.leaf: 50})
			c.rib.Reselect()
			if got := c.winner(t); got != c.other {
				t.Fatalf("%s back to 50: winner %v, want %s", tc.leaf, got, tc.leaf)
			}
		})
	}
}
