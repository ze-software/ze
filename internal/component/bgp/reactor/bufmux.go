// Design: docs/architecture/forward-congestion-pool.md -- block-backed buffer multiplexer
// Related: session.go -- read/build buffer pools replaced by BufMux instances
// Related: forward_pool.go -- overflow pool uses same buffers, held longer during congestion
// Related: forward_pool_weight.go -- burst fraction, buffer demand for pool sizing

package reactor

import (
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// BufHandle is a buffer obtained from a BufMux. The ID and idx fields route
// returns to the correct block and slot. Callers use Buf for reads/writes
// and pass the full handle to Return().
//
// Zero value (Buf == nil) means "no buffer available" (pool exhausted).
// Caller MUST call Return() (or the appropriate return function) after use.
type BufHandle struct {
	ID  uint32 // block this buffer belongs to
	idx int    // buffer index within the block (internal routing)
	Buf []byte // the buffer slice (into block's backing array)
}

// bufBlock is one contiguous backing array divided into equal-sized buffers.
// Each block tracks how many of its buffers are free for deterministic
// lifetime management: when freeCount == total, every buffer is home and
// the block can be safely deleted.
type bufBlock struct {
	backing   []byte // single allocation: make([]byte, total*bufSize)
	free      []int  // indices of available buffers within this block
	inUse     []bool // tracks which buffers are currently out (double-return guard)
	total     int    // number of buffers in this block
	freeCount int    // number of buffers currently in free list
	bufSize   int    // size of each buffer
	id        uint32 // block ID (matches BufHandle.ID)
}

// get takes a buffer from this block's free list.
// Returns the buffer index and slice, or -1 and nil if empty.
func (b *bufBlock) get() (int, []byte) {
	if len(b.free) == 0 {
		return -1, nil
	}
	// Pop from end (O(1)).
	idx := b.free[len(b.free)-1]
	b.free = b.free[:len(b.free)-1]
	b.freeCount--
	b.inUse[idx] = true
	off := idx * b.bufSize
	return idx, b.backing[off : off+b.bufSize]
}

// put returns a buffer to this block's free list by index.
// Guards against out-of-bounds index and double-return.
func (b *bufBlock) put(idx int) {
	if idx < 0 || idx >= b.total {
		fwdLogger().Error("bufmux: invalid buffer index in put",
			"idx", idx, "total", b.total, "blockID", b.id)
		return
	}
	if !b.inUse[idx] {
		fwdLogger().Error("bufmux: double return detected",
			"idx", idx, "blockID", b.id)
		return
	}
	b.inUse[idx] = false
	b.free = append(b.free, idx)
	b.freeCount++
}

// fullyReturned reports whether every buffer in this block is free.
func (b *bufBlock) fullyReturned() bool {
	return b.freeCount == b.total
}

// freeRatio returns the fraction of buffers currently free (0.0 to 1.0).
func (b *bufBlock) freeRatio() float64 {
	if b.total == 0 {
		return 0
	}
	return float64(b.freeCount) / float64(b.total)
}

// newBufBlock allocates a block with one contiguous backing array.
func newBufBlock(id uint32, bufSize, count int) *bufBlock {
	b := &bufBlock{
		backing:   make([]byte, count*bufSize),
		free:      make([]int, count),
		inUse:     make([]bool, count),
		total:     count,
		freeCount: count,
		bufSize:   bufSize,
		id:        id,
	}
	// All buffers start free.
	for i := range b.free {
		b.free[i] = i
	}
	return b
}

// combinedBudget tracks total allocated bytes across multiple BufMux instances
// using an atomic counter. Lock-free reads make it safe to call from within
// growLocked() without risking cross-mux deadlock (AC-27).
//
// Each BufMux that shares a budget calls recordGrow/recordCollapse when
// blocks are added/removed. The canGrow check is O(1) and never acquires
// another mux's lock.
type combinedBudget struct {
	allocated atomic.Int64 // total allocated bytes across all muxes
	maxBytes  atomic.Int64 // 0 = unlimited; updated by weight tracker
}

// newCombinedBudget creates a shared budget. maxBytes <= 0 means unlimited.
func newCombinedBudget(maxBytes int64) *combinedBudget {
	cb := &combinedBudget{}
	cb.maxBytes.Store(maxBytes)
	return cb
}

// tryReserve atomically checks whether adding blockBytes would stay within
// budget and, if so, reserves the space. Returns true if the reservation
// succeeded. Uses a CAS loop to eliminate the TOCTOU gap between checking
// and recording — two muxes cannot both pass the check concurrently.
func (cb *combinedBudget) tryReserve(blockBytes int) bool {
	add := int64(blockBytes)
	limit := cb.maxBytes.Load()
	if limit <= 0 {
		cb.allocated.Add(add)
		return true
	}
	for {
		cur := cb.allocated.Load()
		if cur+add > limit {
			return false
		}
		if cb.allocated.CompareAndSwap(cur, cur+add) {
			return true
		}
	}
}

// releaseBytes removes blockBytes from the allocation counter.
func (cb *combinedBudget) releaseBytes(blockBytes int) {
	cb.allocated.Add(-int64(blockBytes))
}

// AllocatedBytes returns the current total across all muxes.
func (cb *combinedBudget) AllocatedBytes() int64 {
	return cb.allocated.Load()
}

// BufMux is a block-backed buffer multiplexer that replaces sync.Pool.
//
// Three rules govern its behavior:
//  1. Allocate from lowest block with free buffers (steady-state packs low,
//     higher blocks drain and become collapse candidates).
//  2. Grow a new block when Get() finds no free buffer (subject to budget).
//  3. Collapse highest block (via tryCollapse) when fully returned and
//     block below has >=50% free. Triggered externally by probedPool on a
//     traffic-driven interval — no timer, no per-Return check.
//
// No permanent block. No speculative growth. No shrink-on-return.
//
// Thread-safe: one mutex protects all operations. This is acceptable
// because Get() and Return() are O(1).
type BufMux struct {
	mu        sync.Mutex
	blocks    []*bufBlock     // ordered by creation; index may not equal ID after collapse
	bufSize   int             // buffer size for this multiplexer
	blockSize int             // buffers per block
	maxBlocks int             // 0 = unlimited
	nextID    uint32          // next block ID to assign
	budget    *combinedBudget // shared budget across mux instances; nil = unlimited
}

// newBufMux creates a multiplexer for buffers of the given size.
// blockSize is the number of buffers per block.
// The multiplexer starts with zero blocks; the first Get() allocates block 0.
func newBufMux(bufSize, blockSize int) *BufMux {
	return &BufMux{
		bufSize:   bufSize,
		blockSize: blockSize,
	}
}

// SetMaxBlocks limits the number of blocks. 0 = unlimited.
// Must be called before concurrent use.
func (m *BufMux) SetMaxBlocks(n int) {
	m.mu.Lock()
	m.maxBlocks = n
	m.mu.Unlock()
}

// SetBudget sets a shared budget that limits combined allocated bytes
// across multiple BufMux instances. The budget is an atomic counter --
// safe to check from growLocked without cross-mux deadlock.
// Nil = unlimited growth (default). Safe for concurrent use.
//
// If blocks already exist (budget set after initial growth), their bytes
// are added to the budget counter so accounting stays consistent.
func (m *BufMux) SetBudget(cb *combinedBudget) {
	m.mu.Lock()
	m.budget = cb
	if cb != nil {
		for range m.blocks {
			cb.allocated.Add(int64(m.blockBytes()))
		}
	}
	m.mu.Unlock()
}

// Get returns a buffer handle from the lowest block with free buffers.
// If no block has free buffers, a new block is grown (unless at maximum).
// Returns zero-value BufHandle (Buf == nil) when pool is exhausted.
//
// Caller MUST call Return() when done with the buffer to avoid resource
// exhaustion. Every Get() must be paired with exactly one Return().
func (m *BufMux) Get() BufHandle {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getLocked()
}

// getLocked allocates from lowest block or grows. Caller must hold mu.
// Lowest-first keeps steady-state traffic in low blocks, letting higher
// blocks drain and become collapse candidates.
func (m *BufMux) getLocked() BufHandle {
	// Walk blocks from lowest to highest looking for a free buffer.
	for i := range m.blocks {
		b := m.blocks[i]
		if idx, buf := b.get(); idx >= 0 {
			return BufHandle{ID: b.id, idx: idx, Buf: buf}
		}
	}

	// No free buffer in any block. Try to grow.
	b := m.growLocked()
	if b == nil {
		return BufHandle{} // exhausted
	}
	idx, buf := b.get()
	if idx < 0 {
		// Should never happen: fresh block has all buffers free.
		return BufHandle{}
	}
	return BufHandle{ID: b.id, idx: idx, Buf: buf}
}

// Return releases a buffer handle back to its origin block.
// The handle's ID routes to the block, idx identifies the slot. O(1).
//
// Must be called exactly once per Get(). Returning a zero-value handle
// (Buf == nil) is a no-op.
func (m *BufMux) Return(h BufHandle) {
	if h.Buf == nil {
		return // zero handle, nothing to return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	b := m.blockByID(h.ID)
	if b == nil {
		// Block was already collapsed. This should not happen if
		// callers follow the contract (return before collapse), but
		// defensive: log and discard.
		fwdLogger().Error("bufmux: return to deleted block", "blockID", h.ID)
		return
	}

	b.put(h.idx)
}

// Stats returns the total allocated buffer slots and the number currently
// in use across all blocks. Safe for concurrent use (acquires mu).
// The values are a point-in-time snapshot.
func (m *BufMux) Stats() (allocated, inUse int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.blocks {
		allocated += b.total
		inUse += b.total - b.freeCount
	}
	return
}

// blockBytes returns the byte size of one block (blockSize * bufSize).
func (m *BufMux) blockBytes() int {
	return m.blockSize * m.bufSize
}

// growLocked allocates a new block. Returns nil if at maximum or if
// the budget denies. Caller holds mu.
func (m *BufMux) growLocked() *bufBlock {
	if m.maxBlocks > 0 && len(m.blocks) >= m.maxBlocks {
		return nil
	}
	if m.budget != nil && !m.budget.tryReserve(m.blockBytes()) {
		return nil
	}
	b := newBufBlock(m.nextID, m.bufSize, m.blockSize)
	m.blocks = append(m.blocks, b)
	m.nextID++
	return b
}

// collapseLocked deletes the highest block if fully returned and the
// block below has >=50% free. Cascades downward.
// Caller must hold mu.
func (m *BufMux) collapseLocked() {
	for len(m.blocks) > 1 {
		highest := m.blocks[len(m.blocks)-1]
		below := m.blocks[len(m.blocks)-2]

		if !highest.fullyReturned() {
			return
		}
		if below.freeRatio() < 0.5 {
			return
		}

		// Delete highest: trim slice. The removed element becomes
		// unreachable and eligible for GC.
		m.blocks = m.blocks[:len(m.blocks)-1]
		if m.budget != nil {
			m.budget.releaseBytes(m.blockBytes())
		}
	}
}

// tryCollapse runs a collapse check on the block list. Called by probedPool
// on a traffic-driven interval to reclaim fully-returned overflow blocks.
func (m *BufMux) tryCollapse() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collapseLocked()
}

// blockByID finds a block by its ID. Returns nil if not found.
// Caller must hold mu.
func (m *BufMux) blockByID(id uint32) *bufBlock {
	for _, b := range m.blocks {
		if b.id == id {
			return b
		}
	}
	return nil
}

// blockCount returns the number of active blocks (for testing).
func (m *BufMux) blockCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.blocks)
}

// probedPool wraps a BufMux and fires a probe callback on every Get().
// The wrapper is a pure trigger — it holds no counter or interval. The
// probe target (overflow pool) owns the counter and decides when to act.
// This uses regular network I/O as an implicit clock instead of a timer.
type probedPool struct {
	mux   *BufMux
	probe func() // called on every Get(); nil = no-op
}

// newProbedPool creates a buffer multiplexer wrapper. The probe callback
// is not set — call SetProbe to wire monitoring.
func newProbedPool(bufSize, blockSize int) *probedPool {
	return &probedPool{
		mux: newBufMux(bufSize, blockSize),
	}
}

// Get returns a buffer handle from the underlying BufMux.
// Fires the probe callback on every call — the probe target decides
// whether to act (counter and interval are the target's responsibility).
func (p *probedPool) Get() BufHandle {
	if p.probe != nil {
		p.probe()
	}
	return p.mux.Get()
}

// Return releases a buffer handle back to the underlying BufMux.
func (p *probedPool) Return(h BufHandle) {
	p.mux.Return(h)
}

// SetProbe sets the function fired on every Get(). The probe target owns
// the counter and decides when to check. Must be called before concurrent use.
func (p *probedPool) SetProbe(fn func()) {
	p.probe = fn
}

// AddProbe chains an additional probe callback. The new probe fires after
// any existing probe on every Get(). Use this to add overflow/backpressure
// monitoring without replacing the collapse probe.
// Must be called before concurrent use.
func (p *probedPool) AddProbe(fn func()) {
	old := p.probe
	if old == nil {
		p.probe = fn
	} else {
		p.probe = func() { old(); fn() }
	}
}

// Stats returns (allocated, inUse) buffer slot counts from the underlying BufMux.
func (p *probedPool) Stats() (allocated, inUse int) {
	return p.mux.Stats()
}

// SetMaxBlocks limits the number of blocks in the underlying BufMux.
// Must be called before concurrent use.
func (p *probedPool) SetMaxBlocks(n int) {
	p.mux.SetMaxBlocks(n)
}

// SetBudget sets the shared budget on the underlying BufMux.
// Must be called before concurrent use.
func (p *probedPool) SetBudget(cb *combinedBudget) {
	p.mux.SetBudget(cb)
}

// tryCollapse triggers a collapse check on the underlying BufMux.
func (p *probedPool) tryCollapse() {
	p.mux.tryCollapse()
}

// blockCount returns the number of active blocks (for testing).
func (p *probedPool) blockCount() int {
	return p.mux.blockCount()
}

// combinedMuxStats returns total allocated and in-use byte counts across two BufMux instances.
// Used for shared memory budget decisions (AC-27).
func combinedMuxStats(a, b *BufMux) (totalBytes, usedBytes int64) {
	aAlloc, aUsed := a.Stats()
	bAlloc, bUsed := b.Stats()
	totalBytes = int64(aAlloc)*int64(a.bufSize) + int64(bAlloc)*int64(b.bufSize)
	usedBytes = int64(aUsed)*int64(a.bufSize) + int64(bUsed)*int64(b.bufSize)
	return
}

// combinedMuxUsedRatio returns the fraction of allocated bytes in use across
// two BufMux instances (0.0 to 1.0). Returns 0.0 if nothing is allocated.
// Clamped to 1.0 because the two Stats() calls are not atomic — transient
// inconsistency can produce used > total.
func combinedMuxUsedRatio(a, b *BufMux) float64 {
	total, used := combinedMuxStats(a, b)
	if total == 0 {
		return 0.0
	}
	return min(float64(used)/float64(total), 1.0)
}

// MixedBufMux is a shared overflow pool backed by a single pool of 64K blocks.
//
// Three-level slicing from contiguous chunk allocations:
//  1. Chunk level: make([]byte, N*64K) -- amortized allocation, one per growth event
//  2. Block level: 64K slice into a chunk, tracked by overflowBlock
//  3. Slice level: 16 x 4K slices within a subdivided block
//
// Each block can be used in one of two modes:
//   - Whole: the entire 64K is handed out as one buffer (ExtMsg peers).
//   - Subdivided: split into 16 x 4K slices, each handed out individually.
//     When all 16 slices return, the block goes back to the free list.
//
// A block returned to the free list can be reused for either mode next time.
// This avoids partitioning memory between 4K and 64K request paths.
//
// Budget bounds total allocated blocks (active + free). The congestion
// controller polls UsedRatio() = totalBlocks / maxBlocks for graduated
// backpressure (80% denial, 95% teardown). Get returns nil when at max
// capacity with no free blocks.
//
// Thread-safe. All methods may be called from any goroutine.
type MixedBufMux struct {
	mu        sync.Mutex
	chunks    [][]byte         // backing arrays, each overflowChunkBlocks * 64K
	blocks    []*overflowBlock // all blocks indexed by ID
	active    map[uint32]bool  // block IDs with outstanding handles
	free      []uint32         // block IDs available for reuse
	maxBlocks int              // hard ceiling (0 = unlimited)
	minBlocks int              // floor: never collapse below this
}

// overflowBlockSize is the backing size for each block (64K).
const overflowBlockSize = 65536

// overflowSliceCount is the number of 4K slices per subdivided block.
// 65536 / 4096 = 16.
const overflowSliceCount = 16

// overflowChunkBlocks is the number of blocks per chunk allocation.
// Each chunk = 16 x 64K = 1MB. Amortizes make() calls.
const overflowChunkBlocks = 16

// overflowBlock tracks one 64K region within a chunk.
type overflowBlock struct {
	backing []byte // 64K slice into parent chunk
	id      uint32

	// Mode: whole = entire block handed out as one handle.
	// When false and sliceOut > 0, block is subdivided with slices out.
	// A free block has whole=false, sliceOut=0.
	whole bool

	// Subdivision tracking (bitmask for O(1) free-slot finding).
	sliceFree uint16 // bit i set = slice i is free (0-15)
	sliceOut  int    // count of slices currently handed out
}

// initSubdivided prepares the block for 4K slice mode. All 16 slices free.
func (b *overflowBlock) initSubdivided() {
	b.whole = false
	b.sliceFree = 0xFFFF // all 16 bits set
	b.sliceOut = 0
}

// getSlice takes a 4K slice. Returns slice index and buffer, or -1, nil if none free.
func (b *overflowBlock) getSlice() (int, []byte) {
	if b.sliceFree == 0 {
		return -1, nil
	}
	// Find lowest set bit.
	idx := 0
	mask := b.sliceFree
	for mask&1 == 0 {
		idx++
		mask >>= 1
	}
	b.sliceFree &^= 1 << uint(idx)
	b.sliceOut++
	off := idx * message.MaxMsgLen
	return idx, b.backing[off : off+message.MaxMsgLen]
}

// putSlice returns a 4K slice. Returns true if all 16 slices are now back.
func (b *overflowBlock) putSlice(idx int) bool {
	if idx < 0 || idx >= overflowSliceCount {
		fwdLogger().Error("overflowblock: invalid slice index", "idx", idx, "blockID", b.id)
		return false
	}
	bit := uint16(1) << uint(idx)
	if b.sliceFree&bit != 0 {
		fwdLogger().Error("overflowblock: double return on slice", "idx", idx, "blockID", b.id)
		return false
	}
	b.sliceFree |= bit
	b.sliceOut--
	return b.sliceOut == 0
}

// newMixedBufMux creates a mixed-size overflow pool. Starts with zero blocks.
// Call SetByteBudget to set the max block count (maxBytes / 64K).
func newMixedBufMux() *MixedBufMux {
	return &MixedBufMux{
		active: make(map[uint32]bool),
	}
}

// SetByteBudget sets the byte budget by converting to a max block count.
// maxBlocks = maxBytes / 64K. 0 = unlimited.
func (m *MixedBufMux) SetByteBudget(maxBytes int64) {
	m.mu.Lock()
	if maxBytes <= 0 {
		m.maxBlocks = 0
	} else {
		m.maxBlocks = max(int(maxBytes/overflowBlockSize), 1)
	}
	m.mu.Unlock()
}

// ByteBudget returns the current byte budget (maxBlocks * 64K). 0 = unlimited.
func (m *MixedBufMux) ByteBudget() int64 {
	m.mu.Lock()
	n := m.maxBlocks
	m.mu.Unlock()
	if n <= 0 {
		return 0
	}
	return int64(n) * overflowBlockSize
}

// Get4K returns a 4096-byte buffer from a subdivided 64K block.
// Reuses an existing subdivided block with free slices, or takes a free block
// and subdivides it, or grows the pool. Returns zero BufHandle when at capacity.
func (m *MixedBufMux) Get4K() BufHandle {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Find an active subdivided block with free slices.
	for id := range m.active {
		b := m.blocks[id]
		if b.whole || b.sliceFree == 0 {
			continue
		}
		idx, buf := b.getSlice()
		if idx >= 0 {
			return BufHandle{ID: b.id, idx: idx, Buf: buf}
		}
	}

	// 2. Take a free block or grow a new chunk.
	b := m.acquireBlockLocked()
	if b == nil {
		return BufHandle{} // at capacity
	}
	b.initSubdivided()
	m.active[b.id] = true
	idx, buf := b.getSlice()
	return BufHandle{ID: b.id, idx: idx, Buf: buf}
}

// Get64K returns a 65535-byte buffer (one whole 64K block).
// Takes a free block or grows the pool. Returns zero BufHandle when at capacity.
func (m *MixedBufMux) Get64K() BufHandle {
	m.mu.Lock()
	defer m.mu.Unlock()

	b := m.acquireBlockLocked()
	if b == nil {
		return BufHandle{} // at capacity
	}
	b.whole = true
	m.active[b.id] = true
	return BufHandle{ID: b.id, idx: 0, Buf: b.backing[:message.ExtMsgLen]}
}

// Return releases a buffer handle back to the pool. Routes by buffer length.
// When all 16 slices of a subdivided block are back, the block goes to the free list.
// Whole blocks go to the free list immediately.
func (m *MixedBufMux) Return(h BufHandle) {
	if h.Buf == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if int(h.ID) >= len(m.blocks) {
		fwdLogger().Error("mixedbuf: return to unknown block", "blockID", h.ID)
		return
	}
	b := m.blocks[h.ID]

	if b.whole {
		b.whole = false
		delete(m.active, b.id)
		m.free = append(m.free, b.id)
		return
	}

	allBack := b.putSlice(h.idx)
	if allBack {
		delete(m.active, b.id)
		m.free = append(m.free, b.id)
	}
}

// Stats returns total allocated bytes and in-use bytes.
// Total = all blocks (active + free) * 64K. Used = active blocks * 64K.
func (m *MixedBufMux) Stats() (totalBytes, usedBytes int64) {
	m.mu.Lock()
	total := int64(len(m.blocks)) * overflowBlockSize
	used := int64(len(m.active)) * overflowBlockSize
	m.mu.Unlock()
	return total, used
}

// UsedRatio returns totalBlocks / maxBlocks (0.0 to 1.0).
// This reflects real memory pressure -- allocated blocks (active + free)
// against the hard ceiling. The congestion controller uses this for
// 80% denial and 95% teardown. Returns 0.0 when unlimited.
func (m *MixedBufMux) UsedRatio() float64 {
	m.mu.Lock()
	maxB := m.maxBlocks
	totalB := len(m.blocks)
	m.mu.Unlock()
	if maxB <= 0 {
		return 0.0
	}
	return min(float64(totalB)/float64(maxB), 1.0)
}

// tryCollapse releases free blocks above minBlocks. Frees the backing
// memory (chunk references become unreachable when all blocks in a chunk
// are released). Called on a traffic-driven interval.
func (m *MixedBufMux) tryCollapse() {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Release free blocks until we're at minBlocks or out of free blocks.
	// Swap-delete from blocks slice to keep it dense.
	for len(m.free) > 0 && len(m.blocks) > m.minBlocks {
		freeID := m.free[len(m.free)-1]
		m.free = m.free[:len(m.free)-1]

		lastIdx := len(m.blocks) - 1
		freeIdx := int(freeID)
		if freeIdx != lastIdx {
			// Move last block into the freed slot.
			moved := m.blocks[lastIdx]
			m.blocks[freeIdx] = moved
			oldID := moved.id
			moved.id = uint32(freeIdx)
			// Update active/free references.
			if m.active[oldID] {
				delete(m.active, oldID)
				m.active[moved.id] = true
			} else {
				for i, id := range m.free {
					if id == oldID {
						m.free[i] = moved.id
						break
					}
				}
			}
		}
		m.blocks[lastIdx] = nil
		m.blocks = m.blocks[:lastIdx]
	}
}

// blockCount returns the number of active blocks (with outstanding handles).
func (m *MixedBufMux) blockCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.active)
}

// acquireBlockLocked returns a free block or grows the pool.
// Returns nil if at max capacity. Caller must hold mu.
func (m *MixedBufMux) acquireBlockLocked() *overflowBlock {
	// Reuse a free block.
	if len(m.free) > 0 {
		id := m.free[len(m.free)-1]
		m.free = m.free[:len(m.free)-1]
		return m.blocks[id]
	}

	// Grow: check budget.
	if m.maxBlocks > 0 && len(m.blocks) >= m.maxBlocks {
		return nil
	}

	// Allocate a chunk of blocks to amortize make().
	needed := overflowChunkBlocks
	if m.maxBlocks > 0 {
		remaining := m.maxBlocks - len(m.blocks)
		if needed > remaining {
			needed = remaining
		}
	}
	if needed <= 0 {
		return nil
	}

	chunk := make([]byte, needed*overflowBlockSize)
	m.chunks = append(m.chunks, chunk)

	firstID := uint32(len(m.blocks))
	for i := range needed {
		off := i * overflowBlockSize
		b := &overflowBlock{
			backing: chunk[off : off+overflowBlockSize],
			id:      firstID + uint32(i),
		}
		m.blocks = append(m.blocks, b)
		if i > 0 {
			m.free = append(m.free, b.id)
		}
	}

	return m.blocks[firstID]
}

// withCollapseProbe wires a traffic-driven collapse probe to a pool.
// The counter lives in the closure — it belongs to the probe target
// (overflow pool), not to the wrapper. When the overflow pool type is
// built, the counter will move there.
func withCollapseProbe(pp *probedPool, interval int) *probedPool {
	var count atomic.Int64
	every := int64(interval)
	pp.SetProbe(func() {
		if n := count.Add(1); n%every == 0 {
			pp.tryCollapse()
		}
	})
	return pp
}
