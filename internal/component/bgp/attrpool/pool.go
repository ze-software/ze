// Design: docs/architecture/pool-architecture.md — attribute and NLRI pools

//nolint:gosec // G115: Pool has explicit size limits preventing overflow; unsafe usage audited
package attrpool

import (
	"errors"
	"maps"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ze-software/ze/internal/core/memguard"
)

// ErrPoolShutdown is returned when operations are attempted on a shutdown pool.
var ErrPoolShutdown = errors.New("pool is shutdown")

// ErrDataTooLarge is returned when data exceeds MaxDataLength.
var ErrDataTooLarge = errors.New("data exceeds maximum length (65535 bytes)")

// ErrInvalidHandle is returned when an invalid handle is used.
var ErrInvalidHandle = errors.New("invalid handle")

// ErrWrongPool is returned when a handle from a different pool is used.
var ErrWrongPool = errors.New("handle belongs to different pool")

// ErrSlotOutOfBounds is returned when handle references non-existent slot.
var ErrSlotOutOfBounds = errors.New("handle slot out of bounds")

// ErrSlotDead is returned when handle references a released slot.
var ErrSlotDead = errors.New("handle references dead slot")

// ErrPoolFull is returned when a shard has reached MaxSlotsPerShard.
var ErrPoolFull = errors.New("pool shard has reached maximum slot count (4,194,304 per shard)")

// ErrInvalidIdx is returned when pool idx is >= 31 (reserved for InvalidHandle).
var ErrInvalidIdx = errors.New("pool idx must be 0-30 (31 reserved for InvalidHandle)")

// ErrInvalidShardCount is returned when a requested shard count is not a power
// of two in the range 1..numShards.
var ErrInvalidShardCount = errors.New("shard count must be a power of two between 1 and 16")

// MaxDataLength is the maximum length of data that can be interned.
// Limited by uint16 length field in slot struct.
const MaxDataLength = 65535

// numShards is the maximum (and default) number of independent sub-pools per
// logical Pool. Each shard owns its own lock, dedup index, slot table and double
// buffer, so concurrent Intern/Get to different shards proceed in parallel and
// each shard's RWMutex reader-count word lives on its own cache line.
//
// The per-pool shard count is chosen at construction (NewWithShards) and may be
// any power of two in 1..numShards. numShards is the ceiling because the shard
// id is carved from shardIDBits (handle.go) of the Slot space. A pool created
// with 1 shard degenerates to the pre-sharding single-lock behavior (one lock,
// one index, one buffer, shard id always 0) with no extra fixed overhead, which
// is preferable for low-cardinality pools (e.g. ORIGIN) whose few hot values
// would monopolize a single shard and gain nothing from sharding.
const numShards = 1 << shardIDBits // 16

// MaxSlotsPerShard is the maximum number of slots in a single shard.
// Limited by the 22-bit per-shard slot sub-field of Handle.
const MaxSlotsPerShard = shardSlotMask + 1 // 4,194,304

// MaxSlots is the maximum number of slots per logical pool, aggregated across
// the maximum shard count. With 16 shards of 22-bit slot space this is
// 67,108,864, which exceeds the pre-sharding 24-bit limit (16,777,215). A pool
// configured with fewer shards has a proportionally smaller aggregate capacity
// (shardCount * MaxSlotsPerShard); a 1-shard pool holds up to MaxSlotsPerShard.
const MaxSlots = numShards * MaxSlotsPerShard // 67,108,864

// PoolState indicates the current compaction state.
type PoolState int

const (
	// PoolNormal means no compaction in progress.
	PoolNormal PoolState = iota
	// PoolCompacting means incremental compaction is in progress.
	PoolCompacting
)

// buffer holds data for one side of the double-buffer.
type buffer struct {
	data     []byte       // Buffer data
	pos      int          // Write cursor
	refCount atomic.Int32 // Number of handles pointing to this buffer
}

// slot tracks a single interned entry.
type slot struct {
	offsets  [2]uint32 // offset in EACH buffer (both valid during compaction)
	length   uint16    // data length
	refCount int32     // reference count
	dead     bool      // marked for removal
	// migrated marks a slot whose live bytes already reside in the current
	// (new) buffer during an active compaction, so migrateBatch must not copy
	// it from a stale old-buffer offset. Set when a free-list slot is reused by
	// intern mid-compaction; only meaningful while state == PoolCompacting and
	// cleared at the start of each compaction. Fits the slot's existing padding
	// (no per-slot memory cost).
	migrated bool
}

// shard is one independent sub-pool. It holds the full per-pool state (lock,
// dedup index, slot table, free list, double buffer and compaction cursor) that
// before sharding lived directly on Pool. A handle's shard id (high bits of its
// Slot field) selects which shard owns it; within the shard the low 22 bits of
// Slot index the slot table.
type shard struct {
	mu sync.RWMutex

	id       uint32       // shard index, packed into handle Slot high bits
	idx      uint8        // owning pool index, for handle encoding/validation
	shutdown *atomic.Bool // owning pool's shutdown flag (shared across shards)

	// Double buffer - alternates between compaction cycles
	buffers    [2]buffer
	currentBit uint32 // 0 or 1 - which buffer is current

	// Compaction state
	state            PoolState
	compactCursor    uint32 // Migration progress (slot index)
	compactSlotCount uint32 // Slot count when compaction started (don't migrate new slots)

	// Slot table - indexed by handle's per-shard slot portion
	slots []slot

	// Free list for slot reuse
	freeSlots []uint32

	// Dedup index: data content → Handle
	// Keys use unsafe.String pointing into data buffer (zero-copy)
	index map[string]Handle

	// Activity tracking for scheduler
	lastActivity atomic.Int64 // Unix nano timestamp

	// Metrics counters
	internTotal atomic.Int64 // total Intern() calls routed to this shard
	internHits  atomic.Int64 // Intern() calls that hit an existing entry
}

// Pool provides zero-copy byte slice deduplication for BGP attributes and NLRI.
//
// Thread-safe. Uses reference counting for lifecycle management.
// Designed for high-frequency access patterns with many duplicate entries.
// Uses double-buffer design for non-blocking incremental compaction.
//
// Internally a Pool is sharded into 1..numShards independent sub-pools selected
// by a content hash of the interned bytes; the count is fixed at construction.
// Sharding is invisible to callers: the Handle ABI, global deduplication,
// reference counting and incremental compaction semantics are unchanged. A
// 1-shard pool behaves exactly like the pre-sharding single-lock pool.
type Pool struct {
	// Pool index for handle encoding (0-30, 31 reserved for InvalidHandle)
	idx uint8

	// Shutdown state (shared by all shards)
	shutdown atomic.Bool

	// shardMask = len(shards)-1; selects a shard from a content hash with a
	// single AND. len(shards) is a power of two in 1..numShards.
	shardMask uint32

	// Independent sub-pools selected by shardOf(data, shardMask).
	shards []shard
}

// New creates a pool with idx=0, the default shard count, and the given initial
// buffer capacity. For pools with specific idx, use NewWithIdx; to choose the
// shard count, use NewWithShards.
func New(initialCapacity int) *Pool {
	p, _ := NewWithShards(0, initialCapacity, numShards) // idx=0, default shards
	return p
}

// NewWithIdx creates a pool with the given index, the default shard count, and
// the given initial buffer capacity.
// idx must be 0-30 (31 is reserved for InvalidHandle).
// Returns ErrInvalidIdx if idx >= 31.
func NewWithIdx(idx uint8, initialCapacity int) (*Pool, error) {
	return NewWithShards(idx, initialCapacity, numShards)
}

// NewWithShards creates a pool with the given index, initial buffer capacity and
// shard count. shards must be a power of two in 1..numShards (16); shards=1
// gives the pre-sharding single-lock pool with no extra fixed overhead, suited
// to low-cardinality pools whose hot values cannot spread across shards.
//
// idx must be 0-30 (31 is reserved for InvalidHandle); returns ErrInvalidIdx
// otherwise, or ErrInvalidShardCount for an invalid shard count.
//
// initialCapacity is the total starting buffer capacity for the logical pool;
// it is divided across the shards (each shard gets at least 64 bytes).
func NewWithShards(idx uint8, initialCapacity, shards int) (*Pool, error) {
	if idx >= 31 {
		return nil, ErrInvalidIdx
	}
	if shards < 1 || shards > numShards || shards&(shards-1) != 0 {
		return nil, ErrInvalidShardCount
	}

	perShardCapacity := max(initialCapacity/shards, 64)

	p := &Pool{idx: idx, shardMask: uint32(shards - 1)}
	p.shards = make([]shard, shards)
	for i := range p.shards {
		s := &p.shards[i]
		s.id = uint32(i)
		s.idx = idx
		s.shutdown = &p.shutdown
		s.currentBit = 0
		s.state = PoolNormal
		s.slots = make([]slot, 0, 64)
		s.index = make(map[string]Handle, 64)
		// Initialize buffer 0 (currentBit starts at 0)
		s.buffers[0].data = make([]byte, 0, perShardCapacity)
	}
	return p, nil
}

// shardOf selects a shard for the given bytes using an allocation-free FNV-1a
// hash folded and masked to the shard count. Identical bytes always select the
// same shard, which is what preserves the global deduplication guarantee.
//
// FNV-1a's final step is a multiply, so its low bits carry the weakest mixing;
// masking them directly would skew distribution for inputs that differ mainly in
// their high bits. A single xor-fold of the high half into the low half before
// masking restores avalanche at negligible cost.
func shardOf(data []byte, mask uint32) uint32 {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)
	h := uint32(offset32)
	for _, b := range data {
		h ^= uint32(b)
		h *= prime32
	}
	h ^= h >> 16
	return h & mask
}

// shardForHandle returns the shard that owns the given handle, or ErrWrongPool
// if the handle's shard id is outside this pool's shard range. A valid handle
// minted by this pool always has shardID < len(shards) (shardOf masks with
// shardMask), so an out-of-range id means the handle is foreign or corrupt.
// Bounds-checking here keeps a stray handle from panicking on shards[] before
// validateHandle runs.
func (p *Pool) shardForHandle(h Handle) (*shard, error) {
	sid := h.shardID()
	if sid >= uint32(len(p.shards)) {
		return nil, ErrWrongPool
	}
	return &p.shards[sid], nil
}

// Touch marks every shard as recently active.
// Used by scheduler to determine when compaction is safe.
func (p *Pool) Touch() {
	now := time.Now().UnixNano()
	for i := range p.shards {
		p.shards[i].lastActivity.Store(now)
	}
}

// IsIdle returns true if every shard has been inactive for the given duration.
func (p *Pool) IsIdle(d time.Duration) bool {
	cutoff := time.Now().Add(-d).UnixNano()
	for i := range p.shards {
		last := p.shards[i].lastActivity.Load()
		if last != 0 && last > cutoff {
			return false // a shard was active more recently than the cutoff
		}
	}
	return true
}

// Intern stores data in the pool with deduplication.
// Returns a handle that can be used to retrieve the data.
// If identical data already exists, increments refCount and returns existing handle.
// Returns ErrPoolShutdown if pool is shutdown.
// Returns ErrDataTooLarge if data exceeds MaxDataLength (65535 bytes).
// Returns ErrPoolFull if the owning shard has reached MaxSlotsPerShard.
func (p *Pool) Intern(data []byte) (Handle, error) {
	// Treat nil as empty
	if data == nil {
		data = []byte{}
	}

	// Validate length fits in uint16
	if len(data) > MaxDataLength {
		return InvalidHandle, ErrDataTooLarge
	}

	return p.shards[shardOf(data, p.shardMask)].intern(data)
}

// intern performs the actual intern operation within a single shard.
func (s *shard) intern(data []byte) (Handle, error) {
	lookupKey := bytesToString(data)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check shutdown under lock
	if s.shutdown.Load() {
		return InvalidHandle, ErrPoolShutdown
	}

	// Track metrics
	s.internTotal.Add(1)

	// Check for existing entry (deduplication)
	// Index always contains handles with currentBit
	if h, ok := s.index[lookupKey]; ok {
		sl := &s.slots[h.shardSlot()]
		if !sl.dead && sl.refCount > 0 {
			sl.refCount++
			s.buffers[h.BufferBit()].refCount.Add(1)
			s.internHits.Add(1) // Deduplication hit
			return h, nil
		}
	}

	// Mark activity only on new entry creation, not dedup hits.
	// Dedup hits don't change pool structure, so compaction scheduling
	// (which uses IsIdle) doesn't need to see them as activity.
	s.lastActivity.Store(time.Now().UnixNano())

	// Check slot limit under lock (no race). In debug builds freed slots are
	// not reused (ABA guard), so a non-empty free list does not relieve the
	// limit; slotReuseEnabled folds this back to the original check in release.
	if len(s.slots) >= MaxSlotsPerShard && (!slotReuseEnabled || len(s.freeSlots) == 0) {
		return InvalidHandle, ErrPoolFull
	}

	// Allocate new entry in current buffer
	bufIdx := s.currentBit
	buf := &s.buffers[bufIdx]

	s.ensureCapacity(len(data))

	offset := uint32(buf.pos)
	buf.data = append(buf.data, data...)
	buf.pos += len(data)

	// Allocate or reuse slot. Debug builds never reuse (slotReuseEnabled=false):
	// a freed slot stays dead so a stale handle to it trips ErrSlotDead instead
	// of resolving to this new occupant's bytes.
	var slotIdx uint32
	if slotReuseEnabled && len(s.freeSlots) > 0 {
		slotIdx = s.freeSlots[len(s.freeSlots)-1]
		s.freeSlots = s.freeSlots[:len(s.freeSlots)-1]
		sl := &s.slots[slotIdx]
		sl.offsets[bufIdx] = offset
		sl.length = uint16(len(data))
		sl.refCount = 1
		sl.dead = false
		// During compaction the reused slot's bytes are written into the new
		// (current) buffer; its old-buffer offset is stale from the previous
		// occupant. Mark it so migrateBatch skips it rather than copying from
		// that stale offset and clobbering the live new-buffer offset.
		sl.migrated = s.state == PoolCompacting
	} else {
		slotIdx = uint32(len(s.slots))
		newSlot := slot{
			length:   uint16(len(data)),
			refCount: 1,
			dead:     false,
		}
		newSlot.offsets[bufIdx] = offset
		s.slots = append(s.slots, newSlot)
	}

	// Create handle with pool idx, shard id and buffer bit encoded
	h := NewHandleWithBuffer(bufIdx, s.idx, packShardSlot(s.id, slotIdx))

	// Track buffer reference
	buf.refCount.Add(1)

	// Index with key pointing to buffer memory (zero-copy)
	bufferKey := bytesToString(buf.data[offset : offset+uint32(len(data))])
	s.index[bufferKey] = h

	return h, nil
}

// Get returns the data associated with the handle.
// Returns a slice pointing into the pool's buffer (zero-copy).
// The returned slice is only valid while the handle is live.
// Returns error if handle is invalid, from wrong pool, or references dead slot.
func (p *Pool) Get(h Handle) ([]byte, error) {
	s, err := p.shardForHandle(h)
	if err != nil {
		return nil, err
	}
	return s.get(h)
}

func (s *shard) get(h Handle) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.validateHandle(h); err != nil {
		return nil, err
	}

	bufIdx := h.BufferBit()
	slotIdx := h.shardSlot()
	sl := &s.slots[slotIdx]
	offset := sl.offsets[bufIdx]
	return s.buffers[bufIdx].data[offset : offset+uint32(sl.length)], nil
}

// Length returns the length of data associated with the handle.
// Returns error if handle is invalid, from wrong pool, or references dead slot.
func (p *Pool) Length(h Handle) (int, error) {
	s, err := p.shardForHandle(h)
	if err != nil {
		return 0, err
	}
	return s.length(h)
}

func (s *shard) length(h Handle) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.validateHandle(h); err != nil {
		return 0, err
	}

	return int(s.slots[h.shardSlot()].length), nil
}

// Release decrements the reference count for the handle.
// When refCount reaches zero, the entry is marked dead and eligible for reclamation.
// Returns error if handle is invalid, from wrong pool, or slot out of bounds.
func (p *Pool) Release(h Handle) error {
	s, err := p.shardForHandle(h)
	if err != nil {
		return err
	}
	return s.release(h)
}

func (s *shard) release(h Handle) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateHandleForRelease(h); err != nil {
		return err
	}

	bufIdx := h.BufferBit()
	slotIdx := h.shardSlot()
	sl := &s.slots[slotIdx]

	sl.refCount--
	s.buffers[bufIdx].refCount.Add(-1)

	if sl.refCount <= 0 {
		s.retireSlot(sl, slotIdx, bufIdx)
	}

	return nil
}

// retireSlot marks a slot dead, evicts its dedup index entry, poisons its bytes
// in debug builds, and returns it to the free list. Called with the shard lock
// held once refCount reaches zero. slotIdx/bufIdx address the slot's live bytes.
//
// The index delete must precede the poison: delete hashes the current bytes to
// find the entry, so poisoning first would leak a stale index entry. Poisoning
// the freed region makes any borrowed slice still reading it surface as poison
// in debug; in release memguard.Enabled folds the poison away (zero cost, bytes
// stay intact until compaction reclaims the space).
//
// The raw-slice poison targets only the caller's bufIdx. A slot migrated
// mid-compaction has live bytes in the other buffer, so a raw slice borrowed
// from a post-migration Get into that buffer would not be poisoned here; the
// stale-handle case is still caught unconditionally by validateHandle's
// dead-slot check, so this only narrows the secondary raw-slice canary.
func (s *shard) retireSlot(sl *slot, slotIdx, bufIdx uint32) {
	sl.dead = true

	buf := &s.buffers[bufIdx]
	if len(buf.data) > 0 {
		offset := sl.offsets[bufIdx]
		bufferKey := bytesToString(buf.data[offset : offset+uint32(sl.length)])
		delete(s.index, bufferKey)
		if memguard.Enabled {
			memguard.Poison(buf.data[offset : offset+uint32(sl.length)])
		}
	}

	s.freeSlots = append(s.freeSlots, slotIdx)
}

// Shutdown marks the pool as shutdown, rejecting new operations.
// Existing handles remain valid for Get() and Release().
// Safe to call multiple times.
func (p *Pool) Shutdown() {
	p.shutdown.Store(true)
}

// IsShutdown returns true if the pool has been shutdown.
func (p *Pool) IsShutdown() bool {
	return p.shutdown.Load()
}

// AddRef increments the reference count for the handle.
// Use when sharing a handle between multiple owners.
// Returns error if handle is invalid or from wrong pool.
func (p *Pool) AddRef(h Handle) error {
	s, err := p.shardForHandle(h)
	if err != nil {
		return err
	}
	return s.addRef(h)
}

func (s *shard) addRef(h Handle) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateHandle(h); err != nil {
		return err
	}

	bufIdx := h.BufferBit()
	slotIdx := h.shardSlot()
	sl := &s.slots[slotIdx]

	sl.refCount++
	s.buffers[bufIdx].refCount.Add(1)

	return nil
}

// GetBySlot returns data for a normalized slot index.
// Auto-selects the correct buffer based on compaction state.
// Use when handles are stored normalized (slot only, no bufferBit).
//
// slotIdx is the full 26-bit Slot field (shard id in the high bits); it is
// routed to the owning shard exactly like a handle's slot.
func (p *Pool) GetBySlot(slotIdx uint32) ([]byte, error) {
	shard := (slotIdx >> shardSlotBits) & shardIDMask
	if shard >= uint32(len(p.shards)) {
		return nil, ErrSlotOutOfBounds
	}
	return p.shards[shard].getBySlot(slotIdx & shardSlotMask)
}

func (s *shard) getBySlot(localSlot uint32) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if localSlot >= uint32(len(s.slots)) {
		return nil, ErrSlotOutOfBounds
	}

	sl := &s.slots[localSlot]
	if sl.dead || sl.refCount <= 0 {
		return nil, ErrSlotDead
	}

	// Determine correct buffer. A slot reused by intern during compaction
	// (sl.migrated) already lives in the current buffer even though the cursor
	// has not reached it, so it must not be read from the old buffer.
	bufIdx := s.currentBit
	if s.state == PoolCompacting && localSlot >= s.compactCursor && !sl.migrated {
		// Slot not yet migrated, use old buffer
		bufIdx = 1 - s.currentBit
	}

	offset := sl.offsets[bufIdx]
	return s.buffers[bufIdx].data[offset : offset+uint32(sl.length)], nil
}

// ReleaseBySlot decrements reference count for a normalized slot index.
// Auto-selects the correct buffer based on compaction state.
// Use when handles are stored normalized (slot only, no bufferBit).
func (p *Pool) ReleaseBySlot(slotIdx uint32) error {
	shard := (slotIdx >> shardSlotBits) & shardIDMask
	if shard >= uint32(len(p.shards)) {
		return ErrSlotOutOfBounds
	}
	return p.shards[shard].releaseBySlot(slotIdx & shardSlotMask)
}

func (s *shard) releaseBySlot(localSlot uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if localSlot >= uint32(len(s.slots)) {
		return ErrSlotOutOfBounds
	}

	sl := &s.slots[localSlot]

	// Determine correct buffer. A slot reused by intern during compaction
	// (sl.migrated) already lives in the current buffer, so its buffer
	// reference is the current buffer's, not the old one's.
	bufIdx := s.currentBit
	if s.state == PoolCompacting && localSlot >= s.compactCursor && !sl.migrated {
		bufIdx = 1 - s.currentBit
	}

	sl.refCount--
	s.buffers[bufIdx].refCount.Add(-1)

	if sl.refCount <= 0 {
		s.retireSlot(sl, localSlot, bufIdx)
	}

	return nil
}

// State returns the current compaction state.
// Reports PoolCompacting if any shard is compacting.
func (p *Pool) State() PoolState {
	for i := range p.shards {
		if p.shards[i].isCompacting() {
			return PoolCompacting
		}
	}
	return PoolNormal
}

func (s *shard) isCompacting() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == PoolCompacting
}

// deadRatioExceeds reports whether this shard is fragmented enough to be worth
// compacting: its dead-slot ratio meets threshold. The dead/total definition
// matches Metrics (DeadSlots = dead slots whose bytes still occupy the buffer;
// TotalSlots = len(slots)), so per-shard scheduling matches observable metrics.
// Deciding per shard avoids the dilution where one heavily-fragmented shard
// hides below a pool-wide average and never gets compacted.
//
// Fast path: every slot released to zero is pushed onto freeSlots, so an empty
// free list means there are no dead slots and the O(slots) walk is skipped. This
// keeps the scheduler's per-tick scan over many shards cheap when shards are
// clean (the common case).
func (s *shard) deadRatioExceeds(threshold float64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.freeSlots) == 0 {
		return false // no released slots → nothing dead to reclaim
	}

	total := len(s.slots)
	if total == 0 {
		return false
	}

	dead := 0
	for i := range s.slots {
		sl := &s.slots[i]
		if (sl.dead || sl.refCount <= 0) && sl.length > 0 {
			dead++
		}
	}
	if dead == 0 {
		return false
	}
	return float64(dead)/float64(total) >= threshold
}

// StartCompaction begins incremental compaction on every shard.
// Allocates new buffers and sets state to PoolCompacting.
// Call MigrateBatch() repeatedly until it returns true.
func (p *Pool) StartCompaction() {
	for i := range p.shards {
		p.shards[i].startCompaction()
	}
}

func (s *shard) startCompaction() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == PoolCompacting {
		return // Already compacting
	}

	// Count live bytes for new buffer sizing. Also clear any stale migrated
	// marks from a prior cycle: at the start of a fresh compaction every
	// surviving slot's live bytes are in the old buffer and must be migrated.
	var liveBytes int64
	for i := range s.slots {
		sl := &s.slots[i]
		sl.migrated = false
		if !sl.dead && sl.refCount > 0 {
			liveBytes += int64(sl.length)
		}
	}

	// Flip to new buffer
	oldBit := s.currentBit
	newBit := 1 - oldBit
	s.currentBit = newBit

	// Allocate new buffer with headroom
	newSize := max(liveBytes+liveBytes/4, 64)
	s.buffers[newBit].data = make([]byte, 0, newSize)
	s.buffers[newBit].pos = 0
	s.buffers[newBit].refCount.Store(0)

	s.state = PoolCompacting
	s.compactCursor = 0
	s.compactSlotCount = uint32(len(s.slots)) // Only migrate slots that existed at start
}

// MigrateBatch migrates a batch of slots per shard to the new buffer.
// Returns true only when every shard's migration is complete.
// Call repeatedly until it returns true.
func (p *Pool) MigrateBatch(batchSize int) bool {
	done := true
	for i := range p.shards {
		if !p.shards[i].migrateBatch(batchSize) {
			done = false
		}
	}
	return done
}

func (s *shard) migrateBatch(batchSize int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != PoolCompacting {
		return true
	}

	oldBit := 1 - s.currentBit
	newBit := s.currentBit
	oldBuf := &s.buffers[oldBit]
	newBuf := &s.buffers[newBit]

	migrated := 0
	for s.compactCursor < s.compactSlotCount && migrated < batchSize {
		sl := &s.slots[s.compactCursor]

		switch {
		case sl.dead || sl.refCount <= 0:
			// Clear dead slot data reference (so Metrics doesn't count as dead)
			sl.offsets[oldBit] = 0
			sl.offsets[newBit] = 0
			sl.length = 0
		case sl.migrated:
			// Slot was reused by intern during this compaction: its live bytes
			// already reside in the new buffer at offsets[newBit] and its index
			// entry already points there. Migrating it would copy from the stale
			// old-buffer offset and clobber the live offset, so leave it as is.
		default:
			// Copy data from old buffer to new buffer
			oldOffset := sl.offsets[oldBit]
			oldData := oldBuf.data[oldOffset : oldOffset+uint32(sl.length)]

			newOffset := uint32(newBuf.pos)
			newBuf.data = append(newBuf.data, oldData...)
			newBuf.pos += int(sl.length)

			sl.offsets[newBit] = newOffset

			// Update index with new handle
			oldKey := bytesToString(oldData)
			delete(s.index, oldKey)

			newKey := bytesToString(newBuf.data[newOffset : newOffset+uint32(sl.length)])
			newHandle := NewHandleWithBuffer(newBit, s.idx, packShardSlot(s.id, s.compactCursor))
			s.index[newKey] = newHandle

			migrated++
		}

		s.compactCursor++
	}

	// Check if migration complete
	if s.compactCursor >= s.compactSlotCount {
		// All slots processed
		if oldBuf.refCount.Load() == 0 {
			s.finishCompaction()
		}
		return true
	}

	return false
}

// finishCompaction completes compaction by freeing old buffer.
// Called with lock held.
func (s *shard) finishCompaction() {
	oldBit := 1 - s.currentBit
	s.buffers[oldBit].data = nil
	s.buffers[oldBit].pos = 0
	s.state = PoolNormal
}

// CheckOldBufferRelease checks if old buffers can be freed after compaction.
// Call periodically after MigrateBatch returns true.
func (p *Pool) CheckOldBufferRelease() {
	for i := range p.shards {
		p.shards[i].checkOldBufferRelease()
	}
}

func (s *shard) checkOldBufferRelease() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != PoolCompacting {
		return
	}

	oldBit := 1 - s.currentBit
	if s.buffers[oldBit].refCount.Load() == 0 {
		s.finishCompaction()
	}
}

// ensureCapacity ensures the current buffer can hold additional bytes.
// Called with lock held.
func (s *shard) ensureCapacity(needed int) {
	buf := &s.buffers[s.currentBit]
	required := buf.pos + needed
	if required <= cap(buf.data) {
		return
	}

	// Grow buffer
	newCap := max(cap(buf.data)*2, required)

	oldData := buf.data
	buf.data = make([]byte, len(oldData), newCap)
	copy(buf.data, oldData)

	// Rebuild index - old keys reference old memory
	s.rebuildIndex()
}

// rebuildIndex recreates the index with keys pointing to current buffer.
// Called with lock held after buffer reallocation.
func (s *shard) rebuildIndex() {
	bufIdx := s.currentBit
	buf := &s.buffers[bufIdx]

	// During compaction, preserve entries pointing to old buffer
	// (their keys still reference valid old buffer memory)
	var preserved map[string]Handle
	if s.state == PoolCompacting {
		oldBit := 1 - bufIdx
		preserved = make(map[string]Handle)
		for k, h := range s.index {
			if h.BufferBit() == oldBit {
				preserved[k] = h
			}
		}
	}

	s.index = make(map[string]Handle, len(s.slots))

	// Restore preserved old-buffer entries
	maps.Copy(s.index, preserved)

	// Rebuild entries for current buffer
	for i := range s.slots {
		sl := &s.slots[i]
		if !sl.dead && sl.refCount > 0 {
			// During compaction, skip unmigrated slots - they're already
			// in preserved entries pointing to old buffer. A slot reused by
			// intern mid-compaction (sl.migrated) already lives in the current
			// buffer, so it must be re-indexed here, not skipped.
			if s.state == PoolCompacting && uint32(i) >= s.compactCursor && !sl.migrated {
				continue
			}
			offset := sl.offsets[bufIdx]
			key := bytesToString(buf.data[offset : offset+uint32(sl.length)])
			s.index[key] = NewHandleWithBuffer(bufIdx, s.idx, packShardSlot(s.id, uint32(i)))
		}
	}
}

// bytesToString converts a byte slice to a string without copying.
// The string is only valid while the underlying byte slice is valid.
func bytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}

// Metrics holds pool statistics.
type Metrics struct {
	LiveSlots  int32 // slots with refCount > 0
	DeadSlots  int32 // slots marked dead (refCount <= 0)
	LiveBytes  int64 // bytes in live slots
	DeadBytes  int64 // bytes in dead slots
	TotalSlots int32 // total slot count
	BufferSize int64 // current buffer size
	BufferCap  int64 // current buffer capacity

	// Deduplication metrics
	InternTotal int64 // total Intern() calls
	InternHits  int64 // Intern() calls that hit existing entry
}

// DeduplicationRate returns the ratio of deduplication hits to total interns.
// Returns 0 if no interns have occurred.
func (m Metrics) DeduplicationRate() float64 {
	if m.InternTotal == 0 {
		return 0
	}
	return float64(m.InternHits) / float64(m.InternTotal)
}

// Metrics returns current pool statistics, aggregated across all shards.
func (p *Pool) Metrics() Metrics {
	var m Metrics
	for i := range p.shards {
		p.shards[i].accumulateMetrics(&m)
	}
	return m
}

// accumulateMetrics adds this shard's statistics into m.
func (s *shard) accumulateMetrics(m *Metrics) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buf := &s.buffers[s.currentBit]

	m.TotalSlots += int32(len(s.slots))
	m.BufferSize += int64(len(buf.data))
	m.BufferCap += int64(cap(buf.data))
	m.InternTotal += s.internTotal.Load()
	m.InternHits += s.internHits.Load()

	for i := range s.slots {
		sl := &s.slots[i]
		if !sl.dead && sl.refCount > 0 {
			m.LiveSlots++
			m.LiveBytes += int64(sl.length)
		} else if sl.length > 0 {
			// Dead slot with data still in buffer (not yet compacted)
			m.DeadSlots++
			m.DeadBytes += int64(sl.length)
		}
		// Slots with length=0 are reclaimed/free, not counted as dead
	}
}

// Compact removes dead entries and reclaims buffer memory in every shard.
// Live handles remain valid after compaction.
// Note: This is stop-the-world per shard. Use StartCompaction/MigrateBatch for non-blocking.
// If incremental compaction is in progress on a shard, that shard is skipped.
func (p *Pool) Compact() {
	for i := range p.shards {
		p.shards[i].compact()
	}
}

func (s *shard) compact() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Don't interfere with incremental compaction
	if s.state == PoolCompacting {
		return
	}

	bufIdx := s.currentBit
	buf := &s.buffers[bufIdx]

	// Count live bytes
	var liveBytes int
	for i := range s.slots {
		sl := &s.slots[i]
		if !sl.dead && sl.refCount > 0 {
			liveBytes += int(sl.length)
		}
	}

	// Nothing to compact if no dead entries
	if len(s.freeSlots) == 0 {
		return
	}

	// Create new buffer with only live data
	newData := make([]byte, 0, liveBytes+liveBytes/4) // 25% headroom
	newPos := 0

	// Copy live data to new buffer, update slot offsets
	for i := range s.slots {
		sl := &s.slots[i]
		if !sl.dead && sl.refCount > 0 {
			// Copy data to new buffer
			oldData := buf.data[sl.offsets[bufIdx] : sl.offsets[bufIdx]+uint32(sl.length)]
			newOffset := uint32(newPos)
			newData = append(newData, oldData...)
			newPos += int(sl.length)

			// Update slot offset in current buffer
			sl.offsets[bufIdx] = newOffset
		} else {
			// Clear dead slot data reference
			sl.offsets[bufIdx] = 0
			sl.length = 0
		}
	}

	// Update buffer
	buf.data = newData
	buf.pos = newPos

	// Rebuild index with new buffer pointers
	s.rebuildIndex()
}
