package static

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// rejectTagChain hides every path tagged 4242 and admits the rest.
type rejectTagChain struct{}

func (rejectTagChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict { //nolint:gocritic // hugeParam: the Chain signature passes Route by value
	if r.HasTag && r.Tag == 4242 {
		return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
	}
	return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
}

// TestStaticTagReachesGate is AC-11 in-process: a static route's `tag` leaf
// rides on the Path the plugin inserts (staticPath), so a Loc-RIB chain
// rejecting tag 4242 hides it, while a route with no tag leaf reaches the gate
// carrying no tag (not tag 0) and is selected. A tag 0 leaf is carried as a
// real tag, and a tag the reader cannot parse is refused at load.
func TestStaticTagReachesGate(t *testing.T) {
	// A route lives only under `static table <name> route` (ze-static-conf.yang).
	input := `{"static":{"table":{"default":{"route":{` +
		`"10.42.0.0/24":{"tag":"4242","next":{"hop":{"192.0.2.1":{}}}},` +
		`"10.43.0.0/24":{"next":{"hop":{"192.0.2.1":{}}}},` +
		`"10.44.0.0/24":{"tag":"0","next":{"hop":{"192.0.2.1":{}}}}}}}}}`
	routes, err := parseStaticConfig(input, defaultOnlyRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 3 {
		t.Fatalf("parsed %d routes, want 3: every assertion below needs all of them", len(routes))
	}
	snap, err := ribpolicy.NewSnapshot(nil, rejectTagChain{})
	if err != nil {
		t.Fatal(err)
	}
	ribpolicy.Set(snap)
	t.Cleanup(func() { ribpolicy.Set(nil) })

	rib := locrib.NewRIB()
	carried := map[netip.Prefix]bool{}
	for _, r := range routes {
		path, err := staticPath(r)
		if err != nil {
			t.Fatalf("staticPath %s: %v", r.Prefix, err)
		}
		carried[r.Prefix] = path.HasTag
		rib.Insert(family.IPv4Unicast, r.Prefix, path)
	}
	if _, ok := rib.Best(family.IPv4Unicast, netip.MustParsePrefix("10.42.0.0/24")); ok {
		t.Error("static route tagged 4242 selected: its tag never reached the gate")
	}
	if _, ok := rib.Best(family.IPv4Unicast, netip.MustParsePrefix("10.43.0.0/24")); !ok {
		t.Error("untagged static route hidden")
	}
	if carried[netip.MustParsePrefix("10.43.0.0/24")] {
		t.Error("a route with no tag leaf carries a tag")
	}
	if !carried[netip.MustParsePrefix("10.44.0.0/24")] {
		t.Error("tag 0 is a real tag and must be carried")
	}

	bad := `{"static":{"table":{"default":{"route":{"10.45.0.0/24":{"tag":"many","next":{"hop":{"192.0.2.1":{}}}}}}}}}`
	if _, err := parseStaticConfig(bad, defaultOnlyRegistry()); err == nil {
		t.Error("a tag that is not a number was accepted")
	}
}
