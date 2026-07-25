package locrib

import (
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// Stable test ProtocolIDs. We do not use redistevents.RegisterProtocol
// because it is process-global; parallel tests would race on the registry
// state. Loc-RIB only cares about the numeric value of Source, not its
// registered name.
const (
	idStatic redistevents.ProtocolID = 1
	idBGP    redistevents.ProtocolID = 2
	idOSPF   redistevents.ProtocolID = 3
)

var (
	famV4 = family.IPv4Unicast
	pfx   = netip.MustParsePrefix("10.0.0.0/24")
)

func pathStatic() Path {
	return Path{
		Source:        idStatic,
		Instance:      0,
		NextHop:       netip.MustParseAddr("192.0.2.1"),
		AdminDistance: 1,
	}
}

func pathBGP(instance, metric uint32) Path {
	return Path{
		Source:        idBGP,
		Instance:      instance,
		NextHop:       netip.MustParseAddr("192.0.2.2"),
		AdminDistance: 20,
		Metric:        metric,
	}
}

func pathOSPF(metric uint32) Path {
	return Path{
		Source:        idOSPF,
		Instance:      0,
		NextHop:       netip.MustParseAddr("192.0.2.3"),
		AdminDistance: 110,
		Metric:        metric,
	}
}

// TestInsertSelectsByAdminDistance validates the cross-protocol best-path
// ranking: Static (1) beats BGP (20) beats OSPF (110).
//
// VALIDATES: selectBest picks the lowest AdminDistance across protocols.
// PREVENTS: routes from a less-trusted protocol overriding a more-trusted one.
func TestInsertSelectsByAdminDistance(t *testing.T) {
	r := NewRIB()

	// First insert: OSPF only -- it wins by default.
	best, changed := r.Insert(famV4, pfx, pathOSPF(10))
	require.True(t, changed)
	assert.Equal(t, idOSPF, best.Source)

	// BGP arrives with lower distance -- becomes new best.
	best, changed = r.Insert(famV4, pfx, pathBGP(1, 50))
	require.True(t, changed)
	assert.Equal(t, idBGP, best.Source)

	// Static trumps BGP.
	best, changed = r.Insert(famV4, pfx, pathStatic())
	require.True(t, changed)
	assert.Equal(t, idStatic, best.Source)

	// Another OSPF re-advertise: best unchanged.
	_, changed = r.Insert(famV4, pfx, pathOSPF(5))
	assert.False(t, changed, "re-advertising a non-best path does not change best")
}

// TestTiebreakByMetric validates the within-AdminDistance tiebreaker: lower
// Metric wins.
func TestTiebreakByMetric(t *testing.T) {
	r := NewRIB()

	r.Insert(famV4, pfx, pathBGP(1, 100))
	best, changed := r.Insert(famV4, pfx, pathBGP(2, 50))
	require.True(t, changed, "lower-metric BGP should become new best")
	assert.Equal(t, uint32(50), best.Metric)
	assert.Equal(t, uint32(2), best.Instance)
}

// TestUpsertReplacesSameSourceInstance verifies that re-inserting with the
// same (Source, Instance) overwrites in place rather than appending.
func TestUpsertReplacesSameSourceInstance(t *testing.T) {
	r := NewRIB()
	r.Insert(famV4, pfx, pathBGP(1, 100))
	r.Insert(famV4, pfx, pathBGP(1, 50))

	g, ok := r.Lookup(famV4, pfx)
	require.True(t, ok)
	assert.Len(t, g.Paths, 1, "same (source,instance) must upsert in place")
	assert.Equal(t, uint32(50), g.Paths[0].Metric)
}

// TestRemoveFallsBackToNextBest validates that removing the current best
// surfaces the next-best Path.
func TestRemoveFallsBackToNextBest(t *testing.T) {
	r := NewRIB()
	r.Insert(famV4, pfx, pathStatic())
	r.Insert(famV4, pfx, pathBGP(1, 10))

	best, changed := r.Remove(famV4, pfx, idStatic, 0)
	require.True(t, changed, "removing best must report change")
	assert.Equal(t, idBGP, best.Source, "BGP falls through as new best")

	_, changed = r.Remove(famV4, pfx, idBGP, 1)
	require.True(t, changed)

	_, ok := r.Lookup(famV4, pfx)
	assert.False(t, ok, "last path removed deletes the prefix entry")
}

// TestRemoveAbsent is a no-op returning (zero, false).
func TestRemoveAbsent(t *testing.T) {
	r := NewRIB()
	_, changed := r.Remove(famV4, pfx, idBGP, 1)
	assert.False(t, changed)
}

// TestBestAndLookup verify the read-only accessors.
func TestBestAndLookup(t *testing.T) {
	r := NewRIB()
	r.Insert(famV4, pfx, pathStatic())
	r.Insert(famV4, pfx, pathBGP(1, 10))

	best, ok := r.Best(famV4, pfx)
	require.True(t, ok)
	assert.Equal(t, idStatic, best.Source)

	g, ok := r.Lookup(famV4, pfx)
	require.True(t, ok)
	assert.Len(t, g.Paths, 2)
	assert.Equal(t, 1, r.Len(famV4), "two paths share one prefix => Len is 1")
}

// TestInvalidPathRejected verifies that a Path with Source=Unspecified is
// silently rejected.
func TestInvalidPathRejected(t *testing.T) {
	r := NewRIB()
	_, changed := r.Insert(famV4, pfx, Path{}) // zero Source == Unspecified
	assert.False(t, changed)
	assert.Equal(t, 0, r.Len(famV4))
}

// TestIterate walks every prefix in the family.
func TestIterate(t *testing.T) {
	r := NewRIB()
	r.Insert(famV4, netip.MustParsePrefix("10.0.0.0/24"), pathBGP(1, 10))
	r.Insert(famV4, netip.MustParsePrefix("10.1.0.0/24"), pathBGP(1, 10))
	r.Insert(famV4, netip.MustParsePrefix("10.2.0.0/24"), pathBGP(1, 10))

	seen := map[netip.Prefix]bool{}
	r.Iterate(famV4, func(p netip.Prefix, _ PathGroup) bool {
		seen[p] = true
		return true
	})
	assert.Len(t, seen, 3)
}

// TestFamilies returns every family that has at least one prefix.
func TestFamilies(t *testing.T) {
	r := NewRIB()
	assert.Empty(t, r.Families())

	r.Insert(famV4, pfx, pathBGP(1, 10))
	assert.Equal(t, []family.Family{famV4}, r.Families())
}

// TestOnChangeFires validates that Insert/Remove invoke subscribed handlers
// with the correct ChangeKind and Best.
func TestOnChangeFires(t *testing.T) {
	r := NewRIB()
	var changes []Change
	unsub := r.OnChange(func(c Change) { changes = append(changes, c) })

	// First insert on a new prefix => Add.
	r.Insert(famV4, pfx, pathBGP(1, 10))
	require.Len(t, changes, 1)
	assert.Equal(t, ChangeAdd, changes[0].Kind)
	assert.Equal(t, idBGP, changes[0].Best.Source)

	// Replacing best with a lower-distance path => Update.
	r.Insert(famV4, pfx, pathStatic())
	require.Len(t, changes, 2)
	assert.Equal(t, ChangeUpdate, changes[1].Kind)
	assert.Equal(t, idStatic, changes[1].Best.Source)

	// Inserting a worse BGP path behind Static => no change.
	r.Insert(famV4, pfx, pathBGP(2, 5))
	assert.Len(t, changes, 2, "worse path must not fire a change")

	// Removing the best falls back to next-best => Update.
	// BGP(2, metric=5) wins over BGP(1, metric=10) on the metric tiebreak.
	r.Remove(famV4, pfx, idStatic, 0)
	require.Len(t, changes, 3)
	assert.Equal(t, ChangeUpdate, changes[2].Kind)
	assert.Equal(t, idBGP, changes[2].Best.Source)
	assert.Equal(t, uint32(2), changes[2].Best.Instance)

	// Removing a non-best path fires nothing.
	r.Remove(famV4, pfx, idBGP, 1)
	assert.Len(t, changes, 3, "removing a non-best path must not fire")

	// Removing the last path => Remove.
	r.Remove(famV4, pfx, idBGP, 2)
	require.Len(t, changes, 4)
	assert.Equal(t, ChangeRemove, changes[3].Kind)
	assert.Equal(t, Path{}, changes[3].Best)

	// Unsubscribe stops delivery.
	unsub()
	r.Insert(famV4, pfx, pathBGP(1, 10))
	assert.Len(t, changes, 4, "unsubscribed handler must not fire")
}

// TestOnChangeCarriesECMPSiblings validates that a Change emitted for a
// multi-path PathGroup carries the intra-source equal-cost sibling next-hops on
// Change.ECMP (computed at emit, so a consumer needs no re-lookup), excluding
// the best's own next-hop, and that a single-path insert carries nil ECMP.
//
// VALIDATES: siblingNextHops populates Change.ECMP with the equal-cost siblings
// of Best (same Source, same AdminDistance + Metric, different valid NextHop)
// for an IS-IS-shaped group (one Path per next-hop, distinct Instance), on both
// the insert() best-change path and the Remove() fallback path.
// PREVENTS: a regression where multipath ECMP collapses to the single best
// next-hop because the Change drops the siblings, forcing consumers back to a
// per-change RIB Lookup.
func TestOnChangeCarriesECMPSiblings(t *testing.T) {
	r := NewRIB()
	var changes []Change
	r.OnChange(func(c Change) { changes = append(changes, c) })

	ecmpPfx := netip.MustParsePrefix("10.50.0.0/24")
	nh1 := netip.MustParseAddr("10.0.0.1")
	nh2 := netip.MustParseAddr("10.0.0.2")

	isis := func(instance uint32, nh netip.Addr, metric uint32) Path {
		return Path{Source: idOSPF, Instance: instance, NextHop: nh, AdminDistance: 115, Metric: metric}
	}

	// First equal-cost IS-IS Path: a fresh single-path group => nil ECMP. This
	// is the "single-path insert yields nil ECMP" assertion.
	r.Insert(famV4, ecmpPfx, isis(0, nh1, 30))
	require.Len(t, changes, 1)
	assert.Equal(t, ChangeAdd, changes[0].Kind)
	assert.Nil(t, changes[0].ECMP, "single-path group must carry nil ECMP")

	// Second IS-IS Path with a strictly lower metric becomes the new best on its
	// own (still single equal-cost member: nh1 at metric 30 is NOT equal-cost to
	// nh2 at metric 20), so this Change also carries nil ECMP.
	r.Insert(famV4, ecmpPfx, isis(1, nh2, 20))
	require.Len(t, changes, 2)
	assert.Equal(t, ChangeUpdate, changes[1].Kind)
	assert.Equal(t, nh2, changes[1].Best.NextHop)
	assert.Nil(t, changes[1].ECMP, "non-equal-cost group must carry nil ECMP")

	// Update the first Path down to the same metric: now nh1 and nh2 are two
	// equal-cost members (same Source, same AdminDistance + Metric, distinct
	// Instance, different next-hop) -- the IS-IS ECMP shape. The best identity
	// changes (Metric of Instance 0 went 30 -> 20, and first-seen tiebreak now
	// favors it), so a Change fires carrying the sibling on ECMP.
	r.Insert(famV4, ecmpPfx, isis(0, nh1, 20))
	require.Len(t, changes, 3)
	assert.Equal(t, ChangeUpdate, changes[2].Kind)
	assertECMPGroup(t, changes[2], nh1, nh2)

	// Remove() fallback: drop the current best; the surviving Path is the only
	// remaining member, so the synthesized ChangeUpdate carries nil ECMP (no
	// sibling left). This exercises the Remove path's siblingNextHops call.
	best := changes[2].Best
	r.Remove(famV4, ecmpPfx, best.Source, best.Instance)
	require.Len(t, changes, 4)
	assert.Equal(t, ChangeUpdate, changes[3].Kind)
	assert.Nil(t, changes[3].ECMP, "single surviving member must carry nil ECMP")
}

// TestOnChangeCarriesBestPathECMP validates the BGP-multipath shape (rib-arch-4):
// a source that arbitrates ONE best Path and carries its own equal-cost set on
// Best.ECMP has those next-hops surfaced on Change.ECMP without inserting one
// Path per next-hop, and an ECMP-set change (best next-hop stable) fires a
// membership-only ChangeUpdate.
//
// VALIDATES: siblingNextHops returns Best.ECMP directly; insert()'s ecmpChanged
// branch dispatches when only the ECMP set changes.
// PREVENTS: BGP multipath collapsing to a single kernel next-hop because BGP
// arbitrates one best Path and the siblings never entered the PathGroup.
func TestOnChangeCarriesBestPathECMP(t *testing.T) {
	r := NewRIB()
	var changes []Change
	r.OnChange(func(c Change) { changes = append(changes, c) })

	pfx := netip.MustParsePrefix("10.60.0.0/24")
	nh1 := netip.MustParseAddr("10.0.0.1")
	nh2 := netip.MustParseAddr("10.0.0.2")
	nh3 := netip.MustParseAddr("10.0.0.3")

	bgpMulti := func(ecmp ...netip.Addr) Path {
		return Path{Source: idBGP, Instance: 0, NextHop: nh1, AdminDistance: 20, Metric: 0, ECMP: ecmp}
	}

	// One BGP best Path carrying its own ECMP siblings (single Path, no per-
	// next-hop Instances).
	r.InsertForward(famV4, pfx, bgpMulti(nh2, nh3), nil)
	require.Len(t, changes, 1)
	assert.Equal(t, ChangeAdd, changes[0].Kind)
	assert.Equal(t, nh1, changes[0].Best.NextHop)
	assert.Equal(t, []netip.Addr{nh2, nh3}, changes[0].ECMP, "Best.ECMP must surface on Change.ECMP")

	// Shrink the ECMP set to [nh2]: same best next-hop, so this is an ECMP
	// membership-only ChangeUpdate.
	r.InsertForward(famV4, pfx, bgpMulti(nh2), nil)
	require.Len(t, changes, 2)
	assert.Equal(t, ChangeUpdate, changes[1].Kind)
	assert.Equal(t, []netip.Addr{nh2}, changes[1].ECMP)

	// Drop the ECMP set entirely: another membership-only ChangeUpdate to nil.
	r.InsertForward(famV4, pfx, bgpMulti(), nil)
	require.Len(t, changes, 3)
	assert.Equal(t, ChangeUpdate, changes[2].Kind)
	assert.Nil(t, changes[2].ECMP, "cleared ECMP set surfaces as nil")
}

// TestOnChangeDispatchesECMPMembershipChanges validates that adding or removing
// an equal-cost sibling emits ChangeUpdate even when the stable first-seen best
// Path does not change.
//
// VALIDATES: Loc-RIB dispatches ECMP membership-only changes so sysrib receives
// the final multipath set after protocols insert one Path per next-hop.
// PREVENTS: OSPF/IS-IS ECMP collapsing to a single kernel next-hop because the
// first Insert emitted a single-path Add and later equal-cost siblings were silent.
func TestOnChangeDispatchesECMPMembershipChanges(t *testing.T) {
	r := NewRIB()
	var changes []Change
	r.OnChange(func(c Change) { changes = append(changes, c) })

	ecmpPfx := netip.MustParsePrefix("10.51.0.0/24")
	nh1 := netip.MustParseAddr("10.0.1.1")
	nh2 := netip.MustParseAddr("10.0.1.2")
	nh3 := netip.MustParseAddr("10.0.1.3")
	path := func(instance uint32, nh netip.Addr) Path {
		return Path{Source: idOSPF, Instance: instance, NextHop: nh, AdminDistance: 115, Metric: 10}
	}

	r.Insert(famV4, ecmpPfx, path(0, nh1))
	require.Len(t, changes, 1)
	assert.Equal(t, ChangeAdd, changes[0].Kind)
	assert.Nil(t, changes[0].ECMP)

	r.Insert(famV4, ecmpPfx, path(1, nh2))
	require.Len(t, changes, 2, "adding an equal-cost sibling must dispatch")
	assert.Equal(t, ChangeUpdate, changes[1].Kind)
	assert.Equal(t, nh1, changes[1].Best.NextHop, "first-seen best must stay stable")
	assertECMPGroup(t, changes[1], nh1, nh2)

	r.Insert(famV4, ecmpPfx, path(2, nh3))
	require.Len(t, changes, 3, "adding another equal-cost sibling must dispatch")
	assert.Equal(t, ChangeUpdate, changes[2].Kind)
	assert.Equal(t, nh1, changes[2].Best.NextHop)
	assert.ElementsMatch(t, []netip.Addr{nh2, nh3}, changes[2].ECMP)

	r.Remove(famV4, ecmpPfx, idOSPF, 2)
	require.Len(t, changes, 4, "removing a non-best equal-cost sibling must dispatch")
	assert.Equal(t, ChangeUpdate, changes[3].Kind)
	assert.Equal(t, nh1, changes[3].Best.NextHop)
	assert.Equal(t, []netip.Addr{nh2}, changes[3].ECMP)

	r.Remove(famV4, ecmpPfx, idOSPF, 1)
	require.Len(t, changes, 5, "shrinking to a single best must dispatch")
	assert.Equal(t, ChangeUpdate, changes[4].Kind)
	assert.Equal(t, nh1, changes[4].Best.NextHop)
	assert.Nil(t, changes[4].ECMP)
}

// assertECMPGroup asserts that c describes a two-member equal-cost group over
// {a, b}: Best.NextHop is one of them, ECMP holds exactly the other, and ECMP
// never contains Best.NextHop. Order-independent so it does not depend on the
// internal slice ordering of selectBest.
func assertECMPGroup(t *testing.T, c Change, a, b netip.Addr) {
	t.Helper()
	best := c.Best.NextHop
	if best != a && best != b {
		t.Fatalf("Best.NextHop = %s, want one of {%s, %s}", best, a, b)
	}
	require.Len(t, c.ECMP, 1, "ECMP must hold exactly the one sibling")
	assert.NotEqual(t, best, c.ECMP[0], "ECMP must never contain Best.NextHop")
	union := map[netip.Addr]bool{best: true, c.ECMP[0]: true}
	assert.True(t, union[a] && union[b],
		"Best.NextHop + ECMP = {%s, %s}, want {%s, %s}", best, c.ECMP[0], a, b)
}

// countingHandle is a ForwardHandle used by the fastpath tests. AddRef /
// Release increment counters so tests can assert the reactor-side
// refcount contract without dragging the reactor into locrib.
type countingHandle struct {
	addRefs  atomic.Int32
	releases atomic.Int32
}

func (h *countingHandle) AddRef()  { h.addRefs.Add(1) }
func (h *countingHandle) Release() { h.releases.Add(1) }

// bytesHandle also satisfies the optional ForwardBytes interface,
// exercising the type-assertion path subscribers use to read wire
// payload.
type bytesHandle struct {
	countingHandle
	payload []byte
}

func (h *bytesHandle) Bytes() []byte { return h.payload }

// TestInsertLeavesForwardNil validates that the legacy Insert entry point
// dispatches Changes with Forward == nil. Non-BGP producers rely on this.
//
// VALIDATES: design-rib-rs-fastpath.md -- non-BGP producers leave Forward nil.
// PREVENTS: accidental handle propagation from refactors that change Insert.
func TestInsertLeavesForwardNil(t *testing.T) {
	r := NewRIB()
	var last Change
	r.OnChange(func(c Change) { last = c })

	r.Insert(famV4, pfx, pathStatic())

	assert.Equal(t, ChangeAdd, last.Kind)
	assert.Nil(t, last.Forward, "Insert without handle must leave Change.Forward nil")
}

// TestInsertForwardPropagates validates that InsertForward places the
// handle on ChangeAdd and ChangeUpdate dispatches.
//
// VALIDATES: design-rib-rs-fastpath.md -- BGP producer populates Forward.
// PREVENTS: a subscriber seeing a nil handle when BGP supplied one.
func TestInsertForwardPropagates(t *testing.T) {
	r := NewRIB()
	var seen []Change
	r.OnChange(func(c Change) { seen = append(seen, c) })

	h1 := &countingHandle{}
	r.InsertForward(famV4, pfx, pathBGP(1, 50), h1)
	require.Len(t, seen, 1)
	assert.Equal(t, ChangeAdd, seen[0].Kind)
	assert.Same(t, h1, seen[0].Forward, "Add must carry the handle")

	// Update with a new handle replaces the forward on the next Change.
	h2 := &countingHandle{}
	r.InsertForward(famV4, pfx, pathStatic(), h2)
	require.Len(t, seen, 2)
	assert.Equal(t, ChangeUpdate, seen[1].Kind)
	assert.Same(t, h2, seen[1].Forward, "Update must carry the new handle")

	// locrib does NOT AddRef / Release on the hot path -- subscribers do.
	assert.Zero(t, h1.addRefs.Load(), "locrib must not AddRef")
	assert.Zero(t, h1.releases.Load(), "locrib must not Release")
	assert.Zero(t, h2.addRefs.Load(), "locrib must not AddRef")
	assert.Zero(t, h2.releases.Load(), "locrib must not Release")
}

// TestInsertForwardNilHandle documents that a nil handle is legal and
// passes through as nil on the dispatched Change.
//
// VALIDATES: InsertForward is equivalent to Insert when handle is nil.
// PREVENTS: forced-handle regressions that would alloc on non-forward paths.
func TestInsertForwardNilHandle(t *testing.T) {
	r := NewRIB()
	var last Change
	r.OnChange(func(c Change) { last = c })

	r.InsertForward(famV4, pfx, pathBGP(1, 50), nil)
	assert.Equal(t, ChangeAdd, last.Kind)
	assert.Nil(t, last.Forward)
}

// TestInsertForwardSubscriberAddRef exercises the documented contract: a
// subscriber that wants to retain the buffer past dispatch calls AddRef
// from within the handler; Release happens later.
//
// VALIDATES: subscribers own the retention decision; locrib does not.
// PREVENTS: regressions that move AddRef into locrib and break producers
// that don't want an extra ref on the hot path.
func TestInsertForwardSubscriberAddRef(t *testing.T) {
	r := NewRIB()
	h := &countingHandle{}

	r.OnChange(func(c Change) {
		if c.Forward != nil {
			c.Forward.AddRef()
		}
	})

	r.InsertForward(famV4, pfx, pathBGP(1, 50), h)

	assert.Equal(t, int32(1), h.addRefs.Load(), "subscriber AddRef must fire exactly once")
	assert.Zero(t, h.releases.Load(), "subscriber had not Released yet")
}

// TestInsertForwardBytesOptional validates that a ForwardHandle that
// also implements ForwardBytes is reachable by a subscriber via type
// assertion, and that the Bytes() contract is visible through the
// interface alone (no rib-package import needed).
//
// VALIDATES: ForwardBytes optional capability.
// PREVENTS: a subscriber being forced to import the producer's package
// to read the retained wire bytes.
func TestInsertForwardBytesOptional(t *testing.T) {
	r := NewRIB()
	h := &bytesHandle{payload: []byte{0xde, 0xad, 0xbe, 0xef}}

	var got []byte
	r.OnChange(func(c Change) {
		if c.Forward == nil {
			return
		}
		c.Forward.AddRef()
		if reader, ok := c.Forward.(ForwardBytes); ok {
			got = reader.Bytes()
		}
	})

	r.InsertForward(famV4, pfx, pathBGP(1, 50), h)
	assert.Equal(t, h.payload, got, "subscriber must read the retained payload via ForwardBytes")
}

// TestInsertForwardRemoveCarriesNoHandle validates that a ChangeRemove
// triggered from Insert carries no handle, per design scope.
//
// VALIDATES: Remove-shaped changes cannot share a producer's wire buffer.
// PREVENTS: a subscriber assuming Forward is live on Remove and derefing.
func TestInsertForwardRemoveCarriesNoHandle(t *testing.T) {
	r := NewRIB()
	var seen []Change
	r.OnChange(func(c Change) { seen = append(seen, c) })

	r.InsertForward(famV4, pfx, pathBGP(1, 50), &countingHandle{})
	r.Remove(famV4, pfx, idBGP, 1)

	require.Len(t, seen, 2)
	assert.Equal(t, ChangeRemove, seen[1].Kind)
	assert.Nil(t, seen[1].Forward, "Remove carries no forward handle")
}

// BenchmarkLocribInsert establishes the no-handle Insert baseline. Each
// iteration upserts the best path for a distinct prefix so the hot path
// is "new prefix -> ChangeAdd dispatch".
func BenchmarkLocribInsert(b *testing.B) {
	r := NewRIB()
	p := pathBGP(1, 50)
	for i := range b.N {
		pfx := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)}), 24)
		r.Insert(famV4, pfx, p)
	}
}

// BenchmarkLocribInsertForwardNil exercises InsertForward with a nil
// handle. Compared against BenchmarkLocribInsert, delta is the extra
// interface-valued argument on the insert path. Design gate: within 3
// percent of BenchmarkLocribInsert.
func BenchmarkLocribInsertForwardNil(b *testing.B) {
	r := NewRIB()
	p := pathBGP(1, 50)
	for i := range b.N {
		pfx := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)}), 24)
		r.InsertForward(famV4, pfx, p, nil)
	}
}

// BenchmarkLocribInsertForwardHandle measures InsertForward with a
// handle attached. locrib itself does not AddRef / Release on the hot
// path, so the delta vs. the nil-handle variant should be near zero.
func BenchmarkLocribInsertForwardHandle(b *testing.B) {
	r := NewRIB()
	p := pathBGP(1, 50)
	h := &countingHandle{}
	for i := range b.N {
		pfx := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)}), 24)
		r.InsertForward(famV4, pfx, p, h)
	}
}

func TestLocRIB_LPM(t *testing.T) {
	r := NewRIB()

	r.Insert(famV4, netip.MustParsePrefix("10.0.0.0/8"), pathBGP(1, 100))
	r.Insert(famV4, netip.MustParsePrefix("10.1.0.0/16"), pathBGP(2, 50))
	r.Insert(famV4, netip.MustParsePrefix("10.1.2.0/24"), pathBGP(3, 10))

	best, pfx, ok := r.LPM(famV4, netip.MustParseAddr("10.1.2.5"))
	require.True(t, ok)
	assert.Equal(t, netip.MustParsePrefix("10.1.2.0/24"), pfx)
	assert.Equal(t, uint32(3), best.Instance)
}

func TestLocRIB_LPM_NoFamily(t *testing.T) {
	r := NewRIB()

	_, _, ok := r.LPM(family.IPv6Multicast, netip.MustParseAddr("ff00::1"))
	assert.False(t, ok)
}

func TestLocRIB_LPM_BestPath(t *testing.T) {
	r := NewRIB()
	fam := family.IPv4Multicast

	r.Insert(fam, netip.MustParsePrefix("224.0.0.0/4"), pathOSPF(100))
	r.Insert(fam, netip.MustParsePrefix("224.0.0.0/4"), pathStatic())

	best, pfx, ok := r.LPM(fam, netip.MustParseAddr("224.1.2.3"))
	require.True(t, ok)
	assert.Equal(t, netip.MustParsePrefix("224.0.0.0/4"), pfx)
	assert.Equal(t, idStatic, best.Source, "LPM returns the best path (lowest admin distance)")
}

func TestLocRIB_LPM_NoMatch(t *testing.T) {
	r := NewRIB()
	r.Insert(famV4, netip.MustParsePrefix("10.0.0.0/8"), pathBGP(1, 10))

	_, _, ok := r.LPM(famV4, netip.MustParseAddr("192.168.1.1"))
	assert.False(t, ok)
}

func TestLocRIB_LPM_InvalidAddr(t *testing.T) {
	r := NewRIB()
	r.Insert(famV4, netip.MustParsePrefix("10.0.0.0/8"), pathBGP(1, 10))

	_, _, ok := r.LPM(famV4, netip.Addr{})
	assert.False(t, ok)
}

// TestChangeKindString verifies ChangeKind.String() returns correct values.
//
// VALIDATES: String representation of each ChangeKind variant.
// PREVENTS: Wrong change kind in logs and diagnostics.
func TestChangeKindString(t *testing.T) {
	assert.Equal(t, "add", ChangeAdd.String())
	assert.Equal(t, "update", ChangeUpdate.String())
	assert.Equal(t, "remove", ChangeRemove.String())
	assert.Equal(t, "unspecified", ChangeUnspecified.String())
	assert.Equal(t, "unspecified", ChangeKind(255).String())
}

// TestAdminDistanceTrumpsMetric verifies that a path with lower AdminDistance
// wins even when a competing path has much lower Metric.
//
// VALIDATES: selectBest skips higher-AD paths regardless of Metric.
// PREVENTS: Metric comparison across different AdminDistance levels.
func TestAdminDistanceTrumpsMetric(t *testing.T) {
	r := NewRIB()

	// OSPF with very low metric, then Static with very high metric.
	r.Insert(famV4, pfx, pathOSPF(1))
	best, changed := r.Insert(famV4, pfx, Path{
		Source:        idStatic,
		Instance:      0,
		NextHop:       netip.MustParseAddr("192.0.2.1"),
		AdminDistance: 1,
		Metric:        999999,
	})
	require.True(t, changed)
	assert.Equal(t, idStatic, best.Source, "lower AdminDistance must win despite higher Metric")
}

// TestMetricTiebreakStable verifies that equal-AD paths with different metrics
// select the lower metric, and a path with higher metric does not replace it.
//
// VALIDATES: Metric comparison is strictly less-than, not less-or-equal or not-equal.
// PREVENTS: Higher-metric path winning within the same AdminDistance.
func TestMetricTiebreakStable(t *testing.T) {
	r := NewRIB()

	best, _ := r.Insert(famV4, pfx, pathBGP(1, 50))
	assert.Equal(t, uint32(50), best.Metric)

	best, changed := r.Insert(famV4, pfx, pathBGP(2, 200))
	assert.False(t, changed, "higher-metric path must not become best")
	assert.Equal(t, uint32(50), best.Metric)
}

// TestRemoveNotFoundReturnsFalse verifies remove on a non-existent path returns false.
//
// VALIDATES: PathGroup.remove returns false for missing paths.
// PREVENTS: False-positive removal success.
func TestRemoveNotFoundReturnsFalse(t *testing.T) {
	r := NewRIB()
	r.Insert(famV4, pfx, pathBGP(1, 10))
	// Remove a path that was never inserted.
	_, changed := r.Remove(famV4, pfx, idOSPF, 0)
	assert.False(t, changed)
}
