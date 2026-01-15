//go:build !debug

package pool

// validateHandle checks handle validity in release builds.
// Returns error if invalid.
func (p *Pool) validateHandle(h Handle) error {
	if !h.Valid() {
		return ErrInvalidHandle
	}

	if h.PoolIdx() != p.idx {
		return ErrWrongPool
	}

	slot := h.Slot()
	if int(slot) >= len(p.slots) {
		return ErrSlotOutOfBounds
	}

	if p.slots[slot].dead {
		return ErrSlotDead
	}

	return nil
}

// validateHandleForRelease checks handle validity for Release (slot can be dead).
func (p *Pool) validateHandleForRelease(h Handle) error {
	if !h.Valid() {
		return ErrInvalidHandle
	}

	if h.PoolIdx() != p.idx {
		return ErrWrongPool
	}

	slot := h.Slot()
	if int(slot) >= len(p.slots) {
		return ErrSlotOutOfBounds
	}

	return nil
}
