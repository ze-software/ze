package context

import (
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/capability"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
)

// addPathCheckFunc is a function type for checking ADD-PATH mode.
type addPathCheckFunc func(mode capability.AddPathMode) bool

// canReceive returns true if the mode includes Receive (Receive or Both).
// RFC 7911: We can receive if mode is Receive or Both.
func canReceive(mode capability.AddPathMode) bool {
	return mode == capability.AddPathReceive || mode == capability.AddPathBoth
}

// canSend returns true if the mode includes Send (Send or Both).
// RFC 7911: We can send if mode is Send or Both.
func canSend(mode capability.AddPathMode) bool {
	return mode == capability.AddPathSend || mode == capability.AddPathBoth
}

// fromNegotiated creates an EncodingContext from capability negotiation.
// The addPathCheck function determines which ADD-PATH modes are relevant.
//
// Note: For new code, prefer FromNegotiatedRecvWire/FromNegotiatedSendWire
// which use the composite sub-components for zero duplication.
func fromNegotiated(neg *capability.Negotiated, addPathCheck addPathCheckFunc) *EncodingContext {
	if neg == nil {
		return nil
	}

	ctx := &EncodingContext{
		ASN4:            neg.ASN4,
		AddPath:         make(map[nlri.Family]bool),
		ExtendedNextHop: make(map[nlri.Family]nlri.AFI),
		IsIBGP:          neg.LocalASN == neg.PeerASN,
		LocalAS:         neg.LocalASN,
		PeerAS:          neg.PeerASN,
	}

	// RFC 7911: ADD-PATH per family
	for _, f := range neg.Families() {
		mode := neg.AddPathMode(f)
		if addPathCheck(mode) {
			// f is capability.Family which is now nlri.Family (type alias)
			ctx.AddPath[f] = true
		}
	}

	// RFC 8950: Extended next-hop - store the next-hop AFI
	for _, f := range neg.Families() {
		nhAFI := neg.ExtendedNextHopAFI(f)
		if nhAFI != 0 {
			ctx.ExtendedNextHop[f] = nhAFI
		}
	}

	return ctx
}

// FromNegotiatedRecv creates a receive context from capability negotiation.
// The receive context is used when parsing routes FROM the peer.
//
// ADD-PATH: We need path IDs if we can receive them (mode = Receive or Both).
// RFC 7911: The negotiated mode indicates what we are allowed to do.
//
// Note: For new code, prefer FromNegotiatedRecvWire which uses composite sub-components.
func FromNegotiatedRecv(neg *capability.Negotiated) *EncodingContext {
	return fromNegotiated(neg, canReceive)
}

// FromNegotiatedSend creates a send context from capability negotiation.
// The send context is used when encoding routes TO the peer.
//
// ADD-PATH: We include path IDs if we can send them (mode = Send or Both).
// RFC 7911: The negotiated mode indicates what we are allowed to do.
//
// Note: For new code, prefer FromNegotiatedSendWire which uses composite sub-components.
func FromNegotiatedSend(neg *capability.Negotiated) *EncodingContext {
	return fromNegotiated(neg, canSend)
}

// FromNegotiatedRecvWire creates a receive WireContext from capability negotiation.
// The receive context is used when parsing routes FROM the peer.
//
// Uses the composite sub-components from Negotiated (Identity, Encoding).
// ADD-PATH: Derives from Encoding.AddPathMode with Receive direction.
// RFC 7911: The negotiated mode indicates what we are allowed to do.
func FromNegotiatedRecvWire(neg *capability.Negotiated) *WireContext {
	if neg == nil {
		return nil
	}
	return NewWireContext(neg.Identity, neg.Encoding, DirectionRecv)
}

// FromNegotiatedSendWire creates a send WireContext from capability negotiation.
// The send context is used when encoding routes TO the peer.
//
// Uses the composite sub-components from Negotiated (Identity, Encoding).
// ADD-PATH: Derives from Encoding.AddPathMode with Send direction.
// RFC 7911: The negotiated mode indicates what we are allowed to do.
func FromNegotiatedSendWire(neg *capability.Negotiated) *WireContext {
	if neg == nil {
		return nil
	}
	return NewWireContext(neg.Identity, neg.Encoding, DirectionSend)
}
