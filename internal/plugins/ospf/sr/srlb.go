// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing shared control
// plane. Bounded SRLB local-label allocator (the LDP nextLabel/MaxLabel bounded
// 20-bit pool shape, seeded/capped by the configured SRLB range). Shared by both
// address families; used for Adj-SID and LAN-Adj-SID local labels.
// RFC: rfc/short/rfc8665.md (§3.3 SR Local Block; Adj-SIDs allocated from the SRLB)

package sr

// LabelAllocator hands out MPLS labels from the configured SR Local Block (SRLB).
// It mirrors the LDP bounded-pool allocator: a cursor walks the configured ranges
// in order, skips labels already in use, and wraps when it reaches the end, so a
// freed label is reused rather than leaked. It is not safe for concurrent use;
// callers serialize allocation on the engine goroutine.
type LabelAllocator struct {
	ranges   []LabelRange
	used     map[uint32]bool
	curRange int
	curLabel uint32
}

// NewLabelAllocator builds an allocator over the SRLB ranges. Zero-size ranges
// are dropped. The cursor starts at the first label of the first range.
func NewLabelAllocator(ranges []LabelRange) *LabelAllocator {
	kept := make([]LabelRange, 0, len(ranges))
	for _, r := range ranges {
		if r.Size > 0 {
			kept = append(kept, r)
		}
	}
	a := &LabelAllocator{ranges: kept, used: make(map[uint32]bool)}
	if len(kept) > 0 {
		a.curLabel = kept[0].Base
	}
	return a
}

// Capacity is the total number of labels the SRLB can hold.
func (a *LabelAllocator) Capacity() uint32 {
	var total uint32
	for _, r := range a.ranges {
		total += r.Size
	}
	return total
}

// InUse is the number of currently allocated labels.
func (a *LabelAllocator) InUse() int { return len(a.used) }

// inRange reports whether label falls inside any configured SRLB range.
func (a *LabelAllocator) inRange(label uint32) bool {
	for _, r := range a.ranges {
		if r.contains(label) {
			return true
		}
	}
	return false
}

// advance moves the cursor to the next label, wrapping across ranges back to the
// start when it runs off the end.
func (a *LabelAllocator) advance() {
	if len(a.ranges) == 0 {
		return
	}
	a.curLabel++
	if a.curLabel > a.ranges[a.curRange].Last() {
		a.curRange++
		if a.curRange >= len(a.ranges) {
			a.curRange = 0
		}
		a.curLabel = a.ranges[a.curRange].Base
	}
}

// Allocate returns the next free SRLB label. It returns false when the block is
// exhausted (every label in use). The bounded walk tries at most Capacity labels.
func (a *LabelAllocator) Allocate() (uint32, bool) {
	total := a.Capacity()
	if total == 0 || uint32(len(a.used)) >= total {
		return 0, false
	}
	for range total {
		label := a.curLabel
		a.advance()
		if !a.used[label] {
			a.used[label] = true
			return label, true
		}
	}
	return 0, false
}

// Reserve marks a specific label as used (e.g. a persistent Adj-SID). It returns
// false if the label is outside the SRLB or already allocated.
func (a *LabelAllocator) Reserve(label uint32) bool {
	if !a.inRange(label) || a.used[label] {
		return false
	}
	a.used[label] = true
	return true
}

// Free releases a previously allocated label back to the pool. Freeing a label
// that is not allocated is a no-op.
func (a *LabelAllocator) Free(label uint32) { delete(a.used, label) }
