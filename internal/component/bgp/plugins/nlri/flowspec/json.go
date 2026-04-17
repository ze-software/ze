// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec in-process JSON writer
//
// AppendJSON streams the FlowSpec NLRI JSON representation directly into a
// caller-provided []byte. It skips both the RPC-level hex round-trip
// (DecodeNLRIHex) AND the previous map+json.Marshal round-trip (flowSpecToJSON).
//
// Components are grouped by type and emitted in RFC 8955 §4.2 on-wire
// order (types 1..13). Multiple components of the same type are merged
// into one JSON key with concatenated OR groups, matching flowSpecToJSON.
// This differs from the previous alphabetical (map-sorted) key order;
// tests use assert.Contains so ordering is not observed.

package flowspec

import (
	"slices"
	"strconv"
	"strings"
)

// AppendJSON satisfies nlri.JSONAppender for FlowSpec (SAFI 133).
func (f *FlowSpec) AppendJSON(buf []byte) []byte {
	return appendFlowSpecJSON(buf, f, f.family.String(), nil)
}

// AppendJSON satisfies nlri.JSONAppender for FlowSpecVPN (SAFI 134).
func (f *FlowSpecVPN) AppendJSON(buf []byte) []byte {
	rd := f.rd
	return appendFlowSpecJSON(buf, f.flowSpec, f.Family().String(), &rd)
}

// appendFlowSpecJSON streams FlowSpec components as JSON into buf.
// rd is emitted first (VPN only), followed by one key per populated
// component type in type order. Unknown component types are emitted last as
// "type-N":[] to match the legacy componentToJSON fallback; in practice
// ParseFlowSpec rejects unknown types so this path is defensive only.
func appendFlowSpecJSON(buf []byte, fs *FlowSpec, family string, rd *RouteDistinguisher) []byte {
	isIPv6 := strings.Contains(family, "ipv6")

	// Index 0 unused; indices 1..13 hold accumulated OR groups per type.
	var matchesByType [14][][]string
	var hasType [14]bool
	var unknownTypes []FlowComponentType

	for _, comp := range fs.Components() {
		ct := comp.Type()
		switch ct {
		case FlowDestPrefix, FlowSourcePrefix:
			hasType[ct] = true
			matchesByType[ct] = append(matchesByType[ct], []string{formatPrefixWithOffset(comp)})
		case FlowTCPFlags:
			hasType[ct] = true
			matchesByType[ct] = append(matchesByType[ct], formatBitmaskMatches(comp, tcpFlagValueToNames)...)
		case FlowFragment:
			hasType[ct] = true
			matchesByType[ct] = append(matchesByType[ct], formatBitmaskMatches(comp, fragmentFlagValueToNames)...)
		case FlowIPProtocol, FlowPort, FlowDestPort, FlowSourcePort,
			FlowICMPType, FlowICMPCode, FlowPacketLength, FlowDSCP, FlowFlowLabel:
			hasType[ct] = true
			matchesByType[ct] = append(matchesByType[ct], formatNumericMatches(comp, ct)...)
		default: // unknown component type — preserve legacy "type-N":[] fallback
			if !slices.Contains(unknownTypes, ct) {
				unknownTypes = append(unknownTypes, ct)
			}
		}
	}

	buf = append(buf, '{')
	first := true
	if rd != nil {
		buf = append(buf, `"rd":"`...)
		buf = append(buf, rd.String()...)
		buf = append(buf, '"')
		first = false
	}

	for ct := FlowComponentType(1); ct <= FlowFlowLabel; ct++ {
		if !hasType[ct] {
			continue
		}
		if !first {
			buf = append(buf, ',')
		}
		first = false

		buf = append(buf, '"')
		buf = append(buf, flowSpecKey(ct, isIPv6)...)
		buf = append(buf, `":`...)
		buf = appendNestedStringArray(buf, matchesByType[ct])
	}

	for _, ct := range unknownTypes {
		if !first {
			buf = append(buf, ',')
		}
		first = false
		buf = append(buf, `"type-`...)
		buf = strconv.AppendInt(buf, int64(ct), 10)
		buf = append(buf, `":[]`...)
	}

	buf = append(buf, '}')
	return buf
}

// flowSpecKey returns the JSON key for a FlowSpec component type.
// IPv6 families use different keys for destination/source/protocol per RFC 8956.
func flowSpecKey(ct FlowComponentType, isIPv6 bool) string {
	switch ct {
	case FlowDestPrefix:
		if isIPv6 {
			return kwDestinationIPv6
		}
		return kwDestination
	case FlowSourcePrefix:
		if isIPv6 {
			return kwSourceIPv6
		}
		return kwSource
	case FlowIPProtocol:
		if isIPv6 {
			return kwNextHeader
		}
		return kwProtocol
	case FlowPort:
		return kwPort
	case FlowDestPort:
		return kwDestPort
	case FlowSourcePort:
		return kwSourcePort
	case FlowICMPType:
		return kwICMPType
	case FlowICMPCode:
		return kwICMPCode
	case FlowTCPFlags:
		return kwTCPFlags
	case FlowPacketLength:
		return kwPacketLength
	case FlowDSCP:
		return kwDSCP
	case FlowFragment:
		return kwFragment
	case FlowFlowLabel:
		return kwFlowLabel
	}
	return "" // unreachable: caller bounds ct to 1..FlowFlowLabel
}

// appendNestedStringArray writes [["a","b"],["c"]] into buf.
// Values come from FlowSpec formatters (operators, digits, protocol and flag
// names) and contain only JSON-safe ASCII.
func appendNestedStringArray(buf []byte, groups [][]string) []byte {
	buf = append(buf, '[')
	for i, g := range groups {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '[')
		for j, v := range g {
			if j > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, '"')
			buf = append(buf, v...)
			buf = append(buf, '"')
		}
		buf = append(buf, ']')
	}
	buf = append(buf, ']')
	return buf
}
