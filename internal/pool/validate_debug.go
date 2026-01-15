//go:build debug

package pool

import "fmt"

// validateHandle checks handle validity in debug builds.
// Returns error if invalid, with detailed message for debugging.
func (p *Pool) validateHandle(h Handle) error {
	if !h.Valid() {
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

// validateHandleForRelease checks handle validity for Release (slot can be dead).
func (p *Pool) validateHandleForRelease(h Handle) error {
	if !h.Valid() {
		return fmt.Errorf("%w: handle=%d", ErrInvalidHandle, h)
	}

	if h.PoolIdx() != p.idx {
		return fmt.Errorf("%w: handle pool=%d, this pool=%d", ErrWrongPool, h.PoolIdx(), p.idx)
	}

	slot := h.Slot()
	if int(slot) >= len(p.slots) {
		return fmt.Errorf("%w: slot=%d, max=%d", ErrSlotOutOfBounds, slot, len(p.slots))
	}

	return nil
}
