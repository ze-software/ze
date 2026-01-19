package pool

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// PoolConfig configures pool behavior.
type PoolConfig struct {
	InitialBufferSize int // Initial buffer capacity in bytes
	ExpectedEntries   int // Expected number of unique entries
}

// PoolState tracks pool lifecycle.
type PoolState int

const (
	// PoolNormal is the normal operating state.
	PoolNormal PoolState = iota
	// PoolCompacting indicates compaction is in progress.
	PoolCompacting
)

// Slot represents a single entry in the pool.
type Slot struct {
	offsets  [2]uint32 // Offset in each buffer (both valid during compaction)
	length   uint16    // Data length
	refCount int32     // Reference count
	dead     bool      // Marked for removal
}

// buffer holds pooled data.
type buffer struct {
	data     []byte
	pos      int          // Write cursor
	refCount atomic.Int32 // Handles pointing here
}

// Pool provides deduplicated byte storage with reference counting.
//
// Thread-safe for concurrent access. Uses double-buffer design for
// non-blocking compaction.
//
// See .claude/zebgp/POOL_ARCHITECTURE.md for design details.
type Pool struct {
	mu sync.RWMutex

	// Double buffer - alternates between compaction cycles
	buffers    [2]buffer
	currentBit uint32 // 0 or 1 - which buffer is current

	// Slot table - indexed by handle.SlotIndex()
	slots    []Slot
	freeList []uint32 // Recycled slot indices

	// Dedup index: data content → Handle
	// Keys use unsafe.String pointing into buffer (zero-copy)
	index map[string]Handle

	// Compaction state (Phase 5)
	_state         PoolState // nolint:unused // Phase 5: compaction
	_compactCursor uint32    // nolint:unused // Phase 5: compaction

	// Metrics
	liveBytes int64
	liveCount int32
	deadCount int32

	// Configuration
	config PoolConfig
}

// NewPool creates a new pool with the given configuration.
func NewPool(cfg PoolConfig) *Pool {
	if cfg.InitialBufferSize <= 0 {
		cfg.InitialBufferSize = 1 << 16 // 64KB default
	}
	if cfg.ExpectedEntries <= 0 {
		cfg.ExpectedEntries = 1000
	}

	p := &Pool{
		config:     cfg,
		currentBit: 0,
		index:      make(map[string]Handle, cfg.ExpectedEntries),
		slots:      make([]Slot, 0, cfg.ExpectedEntries),
	}

	// Initialize first buffer
	p.buffers[0].data = make([]byte, cfg.InitialBufferSize)

	return p
}

// Intern stores data and returns a handle.
//
// If identical data already exists, returns the existing handle and
// increments its reference count. Otherwise allocates new storage.
//
// The returned handle starts with refCount=1.
func (p *Pool) Intern(data []byte) Handle {
	// Create lookup key from input data
	lookupKey := bytesToString(data)

	p.mu.Lock()
	defer p.mu.Unlock()

	// Check for existing (deduplication)
	if h, ok := p.index[lookupKey]; ok {
		slot := &p.slots[h.SlotIndex()]
		if !slot.dead && slot.refCount > 0 {
			slot.refCount++
			p.buffers[h.BufferBit()].refCount.Add(1)
			return h
		}
		// Entry is dead or has zero refs - fall through to create new
	}

	// Allocate new entry in current buffer
	bufIdx := p.currentBit
	buf := &p.buffers[bufIdx]

	p.ensureCapacity(bufIdx, len(data))
	offset := uint32(buf.pos) //nolint:gosec // G115: buf.pos bounded by buffer capacity
	copy(buf.data[buf.pos:], data)
	buf.pos += len(data)

	// Allocate slot
	slotIdx := p.allocSlot()
	slot := &p.slots[slotIdx]
	slot.offsets[bufIdx] = offset
	slot.length = uint16(len(data)) //nolint:gosec // G115: length bounded by buffer
	slot.refCount = 1
	slot.dead = false

	// Create handle with current buffer bit
	h := MakeHandle(slotIdx, bufIdx)

	// Track buffer reference
	buf.refCount.Add(1)

	// Index with key pointing to buffer memory (zero-copy)
	bufferKey := bytesToString(buf.data[offset : offset+uint32(len(data))]) //nolint:gosec // G115: len bounded
	p.index[bufferKey] = h

	p.liveBytes += int64(len(data))
	p.liveCount++

	return h
}

// Get returns the data for handle h.
//
// The returned slice references pool-owned memory. Do not modify.
// The slice is valid as long as the handle has positive refCount.
func (p *Pool) Get(h Handle) []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()

	bufIdx := h.BufferBit()
	slot := &p.slots[h.SlotIndex()]

	offset := slot.offsets[bufIdx]
	return p.buffers[bufIdx].data[offset : offset+uint32(slot.length)]
}

// Length returns the byte length of data at handle h.
func (p *Pool) Length(h Handle) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return int(p.slots[h.SlotIndex()].length)
}

// AddRef increments the reference count for handle h.
//
// Use when sharing a handle across multiple owners.
func (p *Pool) AddRef(h Handle) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.slots[h.SlotIndex()].refCount++
	p.buffers[h.BufferBit()].refCount.Add(1)
}

// Release decrements the reference count for handle h.
//
// When refCount reaches 0, the slot is marked dead and may be
// reclaimed during compaction.
func (p *Pool) Release(h Handle) {
	p.mu.Lock()
	defer p.mu.Unlock()

	bufIdx := h.BufferBit()
	slotIdx := h.SlotIndex()
	slot := &p.slots[slotIdx]

	slot.refCount--
	p.buffers[bufIdx].refCount.Add(-1)

	if slot.refCount <= 0 {
		slot.dead = true
		p.deadCount++
		p.liveCount--
		p.liveBytes -= int64(slot.length)

		// Remove from index if this is the current buffer
		if bufIdx == p.currentBit {
			offset := slot.offsets[bufIdx]
			bufferKey := bytesToString(p.buffers[bufIdx].data[offset : offset+uint32(slot.length)])
			delete(p.index, bufferKey)
		}

		// Add to freelist for slot reuse
		p.freeList = append(p.freeList, slotIdx)
	}
}

// allocSlot returns a slot index (reusing freed slots when available).
// Caller must hold write lock.
func (p *Pool) allocSlot() uint32 {
	if len(p.freeList) > 0 {
		idx := p.freeList[len(p.freeList)-1]
		p.freeList = p.freeList[:len(p.freeList)-1]
		return idx
	}

	// Allocate new slot
	idx := uint32(len(p.slots)) //nolint:gosec // G115: slots bounded by uint32 max
	p.slots = append(p.slots, Slot{})
	return idx
}

// ensureCapacity ensures buffer has room for n more bytes.
// Grows buffer if needed, preserving existing data and rebuilding index.
// Caller must hold write lock.
func (p *Pool) ensureCapacity(bufIdx uint32, needed int) {
	buf := &p.buffers[bufIdx]
	required := buf.pos + needed

	if required <= cap(buf.data) {
		// Have capacity, extend length if needed
		if required > len(buf.data) {
			buf.data = buf.data[:required]
		}
		return
	}

	// Need to grow - allocate new and copy existing data
	newCap := cap(buf.data) * 2
	if newCap < required {
		newCap = required
	}

	newData := make([]byte, newCap)
	copy(newData, buf.data[:buf.pos])
	buf.data = newData

	// Rebuild index entries pointing to this buffer
	p.rebuildIndexForBuffer(bufIdx)
}

// rebuildIndexForBuffer recreates index entries after buffer reallocation.
// Keys must point into new buffer memory to allow old buffer GC.
// Caller must hold write lock.
func (p *Pool) rebuildIndexForBuffer(bufIdx uint32) {
	buf := &p.buffers[bufIdx]

	// Rebuild entire index from live slots
	p.index = make(map[string]Handle, len(p.slots))
	for i := range p.slots {
		slot := &p.slots[i]
		if !slot.dead && slot.refCount > 0 {
			// Create handle and key for current buffer
			h := MakeHandle(uint32(i), p.currentBit) //nolint:gosec // G115: i bounded by slots len
			offset := slot.offsets[p.currentBit]
			key := bytesToString(buf.data[offset : offset+uint32(slot.length)])
			p.index[key] = h
		}
	}
}

// bytesToString converts []byte to string without allocation.
// The string references the same memory as the slice.
// WARNING: The string is only valid while the slice memory is valid.
//
//nolint:gosec // G103: Intentional use of unsafe for zero-copy string keys
func bytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}
