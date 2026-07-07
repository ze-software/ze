//go:build debug

// Design: docs/architecture/pool-architecture.md — attribute and NLRI pools

package attrpool

import "fmt"

// slotReuseEnabled controls whether intern reuses freed slots. Debug builds
// disable reuse so a released slot stays dead: a stale Handle to it then trips
// ErrSlotDead (the ABA guard) instead of silently resolving to the next
// occupant's bytes. The Handle is fully packed (no bit for a generation tag),
// so refusing reuse is the only zero-ABI way to detect stale-after-reuse.
// See docs/architecture/memory/lifetime-contracts.md.
const slotReuseEnabled = false

// validateHandle checks handle validity in debug builds.
// Operates within a single shard; the handle's per-shard slot indexes s.slots.
// Returns error if invalid, with detailed message for debugging.
func (s *shard) validateHandle(h Handle) error {
	if !h.IsValid() {
		return fmt.Errorf("%w: handle=%d", ErrInvalidHandle, h)
	}

	if h.PoolIdx() != s.idx {
		return fmt.Errorf("%w: handle pool=%d, this pool=%d", ErrWrongPool, h.PoolIdx(), s.idx)
	}

	slot := h.shardSlot()
	if int(slot) >= len(s.slots) {
		return fmt.Errorf("%w: slot=%d, max=%d", ErrSlotOutOfBounds, slot, len(s.slots))
	}

	sl := &s.slots[slot]
	if sl.dead {
		return fmt.Errorf("%w: slot=%d", ErrSlotDead, slot)
	}

	return nil
}

// validateHandleForRelease checks handle validity for Release.
// Returns ErrSlotDead if already released (prevents double-release corruption).
func (s *shard) validateHandleForRelease(h Handle) error {
	if !h.IsValid() {
		return fmt.Errorf("%w: handle=%d", ErrInvalidHandle, h)
	}

	if h.PoolIdx() != s.idx {
		return fmt.Errorf("%w: handle pool=%d, this pool=%d", ErrWrongPool, h.PoolIdx(), s.idx)
	}

	slot := h.shardSlot()
	if int(slot) >= len(s.slots) {
		return fmt.Errorf("%w: slot=%d, max=%d", ErrSlotOutOfBounds, slot, len(s.slots))
	}

	if s.slots[slot].dead {
		return fmt.Errorf("%w: slot=%d (double-release)", ErrSlotDead, slot)
	}

	return nil
}
