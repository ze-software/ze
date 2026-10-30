// VALIDATES: the import policy gate on the Loc-RIB. A path a chain rejects is
// stored hidden: never selected, never an ECMP sibling, and a prefix whose every
// path is hidden has no best and is withdrawn. Publishing a new snapshot
// re-judges every stored path, hidden ones included, from the producer's own
// values, so relaxing a filter re-admits a path without the producer re-sending
// it and a set action is never applied twice.
// PREVENTS: a filtered prefix staying in the FIB, a relaxed filter leaving a
// route out until its protocol re-advertises it, a reload stacking a set action
// on its own earlier result, and the gate costing the BGP full-table insert path
// an allocation when no chain is configured.

package locrib

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// chainFunc adapts a function to ribpolicy.Chain, the interface the config
// component's compiled chains implement.
type chainFunc func(r ribpolicy.Route) ribpolicy.Verdict

func (f chainFunc) Evaluate(r ribpolicy.Route) ribpolicy.Verdict { return f(r) } //nolint:gocritic // hugeParam: the Chain signature passes Route by value

var (
	acceptAll = chainFunc(func(ribpolicy.Route) ribpolicy.Verdict {
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
	})
	rejectAll = chainFunc(func(ribpolicy.Route) ribpolicy.Verdict {
		return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
	})
)

// usePolicy publishes a snapshot on the seam, re-judges r's stored paths the
// way PublishPolicy does for the default RIB, and clears the seam when the test
// ends so no other test sees the chains.
func usePolicy(t *testing.T, r *RIB, sources map[redistevents.ProtocolID]ribpolicy.Chain, central ribpolicy.Chain) {
	t.Helper()
	snap, err := ribpolicy.NewSnapshot(sources, central)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	ribpolicy.Set(snap)
	t.Cleanup(func() { ribpolicy.Set(nil) })
	r.Reselect()
}

// clearPolicy removes every chain and re-judges r's stored paths.
func clearPolicy(r *RIB) {
	ribpolicy.Set(nil)
	r.Reselect()
}

func ospfPath(nextHop string, instance uint32) Path {
	return Path{
		Source:   redistevents.RegisterProtocol("ospf"),
		Instance: instance,
		NextHop:  netip.MustParseAddr(nextHop),
		Metric:   10,
	}
}

func hiddenCount(t *testing.T, r *RIB, prefix netip.Prefix) int {
	t.Helper()
	g, ok := r.Lookup(family.IPv4Unicast, prefix)
	if !ok {
		t.Fatalf("%s has no entry: a hidden path must stay stored", prefix)
	}
	hidden := 0
	for i := range g.Paths {
		if g.Paths[i].Hidden {
			hidden++
		}
	}
	return hidden
}

// TestLocRIBPolicyFieldsInEqualNotKey pins the per-field contract of the new
// Path fields: tag, level, OSPF route type and the hidden flag are compared by
// Equal, so a change reaches consumers, and stay out of key(), so a source
// re-advertising with a new tag updates its one path.
func TestLocRIBPolicyFieldsInEqualNotKey(t *testing.T) {
	base := ospfPath("192.0.2.1", 1)
	variants := map[string]func(*Path){
		"tag":        func(p *Path) { p.Tag = 7 },
		"has-tag":    func(p *Path) { p.HasTag = true },
		"level":      func(p *Path) { p.Level = ribpolicy.ISISLevel2 },
		"route-type": func(p *Path) { p.OSPFRouteType = ribpolicy.OSPFExternal2 },
		"hidden":     func(p *Path) { p.Hidden = true },
		"hidden-by":  func(p *Path) { p.HiddenBy = ribpolicy.ChainRoleCentral },
	}
	for name, change := range variants {
		changed := base
		change(&changed)
		if base.Equal(changed) {
			t.Errorf("%s: Equal ignores the field, so a change never reaches a consumer", name)
		}
		if base.key() != changed.key() {
			t.Errorf("%s: the field is in key(), so one source would hold two paths", name)
		}
	}
}

// TestLocRIBHiddenPathNotSelected inserts two equal-cost OSPF paths under a
// chain that rejects one next-hop: the rejected path is stored hidden, is never
// the best and is never an ECMP sibling, and Lookup still lists it.
func TestLocRIBHiddenPathNotSelected(t *testing.T) {
	r := NewRIB()
	log := &changeLog{}
	t.Cleanup(r.OnChange(log.record))
	accepted := netip.MustParseAddr("192.0.2.1")
	sibling := netip.MustParseAddr("192.0.2.2") // ties the accepted path: an ECMP member if selectable
	cheaper := netip.MustParseAddr("192.0.2.3") // beats the accepted path on metric: the best if selectable
	usePolicy(t, r, map[redistevents.ProtocolID]ribpolicy.Chain{
		redistevents.RegisterProtocol("ospf"): chainFunc(func(rt ribpolicy.Route) ribpolicy.Verdict {
			if rt.NextHop == accepted {
				return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
			}
			return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
		}),
	}, nil)

	prefix := netip.MustParsePrefix("10.1.0.0/24")
	cheap := ospfPath("192.0.2.3", 3)
	cheap.Metric = 5
	r.Insert(family.IPv4Unicast, prefix, ospfPath("192.0.2.1", 1))
	r.Insert(family.IPv4Unicast, prefix, ospfPath("192.0.2.2", 2))
	r.Insert(family.IPv4Unicast, prefix, cheap)

	best, ok := r.Best(family.IPv4Unicast, prefix)
	if !ok {
		t.Fatal("the accepted path must be selected")
	}
	if best.NextHop != accepted {
		t.Fatalf("Best returned %s, a hidden path", best.NextHop)
	}
	if got := log.last(t); len(got.ECMP) != 0 {
		t.Fatalf("a hidden path is an ECMP sibling: %v", got.ECMP)
	}
	if n := hiddenCount(t, r, prefix); n != 2 {
		t.Fatalf("hidden paths = %d, want 2: Lookup must still list the rejected paths", n)
	}
	if lpm, _, ok := r.LPM(family.IPv4Unicast, netip.MustParseAddr("10.1.0.9")); !ok || lpm.NextHop != accepted {
		t.Fatalf("LPM = %s, %v: want the accepted path", lpm.NextHop, ok)
	}

	// With the chain gone the cheaper path wins, and once it is gone the
	// re-admitted sibling joins the accepted path's equal-cost set.
	clearPolicy(r)
	if got := log.last(t); got.Best.NextHop != cheaper {
		t.Fatalf("after the filter is removed best = %s, want the re-admitted %s", got.Best.NextHop, cheaper)
	}
	r.Remove(family.IPv4Unicast, prefix, cheap.Source, cheap.Instance)
	if got := log.last(t); len(got.ECMP) != 1 || got.ECMP[0].Addr != sibling {
		t.Fatalf("ECMP = %v, want the re-admitted sibling %s", got.ECMP, sibling)
	}
}

// TestLocRIBAllHiddenDispatchesRemove is AC-8: when every path of a selected
// prefix becomes hidden, through a reload or through a re-advertisement the
// chain rejects, the subscriber sees ChangeRemove, the event sysrib turns into
// a FIB withdrawal. A new prefix whose only path is hidden dispatches nothing.
func TestLocRIBAllHiddenDispatchesRemove(t *testing.T) {
	r := NewRIB()
	log := &changeLog{}
	t.Cleanup(r.OnChange(log.record))
	prefix := netip.MustParsePrefix("10.2.0.0/24")
	r.Insert(family.IPv4Unicast, prefix, staticRoute())

	usePolicy(t, r, nil, rejectAll)
	if got := log.last(t); got.Kind != ChangeRemove {
		t.Fatalf("reload hiding the only path dispatched %v, want ChangeRemove", got.Kind)
	}
	if _, ok := r.Best(family.IPv4Unicast, prefix); ok {
		t.Fatal("Best answered for a prefix whose every path is hidden")
	}
	if _, _, ok := r.LPM(family.IPv4Unicast, netip.MustParseAddr("10.2.0.1")); ok {
		t.Fatal("LPM answered with a hidden path")
	}

	// A new prefix whose only path is rejected dispatches nothing.
	log.reset()
	fresh := netip.MustParsePrefix("10.3.0.0/24")
	if _, changed := r.Insert(family.IPv4Unicast, fresh, staticRoute()); changed {
		t.Fatal("an insert the chain rejects reported a best change")
	}
	if len(log.changes) != 0 {
		t.Fatalf("a hidden-only new prefix dispatched %d changes", len(log.changes))
	}
	if n := hiddenCount(t, r, fresh); n != 1 {
		t.Fatalf("hidden paths = %d, want 1", n)
	}

	// A selected path whose re-advertisement the chain rejects is withdrawn.
	clearPolicy(r)
	sel := netip.MustParsePrefix("10.4.0.0/24")
	r.Insert(family.IPv4Unicast, sel, ospfPath("192.0.2.1", 1))
	usePolicy(t, r, map[redistevents.ProtocolID]ribpolicy.Chain{
		redistevents.RegisterProtocol("ospf"): chainFunc(func(rt ribpolicy.Route) ribpolicy.Verdict {
			if rt.Metric > 10 {
				return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
			}
			return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
		}),
	}, nil)
	log.reset()
	readvert := ospfPath("192.0.2.1", 1)
	readvert.Metric = 20
	if _, changed := r.Insert(family.IPv4Unicast, sel, readvert); !changed {
		t.Fatal("hiding the selected path must report a best change")
	}
	if got := log.last(t); got.Kind != ChangeRemove {
		t.Fatalf("a rejected re-advertisement dispatched %v, want ChangeRemove", got.Kind)
	}
}

// TestLocRIBReselectReadmitsHidden is AC-5: a path hidden by a chain is
// installed when the chain is relaxed, with no producer event, and the
// subscriber sees ChangeAdd. PublishPolicy does the same for the default RIB.
func TestLocRIBReselectReadmitsHidden(t *testing.T) {
	r := NewRIB()
	log := &changeLog{}
	t.Cleanup(r.OnChange(log.record))
	usePolicy(t, r, nil, rejectAll)
	prefix := netip.MustParsePrefix("10.5.0.0/24")
	r.Insert(family.IPv4Unicast, prefix, staticRoute())
	if _, ok := r.Best(family.IPv4Unicast, prefix); ok {
		t.Fatal("a rejected path was selected")
	}

	usePolicy(t, r, nil, acceptAll)
	got := log.last(t)
	if got.Kind != ChangeAdd {
		t.Fatalf("re-admission dispatched %v, want ChangeAdd", got.Kind)
	}
	if got.Best.Hidden {
		t.Fatal("the re-admitted best is still flagged hidden")
	}
	if n := hiddenCount(t, r, prefix); n != 0 {
		t.Fatalf("hidden paths = %d after the relax, want 0", n)
	}

	// PublishPolicy's body publishes and re-judges the RIB it is handed (the
	// process-wide one in production; Default itself reads an env key this
	// package's test binary does not register).
	snap, err := ribpolicy.NewSnapshot(nil, rejectAll)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	publishPolicy(snap, r)
	if _, ok := r.Best(family.IPv4Unicast, prefix); ok {
		t.Fatal("publishing a reject did not re-judge the stored path")
	}
	publishPolicy(nil, r)
	if _, ok := r.Best(family.IPv4Unicast, prefix); !ok {
		t.Fatal("clearing the policy did not re-admit the stored path")
	}
	publishPolicy(nil, nil) // a forked plugin process: nothing to re-judge, no panic
}

// TestLocRIBSetActionsFromOriginal is R-3: a chain whose set actions depend on
// the value they replace (tag + 1, and distance 90 only for the producer's own
// tag) gives the same result however many times the policy is re-published, and
// removing the chain restores the producer's values.
func TestLocRIBSetActionsFromOriginal(t *testing.T) {
	declare(t, map[string]uint8{"static": 10})
	r := NewRIB()
	const producerTag = 4242
	bump := chainFunc(func(rt ribpolicy.Route) ribpolicy.Verdict {
		sets := ribpolicy.Sets{Tag: rt.Tag + 1, HasTag: true}
		if rt.HasTag && rt.Tag == producerTag {
			sets.Distance, sets.HasDistance = 90, true
		}
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept, Sets: sets}
	})
	static := staticRoute()
	static.Tag, static.HasTag = producerTag, true
	prefix := netip.MustParsePrefix("10.6.0.0/24")
	usePolicy(t, r, map[redistevents.ProtocolID]ribpolicy.Chain{static.Source: bump}, nil)
	r.Insert(family.IPv4Unicast, prefix, static)

	for range 3 {
		r.Reselect()
	}
	best, ok := r.Best(family.IPv4Unicast, prefix)
	if !ok {
		t.Fatal("the accepted path has no best")
	}
	if best.Tag != producerTag+1 {
		t.Fatalf("tag = %d after three reloads, want %d: the set action stacked", best.Tag, producerTag+1)
	}
	if best.AdminDistance != 90 {
		t.Fatalf("distance = %d, want 90: the match read a rewritten tag", best.AdminDistance)
	}

	clearPolicy(r)
	best, _ = r.Best(family.IPv4Unicast, prefix)
	if best.Tag != producerTag {
		t.Fatalf("tag = %d after the chain is removed, want the producer's %d", best.Tag, producerTag)
	}
	if best.HasDistanceOverride {
		t.Fatal("the set distance outlived the chain")
	}
	if best.AdminDistance != 10 {
		t.Fatalf("distance = %d after the chain is removed, want the declared 10", best.AdminDistance)
	}
}

// TestLocRIBCentralChainAllSources is AC-7: the central chain judges every
// source's path, BGP's best included, and a prefix it rejects has no best.
func TestLocRIBCentralChainAllSources(t *testing.T) {
	r := NewRIB()
	log := &changeLog{}
	t.Cleanup(r.OnChange(log.record))
	bogon := netip.MustParsePrefix("192.0.2.0/24")
	usePolicy(t, r, nil, chainFunc(func(rt ribpolicy.Route) ribpolicy.Verdict {
		if rt.Prefix == bogon {
			return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
		}
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
	}))

	bgp := Path{Source: redistevents.RegisterProtocol("bgp"), NextHop: netip.MustParseAddr("203.0.113.1"), IsBGP: true, IsEBGP: true}
	for _, p := range []Path{bgp, ospfPath("192.0.2.9", 1), staticRoute()} {
		r.Insert(family.IPv4Unicast, bogon, p)
	}
	if _, ok := r.Best(family.IPv4Unicast, bogon); ok {
		t.Fatal("a source's path for the rejected prefix was selected")
	}
	if n := hiddenCount(t, r, bogon); n != 3 {
		t.Fatalf("hidden paths = %d, want every source's path (3)", n)
	}
	if len(log.changes) != 0 {
		t.Fatalf("a rejected prefix dispatched %d changes", len(log.changes))
	}
	other := netip.MustParsePrefix("10.7.0.0/24")
	r.Insert(family.IPv4Unicast, other, bgp)
	if _, ok := r.Best(family.IPv4Unicast, other); !ok {
		t.Fatal("the central chain rejected a prefix it accepts")
	}
}

// TestLocRIBSetDistanceByTag is AC-9 at the Loc-RIB: an OSPF chain setting
// distance 90 on tag 100 makes the tagged OSPF route beat static at 100, the
// untagged one stays at OSPF's declared 110 and loses, and removing the chain
// returns the tagged one to 110.
func TestLocRIBSetDistanceByTag(t *testing.T) {
	declare(t, map[string]uint8{"static": 100, "ospf": 110})
	r := NewRIB()
	ospf := redistevents.RegisterProtocol("ospf")
	usePolicy(t, r, map[redistevents.ProtocolID]ribpolicy.Chain{
		ospf: chainFunc(func(rt ribpolicy.Route) ribpolicy.Verdict {
			v := ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
			if rt.HasTag && rt.Tag == 100 {
				v.Sets = ribpolicy.Sets{Distance: 90, HasDistance: true}
			}
			return v
		}),
	}, nil)

	tagged := netip.MustParsePrefix("10.8.0.0/24")
	plain := netip.MustParsePrefix("10.9.0.0/24")
	taggedPath := ospfPath("192.0.2.1", 1)
	taggedPath.Tag, taggedPath.HasTag = 100, true
	taggedPath.OSPFRouteType = ribpolicy.OSPFExternal2
	for _, prefix := range []netip.Prefix{tagged, plain} {
		r.Insert(family.IPv4Unicast, prefix, staticRoute())
	}
	r.Insert(family.IPv4Unicast, tagged, taggedPath)
	r.Insert(family.IPv4Unicast, plain, ospfPath("192.0.2.1", 1))

	winner := func(prefix netip.Prefix) Path {
		t.Helper()
		best, ok := r.Best(family.IPv4Unicast, prefix)
		if !ok {
			t.Fatalf("%s has no best", prefix)
		}
		return best
	}
	if got := winner(tagged); got.Source != ospf || got.AdminDistance != 90 {
		t.Fatalf("tagged prefix: winner %d at %d, want OSPF at 90", got.Source, got.AdminDistance)
	}
	if got := winner(plain); got.Source == ospf {
		t.Fatal("untagged prefix: OSPF at its declared 110 beat static at 100")
	}

	clearPolicy(r)
	if got := winner(tagged); got.Source == ospf {
		t.Fatal("after the chain is removed, OSPF still beats static")
	}
}

// TestLocRIBInsertNoChainZeroAlloc is AC-16: with no chain configured, and
// with a chain configured only for another source, the insert of a BGP path
// that updates an existing prefix allocates nothing.
func TestLocRIBInsertNoChainZeroAlloc(t *testing.T) {
	r := NewRIB()
	prefix := netip.MustParsePrefix("10.10.0.0/24")
	bgp := Path{Source: redistevents.RegisterProtocol("bgp"), NextHop: netip.MustParseAddr("203.0.113.1"), IsBGP: true}
	r.Insert(family.IPv4Unicast, prefix, bgp)

	ribpolicy.Set(nil)
	measure := func(name string) {
		t.Helper()
		metric := uint32(0)
		allocs := testing.AllocsPerRun(1000, func() {
			metric++
			bgp.Metric = metric
			r.Insert(family.IPv4Unicast, prefix, bgp)
		})
		if allocs != 0 {
			t.Errorf("%s: insert allocates %.1f times per call, want 0", name, allocs)
		}
	}
	measure("no chain")

	usePolicy(t, r, map[redistevents.ProtocolID]ribpolicy.Chain{
		redistevents.RegisterProtocol("ospf"): rejectAll,
	}, nil)
	measure("a chain for another source")
}

// TestLocRIBSnapshotRace is R-7 under -race: inserts on many prefixes run while
// snapshots are published and re-judged. Once the last publication's Reselect
// returns, every stored path reflects the last snapshot, whichever order the
// inserts and publications interleaved in.
func TestLocRIBSnapshotRace(t *testing.T) {
	r := NewRIB()
	t.Cleanup(func() { ribpolicy.Set(nil) })
	reject, err := ribpolicy.NewSnapshot(nil, rejectAll)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	accept, err := ribpolicy.NewSnapshot(nil, acceptAll)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}

	prefixes := make([]netip.Prefix, 64)
	for i := range prefixes {
		prefixes[i] = netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 20, byte(i), 0}), 24)
	}
	var wg sync.WaitGroup
	for w := range 4 {
		wg.Go(func() {
			for n := range 200 {
				p := ospfPath("192.0.2.1", uint32(w))
				p.Metric = uint32(n)
				r.Insert(family.IPv4Unicast, prefixes[(n+w)%len(prefixes)], p)
			}
		})
	}
	wg.Go(func() {
		for n := range 50 {
			if n%2 == 0 {
				ribpolicy.Set(reject)
			} else {
				ribpolicy.Set(accept)
			}
			r.Reselect()
		}
	})
	wg.Wait()

	ribpolicy.Set(reject)
	r.Reselect()
	for _, prefix := range prefixes {
		g, ok := r.Lookup(family.IPv4Unicast, prefix)
		if !ok {
			continue
		}
		for i := range g.Paths {
			if !g.Paths[i].Hidden {
				t.Fatalf("%s: a path escaped the last snapshot's reject", prefix)
			}
		}
	}
}
