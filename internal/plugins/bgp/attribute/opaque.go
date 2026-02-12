package attribute

import (
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wire"
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

// WriteTo writes the opaque attribute data into buf at offset.
func (o *OpaqueAttribute) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], o.data)
}

// WriteToWithContext writes the opaque data - context-independent.
func (o *OpaqueAttribute) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return o.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (o *OpaqueAttribute) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := o.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return o.WriteTo(buf, off), nil
}
