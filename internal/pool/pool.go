// Design: docs/architecture/pool-architecture.md — attribute and NLRI pools

//nolint:gosec // G115: Pool has explicit size limits preventing overflow; unsafe usage audited
package pool

import (
	"errors"
	"maps"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
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

// ErrPoolFull is returned when pool has reached MaxSlots limit.
var ErrPoolFull = errors.New("pool has reached maximum slot count (16,777,215)")

// MaxDataLength is the maximum length of data that can be interned.
// Limited by uint16 length field in slot struct.
const MaxDataLength = 65535

// MaxSlots is the maximum number of slots per pool.
// Limited by 24-bit slot field in Handle.
const MaxSlots = 0xFFFFFF // 16,777,215

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

// Pool provides zero-copy byte slice deduplication for BGP attributes and NLRI.
//
// Thread-safe. Uses reference counting for lifecycle management.
// Designed for high-frequency access patterns with many duplicate entries.
// Uses double-buffer design for non-blocking incremental compaction.
type Pool struct {
	mu sync.RWMutex

	// Pool index for handle encoding (0-30, 31 reserved for InvalidHandle)
	idx uint8

	// Double buffer - alternates between compaction cycles
	buffers    [2]buffer
	currentBit uint32 // 0 or 1 - which buffer is current

	// Compaction state
	state            PoolState
	compactCursor    uint32 // Migration progress (slot index)
	compactSlotCount uint32 // Slot count when compaction started (don't migrate new slots)

	// Slot table - indexed by handle's slot portion
	slots []slot

	// Free list for slot reuse
	freeSlots []uint32

	// Dedup index: data content → Handle
	// Keys use unsafe.String pointing into data buffer (zero-copy)
	index map[string]Handle

	// Activity tracking for scheduler
	lastActivity atomic.Int64 // Unix nano timestamp

	// Metrics counters
	internTotal atomic.Int64 // total Intern() calls
	internHits  atomic.Int64 // Intern() calls that hit existing entry

	// Shutdown state
	shutdown atomic.Bool
}

// slot tracks a single interned entry.
type slot struct {
	offsets  [2]uint32 // offset in EACH buffer (both valid during compaction)
	length   uint16    // data length
	refCount int32     // reference count
	dead     bool      // marked for removal
}

// New creates a pool with idx=0 and the given initial buffer capacity.
// For pools with specific idx, use NewWithIdx.
func New(initialCapacity int) *Pool {
	return NewWithIdx(0, initialCapacity)
}

// NewWithIdx creates a pool with the given index and initial buffer capacity.
// idx must be 0-30 (31 is reserved for InvalidHandle).
// Panics if idx >= 31.
func NewWithIdx(idx uint8, initialCapacity int) *Pool {
	if idx >= 31 {
		panic("pool idx must be 0-30, 31 is reserved for InvalidHandle")
	}
	if initialCapacity < 64 {
		initialCapacity = 64
	}
	p := &Pool{
		idx:        idx,
		currentBit: 0,
		state:      PoolNormal,
		slots:      make([]slot, 0, 64),
		index:      make(map[string]Handle, 64),
	}
	// Initialize buffer 0 (currentBit starts at 0)
	p.buffers[0].data = make([]byte, 0, initialCapacity)
	return p
}

// Touch marks the pool as recently active.
// Used by scheduler to determine when compaction is safe.
func (p *Pool) Touch() {
	p.lastActivity.Store(time.Now().UnixNano())
}

// IsIdle returns true if the pool has been inactive for the given duration.
func (p *Pool) IsIdle(d time.Duration) bool {
	last := p.lastActivity.Load()
	if last == 0 {
		return true // Never used
	}
	return time.Since(time.Unix(0, last)) >= d
}

// Intern stores data in the pool with deduplication.
// Returns a handle that can be used to retrieve the data.
// If identical data already exists, increments refCount and returns existing handle.
// Panics if pool is shutdown, data too large, or pool is full.
// Use InternWithError for error returns instead of panics.
func (p *Pool) Intern(data []byte) Handle {
	h, err := p.internLocked(data)
	if err != nil {
		panic("pool: " + err.Error())
	}
	return h
}

// internLocked performs the actual intern operation under lock.
// Returns error instead of panicking.
func (p *Pool) internLocked(data []byte) (Handle, error) {
	// Treat nil as empty
	if data == nil {
		data = []byte{}
	}

	// Validate length fits in uint16
	if len(data) > MaxDataLength {
		return InvalidHandle, ErrDataTooLarge
	}

	lookupKey := bytesToString(data)

	p.mu.Lock()
	defer p.mu.Unlock()

	// Check shutdown under lock
	if p.shutdown.Load() {
		return InvalidHandle, ErrPoolShutdown
	}

	// Track metrics
	p.internTotal.Add(1)

	// Mark activity
	p.lastActivity.Store(time.Now().UnixNano())

	// Check for existing entry (deduplication)
	// Index always contains handles with currentBit
	if h, ok := p.index[lookupKey]; ok {
		s := &p.slots[h.Slot()]
		if !s.dead && s.refCount > 0 {
			s.refCount++
			p.buffers[h.BufferBit()].refCount.Add(1)
			p.internHits.Add(1) // Deduplication hit
			return h, nil
		}
	}

	// Check slot limit under lock (no race)
	if len(p.slots) >= MaxSlots && len(p.freeSlots) == 0 {
		return InvalidHandle, ErrPoolFull
	}

	// Allocate new entry in current buffer
	bufIdx := p.currentBit
	buf := &p.buffers[bufIdx]

	p.ensureCapacity(len(data))

	offset := uint32(buf.pos)
	buf.data = append(buf.data, data...)
	buf.pos += len(data)

	// Allocate or reuse slot
	var slotIdx uint32
	if len(p.freeSlots) > 0 {
		slotIdx = p.freeSlots[len(p.freeSlots)-1]
		p.freeSlots = p.freeSlots[:len(p.freeSlots)-1]
		s := &p.slots[slotIdx]
		s.offsets[bufIdx] = offset
		s.length = uint16(len(data))
		s.refCount = 1
		s.dead = false
	} else {
		slotIdx = uint32(len(p.slots))
		newSlot := slot{
			length:   uint16(len(data)),
			refCount: 1,
			dead:     false,
		}
		newSlot.offsets[bufIdx] = offset
		p.slots = append(p.slots, newSlot)
	}

	// Create handle with pool idx and buffer bit encoded
	h := NewHandleWithBuffer(bufIdx, p.idx, 0, slotIdx)

	// Track buffer reference
	buf.refCount.Add(1)

	// Index with key pointing to buffer memory (zero-copy)
	bufferKey := bytesToString(buf.data[offset : offset+uint32(len(data))])
	p.index[bufferKey] = h

	return h, nil
}

// Get returns the data associated with the handle.
// Returns a slice pointing into the pool's buffer (zero-copy).
// The returned slice is only valid while the handle is live.
// Returns error if handle is invalid, from wrong pool, or references dead slot.
func (p *Pool) Get(h Handle) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if err := p.validateHandle(h); err != nil {
		return nil, err
	}

	bufIdx := h.BufferBit()
	slotIdx := h.Slot()
	s := &p.slots[slotIdx]
	offset := s.offsets[bufIdx]
	return p.buffers[bufIdx].data[offset : offset+uint32(s.length)], nil
}

// Length returns the length of data associated with the handle.
// Returns error if handle is invalid, from wrong pool, or references dead slot.
func (p *Pool) Length(h Handle) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if err := p.validateHandle(h); err != nil {
		return 0, err
	}

	return int(p.slots[h.Slot()].length), nil
}

// Release decrements the reference count for the handle.
// When refCount reaches zero, the entry is marked dead and eligible for reclamation.
// Returns error if handle is invalid, from wrong pool, or slot out of bounds.
func (p *Pool) Release(h Handle) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.validateHandleForRelease(h); err != nil {
		return err
	}

	bufIdx := h.BufferBit()
	slotIdx := h.Slot()
	s := &p.slots[slotIdx]

	s.refCount--
	p.buffers[bufIdx].refCount.Add(-1)

	if s.refCount <= 0 {
		s.dead = true

		// Remove from index - handle may point to either buffer
		// Must delete to prevent stale entry causing wrong data after slot reuse
		buf := &p.buffers[bufIdx]
		if len(buf.data) > 0 {
			offset := s.offsets[bufIdx]
			bufferKey := bytesToString(buf.data[offset : offset+uint32(s.length)])
			delete(p.index, bufferKey)
		}

		// Add slot to free list for reuse
		p.freeSlots = append(p.freeSlots, slotIdx)
	}

	return nil
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

// InternWithError is like Intern but returns an error instead of panicking.
// Returns ErrPoolShutdown if pool is shutdown.
// Returns ErrDataTooLarge if data exceeds MaxDataLength (65535 bytes).
// Returns ErrPoolFull if pool has reached MaxSlots (16,777,215).
func (p *Pool) InternWithError(data []byte) (Handle, error) {
	return p.internLocked(data)
}

// AddRef increments the reference count for the handle.
// Use when sharing a handle between multiple owners.
// Returns error if handle is invalid or from wrong pool.
func (p *Pool) AddRef(h Handle) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.validateHandle(h); err != nil {
		return err
	}

	bufIdx := h.BufferBit()
	slotIdx := h.Slot()
	s := &p.slots[slotIdx]

	s.refCount++
	p.buffers[bufIdx].refCount.Add(1)

	return nil
}

// GetBySlot returns data for a normalized slot index.
// Auto-selects the correct buffer based on compaction state.
// Use when handles are stored normalized (slot only, no bufferBit).
func (p *Pool) GetBySlot(slotIdx uint32) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if slotIdx >= uint32(len(p.slots)) {
		return nil, ErrSlotOutOfBounds
	}

	s := &p.slots[slotIdx]
	if s.dead || s.refCount <= 0 {
		return nil, ErrSlotDead
	}

	// Determine correct buffer
	bufIdx := p.currentBit
	if p.state == PoolCompacting && slotIdx >= p.compactCursor {
		// Slot not yet migrated, use old buffer
		bufIdx = 1 - p.currentBit
	}

	offset := s.offsets[bufIdx]
	return p.buffers[bufIdx].data[offset : offset+uint32(s.length)], nil
}

// ReleaseBySlot decrements reference count for a normalized slot index.
// Auto-selects the correct buffer based on compaction state.
// Use when handles are stored normalized (slot only, no bufferBit).
func (p *Pool) ReleaseBySlot(slotIdx uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if slotIdx >= uint32(len(p.slots)) {
		return ErrSlotOutOfBounds
	}

	s := &p.slots[slotIdx]

	// Determine correct buffer
	bufIdx := p.currentBit
	if p.state == PoolCompacting && slotIdx >= p.compactCursor {
		bufIdx = 1 - p.currentBit
	}

	s.refCount--
	p.buffers[bufIdx].refCount.Add(-1)

	if s.refCount <= 0 {
		s.dead = true

		// Remove from index - must delete to prevent stale entry after slot reuse
		buf := &p.buffers[bufIdx]
		if len(buf.data) > 0 {
			offset := s.offsets[bufIdx]
			bufferKey := bytesToString(buf.data[offset : offset+uint32(s.length)])
			delete(p.index, bufferKey)
		}

		p.freeSlots = append(p.freeSlots, slotIdx)
	}

	return nil
}

// State returns the current compaction state.
func (p *Pool) State() PoolState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// StartCompaction begins incremental compaction.
// Allocates new buffer and sets state to PoolCompacting.
// Call MigrateBatch() repeatedly until it returns true.
func (p *Pool) StartCompaction() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == PoolCompacting {
		return // Already compacting
	}

	// Count live bytes for new buffer sizing
	var liveBytes int64
	for i := range p.slots {
		s := &p.slots[i]
		if !s.dead && s.refCount > 0 {
			liveBytes += int64(s.length)
		}
	}

	// Flip to new buffer
	oldBit := p.currentBit
	newBit := 1 - oldBit
	p.currentBit = newBit

	// Allocate new buffer with headroom
	newSize := max(liveBytes+liveBytes/4, 64)
	p.buffers[newBit].data = make([]byte, 0, newSize)
	p.buffers[newBit].pos = 0
	p.buffers[newBit].refCount.Store(0)

	p.state = PoolCompacting
	p.compactCursor = 0
	p.compactSlotCount = uint32(len(p.slots)) // Only migrate slots that existed at start
}

// MigrateBatch migrates a batch of slots to the new buffer.
// Returns true when migration is complete.
// Call repeatedly until it returns true, then old buffer will be freed
// when its refCount reaches 0.
func (p *Pool) MigrateBatch(batchSize int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != PoolCompacting {
		return true
	}

	oldBit := 1 - p.currentBit
	newBit := p.currentBit
	oldBuf := &p.buffers[oldBit]
	newBuf := &p.buffers[newBit]

	migrated := 0
	for p.compactCursor < p.compactSlotCount && migrated < batchSize {
		s := &p.slots[p.compactCursor]

		if !s.dead && s.refCount > 0 {
			// Copy data from old buffer to new buffer
			oldOffset := s.offsets[oldBit]
			oldData := oldBuf.data[oldOffset : oldOffset+uint32(s.length)]

			newOffset := uint32(newBuf.pos)
			newBuf.data = append(newBuf.data, oldData...)
			newBuf.pos += int(s.length)

			s.offsets[newBit] = newOffset

			// Update index with new handle
			oldKey := bytesToString(oldData)
			delete(p.index, oldKey)

			newKey := bytesToString(newBuf.data[newOffset : newOffset+uint32(s.length)])
			newHandle := NewHandleWithBuffer(newBit, p.idx, 0, p.compactCursor)
			p.index[newKey] = newHandle

			migrated++
		} else {
			// Clear dead slot data reference (so Metrics doesn't count as dead)
			s.offsets[oldBit] = 0
			s.offsets[newBit] = 0
			s.length = 0
		}

		p.compactCursor++
	}

	// Check if migration complete
	if p.compactCursor >= p.compactSlotCount {
		// All slots processed
		if oldBuf.refCount.Load() == 0 {
			p.finishCompaction()
		}
		return true
	}

	return false
}

// finishCompaction completes compaction by freeing old buffer.
// Called with lock held.
func (p *Pool) finishCompaction() {
	oldBit := 1 - p.currentBit
	p.buffers[oldBit].data = nil
	p.buffers[oldBit].pos = 0
	p.state = PoolNormal
}

// CheckOldBufferRelease checks if old buffer can be freed after compaction.
// Call periodically after MigrateBatch returns true.
func (p *Pool) CheckOldBufferRelease() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != PoolCompacting {
		return
	}

	oldBit := 1 - p.currentBit
	if p.buffers[oldBit].refCount.Load() == 0 {
		p.finishCompaction()
	}
}

// ensureCapacity ensures the current buffer can hold additional bytes.
// Called with lock held.
func (p *Pool) ensureCapacity(needed int) {
	buf := &p.buffers[p.currentBit]
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
	p.rebuildIndex()
}

// rebuildIndex recreates the index with keys pointing to current buffer.
// Called with lock held after buffer reallocation.
func (p *Pool) rebuildIndex() {
	bufIdx := p.currentBit
	buf := &p.buffers[bufIdx]

	// During compaction, preserve entries pointing to old buffer
	// (their keys still reference valid old buffer memory)
	var preserved map[string]Handle
	if p.state == PoolCompacting {
		oldBit := 1 - bufIdx
		preserved = make(map[string]Handle)
		for k, h := range p.index {
			if h.BufferBit() == oldBit {
				preserved[k] = h
			}
		}
	}

	p.index = make(map[string]Handle, len(p.slots))

	// Restore preserved old-buffer entries
	maps.Copy(p.index, preserved)

	// Rebuild entries for current buffer
	for i := range p.slots {
		s := &p.slots[i]
		if !s.dead && s.refCount > 0 {
			// During compaction, skip unmigrated slots - they're already
			// in preserved entries pointing to old buffer
			if p.state == PoolCompacting && uint32(i) >= p.compactCursor {
				continue
			}
			offset := s.offsets[bufIdx]
			key := bytesToString(buf.data[offset : offset+uint32(s.length)])
			p.index[key] = NewHandleWithBuffer(bufIdx, p.idx, 0, uint32(i))
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

// Metrics returns current pool statistics.
func (p *Pool) Metrics() Metrics {
	p.mu.RLock()
	defer p.mu.RUnlock()

	buf := &p.buffers[p.currentBit]

	var m Metrics
	m.TotalSlots = int32(len(p.slots))
	m.BufferSize = int64(len(buf.data))
	m.BufferCap = int64(cap(buf.data))
	m.InternTotal = p.internTotal.Load()
	m.InternHits = p.internHits.Load()

	for i := range p.slots {
		s := &p.slots[i]
		if !s.dead && s.refCount > 0 {
			m.LiveSlots++
			m.LiveBytes += int64(s.length)
		} else if s.length > 0 {
			// Dead slot with data still in buffer (not yet compacted)
			m.DeadSlots++
			m.DeadBytes += int64(s.length)
		}
		// Slots with length=0 are reclaimed/free, not counted as dead
	}

	return m
}

// Compact removes dead entries and reclaims buffer memory.
// Live handles remain valid after compaction.
// Note: This is stop-the-world compaction. Use StartCompaction/MigrateBatch for non-blocking.
// If incremental compaction is in progress, this is a no-op.
func (p *Pool) Compact() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Don't interfere with incremental compaction
	if p.state == PoolCompacting {
		return
	}

	bufIdx := p.currentBit
	buf := &p.buffers[bufIdx]

	// Count live bytes
	var liveBytes int
	for i := range p.slots {
		s := &p.slots[i]
		if !s.dead && s.refCount > 0 {
			liveBytes += int(s.length)
		}
	}

	// Nothing to compact if no dead entries
	if len(p.freeSlots) == 0 {
		return
	}

	// Create new buffer with only live data
	newData := make([]byte, 0, liveBytes+liveBytes/4) // 25% headroom
	newPos := 0

	// Copy live data to new buffer, update slot offsets
	for i := range p.slots {
		s := &p.slots[i]
		if !s.dead && s.refCount > 0 {
			// Copy data to new buffer
			oldData := buf.data[s.offsets[bufIdx] : s.offsets[bufIdx]+uint32(s.length)]
			newOffset := uint32(newPos)
			newData = append(newData, oldData...)
			newPos += int(s.length)

			// Update slot offset in current buffer
			s.offsets[bufIdx] = newOffset
		} else {
			// Clear dead slot data reference
			s.offsets[bufIdx] = 0
			s.length = 0
		}
	}

	// Update buffer
	buf.data = newData
	buf.pos = newPos

	// Rebuild index with new buffer pointers
	p.rebuildIndex()
}
