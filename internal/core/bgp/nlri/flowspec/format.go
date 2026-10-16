// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec component value rendering
// RFC: rfc/short/rfc5575.md
// Overview: types.go — core FlowSpec types, constants, and interface
// Related: json.go — the AppendJSON methods that render through these helpers
// Related: vocabulary.go — the name tables these helpers reverse

package flowspec

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// FormatPrefixWithOffset formats a prefix component as "prefix/length/offset".
func FormatPrefixWithOffset(comp FlowComponent) string {
	prefix := ""
	offset := uint8(0)

	if pc, ok := comp.(interface{ Prefix() netip.Prefix }); ok {
		prefix = pc.Prefix().String()
	}
	if oc, ok := comp.(interface{ Offset() uint8 }); ok {
		offset = oc.Offset()
	}

	var b textbuf.Buffer
	return b.Reset().Str(prefix).Byte('/').Uint8(offset).String()
}

// protocolNumberToName maps protocol numbers to names for output. It is
// derived from ProtocolNameToNumber so the two directions cannot disagree: a
// name this package accepts is a name it prints back.
var protocolNumberToName = reverseProtocolNames()

func reverseProtocolNames() map[uint8]string {
	names := make(map[uint8]string, len(ProtocolNameToNumber))
	for name, number := range ProtocolNameToNumber {
		names[number] = name
	}
	return names
}

// FormatNumericMatches formats numeric component matches for ze-bgp JSON.
// Returns nested arrays: [[value1], [value2]] for OR logic.
func FormatNumericMatches(comp FlowComponent, compType FlowComponentType) [][]string {
	nc, ok := comp.(interface{ Matches() []FlowMatch })
	if !ok {
		return [][]string{}
	}

	matches := nc.Matches()
	result := make([][]string, 0, len(matches))
	var andGroup []string

	for _, m := range matches {
		valStr := formatNumericValue(m, compType)

		if m.And && len(andGroup) > 0 {
			// Continue AND group
			andGroup = append(andGroup, valStr)
		} else {
			// Start new OR group (flush previous AND group if any)
			if len(andGroup) > 0 {
				result = append(result, andGroup)
			}
			andGroup = []string{valStr}
		}
	}

	// Flush final group
	if len(andGroup) > 0 {
		result = append(result, andGroup)
	}

	return result
}

// formatNumericValue formats a single numeric match value.
func formatNumericValue(m FlowMatch, compType FlowComponentType) string {
	// For protocol, try to use name
	if compType == FlowIPProtocol {
		if name, ok := protocolNumberToName[uint8(m.Value)]; ok { //nolint:gosec // Protocol values are 8-bit
			return formatWithOperator(name, m.Op)
		}
	}

	// Format with operator prefix
	return formatWithOperator(textbuf.StringUint(m.Value), m.Op)
}

// formatWithOperator adds operator prefix to a value string.
func formatWithOperator(value string, op FlowOperator) string {
	// Mask out non-comparison bits
	compOp := op &^ (FlowOpEnd | FlowOpAnd | FlowOpLenMask)

	var prefix string
	//exhaustive:ignore // Render only comparison bits after masking the operator framing flags.
	switch compOp {
	case FlowOpEqual:
		prefix = "="
	case FlowOpGreater:
		prefix = ">"
	case FlowOpLess:
		prefix = "<"
	case FlowOpGreater | FlowOpEqual:
		prefix = ">="
	case FlowOpLess | FlowOpEqual:
		prefix = "<="
	case FlowOpNotEq:
		prefix = "!="
	default: // unrecognized operator — treat as equality
		prefix = "="
	}
	var tb textbuf.Buffer
	return tb.Str(prefix).Str(value).String()
}

// TCPFlagValueToNames maps TCP flag bit values to names.
var TCPFlagValueToNames = map[uint8]string{
	0x01: "fin",
	0x02: "syn",
	0x04: "rst",
	0x08: "push", // "push" not "psh"
	0x10: "ack",
	0x20: "urg",
	0x40: "ece",
	0x80: "cwr",
}

// FragmentFlagValueToNames maps fragment flag bit values to names.
var FragmentFlagValueToNames = map[uint8]string{
	0x01: "dont-fragment",
	0x02: "is-fragment",
	0x04: "first-fragment",
	0x08: "last-fragment",
}

// FormatBitmaskMatches formats bitmask component matches (TCP flags, Fragment).
// Returns nested arrays with combined flag names.
func FormatBitmaskMatches(comp FlowComponent, flagMap map[uint8]string) [][]string {
	nc, ok := comp.(interface{ Matches() []FlowMatch })
	if !ok {
		return [][]string{}
	}

	matches := nc.Matches()
	result := make([][]string, 0, len(matches))
	var andGroup []string

	// The outer array is the OR and the inner array is the AND, exactly as
	// FormatNumericMatches builds it. Until 2026-09-20 this loop gave every
	// match an inner array of its own and never read m.And, so
	// `syn | rst & fin & !=push` decoded as four independent alternatives: a
	// filter that matches a SYN-only packet, which the route does not ask for.
	// RFC 8955 Section 4.2.1.1 makes the AND bit part of what the component
	// MEANS, so dropping it is not a display choice.
	for _, m := range matches {
		valStr := formatBitmaskValue(m, flagMap)

		if m.And && len(andGroup) > 0 {
			andGroup = append(andGroup, valStr)
			continue
		}
		if len(andGroup) > 0 {
			result = append(result, andGroup)
		}
		andGroup = []string{valStr}
	}

	if len(andGroup) > 0 {
		result = append(result, andGroup)
	}

	return result
}

// formatBitmaskValue formats one bitmask match as a single string: the
// operator, then every named bit joined with `+`.
//
// RFC 8955 Section 4.2.1.2 gives the operator two bits that are not the same
// question, and both are written here:
//
//   - N (NOT) negates the result, and renders `!`.
//   - M (MATCH) asks for an EXACT match rather than "any of these bits set",
//     and renders `=`.
//
// An operator with neither bit renders BARE. Writing `=` for it, which this
// function did until 2026-09-20, published an exact-match route for one that
// asked for any bit: `tcp-flags syn` and `tcp-flags =syn` select different
// packets, and a reader had no way to tell which the peer sent.
//
// The bits of ONE value are joined rather than listed, because they are one
// operand and not a sequence of matches. `+` is the separator every FlowSpec
// grammar uses for it, including Ze's own config parser.
func formatBitmaskValue(m FlowMatch, flagMap map[uint8]string) string {
	var prefix string
	if m.Op&FlowOpNot != 0 {
		prefix = "!"
	}
	if m.Op&FlowOpMatch != 0 {
		prefix += "="
	}

	flags := uint8(m.Value) //nolint:gosec // Bitmask values are 8-bit
	var b textbuf.Buffer
	b.Reset().Str(prefix)

	named := false
	for bit := uint8(0x01); bit != 0; bit <<= 1 {
		if flags&bit == 0 {
			continue
		}
		name, ok := flagMap[bit]
		if !ok {
			continue
		}
		if named {
			b.Byte('+')
		}
		b.Str(name)
		named = true
	}

	if !named {
		return b.Reset().Str(prefix).Uint8(flags).String()
	}
	return b.String()
}
