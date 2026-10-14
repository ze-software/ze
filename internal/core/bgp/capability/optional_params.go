// Design: docs/architecture/wire/capabilities.md — the one Optional Parameters packer
// Related: capability.go — Parse, the capability TLV decoder ParseFromOptionalParams calls
//
// RFC 4271 Section 4.2 Optional Parameters, base form (one-octet Parameter Length):
//
//	 0                   1
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Parm. Type   | Parm. Length  |  Parameter Value (variable)
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// RFC 9072 Section 2 extended form (two-octet Parameter Length):
//
//	 0                   1                   2
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Parm. Type   |        Parameter Length       |  Value ...
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Offsets per parameter: 0 Type, 1 Length (1 octet, or 2 octets at 1-2 in the
// extended form), then the value at offset 2 (base) or 3 (extended).

package capability

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// paramTypeCapabilities is the Optional Parameter Type of the Capabilities
// parameter (RFC 5492 Section 4).
const paramTypeCapabilities = 2

// optionalParamsBaseMax is the largest Optional Parameters field the RFC 4271
// one-octet Opt Parm Len can describe.
const optionalParamsBaseMax = 255

// optionalParamsExtendedMax is the largest Optional Parameters field the RFC
// 9072 two-octet Extended Opt. Parm. Length can describe.
const optionalParamsExtendedMax = 65535

// ErrOptionalParamsTooLarge reports Optional Parameters that no OPEN encoding
// can describe: above 65535 octets even with two-octet Parameter Lengths.
var ErrOptionalParamsTooLarge = errors.New("capability: optional parameters exceed the RFC 9072 extended length")

// OptionalParam is one Optional Parameter of a BGP OPEN, built only by
// CapabilitiesParam or RawParam, so no caller can build a parameter carrying
// both value forms or a capability list under a Type other than 2.
//
// The zero value is a Raw parameter of Type 0 with an empty value: a
// well-formed parameter (two octets, 00 00) that PackOptionalParams writes as
// given. A receiver refuses Type 0 as an unsupported parameter.
//
// It is a concrete struct, not an interface, so a parameter list built per
// OPEN stays on the caller's stack and the packer's output is the only
// allocation.
type OptionalParam struct {
	paramType    uint8
	capabilities []Capability
	raw          []byte
}

// CapabilitiesParam builds a Capabilities Optional Parameter (Type 2) whose
// value is the capabilities, each written in place by its WriteTo.
//
// The slice is read by PackOptionalParams and never retained or modified.
func CapabilitiesParam(caps []Capability) OptionalParam {
	return OptionalParam{paramType: paramTypeCapabilities, capabilities: caps}
}

// RawParam builds an Optional Parameter of the given Type whose value is the
// octets as given. The packer copies them; the slice is never retained.
func RawParam(paramType uint8, value []byte) OptionalParam {
	return OptionalParam{paramType: paramType, raw: value}
}

// valueOctets is the length of the parameter's value.
func (p *OptionalParam) valueOctets() int {
	if p.capabilities == nil {
		return len(p.raw)
	}
	total := 0
	for _, c := range p.capabilities {
		total += c.Len()
	}
	return total
}

// writeValue writes the parameter's value at buf[off:] and returns the octets
// written. buf MUST hold valueOctets() octets from off.
func (p *OptionalParam) writeValue(buf []byte, off int) int {
	if p.capabilities == nil {
		return copy(buf[off:], p.raw)
	}
	start := off
	for _, c := range p.capabilities {
		off += c.WriteTo(buf, off)
	}
	return off - start
}

// PackOptionalParams is the one encoder of a BGP OPEN's Optional Parameters
// field. It returns the field's octets and whether they use the RFC 9072
// two-octet Parameter Length, which the caller stores in
// message.Open.OptionalParams and ExtendedParams so the envelope and the
// parameters agree. No parameters yield no octets and the base form.
//
// The output is allocated once, at its final size; each capability writes
// itself into it at its offset. The only error is ErrOptionalParamsTooLarge.
// The inverse is ParseFromOptionalParams.
func PackOptionalParams(params []OptionalParam) ([]byte, bool, error) {
	if len(params) == 0 {
		return nil, false, nil
	}

	valueTotal := 0
	for i := range params {
		valueTotal += params[i].valueOctets()
	}

	// RFC 9072 Section 2: "In the event that the length of the Optional
	// Parameters in the BGP OPEN message does not exceed 255, the encodings of
	// the base BGP specification [RFC4271] SHOULD be used without alteration."
	// The length is the whole field, parameter headers included: 2 + value per
	// parameter.
	baseOctets := 2*len(params) + valueTotal
	if baseOctets <= optionalParamsBaseMax {
		return packOptionalParamsWith(params, baseOctets, 1), false, nil
	}

	// RFC 9072 Section 2: "However, if the length of the Optional Parameters in
	// the BGP OPEN message does exceed 255, the OPEN message MUST be encoded
	// according to the procedure below."
	// RFC 9072 Section 2: "The rules for encoding Optional Parameters are
	// unchanged with respect to those given in [RFC4271], except that the
	// Parameter Length field is extended to be a two-octet unsigned integer."
	extendedOctets := 3*len(params) + valueTotal
	if extendedOctets > optionalParamsExtendedMax {
		return nil, false, fmt.Errorf("%w: %d octets, limit %d",
			ErrOptionalParamsTooLarge, extendedOctets, optionalParamsExtendedMax)
	}
	return packOptionalParamsWith(params, extendedOctets, 2), true, nil
}

// packOptionalParamsWith writes every parameter with a Parameter Length of
// lengthOctets (1 or 2) into one buffer of fieldOctets. The caller has checked
// that every length fits the field width.
func packOptionalParamsWith(params []OptionalParam, fieldOctets, lengthOctets int) []byte {
	buf := make([]byte, fieldOctets)
	off := 0
	for i := range params {
		p := &params[i]
		valueOctets := p.valueOctets()
		buf[off] = p.paramType
		if lengthOctets == 1 {
			buf[off+1] = byte(valueOctets) //nolint:gosec // G115: the field total is at most 255
		} else {
			binary.BigEndian.PutUint16(buf[off+1:off+3], uint16(valueOctets)) //nolint:gosec // G115: the field total is at most 65535
		}
		off += 1 + lengthOctets
		off += p.writeValue(buf, off)
	}
	return buf
}

// ParseFromOptionalParams extracts capabilities from OPEN optional parameters.
// RFC 5492 Section 4: Optional Parameters contain type-length-value triples.
// Type 2 indicates the Capabilities Optional Parameter.
// extended selects the RFC 9072 Section 2 parameter framing, whose Parameter
// Length field is two octets rather than one. The caller reads it from the OPEN
// it decoded (message.Open.ExtendedParams); this function cannot derive it,
// because a one-octet and a two-octet length are the same bytes read two ways.
func ParseFromOptionalParams(optParams []byte, extended bool) ([]Capability, error) {
	var caps []Capability
	err := WalkOptionalParams(optParams, extended, func(paramType uint8, value []byte) error {
		// RFC 5492 Section 4: Capability Optional Parameter value is a sequence
		// of capability TLVs. Malformed Type 2 TLVs are protocol errors, not
		// unknown capabilities to ignore.
		if paramType != paramTypeCapabilities {
			// RFC 4271 Section 6.2: "If one of the Optional Parameters in
			// the OPEN message is not recognized, then the Error Subcode
			// MUST be set to Unsupported Optional Parameters."
			return ErrUnsupportedParameter
		}

		// RFC 5492 Section 4: "The parameter contains one or more
		// triples <Capability Code, Capability Length, Capability Value>".
		if len(value) == 0 {
			return ErrInvalidLength
		}
		parsed, err := Parse(value)
		if err != nil {
			return err
		}
		caps = append(caps, parsed...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return caps, nil
}

// WalkOptionalParams calls fn with the type and the value octets of each
// Optional Parameter, in wire order, and stops at the first error fn returns.
// It is the one reader of the parameter framing: ParseFromOptionalParams decodes
// through it, and a reader that needs a capability's raw value octets (the GR
// plugin, the OPEN validation RPC, the MRT decoder) walks with it rather than
// with a one-octet loop of its own. extended is the OPEN's own framing
// (message.Open.ExtendedParams). A parameter whose header or value runs past the
// end of optParams returns ErrShortRead. value aliases optParams.
func WalkOptionalParams(optParams []byte, extended bool, fn func(paramType uint8, value []byte) error) error {
	// RFC 9072 Section 2: "The rules for encoding Optional Parameters are
	// unchanged with respect to those given in [RFC4271], except that the
	// Parameter Length field is extended to be a two-octet unsigned integer."
	// The header is therefore three octets rather than two under that framing.
	hdrLen := 2
	if extended {
		hdrLen = 3
	}

	offset := 0
	for offset < len(optParams) {
		if offset+hdrLen > len(optParams) {
			return ErrShortRead
		}

		paramType := optParams[offset] //nolint:gosec // G602 false positive: bounds-checked above
		paramLen := int(optParams[offset+1])
		if extended {
			paramLen = int(optParams[offset+1])<<8 | int(optParams[offset+2])
		}
		offset += hdrLen

		if offset+paramLen > len(optParams) {
			return ErrShortRead
		}
		if err := fn(paramType, optParams[offset:offset+paramLen]); err != nil {
			return err
		}
		offset += paramLen
	}
	return nil
}
