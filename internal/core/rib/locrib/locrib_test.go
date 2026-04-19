package locrib

import (
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
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
