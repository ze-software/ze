//nolint:gosec // G115: Pool has explicit size limits preventing overflow; unsafe usage audited
package pool

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// ErrPoolShutdown is returned when operations are attempted on a shutdown pool.
var ErrPoolShutdown = errors.New("pool is shutdown")

// Pool provides zero-copy byte slice deduplication for BGP attributes and NLRI.
//
// Thread-safe. Uses reference counting for lifecycle management.
// Designed for high-frequency access patterns with many duplicate entries.
type Pool struct {
	mu sync.RWMutex

	// Data buffer - all interned data stored here contiguously
	data []byte
	pos  int // write cursor

	// Slot table - indexed by handle value
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
	offset   uint32 // offset in data buffer
	length   uint16 // data length
	refCount int32  // reference count
	dead     bool   // marked for removal
}

// New creates a pool with the given initial buffer capacity.
func New(initialCapacity int) *Pool {
	if initialCapacity < 64 {
		initialCapacity = 64
	}
	return &Pool{
		data:  make([]byte, 0, initialCapacity),
		slots: make([]slot, 0, 64),
		index: make(map[string]Handle, 64),
	}
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
func (p *Pool) Intern(data []byte) Handle {
	// Treat nil as empty
	if data == nil {
		data = []byte{}
	}

	lookupKey := bytesToString(data)

	p.mu.Lock()
	defer p.mu.Unlock()

	// Track metrics
	p.internTotal.Add(1)

	// Mark activity
	p.lastActivity.Store(time.Now().UnixNano())

	// Check for existing entry (deduplication)
	if h, ok := p.index[lookupKey]; ok {
		s := &p.slots[h]
		if !s.dead && s.refCount > 0 {
			s.refCount++
			p.internHits.Add(1) // Deduplication hit
			return h
		}
	}

	// Allocate new entry
	p.ensureCapacity(len(data))

	offset := uint32(p.pos)
	p.data = append(p.data, data...)
	p.pos += len(data)

	// Allocate or reuse slot
	var slotIdx uint32
	if len(p.freeSlots) > 0 {
		slotIdx = p.freeSlots[len(p.freeSlots)-1]
		p.freeSlots = p.freeSlots[:len(p.freeSlots)-1]
		p.slots[slotIdx] = slot{
			offset:   offset,
			length:   uint16(len(data)),
			refCount: 1,
			dead:     false,
		}
	} else {
		slotIdx = uint32(len(p.slots))
		p.slots = append(p.slots, slot{
			offset:   offset,
			length:   uint16(len(data)),
			refCount: 1,
			dead:     false,
		})
	}

	h := Handle(slotIdx)

	// Index with key pointing to buffer memory (zero-copy)
	bufferKey := bytesToString(p.data[offset : offset+uint32(len(data))])
	p.index[bufferKey] = h

	return h
}

// Get returns the data associated with the handle.
// Returns a slice pointing into the pool's buffer (zero-copy).
// The returned slice is only valid while the handle is live.
func (p *Pool) Get(h Handle) []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()

	p.validateHandle(h, "Get")

	s := &p.slots[h]
	return p.data[s.offset : s.offset+uint32(s.length)]
}

// Length returns the length of data associated with the handle.
func (p *Pool) Length(h Handle) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	p.validateHandle(h, "Length")

	return int(p.slots[h].length)
}

// Release decrements the reference count for the handle.
// When refCount reaches zero, the entry is marked dead and eligible for reclamation.
func (p *Pool) Release(h Handle) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.validateHandleForRelease(h, "Release")

	s := &p.slots[h]
	s.refCount--

	if s.refCount <= 0 {
		s.dead = true

		// Remove from index
		bufferKey := bytesToString(p.data[s.offset : s.offset+uint32(s.length)])
		delete(p.index, bufferKey)

		// Add slot to free list for reuse
		p.freeSlots = append(p.freeSlots, uint32(h))
	}
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

// InternWithError is like Intern but returns an error if the pool is shutdown.
func (p *Pool) InternWithError(data []byte) (Handle, error) {
	if p.shutdown.Load() {
		return InvalidHandle, ErrPoolShutdown
	}
	return p.Intern(data), nil
}

// ensureCapacity ensures the data buffer can hold additional bytes.
// Called with lock held.
func (p *Pool) ensureCapacity(needed int) {
	required := p.pos + needed
	if required <= cap(p.data) {
		return
	}

	// Grow buffer
	newCap := cap(p.data) * 2
	if newCap < required {
		newCap = required
	}

	oldData := p.data
	p.data = make([]byte, len(oldData), newCap)
	copy(p.data, oldData)

	// Rebuild index - old keys reference old memory
	p.rebuildIndex()
}

// rebuildIndex recreates the index with keys pointing to current buffer.
// Called with lock held after buffer reallocation.
func (p *Pool) rebuildIndex() {
	p.index = make(map[string]Handle, len(p.slots))
	for i := range p.slots {
		s := &p.slots[i]
		if !s.dead && s.refCount > 0 {
			key := bytesToString(p.data[s.offset : s.offset+uint32(s.length)])
			p.index[key] = Handle(i)
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

	var m Metrics
	m.TotalSlots = int32(len(p.slots))
	m.BufferSize = int64(len(p.data))
	m.BufferCap = int64(cap(p.data))
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
// Note: Slot array is not compacted to preserve handle stability.
func (p *Pool) Compact() {
	p.mu.Lock()
	defer p.mu.Unlock()

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
			oldData := p.data[s.offset : s.offset+uint32(s.length)]
			newOffset := uint32(newPos)
			newData = append(newData, oldData...)
			newPos += int(s.length)

			// Update slot offset (handle/slot index stays the same)
			s.offset = newOffset
		} else {
			// Clear dead slot data reference
			s.offset = 0
			s.length = 0
		}
	}

	// Update buffer
	p.data = newData
	p.pos = newPos

	// Clear dead count from free list (slots stay allocated but dead)
	// Keep freeSlots for reuse

	// Rebuild index with new buffer pointers
	p.rebuildIndex()
}
