package context

import (
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/capability"
)

// FromNegotiatedRecv creates a receive EncodingContext from capability negotiation.
// The receive context is used when parsing routes FROM the peer.
//
// Uses the composite sub-components from Negotiated (Identity, Encoding).
// ADD-PATH: Derives from Encoding.AddPathMode with Receive direction.
// RFC 7911: The negotiated mode indicates what we are allowed to do.
func FromNegotiatedRecv(neg *capability.Negotiated) *EncodingContext {
	if neg == nil {
		return nil
	}
	return NewEncodingContext(neg.Identity, neg.Encoding, DirectionRecv)
}

// FromNegotiatedSend creates a send EncodingContext from capability negotiation.
// The send context is used when encoding routes TO the peer.
//
// Uses the composite sub-components from Negotiated (Identity, Encoding).
// ADD-PATH: Derives from Encoding.AddPathMode with Send direction.
// RFC 7911: The negotiated mode indicates what we are allowed to do.
func FromNegotiatedSend(neg *capability.Negotiated) *EncodingContext {
	if neg == nil {
		return nil
	}
	return NewEncodingContext(neg.Identity, neg.Encoding, DirectionSend)
}
