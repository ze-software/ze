// Design: docs/architecture/core-design.md -- The in-process BGP chain, failed-member atomicity.
// Related: chain.go -- Chain.run keeps a member tentative until its sets fold.
// Related: ../policyview/overlay.go -- the journal of replaced view answers.

package policychain

import (
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// overlayState preserves only owned scratch that a tentative overlay changes.
// List and prefix bytes need no copy: each writer uses the inactive ring slot,
// and restoring its index makes every retry use that same inactive slot. The
// scalar attributes have only one slot, so their bounded bytes are saved too.
// Prefix received state must follow the accepted view, not a failed candidate.
// The zero value is ready for begin; one RunSets owns it, without concurrency.
type overlayState struct {
	view                                                policyview.Overlay
	origin                                              [1]byte
	nextHop                                             [4]byte
	aigp                                                [attribute.AIGPWireLen]byte
	communityNext, largeNext, extNext, bodyNext, mpNext uint8
	received                                            policyview.Value
	receivedSet                                         bool
	prefixChanged                                       bool
}

// begin MUST be paired with end before the member finishes or scratch is reused.
func (o *overlayState) begin(view *policyview.View, sets *RunSets) {
	o.origin, o.nextHop, o.aigp = sets.attributes.origin, sets.attributes.nextHop, sets.attributes.aigp
	o.communityNext = sets.communityNext
	o.largeNext, o.extNext = sets.attributes.largeNext, sets.attributes.extNext
	o.bodyNext, o.mpNext = sets.prefix.bodyNext, sets.prefix.mpNext
	o.prefixChanged = sets.member.written&(setKindPrefix.bit()|setKindFamily.bit()) != 0
	if o.prefixChanged {
		o.received, o.receivedSet = sets.prefix.received, sets.prefix.receivedSet
	}
	view.BeginOverlay(&o.view)
}

// end MUST follow begin. A refusal restores ownership positions as well as
// cached answers: restoring slice headers alone would let a repeated failure
// overwrite the buffers the accepted view still borrows.
func (o *overlayState) end(view *policyview.View, sets *RunSets, keep bool) {
	view.EndOverlay(keep)
	if !keep {
		sets.attributes.origin, sets.attributes.nextHop, sets.attributes.aigp = o.origin, o.nextHop, o.aigp
		sets.communityNext = o.communityNext
		sets.attributes.largeNext, sets.attributes.extNext = o.largeNext, o.extNext
		sets.prefix.bodyNext, sets.prefix.mpNext = o.bodyNext, o.mpNext
		if o.prefixChanged {
			sets.prefix.received, sets.prefix.receivedSet = o.received, o.receivedSet
		}
	}
	o.received = policyview.Value{}
}
