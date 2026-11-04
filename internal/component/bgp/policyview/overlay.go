// Design: docs/architecture/core-design.md -- The in-process BGP chain, failed-member atomicity.
// Related: view.go -- the cached slots and shared NLRI state overlays replace.

package policyview

import "math/bits"

// Overlay is reusable scratch for a member's tentative view changes. Its zero
// value is ready for BeginOverlay. NOT safe for concurrent use. It saves only
// slots a setter replaces, not a View or the borrowed wire bytes; the caller
// MUST separately preserve any owned buffers those slots reference.
// Storage is bounded by the same reader limit as View, with no per-route
// allocation or whole-cache copy.
type Overlay struct {
	slots     [slotsMax]slot
	saved     uint64
	nlri      shared
	nlriSaved bool
}

// BeginOverlay starts recording tentative setter changes into overlay. The
// caller MUST call EndOverlay before another BeginOverlay, Reset or Clear,
// and MUST NOT reuse overlay until EndOverlay. Lazy reads remain cached.
func (v *View) BeginOverlay(overlay *Overlay) {
	v.overlay = overlay
}

// EndOverlay ends the matching BeginOverlay, keeping setter changes when keep
// is true and restoring their prior answers otherwise. The caller MUST have
// called BeginOverlay, and MUST restore borrowed scratch bytes on rollback.
// Both outcomes release every reference the journal borrowed from the route.
func (v *View) EndOverlay(keep bool) {
	overlay := v.overlay
	for saved := overlay.saved; saved != 0; saved &= saved - 1 {
		index := bits.TrailingZeros64(saved)
		if !keep {
			v.slots[index] = overlay.slots[index]
		}
		overlay.slots[index] = slot{}
	}
	if overlay.nlriSaved && !keep {
		v.nlri = overlay.nlri
	}
	overlay.saved = 0
	overlay.nlri = shared{}
	overlay.nlriSaved = false
	v.overlay = nil
}

// saveOverlay records a slot before its first setter mutation in this member.
// A later setter may replace it again, but rollback still needs the old answer.
func (v *View) saveOverlay(index int) {
	overlay := v.overlay
	if overlay == nil {
		return
	}
	bit := uint64(1) << index
	if overlay.saved&bit != 0 {
		return
	}
	overlay.slots[index] = v.slots[index]
	overlay.saved |= bit
}
