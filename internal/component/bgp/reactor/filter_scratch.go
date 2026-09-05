// Design: docs/architecture/perf-round-3.md -- filter-delta allocation reduction
// Related: filter_delta.go -- every encoder carves its wire value bytes here
// Related: forward_build.go -- modBufPool, the pool this one mirrors
// Related: filterapi/editset.go -- EditSet.write reads every operation's Buf at rebuild time

package reactor

import (
	"sync"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// valueScratch capacities. Each one is the size at which the common case never
// reaches the grow path, and each grow past it is a correctness fallback rather
// than a normal path.
const (
	// valueScratchBytes covers one modify block's encoded attribute values. A
	// standard UPDATE body is 4096 - 19 octets, so a block whose values sum
	// past this is producing more bytes than the message it edits can carry.
	valueScratchBytes = 4096
	// valueScratchSegments covers the AS path segments one remove-private
	// rewrite reserves. A path carrying more than sixteen segments is already
	// far outside what a peer sends.
	valueScratchSegments = 16
	// valueScratchASNs covers one whole AS path segment: the segment count
	// field is one octet, so 255 ASNs is the most one segment can hold.
	valueScratchASNs = 256
	// valueScratchBytesMax bounds what a released scratch keeps. A value longer
	// than an UPDATE body cannot reach the wire, so one adversarial delta must
	// not pin a buffer of that size in the pool for the life of the process.
	valueScratchBytesMax = maxUpdateBody
	// valueScratchASNsMax and valueScratchSegmentsMax bound the retained AS path
	// arenas for the same reason. ParseASPath refuses a path above
	// MaxASPathTotalLength ASNs, so a rewrite of one cannot reserve more, and a
	// segment carries at least one ASN, so the same figure bounds the segments.
	valueScratchASNsMax     = attribute.MaxASPathTotalLength
	valueScratchSegmentsMax = attribute.MaxASPathTotalLength
)

// valueScratch is the arena one modify block carves its filter-delta values
// from. The block acquires it before the extractors run and releases it after
// buildModifiedPayload returns, which is exactly as long as the operations that
// point into it live.
//
// APPEND-ONLY WITHIN A BLOCK, AND THAT IS LOAD-BEARING. A carve advances the
// offset. Nothing rewinds it, and nothing reuses an earlier carve's region,
// until the scratch is released. Every carved slice is a window into one
// backing array and the rebuild holds all of them at once: an attribute handler
// records one fragment per operation while it plans, and EditSet.write reads
// ops[i].Buf for every fragment when it materializes the payload
// (filterapi/editset.go). A carve that rewound would hand two attributes the
// same bytes, and nothing downstream could tell.
//
// A carve past the current capacity grows the arena and ORPHANS the windows
// already handed out onto the old array. They stay valid to read, because
// nothing writes to that array again, so a grow costs an allocation rather than
// correctness.
//
// NOT safe for concurrent use. One block, one scratch, one goroutine.
type valueScratch struct {
	buf  []byte
	segs []attribute.ASPathSegment
	asns []uint32
}

// carveBytes reserves n zeroed bytes at the end of the byte arena. The result
// has length AND capacity n, so an append by the caller cannot reach the region
// the next carve hands out. It returns nil for a non-positive n, which is the
// zero-length attribute value case where no bytes are owed.
func (s *valueScratch) carveBytes(n int) []byte {
	if n <= 0 {
		return nil
	}

	var window []byte
	s.buf, window = carveArena(s.buf, n)

	// A released scratch keeps the bytes the previous block wrote, and an
	// encoder that fails part way through leaves the rest of its window
	// untouched, so the window is zeroed before it is handed out.
	clear(window)
	return window
}

// carveSegments reserves room for n AS path segments and returns an EMPTY slice
// over it, capped at n. The caller appends up to n segments; a further append
// leaves the arena and allocates, which is correct and merely slower.
func (s *valueScratch) carveSegments(n int) []attribute.ASPathSegment {
	if n <= 0 {
		return nil
	}

	var window []attribute.ASPathSegment
	s.segs, window = carveArena(s.segs, n)
	return window[:0]
}

// carveASNs reserves room for n ASNs and returns an EMPTY slice over it, capped
// at n. Same contract as carveSegments.
func (s *valueScratch) carveASNs(n int) []uint32 {
	if n <= 0 {
		return nil
	}

	var window []uint32
	s.asns, window = carveArena(s.asns, n)
	return window[:0]
}

// carveArena reserves n elements at the end of one arena. It returns the arena
// with its length advanced past the reservation, and the window over it.
//
// It is the ONE implementation of the append-only rule the three carves above
// share, because three copies of this arithmetic would be three chances for one
// of them to rewind. A reservation past the capacity grows the arena and
// orphans the windows already handed out onto the old array, which stay valid
// to read because nothing writes to that array again.
func carveArena[T any](arena []T, n int) (grown, window []T) {
	off := len(arena)
	end := off + n
	if end > cap(arena) {
		next := make([]T, off, max(cap(arena)*2, end))
		copy(next, arena)
		arena = next
	}
	arena = arena[:end]
	return arena, arena[off:end:end]
}

// valueScratchPool supplies one scratch per modify block. A pool rather than a
// field on the accumulator, for the reason spanIndexPool gives: the block is
// reached from call sites with different owners, and filterapi is a near-leaf
// package that cannot name attribute.ASPathSegment.
var valueScratchPool = sync.Pool{
	New: func() any { return newValueScratch() },
}

// newValueScratch builds one scratch with every arena at its common-case size.
func newValueScratch() *valueScratch {
	return &valueScratch{
		buf:  make([]byte, 0, valueScratchBytes),
		segs: make([]attribute.ASPathSegment, 0, valueScratchSegments),
		asns: make([]uint32, 0, valueScratchASNs),
	}
}

// acquireValueScratch takes the scratch for one modify block. The caller MUST
// call releaseValueScratch once the block is done, and MUST NOT read a carved
// window after that: the next block writes over it.
func acquireValueScratch() *valueScratch {
	s, ok := valueScratchPool.Get().(*valueScratch)
	if !ok {
		return newValueScratch()
	}
	return s
}

// releaseValueScratch returns the scratch to the pool. It MUST be called after
// buildModifiedPayload has consumed the operations, because every operation's
// Buf is a window into this arena until then.
func releaseValueScratch(s *valueScratch) {
	s.buf = s.buf[:0]
	if cap(s.buf) > valueScratchBytesMax {
		s.buf = make([]byte, 0, valueScratchBytes)
	}

	// Segments MUST be re-zeroed rather than merely re-sliced: a segment holds
	// an ASNs slice, so a stale header in the backing array keeps the array it
	// points at alive for the whole of the next block. The clear runs over the
	// length, which is bounded by the reservations one block made.
	clear(s.segs)
	s.segs = s.segs[:0]
	if cap(s.segs) > valueScratchSegmentsMax {
		s.segs = make([]attribute.ASPathSegment, 0, valueScratchSegments)
	}

	s.asns = s.asns[:0]
	if cap(s.asns) > valueScratchASNsMax {
		s.asns = make([]uint32, 0, valueScratchASNs)
	}

	valueScratchPool.Put(s)
}
