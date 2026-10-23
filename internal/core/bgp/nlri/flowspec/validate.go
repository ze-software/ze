// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI wire codec
// RFC: rfc/short/rfc8955.md
// Related: types.go — ParseFlowSpec, whose checks this walk shares
// Related: types_prefix.go — prefixComponentShape, the prefix component check
// Related: types_numeric.go — numericComponentEnd, the numeric component check

package flowspec

// ValidateFlowSpec judges one FlowSpec NLRI value (length field and
// components, no Path Identifier) without decoding it. It refuses exactly what
// ParseFlowSpec refuses, because both run the same checks: flowSpecValue,
// classifyComponent, prefixComponentShape, numericComponentEnd and
// checkComponentOrder.
//
// It exists for the receive path, which judges every FlowSpec NLRI of every
// UPDATE before a plugin decodes it, so a well-formed NLRI costs no allocation.
// A refusal allocates its error.
func ValidateFlowSpec(fam Family, data []byte) error {
	value, err := flowSpecValue(data)
	if err != nil {
		return err
	}
	return validateComponents(value, fam)
}

// ValidateFlowSpecVPN judges one FlowSpec VPN NLRI value (length field, Route
// Distinguisher, components) the way ParseFlowSpecVPN reads it, without
// decoding it.
//
// RFC 8955 Section 8: "The NLRI format for this address family consists of a
// fixed-length Route Distinguisher field (8 octets) followed by the Flow
// Specification NLRI value (Section 4.2).  The NLRI length field shall include
// both the 8 octets of the Route Distinguisher as well as the subsequent Flow
// Specification NLRI value."
func ValidateFlowSpecVPN(fam Family, data []byte) error {
	value, err := flowSpecValue(data)
	if err != nil {
		return err
	}
	if len(value) < 8 {
		return ErrFlowSpecTruncated
	}
	return validateComponents(value[8:], Family{AFI: fam.AFI, SAFI: SAFIFlowSpec})
}

// validateComponents walks a component list with parseComponents' checks and
// keeps nothing.
//
// The walk is bounded by len(data): every component consumes at least its type
// octet, so each step shortens the remainder.
func validateComponents(data []byte, fam Family) error {
	previous := FlowComponentType(0)

	for len(data) > 0 {
		compType, octets, err := componentOctets(data, fam)
		if err != nil {
			return err
		}
		if err := checkComponentOrder(previous, compType); err != nil {
			return err
		}

		previous = compType
		data = data[octets:]
	}

	return nil
}

// componentOctets checks the component that opens data and returns its type
// and the octets it occupies, type octet included.
func componentOctets(data []byte, fam Family) (FlowComponentType, int, error) {
	compType, class, err := classifyComponent(data, fam)
	if err != nil {
		return compType, 0, err
	}

	switch class {
	case componentClassPrefix:
		shape, err := prefixComponentShape(data[1:], fam)
		if err != nil {
			return compType, 0, err
		}
		return compType, 1 + shape.octets(), nil
	case componentClassNumeric:
		end, err := numericComponentEnd(compType, data[1:])
		if err != nil {
			return compType, 0, err
		}
		return compType, 1 + end, nil
	case componentClassUnspecified:
		return compType, 0, ErrFlowSpecInvalidType
	}
	panic("BUG: classifyComponent returned an unnamed component class")
}
