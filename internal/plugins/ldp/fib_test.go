// Design: docs/architecture/ldp/mpls-ldp.md -- ldpFIB emit tests (AC-4/AC-5)
package ldp

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: concurrent ProgramPush/Remove on the same FEC from many sessions do
// not race -- the pushed-set update and the bus emit are serialized together.
// Run under -race; without emit-under-lock this trips the detector on the bus.
func TestLDPFIBConcurrentSameFEC(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.9.0.0/24")
	nh := netip.MustParseAddr("10.0.0.2")

	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			fib.ProgramPush(fec, 17000, nh)
			fib.Remove(fec)
		})
	}
	wg.Wait()
}

// captureBus records Emit calls so a test can assert the (namespace, eventType,
// payload) a producer publishes.
type captureBus struct {
	emits []capturedEmit
}

type capturedEmit struct {
	namespace string
	eventType string
	payload   any
}

func (c *captureBus) Emit(namespace, eventType string, payload any) (int, error) {
	c.emits = append(c.emits, capturedEmit{namespace: namespace, eventType: eventType, payload: payload})
	return 0, nil
}

func (c *captureBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

func (c *captureBus) lastEntry(t *testing.T) mplsfibevents.Entry {
	t.Helper()
	require.NotEmpty(t, c.emits, "an event was emitted")
	last := c.emits[len(c.emits)-1]
	assert.Equal(t, mplsfibevents.Namespace, last.namespace)
	assert.Equal(t, mplsfibevents.EventEntry, last.eventType)
	batch, ok := last.payload.(*mplsfibevents.EntryBatch)
	require.True(t, ok, "payload is *EntryBatch")
	require.Len(t, batch.Entries, 1)
	return batch.Entries[0]
}

// VALIDATES: AC-4 -- a received label mapping becomes an ingress push entry on
// the mpls-fib bus toward the advertising peer.
func TestLDPFIBProgramPush(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.9.0.0/24")
	peer := netip.MustParseAddr("10.0.0.2")

	fib.ProgramPush(fec, 17000, peer)

	e := bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.ActionAdd, e.Action)
	assert.Equal(t, mplsfibevents.OpPush, e.Op)
	assert.Equal(t, fec, e.FEC)
	assert.Equal(t, []uint32{17000}, e.OutLabels)
	assert.Equal(t, peer, e.NextHop)
	assert.Equal(t, mplsSourceLDP, e.Source)
}

// VALIDATES: AC-5 -- removing a FEC that was pushed withdraws the push entry.
func TestLDPFIBRemove(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.9.0.0/24")

	fib.ProgramPush(fec, 17000, netip.MustParseAddr("10.0.0.2"))
	fib.Remove(fec)

	e := bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.ActionRemove, e.Action)
	assert.Equal(t, mplsfibevents.OpPush, e.Op)
	assert.Equal(t, fec, e.FEC)
	assert.Equal(t, mplsSourceLDP, e.Source)
}

// VALIDATES: Remove is idempotent -- removing a FEC that was never pushed emits
// nothing, so an implicit-null binding's withdrawal makes no spurious kernel call.
func TestLDPFIBRemoveNotInstalled(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fib.Remove(netip.MustParsePrefix("10.9.0.0/24"))
	assert.Empty(t, bus.emits, "Remove of a never-pushed FEC must emit nothing")
}

// VALIDATES: a second Remove of an already-removed FEC is a no-op (no double
// withdrawal).
func TestLDPFIBRemoveTwice(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.9.0.0/24")
	fib.ProgramPush(fec, 17000, netip.MustParseAddr("10.0.0.2"))
	fib.Remove(fec)
	before := len(bus.emits)
	fib.Remove(fec)
	assert.Equal(t, before, len(bus.emits), "second Remove must not emit")
}

// VALIDATES: LDP source tag differs from RSVP-TE so fib-kernel can attribute
// ownership of each MPLS entry.
func TestLDPFIBSourceTag(t *testing.T) {
	assert.Equal(t, uint16(2), mplsSourceLDP)
}

// VALIDATES: AC-3 -- a local FEC binding becomes an egress pop entry keyed by the
// advertised in-label, with no out-labels (disposition) and the FEC for diagnostics.
func TestLDPFIBProgramPop(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.1.0.0/24")

	fib.ProgramPop(fec, 18000)

	e := bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.ActionAdd, e.Action)
	assert.Equal(t, mplsfibevents.OpPop, e.Op)
	assert.Equal(t, uint32(18000), e.InLabel)
	assert.Equal(t, fec, e.FEC)
	assert.Empty(t, e.OutLabels)
	assert.Equal(t, mplsSourceLDP, e.Source)
}

// VALIDATES: AC-3 -- engine shutdown removes the egress pop entry keyed by in-label.
func TestLDPFIBRemovePop(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.1.0.0/24")

	fib.RemovePop(fec, 18000)

	e := bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.ActionRemove, e.Action)
	assert.Equal(t, mplsfibevents.OpPop, e.Op)
	assert.Equal(t, uint32(18000), e.InLabel)
	assert.Equal(t, mplsSourceLDP, e.Source)
}

// VALIDATES: AC-4 -- a real label learned from a peer becomes an ingress push.
func TestApplyRemoteBindingRealLabel(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.9.0.0/24")
	peer := netip.MustParseAddr("10.0.0.1")

	applyRemoteBinding(fib, fec, 17000, peer, slogutil.DiscardLogger())

	e := bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.OpPush, e.Op)
	assert.Equal(t, []uint32{17000}, e.OutLabels)
}

// VALIDATES: implicit-null (3) means forward as plain IP -- ze must NOT push a
// label (it would otherwise impose label 3 on the wire, breaking forwarding).
func TestApplyRemoteBindingImplicitNull(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.9.0.0/24")
	peer := netip.MustParseAddr("10.0.0.1")

	applyRemoteBinding(fib, fec, ImplicitNull, peer, slogutil.DiscardLogger())

	assert.Empty(t, bus.emits, "implicit-null must not program a push entry")
}

// VALIDATES: explicit-null (0) is a real on-wire label and is imposed normally.
func TestApplyRemoteBindingExplicitNull(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.9.0.0/24")
	peer := netip.MustParseAddr("10.0.0.1")

	applyRemoteBinding(fib, fec, ExplicitNull, peer, slogutil.DiscardLogger())

	e := bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.OpPush, e.Op)
	assert.Equal(t, []uint32{ExplicitNull}, e.OutLabels)
}

// VALIDATES: AC-4/AC-5 -- a real -> implicit-null relabel clears the stale push so
// no label keeps being imposed once the peer signals plain-IP egress. PREVENTS the
// stale-push leak the per-label remove-skip would otherwise cause.
func TestApplyRemoteBindingRelabelToImplicitNull(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.9.0.0/24")
	peer := netip.MustParseAddr("10.0.0.1")

	applyRemoteBinding(fib, fec, 17000, peer, slogutil.DiscardLogger()) // push installed
	applyRemoteBinding(fib, fec, ImplicitNull, peer, slogutil.DiscardLogger())

	require.Len(t, bus.emits, 2, "expected push add then push remove")
	last := bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.ActionRemove, last.Action)
	assert.Equal(t, mplsfibevents.OpPush, last.Op)
	assert.Equal(t, fec, last.FEC)
}

// VALIDATES: withdrawing a binding whose push was installed removes both the LIB
// entry and the kernel push.
func TestWithdrawRemoteBindingRealLabel(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	lib := NewLIB()
	fec := netip.MustParsePrefix("10.9.0.0/24")
	lib.AddRemote(fec, 17000, "peer", netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.1"))
	fib.ProgramPush(fec, 17000, netip.MustParseAddr("10.0.0.1")) // mirror onLabel

	removed := withdrawRemoteBinding(fib, lib, fec, "peer", slogutil.DiscardLogger())
	require.NotNil(t, removed)
	assert.Equal(t, uint32(17000), removed.Label)
	assert.Zero(t, lib.Len(), "binding should be gone from LIB")

	e := bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.ActionRemove, e.Action)
	assert.Equal(t, mplsfibevents.OpPush, e.Op)
	assert.Equal(t, fec, e.FEC)
}

// VALIDATES: withdrawing an implicit-null binding (no push was installed) removes
// the LIB entry but emits NO kernel removal -- no spurious withdrawal.
func TestWithdrawRemoteBindingImplicitNull(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	lib := NewLIB()
	fec := netip.MustParsePrefix("10.9.0.0/24")
	lib.AddRemote(fec, ImplicitNull, "peer", netip.MustParseAddr("10.0.0.1"), netip.Addr{})

	removed := withdrawRemoteBinding(fib, lib, fec, "peer", slogutil.DiscardLogger())
	require.NotNil(t, removed)
	assert.Empty(t, bus.emits, "implicit-null withdraw must not emit a kernel removal")
}

// VALIDATES: two peers withdrawing the same FEC concurrently (each under the
// reconcile lock) end with the push gone and no stale entry toward a removed peer.
// PREVENTS the TOCTOU where one goroutine re-points to a peer another is removing.
// Run under -race.
func TestReconcileConcurrentWithdraw(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	lib := NewLIB()
	fec := netip.MustParsePrefix("10.9.0.0/24")
	a := netip.MustParseAddr("10.0.0.1")
	b := netip.MustParseAddr("10.0.0.2")
	lib.AddRemote(fec, 17000, "peerA", a, a)
	lib.AddRemote(fec, 18000, "peerB", b, b)
	fib.ProgramPush(fec, 17000, a)

	var wg sync.WaitGroup
	for _, pk := range []string{"peerA", "peerB"} {
		wg.Go(func() {
			fib.withReconcileLock(func() {
				withdrawRemoteBinding(fib, lib, fec, pk, slogutil.DiscardLogger())
			})
		})
	}
	wg.Wait()

	assert.Zero(t, lib.Len(), "both peers withdrawn from LIB")
	assert.False(t, fib.pushed[fec.String()], "no push must remain after both peers withdraw")
}

// VALIDATES: peer teardown re-points FECs another peer still advertises and
// withdraws those no peer does, and returns the removed bindings.
func TestReconcilePeerDown(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	lib := NewLIB()
	shared := netip.MustParsePrefix("10.1.0.0/24") // advertised by P and Q
	soloP := netip.MustParsePrefix("10.2.0.0/24")  // advertised only by P
	pAddr := netip.MustParseAddr("10.0.0.1")
	qAddr := netip.MustParseAddr("10.0.0.2")
	lib.AddRemote(shared, 17000, "peerA", pAddr, pAddr)
	lib.AddRemote(soloP, 17001, "peerA", pAddr, pAddr)
	lib.AddRemote(shared, 18000, "peerB", qAddr, qAddr)
	fib.ProgramPush(shared, 17000, pAddr)
	fib.ProgramPush(soloP, 17001, pAddr)

	removed := reconcilePeerDown(fib, lib, "peerA", slogutil.DiscardLogger())

	assert.Len(t, removed, 2, "both of peerA's bindings removed")
	// shared survives via peerB; soloP is gone.
	if _, ok := lib.RemainingForFEC(shared); !ok {
		t.Error("shared FEC should still have peerB's binding")
	}
	assert.True(t, fib.pushed[shared.String()], "shared push re-pointed, still installed")
	assert.False(t, fib.pushed[soloP.String()], "soloP push withdrawn (no surviving peer)")
}

// VALIDATES: a peer teardown running concurrently with another session's reconcile
// for an unrelated FEC does not race (per-FEC reconcile locking). Run under -race.
func TestReconcilePeerDownConcurrent(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	lib := NewLIB()
	a := netip.MustParseAddr("10.0.0.1")
	for i := range 8 {
		fec := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 3, byte(i), 0}), 24)
		lib.AddRemote(fec, uint32(17000+i), "peerA", a, a)
	}
	other := netip.MustParsePrefix("10.9.0.0/24")

	var wg sync.WaitGroup
	wg.Go(func() { reconcilePeerDown(fib, lib, "peerA", slogutil.DiscardLogger()) })
	wg.Go(func() {
		fib.withReconcileLock(func() {
			lib.AddRemote(other, 19000, "peerB", netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("10.0.0.2"))
			reconcileFEC(fib, lib, other, slogutil.DiscardLogger())
		})
	})
	wg.Wait()

	assert.True(t, fib.pushed[other.String()], "the unrelated FEC remains programmed")
}

// VALIDATES: reconcileFEC programs the best (lowest-key) binding for a FEC, using
// the binding's stored next hop, and withdraws the push when no peer advertises it.
func TestReconcileFEC(t *testing.T) {
	fec := netip.MustParsePrefix("10.9.0.0/24")

	t.Run("programs surviving peer with its stored next hop", func(t *testing.T) {
		bus := &captureBus{}
		fib := newLDPFIB(bus, slogutil.DiscardLogger())
		lib := NewLIB()
		// peerB's binding carries a resolved next hop distinct from its transport.
		lib.AddRemote(fec, 18000, "peerB", netip.MustParseAddr("10.0.0.9"), netip.MustParseAddr("10.0.0.2"))
		fib.ProgramPush(fec, 17000, netip.MustParseAddr("10.0.0.1")) // an earlier peerA push

		reconcileFEC(fib, lib, fec, slogutil.DiscardLogger())

		e := bus.lastEntry(t)
		assert.Equal(t, mplsfibevents.ActionAdd, e.Action, "program, not withdraw")
		assert.Equal(t, mplsfibevents.OpPush, e.Op)
		assert.Equal(t, []uint32{18000}, e.OutLabels, "uses surviving peerB's label")
		assert.Equal(t, netip.MustParseAddr("10.0.0.2"), e.NextHop, "uses the stored resolved next hop, not transport")
	})

	t.Run("withdraws when last peer gone", func(t *testing.T) {
		bus := &captureBus{}
		fib := newLDPFIB(bus, slogutil.DiscardLogger())
		lib := NewLIB() // no survivor
		fib.ProgramPush(fec, 17000, netip.MustParseAddr("10.0.0.1"))

		reconcileFEC(fib, lib, fec, slogutil.DiscardLogger())

		e := bus.lastEntry(t)
		assert.Equal(t, mplsfibevents.ActionRemove, e.Action)
		assert.Equal(t, mplsfibevents.OpPush, e.Op)
		assert.Equal(t, fec, e.FEC)
	})

	t.Run("lowest peer key wins deterministically", func(t *testing.T) {
		bus := &captureBus{}
		fib := newLDPFIB(bus, slogutil.DiscardLogger())
		lib := NewLIB()
		lib.AddRemote(fec, 30000, "2", netip.MustParseAddr("10.0.0.3"), netip.MustParseAddr("10.0.0.3"))
		lib.AddRemote(fec, 20000, "1", netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("10.0.0.2"))

		reconcileFEC(fib, lib, fec, slogutil.DiscardLogger())

		e := bus.lastEntry(t)
		assert.Equal(t, []uint32{20000}, e.OutLabels, "lowest key (\"1\") wins")
	})
}

// VALIDATES: ldpFIB with no bus does not panic (degraded mode, e.g. tests or
// pre-start).
func TestLDPFIBNilBus(t *testing.T) {
	fib := newLDPFIB(nil, slogutil.DiscardLogger())
	assert.NotPanics(t, func() {
		fib.ProgramPush(netip.MustParsePrefix("10.9.0.0/24"), 17000, netip.Addr{})
		fib.Remove(netip.MustParsePrefix("10.9.0.0/24"))
		fib.ProgramPop(netip.MustParsePrefix("10.1.0.0/24"), 18000)
		fib.RemovePop(netip.MustParsePrefix("10.1.0.0/24"), 18000)
	})
}
