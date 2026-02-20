//go:build !debug

// Design: docs/architecture/pool-architecture.md — attribute and NLRI pools

package pool

// validateHandle checks handle validity in release builds.
// Returns error if invalid.
func (p *Pool) validateHandle(h Handle) error {
	if !h.IsValid() {
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

// validateHandleForRelease checks handle validity for Release.
// Returns ErrSlotDead if already released (prevents double-release corruption).
func (p *Pool) validateHandleForRelease(h Handle) error {
	if !h.IsValid() {
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
