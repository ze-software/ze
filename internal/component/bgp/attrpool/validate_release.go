//go:build !debug

// Design: docs/architecture/pool-architecture.md — attribute and NLRI pools

package attrpool

// validateHandle checks handle validity in release builds.
// Operates within a single shard; the handle's per-shard slot indexes s.slots.
// Returns error if invalid.
func (s *shard) validateHandle(h Handle) error {
	if !h.IsValid() {
		return ErrInvalidHandle
	}

	if h.PoolIdx() != s.idx {
		return ErrWrongPool
	}

	slot := h.shardSlot()
	if int(slot) >= len(s.slots) {
		return ErrSlotOutOfBounds
	}

	if s.slots[slot].dead {
		return ErrSlotDead
	}

	return nil
}

// validateHandleForRelease checks handle validity for Release.
// Returns ErrSlotDead if already released (prevents double-release corruption).
func (s *shard) validateHandleForRelease(h Handle) error {
	if !h.IsValid() {
		return ErrInvalidHandle
	}

	if h.PoolIdx() != s.idx {
		return ErrWrongPool
	}

	slot := h.shardSlot()
	if int(slot) >= len(s.slots) {
		return ErrSlotOutOfBounds
	}

	if s.slots[slot].dead {
		return ErrSlotDead
	}

	return nil
}
