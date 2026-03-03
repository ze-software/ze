//go:build debug

// Design: docs/architecture/pool-architecture.md — attribute and NLRI pools

package attrpool

import "fmt"

// validateHandle checks handle validity in debug builds.
// Returns error if invalid, with detailed message for debugging.
func (p *Pool) validateHandle(h Handle) error {
	if !h.IsValid() {
		return fmt.Errorf("%w: handle=%d", ErrInvalidHandle, h)
	}

	if h.PoolIdx() != p.idx {
		return fmt.Errorf("%w: handle pool=%d, this pool=%d", ErrWrongPool, h.PoolIdx(), p.idx)
	}

	slot := h.Slot()
	if int(slot) >= len(p.slots) {
		return fmt.Errorf("%w: slot=%d, max=%d", ErrSlotOutOfBounds, slot, len(p.slots))
	}

	s := &p.slots[slot]
	if s.dead {
		return fmt.Errorf("%w: slot=%d", ErrSlotDead, slot)
	}

	return nil
}

// validateHandleForRelease checks handle validity for Release.
// Returns ErrSlotDead if already released (prevents double-release corruption).
func (p *Pool) validateHandleForRelease(h Handle) error {
	if !h.IsValid() {
		return fmt.Errorf("%w: handle=%d", ErrInvalidHandle, h)
	}

	if h.PoolIdx() != p.idx {
		return fmt.Errorf("%w: handle pool=%d, this pool=%d", ErrWrongPool, h.PoolIdx(), p.idx)
	}

	slot := h.Slot()
	if int(slot) >= len(p.slots) {
		return fmt.Errorf("%w: slot=%d, max=%d", ErrSlotOutOfBounds, slot, len(p.slots))
	}

	if p.slots[slot].dead {
		return fmt.Errorf("%w: slot=%d (double-release)", ErrSlotDead, slot)
	}

	return nil
}
