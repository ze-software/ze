// Design: docs/architecture/plugin/rib-storage-design.md — attribute bundle dedup

package storage

import (
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
)

// Bundle groups the 12 non-AS_PATH attribute handles into a single shared
// reference. Bundle is a comparable value type so it can be used as a map
// key for deduplication without serialization.
type Bundle struct {
	Origin           attrpool.Handle
	NextHop          attrpool.Handle
	LocalPref        attrpool.Handle
	MED              attrpool.Handle
	AtomicAggregate  attrpool.Handle
	Aggregator       attrpool.Handle
	Communities      attrpool.Handle
	LargeCommunities attrpool.Handle
	ExtCommunities   attrpool.Handle
	ClusterList      attrpool.Handle
	OriginatorID     attrpool.Handle
	OtherAttrs       attrpool.Handle
}

// NewBundle creates a Bundle with all handles set to InvalidHandle.
func NewBundle() Bundle {
	return Bundle{
		Origin:           attrpool.InvalidHandle,
		NextHop:          attrpool.InvalidHandle,
		LocalPref:        attrpool.InvalidHandle,
		MED:              attrpool.InvalidHandle,
		AtomicAggregate:  attrpool.InvalidHandle,
		Aggregator:       attrpool.InvalidHandle,
		Communities:      attrpool.InvalidHandle,
		LargeCommunities: attrpool.InvalidHandle,
		ExtCommunities:   attrpool.InvalidHandle,
		ClusterList:      attrpool.InvalidHandle,
		OriginatorID:     attrpool.InvalidHandle,
		OtherAttrs:       attrpool.InvalidHandle,
	}
}

func (b Bundle) HasOrigin() bool           { return b.Origin.IsValid() }
func (b Bundle) HasNextHop() bool          { return b.NextHop.IsValid() }
func (b Bundle) HasLocalPref() bool        { return b.LocalPref.IsValid() }
func (b Bundle) HasMED() bool              { return b.MED.IsValid() }
func (b Bundle) HasAtomicAggregate() bool  { return b.AtomicAggregate.IsValid() }
func (b Bundle) HasAggregator() bool       { return b.Aggregator.IsValid() }
func (b Bundle) HasCommunities() bool      { return b.Communities.IsValid() }
func (b Bundle) hasLargeCommunities() bool { return b.LargeCommunities.IsValid() }
func (b Bundle) hasExtCommunities() bool   { return b.ExtCommunities.IsValid() }
func (b Bundle) HasClusterList() bool      { return b.ClusterList.IsValid() }
func (b Bundle) HasOriginatorID() bool     { return b.OriginatorID.IsValid() }
func (b Bundle) HasOtherAttrs() bool       { return b.OtherAttrs.IsValid() }

// releaseInnerHandles releases all 12 per-attribute handles in the bundle.
func (b *Bundle) releaseInnerHandles() {
	if b.Origin.IsValid() {
		_ = pool.Origin.Release(b.Origin)
	}
	if b.NextHop.IsValid() {
		_ = pool.NextHop.Release(b.NextHop)
	}
	if b.LocalPref.IsValid() {
		_ = pool.LocalPref.Release(b.LocalPref)
	}
	if b.MED.IsValid() {
		_ = pool.MED.Release(b.MED)
	}
	if b.AtomicAggregate.IsValid() {
		_ = pool.AtomicAggregate.Release(b.AtomicAggregate)
	}
	if b.Aggregator.IsValid() {
		_ = pool.Aggregator.Release(b.Aggregator)
	}
	if b.Communities.IsValid() {
		_ = pool.Communities.Release(b.Communities)
	}
	if b.LargeCommunities.IsValid() {
		_ = pool.LargeCommunities.Release(b.LargeCommunities)
	}
	if b.ExtCommunities.IsValid() {
		_ = pool.ExtCommunities.Release(b.ExtCommunities)
	}
	if b.ClusterList.IsValid() {
		_ = pool.ClusterList.Release(b.ClusterList)
	}
	if b.OriginatorID.IsValid() {
		_ = pool.OriginatorID.Release(b.OriginatorID)
	}
	if b.OtherAttrs.IsValid() {
		_ = pool.OtherAttrs.Release(b.OtherAttrs)
	}
}

// AddRefInnerHandles increments refcount on all valid inner handles.
// On error, rolls back any increments already made.
func (b *Bundle) AddRefInnerHandles() error {
	type ref struct {
		pool   *attrpool.Pool
		handle attrpool.Handle
	}
	var done []ref

	tryAddRef := func(p *attrpool.Pool, h attrpool.Handle) error {
		if !h.IsValid() {
			return nil
		}
		if err := p.AddRef(h); err != nil {
			return err
		}
		done = append(done, ref{p, h})
		return nil
	}

	if err := tryAddRef(pool.Origin, b.Origin); err != nil {
		goto rollback
	}
	if err := tryAddRef(pool.NextHop, b.NextHop); err != nil {
		goto rollback
	}
	if err := tryAddRef(pool.LocalPref, b.LocalPref); err != nil {
		goto rollback
	}
	if err := tryAddRef(pool.MED, b.MED); err != nil {
		goto rollback
	}
	if err := tryAddRef(pool.AtomicAggregate, b.AtomicAggregate); err != nil {
		goto rollback
	}
	if err := tryAddRef(pool.Aggregator, b.Aggregator); err != nil {
		goto rollback
	}
	if err := tryAddRef(pool.Communities, b.Communities); err != nil {
		goto rollback
	}
	if err := tryAddRef(pool.LargeCommunities, b.LargeCommunities); err != nil {
		goto rollback
	}
	if err := tryAddRef(pool.ExtCommunities, b.ExtCommunities); err != nil {
		goto rollback
	}
	if err := tryAddRef(pool.ClusterList, b.ClusterList); err != nil {
		goto rollback
	}
	if err := tryAddRef(pool.OriginatorID, b.OriginatorID); err != nil {
		goto rollback
	}
	if err := tryAddRef(pool.OtherAttrs, b.OtherAttrs); err != nil {
		goto rollback
	}

	return nil

rollback:
	for _, r := range done {
		_ = r.pool.Release(r.handle)
	}
	return attrpool.ErrPoolShutdown
}

// BundlePool deduplicates Bundle values and manages their lifecycle.
// When a bundle's refcount reaches zero, all 12 inner attribute handles
// are cascade-released.
//
// Thread-safe: all methods are protected by a mutex.
type BundlePool struct {
	mu       sync.RWMutex
	bundles  []Bundle
	refcount []uint32
	index    map[Bundle]uint32
	free     []uint32
}

// Bundles is the global BundlePool instance.
var Bundles = newBundlePool()

// newBundlePool creates a BundlePool.
func newBundlePool() *BundlePool {
	return &BundlePool{
		index: make(map[Bundle]uint32),
	}
}

// Intern stores a bundle and returns a handle. The caller must have one
// fresh reference per inner handle (as produced by pool.Intern).
//
// On dedup hit, the fresh inner handles are released (the existing entry
// already owns equivalent refs), and the existing handle is returned with
// incremented refcount.
//
// On new entry, the inner handle refs transfer to the pool. Refcount = 1.
func (bp *BundlePool) Intern(b Bundle) attrpool.Handle {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if slot, exists := bp.index[b]; exists {
		bp.refcount[slot]++
		b.releaseInnerHandles()
		return attrpool.NewHandle(bundlePoolIdx, slot)
	}

	var slot uint32
	if len(bp.free) > 0 {
		slot = bp.free[len(bp.free)-1]
		bp.free = bp.free[:len(bp.free)-1]
		bp.bundles[slot] = b
		bp.refcount[slot] = 1
	} else {
		slot = uint32(len(bp.bundles))
		bp.bundles = append(bp.bundles, b)
		bp.refcount = append(bp.refcount, 1)
	}

	bp.index[b] = slot
	return attrpool.NewHandle(bundlePoolIdx, slot)
}

// Get returns the Bundle stored at the given handle.
func (bp *BundlePool) Get(h attrpool.Handle) Bundle {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	slot := h.Slot()
	if slot >= uint32(len(bp.bundles)) {
		return NewBundle()
	}
	return bp.bundles[slot]
}

// Release decrements the refcount. When it reaches zero, all 12 inner
// attribute handles are cascade-released and the slot is freed.
func (bp *BundlePool) Release(h attrpool.Handle) {
	bp.mu.Lock()

	slot := h.Slot()
	if slot >= uint32(len(bp.bundles)) {
		bp.mu.Unlock()
		return
	}
	if bp.refcount[slot] == 0 {
		bp.mu.Unlock()
		return
	}

	bp.refcount[slot]--
	if bp.refcount[slot] > 0 {
		bp.mu.Unlock()
		return
	}

	b := bp.bundles[slot]
	delete(bp.index, b)
	bp.bundles[slot] = NewBundle()
	bp.free = append(bp.free, slot)
	bp.mu.Unlock()

	b.releaseInnerHandles()
}

// AddRef increments the refcount for the bundle at the given handle.
func (bp *BundlePool) AddRef(h attrpool.Handle) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	slot := h.Slot()
	if slot >= uint32(len(bp.bundles)) {
		return
	}
	if bp.refcount[slot] == 0 {
		return
	}
	bp.refcount[slot]++
}

// Len returns the number of live bundles in the pool.
func (bp *BundlePool) Len() int {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return len(bp.index)
}

// RefCount returns the refcount for a handle (for testing).
func (bp *BundlePool) RefCount(h attrpool.Handle) uint32 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	slot := h.Slot()
	if slot >= uint32(len(bp.bundles)) {
		return 0
	}
	return bp.refcount[slot]
}

// Reset clears the pool. Used in tests.
func (bp *BundlePool) Reset() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.bundles = bp.bundles[:0]
	bp.refcount = bp.refcount[:0]
	bp.free = bp.free[:0]
	clear(bp.index)
}

// bundlePoolIdx is the pool index used for BundlePool handles.
// Pool indices 2-14 are used by per-attribute pools; 15 is for bundles.
const bundlePoolIdx uint8 = 15
