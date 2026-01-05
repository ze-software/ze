package attribute

import (
	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
)

// OpaqueAttribute represents an unknown or unrecognized BGP path attribute.
//
// RFC 4271 Section 5: "Optional transitive attributes may be partially propagated
// even if not recognized, with the Partial bit set."
//
// OpaqueAttribute preserves the original flags and data for forwarding.
// The data is NOT copied - caller must ensure the underlying buffer outlives
// this attribute (follows zero-copy memory contract).
type OpaqueAttribute struct {
	flags AttributeFlags
	code  AttributeCode
	data  []byte // NOT owned - borrowed from caller
}

// NewOpaqueAttribute creates an OpaqueAttribute for unknown attribute codes.
//
// WARNING: data is NOT copied. Caller retains ownership and must not modify.
//
// flags are preserved exactly for forwarding (including Partial bit per RFC 4271).
// code is the unknown attribute type code.
// data is the attribute value (borrowed, not copied).
func NewOpaqueAttribute(flags AttributeFlags, code AttributeCode, data []byte) *OpaqueAttribute {
	return &OpaqueAttribute{
		flags: flags,
		code:  code,
		data:  data,
	}
}

// Code returns the attribute type code (RFC 4271 Section 4.3).
func (o *OpaqueAttribute) Code() AttributeCode {
	return o.code
}

// Flags returns the preserved attribute flags (RFC 4271 Section 4.3).
//
// For unknown attributes, original flags are preserved for forwarding.
// This includes Optional, Transitive, and Partial bits.
func (o *OpaqueAttribute) Flags() AttributeFlags {
	return o.flags
}

// Len returns the attribute value length in octets.
func (o *OpaqueAttribute) Len() int {
	return len(o.data)
}

// Pack returns the attribute value bytes.
//
// Returns the borrowed data slice unchanged (zero-copy).
// WARNING: Do not modify the returned slice.
func (o *OpaqueAttribute) Pack() []byte {
	return o.data
}

// PackWithContext returns the attribute value bytes.
//
// Unknown attributes cannot be re-encoded (structure unknown),
// so context is ignored and original data is returned unchanged.
//
// RFC 4271: Unknown transitive attributes must be propagated with Partial bit set.
// Flag handling is the caller's responsibility when re-encoding headers.
func (o *OpaqueAttribute) PackWithContext(_, _ *bgpctx.EncodingContext) []byte {
	return o.data
}

// WriteTo writes the opaque attribute data into buf at offset.
func (o *OpaqueAttribute) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], o.data)
}

// WriteToWithContext writes the opaque data - context-independent.
func (o *OpaqueAttribute) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return o.WriteTo(buf, off)
}
