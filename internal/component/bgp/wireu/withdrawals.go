// Design: docs/architecture/wire/messages.md -- wire UPDATE lazy parsing
// RFC: rfc/short/rfc4271.md -- Section 4.3 Withdrawn Routes, Section 9.1.2 route removal
// Related: split.go -- SplitWireUpdate, which splits the same components for size
// Related: wellknown.go -- the RFC 1997 egress gate that refuses the announce half only

package wireu

import "encoding/binary"

// WithdrawalsOnly returns the part of an UPDATE that REMOVES routes: the
// Withdrawn Routes field and the MP_UNREACH_NLRI attribute (RFC 4760 Section 3).
// It returns nil when the UPDATE withdraws nothing.
//
// It exists because an egress prohibition can refuse the announcement half of an
// UPDATE while the withdrawal half must still go out. RFC 1997 forbids
// ADVERTISING a tagged route. It says nothing about taking a route back, and the
// two travel in one message whenever a peer mixes them. A destination that never
// receives the withdrawal keeps a prefix Ze can no longer remove until the
// session resets. That is a worse outcome than the leak the prohibition prevents
// (reactor/forward_wellknown.go).
//
// THE PATH ATTRIBUTES ARE DROPPED, and that is not a shortcut: a withdrawal
// needs none, and buildCombinedUpdates already omits them from every
// withdrawal-bearing message it emits (split.go, RFC 7606 Section 5.1). It also
// means the returned payload carries no COMMUNITIES attribute, so nothing
// downstream can read a prohibition off bytes that no longer announce anything.
//
// The input is returned UNCHANGED when it announces nothing, so a pure
// withdrawal keeps the caller's zero-copy forward. An allocation happens only for
// the mixed shape, which is the one RFC 7606 Section 5.1 tells a sender not to
// emit in the first place.
//
// An MP_UNREACH_NLRI carrying no prefix is left out. Its value is AFI/SAFI only,
// which is byte-identical to an RFC 4724 Section 2 End-of-RIB marker, and
// emitting one would end a restarting peer's deferral early -- the same reasoning
// buildCombinedUpdates records for the split path. No withdrawal information is
// lost, because there was none.
func WithdrawalsOnly(wu *WireUpdate) *WireUpdate {
	payload := wu.Payload()
	if len(payload) < updateLengthFieldsSize {
		return nil
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	withdrawnEnd := 2 + withdrawnLen
	if len(payload) < withdrawnEnd+2 {
		return nil
	}
	attrLen := int(binary.BigEndian.Uint16(payload[withdrawnEnd : withdrawnEnd+2]))
	attrStart := withdrawnEnd + 2
	attrEnd := attrStart + attrLen
	if len(payload) < attrEnd {
		return nil
	}

	ipv4Withdraws := payload[2:withdrawnEnd]
	ipv4NLRI := payload[attrEnd:]

	_, mpReaches, mpUnreaches, err := separateMPAttributes(payload[attrStart:attrEnd])
	if err != nil {
		return nil
	}

	withdraws := len(ipv4Withdraws) > 0
	for _, attr := range mpUnreaches {
		withdraws = withdraws || mpUnreachHasNLRI(attr)
	}
	if !withdraws {
		return nil
	}
	if len(ipv4NLRI) == 0 && len(mpReaches) == 0 {
		return wu
	}

	// Concatenated rather than taken as one slice. A second MP_UNREACH_NLRI is a
	// duplicate attribute RFC 7606 Section 3(d) already refused on the receive
	// path, so this loop runs once in every payload that reaches it. Taking only
	// the first would silently lose withdrawals if that refusal ever moved.
	var mpUnreach []byte
	for _, attr := range mpUnreaches {
		if mpUnreachHasNLRI(attr) {
			mpUnreach = append(mpUnreach, attr...)
		}
	}

	out := NewWireUpdate(buildUpdatePayload(ipv4Withdraws, nil, mpUnreach, nil, nil), wu.SourceCtxID())
	// The source peer travels with the part, exactly as SplitWireUpdate carries it
	// onto every chunk. The message id is left unset: this is a new message.
	out.SetSourceID(wu.SourceID())
	return out
}
