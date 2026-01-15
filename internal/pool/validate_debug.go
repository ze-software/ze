//go:build debug

package pool

import "fmt"

// validateHandle checks handle validity in debug builds.
// Panics with descriptive message if invalid.
func (p *Pool) validateHandle(h Handle, op string) {
	if !h.Valid() {
		panic(fmt.Sprintf("pool: %s called with InvalidHandle", op))
	}

	// Verify handle belongs to this pool
	if h.PoolIdx() != p.idx {
		panic(fmt.Sprintf("pool: %s called with handle from pool %d on pool %d", op, h.PoolIdx(), p.idx))
	}

	slot := h.Slot()
	if int(slot) >= len(p.slots) {
		panic(fmt.Sprintf("pool: %s called with out-of-bounds slot %d (slots: %d)", op, slot, len(p.slots)))
	}

	s := &p.slots[slot]
	if s.dead {
		panic(fmt.Sprintf("pool: %s called with dead handle %d (slot %d)", op, h, slot))
	}
}

// validateHandleForRelease checks handle validity for Release (slot doesn't need to be alive).
func (p *Pool) validateHandleForRelease(h Handle, op string) {
	if !h.Valid() {
		panic(fmt.Sprintf("pool: %s called with InvalidHandle", op))
	}

	// Verify handle belongs to this pool
	if h.PoolIdx() != p.idx {
		panic(fmt.Sprintf("pool: %s called with handle from pool %d on pool %d", op, h.PoolIdx(), p.idx))
	}

	slot := h.Slot()
	if int(slot) >= len(p.slots) {
		panic(fmt.Sprintf("pool: %s called with out-of-bounds slot %d (slots: %d)", op, slot, len(p.slots)))
	}
}
