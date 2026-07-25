package rib

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// balanceHandle implements locrib.ForwardHandle + ForwardBytes and tracks an
// external AddRef/Release balance so tests can assert the buffer pool never
// leaks (every AddRef matched by exactly one Release).
type balanceHandle struct {
	balance *atomic.Int64
	adds    atomic.Int32
	data    []byte
}

func (h *balanceHandle) AddRef()       { h.balance.Add(1); h.adds.Add(1) }
func (h *balanceHandle) Release()      { h.balance.Add(-1) }
func (h *balanceHandle) Bytes() []byte { return h.data }

var famV4 = family.Family{AFI: 1, SAFI: 1}

// VALIDATES: rib-arch-6 AC-1 — the tracker AddRefs, reads the UPDATE bytes, and
// Releases; the recorded state reflects the forwarded prefix and byte length.
func TestForwardStateTracker_ReadsAndReleases(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()
	tr.Enable()

	var balance atomic.Int64
	h := &balanceHandle{balance: &balance, data: []byte("update-wire-bytes")}
	pfx := netip.MustParsePrefix("10.0.0.0/24")
	tr.onChange(locrib.Change{Family: famV4, Prefix: pfx, Kind: locrib.ChangeAdd, Forward: h})

	require.Eventually(t, func() bool { return tr.snapshot().Forwarded == 1 },
		time.Second, time.Millisecond, "worker must process the forward change")

	s := tr.snapshot()
	assert.Equal(t, uint64(1), s.Forwarded)
	assert.Equal(t, uint64(len(h.data)), s.Bytes)
	assert.Equal(t, 1, s.Prefixes)
	assert.Equal(t, int32(1), h.adds.Load(), "exactly one AddRef")
	assert.Equal(t, int64(0), balance.Load(), "AddRef/Release must balance (no leak)")
}

// VALIDATES: rib-arch-6 — a disabled tracker is fully inert: it never AddRefs
// (no copy-out cost) and records no state.
func TestForwardStateTracker_DisabledIsInert(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()
	// Not enabled.

	var balance atomic.Int64
	h := &balanceHandle{balance: &balance, data: []byte("x")}
	tr.onChange(locrib.Change{Family: famV4, Prefix: netip.MustParsePrefix("10.0.0.0/24"), Kind: locrib.ChangeAdd, Forward: h})

	// Give the worker a chance to (not) run.
	time.Sleep(20 * time.Millisecond)
	s := tr.snapshot()
	assert.False(t, s.Enabled)
	assert.Equal(t, uint64(0), s.Forwarded)
	assert.Equal(t, int32(0), h.adds.Load(), "disabled tracker must not AddRef")
	assert.Equal(t, int64(0), balance.Load())
}

// VALIDATES: rib-arch-6 — a ChangeRemove prunes the per-prefix state (no Forward
// handle involved) and counts the removal.
func TestForwardStateTracker_RemovePrunesState(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	defer tr.Stop()
	tr.Enable()

	var balance atomic.Int64
	pfx := netip.MustParsePrefix("10.0.0.0/24")
	h := &balanceHandle{balance: &balance, data: []byte("bytes")}
	tr.onChange(locrib.Change{Family: famV4, Prefix: pfx, Kind: locrib.ChangeAdd, Forward: h})
	require.Eventually(t, func() bool { return tr.snapshot().Prefixes == 1 }, time.Second, time.Millisecond)

	tr.onChange(locrib.Change{Family: famV4, Prefix: pfx, Kind: locrib.ChangeRemove})
	require.Eventually(t, func() bool { return tr.snapshot().Removes == 1 }, time.Second, time.Millisecond)

	assert.Equal(t, 0, tr.snapshot().Prefixes, "remove must prune the prefix")
	assert.Equal(t, int64(0), balance.Load())
}

// VALIDATES: rib-arch-6 AC-2 — under sustained concurrent churn the buffer-pool
// refcount stays balanced (no leak) and there is no data race. Run under -race.
func TestForwardStateTracker_NoLeakUnderChurn(t *testing.T) {
	loc := locrib.NewRIB()
	tr := newForwardStateTracker(loc)
	tr.Enable()

	var balance atomic.Int64
	const goroutines, per = 8, 500

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range per {
				h := &balanceHandle{balance: &balance, data: []byte("update")}
				pfx := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(g), byte(i), 0}), 24)
				tr.onChange(locrib.Change{Family: famV4, Prefix: pfx, Kind: locrib.ChangeAdd, Forward: h})
			}
		}(g)
	}
	// Concurrent reader to exercise the snapshot lock alongside the worker.
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = tr.snapshot()
			}
		}
	}()
	wg.Wait()
	close(stop)

	// Stop drains the queue and joins the worker; every AddRef'd handle (whether
	// processed or dropped under backpressure) must be Released.
	tr.Stop()
	assert.Equal(t, int64(0), balance.Load(), "AddRef/Release must balance under churn (no buffer-pool leak)")
}
