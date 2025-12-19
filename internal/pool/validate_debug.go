//go:build debug

package pool

import "fmt"

// validateHandle checks handle validity in debug builds.
// Panics with descriptive message if invalid.
func (p *Pool) validateHandle(h Handle, op string) {
	if !h.Valid() {
		panic(fmt.Sprintf("pool: %s called with InvalidHandle", op))
	}

	if int(h) >= len(p.slots) {
		panic(fmt.Sprintf("pool: %s called with out-of-bounds handle %d (slots: %d)", op, h, len(p.slots)))
	}

	s := &p.slots[h]
	if s.dead {
		panic(fmt.Sprintf("pool: %s called with dead handle %d", op, h))
	}
}

// validateHandleForRelease checks handle validity for Release (slot doesn't need to be alive).
func (p *Pool) validateHandleForRelease(h Handle, op string) {
	if !h.Valid() {
		panic(fmt.Sprintf("pool: %s called with InvalidHandle", op))
	}

	if int(h) >= len(p.slots) {
		panic(fmt.Sprintf("pool: %s called with out-of-bounds handle %d (slots: %d)", op, h, len(p.slots)))
	}
}
