// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI plugin
// RFC: rfc/short/rfc5575.md
// Related: config.go -- parseConfigRoute (supplies the match-criteria map)

package flowspec

import (
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
)

// buildFlowSpecComponents builds the FlowSpec from config-format match criteria
// and returns any criterion keys that were PRESENT in the config but produced no
// component: either an UNRECOGNIZED key (typo) or a known key whose value(s)
// failed to parse (e.g. an invalid prefix or a bad port match). Either case would
// silently widen the filter (worst case an all-match rule), so fail-loud callers
// reject a non-empty dropped list. The `seen` set is populated by the blocks
// themselves, so a future criterion missing its block fails loud rather than
// being silently dropped.
func buildFlowSpecComponents(matchCriteria map[string][]string, isIPv6 bool) (*FlowSpec, []string) {
	fam := IPv4FlowSpec
	if isIPv6 {
		fam = IPv6FlowSpec
	}

	fs := NewFlowSpec(fam)
	var dropped []string
	seen := make(map[string]bool, len(matchCriteria))

	// add records the criterion as dropped when the component cannot join the ones
	// already built. AddComponent refuses a second component of a type it cannot
	// merge (RFC 8955 Section 4.2), which is the same fail-loud case as a value
	// that would not parse: the criterion is present in the config and produced no
	// match, so the caller must reject the route rather than widen it.
	add := func(key string, c FlowComponent) {
		if err := fs.AddComponent(c); err != nil {
			dropped = append(dropped, key)
		}
	}

	// Add destination prefix (first value only - prefix is singular). Both
	// spellings are looked up and the operator's own word is what a refusal
	// names, so the message quotes the config rather than an internal key.
	if key, vals, ok := prefixCriterion(matchCriteria, kwDestinationIPv4, kwDestinationIPv6); ok {
		seen[key] = true
		if prefix, offset := parseFlowPrefixWithOffset(first(vals)); !prefix.IsValid() {
			dropped = append(dropped, key)
		} else if prefix.Addr().Is6() && offset > 0 {
			add(key, newFlowDestPrefixComponentWithOffset(prefix, offset))
		} else {
			add(key, NewFlowDestPrefixComponent(prefix))
		}
	}

	// Add source prefix (first value only - prefix is singular)
	if key, vals, ok := prefixCriterion(matchCriteria, kwSourceIPv4, kwSourceIPv6); ok {
		seen[key] = true
		if prefix, offset := parseFlowPrefixWithOffset(first(vals)); !prefix.IsValid() {
			dropped = append(dropped, key)
		} else if prefix.Addr().Is6() && offset > 0 {
			add(key, newFlowSourcePrefixComponentWithOffset(prefix, offset))
		} else {
			add(key, NewFlowSourcePrefixComponent(prefix))
		}
	}

	// Numeric / match criteria: a present key with no parseable value is dropped.
	addNumeric := func(key string, typ FlowComponentType, parse func([]string) []FlowMatch) {
		if vals, ok := matchCriteria[key]; ok {
			seen[key] = true
			if matches := parse(vals); len(matches) > 0 {
				add(key, newFlowNumericComponent(typ, matches))
			} else {
				dropped = append(dropped, key)
			}
		}
	}
	addNumeric(kwProtocol, FlowIPProtocol, parseFlowProtocolMatchesSlice)
	addNumeric(kwNextHeader, FlowIPProtocol, parseFlowProtocolMatchesSlice)
	addNumeric(kwPort, FlowPort, parseFlowMatchesSlice)
	addNumeric(kwDestPort, FlowDestPort, parseFlowMatchesSlice)
	addNumeric(kwSourcePort, FlowSourcePort, parseFlowMatchesSlice)
	addNumeric(kwPacketLength, FlowPacketLength, parseFlowMatchesSlice)
	addNumeric(kwTCPFlags, FlowTCPFlags, parseFlowTCPFlagMatchesSlice)

	if vals, ok := matchCriteria[kwDSCP]; ok {
		seen[kwDSCP] = true
		if octets := parseFlowOctetsSlice(vals); len(octets) > 0 {
			add(kwDSCP, NewFlowDSCPComponent(octets...))
		} else {
			dropped = append(dropped, kwDSCP)
		}
	}
	if vals, ok := matchCriteria["traffic-class"]; ok {
		seen["traffic-class"] = true
		if octets := parseFlowOctetsSlice(vals); len(octets) > 0 {
			add("traffic-class", NewFlowDSCPComponent(octets...))
		} else {
			dropped = append(dropped, "traffic-class")
		}
	}
	if vals, ok := matchCriteria[kwFlowLabel]; ok {
		seen[kwFlowLabel] = true
		if labels := parseFlowLabelsSlice(vals); len(labels) > 0 {
			add(kwFlowLabel, NewFlowFlowLabelComponent(labels...))
		} else {
			dropped = append(dropped, kwFlowLabel)
		}
	}
	if vals, ok := matchCriteria[kwFragment]; ok {
		seen[kwFragment] = true
		if flags := parseFlowFragmentSlice(vals); len(flags) > 0 {
			add(kwFragment, NewFlowFragmentComponent(flags...))
		} else {
			dropped = append(dropped, kwFragment)
		}
	}
	if vals, ok := matchCriteria[kwICMPType]; ok {
		seen[kwICMPType] = true
		if types := parseFlowICMPTypesSlice(vals); len(types) > 0 {
			add(kwICMPType, NewFlowICMPTypeComponent(types...))
		} else {
			dropped = append(dropped, kwICMPType)
		}
	}
	if vals, ok := matchCriteria[kwICMPCode]; ok {
		seen[kwICMPCode] = true
		if codes := parseFlowICMPCodesSlice(vals); len(codes) > 0 {
			add(kwICMPCode, newFlowICMPCodeComponent(codes...))
		} else {
			dropped = append(dropped, kwICMPCode)
		}
	}

	// Any criterion key no block recognized is an unknown criterion (typo).
	for key := range matchCriteria {
		if !seen[key] {
			dropped = append(dropped, key)
		}
	}

	return fs, dropped
}

// first returns the first element of s, or "" if s is empty.
func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

// parseFlowPrefixWithOffset parses a FlowSpec prefix like "10.0.0.1/32" or "::1/128/120".
// Returns the prefix and offset (0 if no offset).
func parseFlowPrefixWithOffset(s string) (netip.Prefix, uint8) {
	// Handle IPv6 offset format: addr/len/offset
	parts := strings.Split(s, "/")
	if len(parts) >= 2 {
		addrStr := parts[0]
		lenStr := parts[1]
		var offset uint8
		if len(parts) >= 3 {
			if off, err := strconv.Atoi(parts[2]); err == nil && off >= 0 && off <= 255 {
				offset = uint8(off) // #nosec G115 -- bounds checked
			}
		}

		addr, err := netip.ParseAddr(addrStr)
		if err != nil {
			return netip.Prefix{}, 0
		}
		prefixLen, err := strconv.Atoi(lenStr)
		if err != nil {
			return netip.Prefix{}, 0
		}
		return netip.PrefixFrom(addr, prefixLen), offset
	}

	// Try parsing as simple prefix
	prefix, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, 0
	}
	return prefix, 0
}

// parseFlowProtocolMatches parses protocol values with operators.
func parseFlowProtocolMatches(s string) []FlowMatch {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []FlowMatch

	for _, p := range parts {
		var op FlowOperator

		// Parse operator prefix
		switch {
		case strings.HasPrefix(p, "!="):
			op = FlowOpNotEq
			p = strings.TrimPrefix(p, "!=")
		case strings.HasPrefix(p, "="):
			op = FlowOpEqual
			p = strings.TrimPrefix(p, "=")
		default: // No operator prefix — bare protocol name/number implies equality
			op = FlowOpEqual
		}

		p = strings.ToLower(p)
		if v, ok := protocolNameToNumber[p]; ok {
			result = append(result, FlowMatch{Op: op, Value: uint64(v)})
		} else if n, err := strconv.ParseUint(p, 10, 8); err == nil {
			result = append(result, FlowMatch{Op: op, Value: n})
		}
	}
	return result
}

// parseFlowMatches parses FlowSpec match expressions with operators.
// Formats: "=80", ">1024", "[ =80 =8080 ]", ">8080&<8088", "!=443".
func parseFlowMatches(s string) []FlowMatch {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []FlowMatch

	for _, p := range parts {
		// Handle range operators like ">8080&<8088" by splitting on &
		rangeParts := strings.Split(p, "&")
		for i, rp := range rangeParts {
			var op FlowOperator
			isAnd := i > 0 // Parts after & are AND-ed with previous

			// Parse operator prefix
			switch {
			case strings.HasPrefix(rp, "!="):
				op = FlowOpNotEq
				rp = strings.TrimPrefix(rp, "!=")
			case strings.HasPrefix(rp, ">="):
				op = FlowOpGreater | FlowOpEqual
				rp = strings.TrimPrefix(rp, ">=")
			case strings.HasPrefix(rp, "<="):
				op = FlowOpLess | FlowOpEqual
				rp = strings.TrimPrefix(rp, "<=")
			case strings.HasPrefix(rp, ">"):
				op = FlowOpGreater
				rp = strings.TrimPrefix(rp, ">")
			case strings.HasPrefix(rp, "<"):
				op = FlowOpLess
				rp = strings.TrimPrefix(rp, "<")
			case strings.HasPrefix(rp, "="):
				op = FlowOpEqual
				rp = strings.TrimPrefix(rp, "=")
			default: // No operator prefix — bare number implies equality
				op = FlowOpEqual
			}

			if n, err := strconv.ParseUint(rp, 10, 32); err == nil {
				result = append(result, FlowMatch{
					Op:    op,
					And:   isAnd,
					Value: n,
				})
			}
		}
	}
	return result
}

// parseFlowOctets parses octet values (DSCP, traffic-class).
func parseFlowOctets(s string) []uint8 {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []uint8

	for _, p := range parts {
		p = strings.TrimPrefix(p, "=")
		if n, err := strconv.ParseUint(p, 10, 8); err == nil {
			result = append(result, uint8(n))
		}
	}
	return result
}

// icmpTypeNames maps ICMP type symbolic names to values.
// Per IANA ICMP Type Numbers: https://www.iana.org/assignments/icmp-parameters
// Uses lowercase kebab-case names.
var icmpTypeNames = map[string]uint8{
	"echo-reply":            0,
	"unreachable":           3,
	"redirect":              5,
	"echo-request":          8,
	"router-advertisement":  9,
	"router-solicit":        10,
	"time-exceeded":         11,
	"parameter-problem":     12,
	"timestamp":             13,
	"timestamp-reply":       14,
	"photuris":              40,
	"experimental-mobility": 41,
	"extended-echo-request": 42,
	"extended-echo-reply":   43,
	"experimental-one":      253,
	"experimental-two":      254,
}

// parseFlowICMPTypes parses ICMP type values or names.
// Handles: [ unreachable echo-request echo-reply ] or [ 3 8 0 ] or [ =3 =8 =0 ].
// Unknown names are logged as warnings and skipped.
func parseFlowICMPTypes(s string) []uint8 {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []uint8

	for _, p := range parts {
		p = strings.TrimPrefix(p, "=")
		// Try numeric first
		if n, err := strconv.ParseUint(p, 10, 8); err == nil {
			result = append(result, uint8(n))
			continue
		}
		// Try symbolic name
		if n, ok := icmpTypeNames[strings.ToLower(p)]; ok {
			result = append(result, n)
			continue
		}
		// Unknown name - log warning
		slog.Warn("unknown ICMP type name", "name", p)
	}
	return result
}

// icmpCodeNames maps ICMP code symbolic names to values.
// Per IANA ICMP Type Numbers: https://www.iana.org/assignments/icmp-parameters
// Uses lowercase kebab-case names.
var icmpCodeNames = map[string]uint8{
	// Destination Unreachable (type 3)
	"network-unreachable":                   0,
	"host-unreachable":                      1,
	"protocol-unreachable":                  2,
	"port-unreachable":                      3,
	"fragmentation-needed":                  4,
	"source-route-failed":                   5,
	"destination-network-unknown":           6,
	"destination-host-unknown":              7,
	"source-host-isolated":                  8,
	"destination-network-prohibited":        9,
	"destination-host-prohibited":           10,
	"network-unreachable-for-tos":           11,
	"host-unreachable-for-tos":              12,
	"communication-prohibited-by-filtering": 13,
	"host-precedence-violation":             14,
	"precedence-cutoff-in-effect":           15,
	// Redirect (type 5)
	"redirect-for-network":      0,
	"redirect-for-host":         1,
	"redirect-for-tos-and-net":  2,
	"redirect-for-tos-and-host": 3,
	// Time Exceeded (type 11)
	"ttl-eq-zero-during-transit":    0,
	"ttl-eq-zero-during-reassembly": 1,
	// Parameter Problem (type 12)
	"required-option-missing": 1,
	"ip-header-bad":           2,
}

// parseFlowICMPCodes parses ICMP code values or names.
// Handles: [ host-unreachable network-unreachable ] or [ 1 0 ] or [ =1 =0 ].
// Unknown names are logged as warnings and skipped.
func parseFlowICMPCodes(s string) []uint8 {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []uint8

	for _, p := range parts {
		p = strings.TrimPrefix(p, "=")
		// Try numeric first
		if n, err := strconv.ParseUint(p, 10, 8); err == nil {
			result = append(result, uint8(n))
			continue
		}
		// Try symbolic name
		if n, ok := icmpCodeNames[strings.ToLower(p)]; ok {
			result = append(result, n)
			continue
		}
		// Unknown name - log warning
		slog.Warn("unknown ICMP code name", "name", p)
	}
	return result
}

// parseFlowFragment parses fragment flags like "[ first-fragment last-fragment ]".
func parseFlowFragment(s string) []FlowFragmentFlag {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []FlowFragmentFlag

	for _, p := range parts {
		if f, ok := fragmentFlagNameToValue[p]; ok {
			result = append(result, FlowFragmentFlag(f))
		}
	}
	return result
}

// parseFlowTCPFlagMatches parses TCP flags with AND and NOT operators.
// TCP flags use bitmask matching:
//   - 0x01 = MATCH (exact match)
//   - 0x02 = NOT (negate)
//   - 0x40 = AND (AND with previous)
func parseFlowTCPFlagMatches(s string) []FlowMatch {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []FlowMatch

	for _, p := range parts {
		// Handle combined flags like "RST&FIN&!=push"
		flagParts := strings.Split(p, "&")
		for i, fp := range flagParts {
			var op FlowOperator
			isAnd := i > 0 // Parts after & are AND-ed

			// Check for != (NOT+MATCH)
			if strings.HasPrefix(fp, "!=") {
				op = 0x02 | 0x01 // NOT | MATCH
				fp = strings.TrimPrefix(fp, "!=")
			}
			// For simple flags, use no operator (INCLUDE)

			if isAnd {
				op |= 0x40 // AND
			}

			fp = strings.ToLower(fp)
			if f, ok := tcpFlagNameToValue[fp]; ok {
				result = append(result, FlowMatch{Op: op, And: isAnd, Value: uint64(f)})
			}
		}
	}
	return result
}

// parseFlowLabels parses flow-label values like "2013" or "=2013".
func parseFlowLabels(s string) []uint32 {
	var result []uint32
	s = strings.Trim(s, "[]")
	parts := strings.FieldsSeq(s)
	for p := range parts {
		p = strings.TrimPrefix(p, "=")
		val, err := strconv.ParseUint(p, 10, 32)
		if err == nil {
			result = append(result, uint32(val))
		}
	}
	return result
}

// --- Slice helpers for map[string][]string NLRI format ---

// parseFlowProtocolMatchesSlice parses protocol values from a pre-split slice.
func parseFlowProtocolMatchesSlice(vals []string) []FlowMatch {
	result := make([]FlowMatch, 0, len(vals))
	for _, v := range vals {
		result = append(result, parseFlowProtocolMatches(v)...)
	}
	return result
}

// parseFlowMatchesSlice parses numeric match expressions from a pre-split slice.
func parseFlowMatchesSlice(vals []string) []FlowMatch {
	result := make([]FlowMatch, 0, len(vals))
	for _, v := range vals {
		result = append(result, parseFlowMatches(v)...)
	}
	return result
}

// parseFlowOctetsSlice parses octet values from a pre-split slice.
func parseFlowOctetsSlice(vals []string) []uint8 {
	result := make([]uint8, 0, len(vals))
	for _, v := range vals {
		result = append(result, parseFlowOctets(v)...)
	}
	return result
}

// parseFlowLabelsSlice parses flow-label values from a pre-split slice.
func parseFlowLabelsSlice(vals []string) []uint32 {
	result := make([]uint32, 0, len(vals))
	for _, v := range vals {
		result = append(result, parseFlowLabels(v)...)
	}
	return result
}

// parseFlowFragmentSlice parses fragment flags from a pre-split slice.
func parseFlowFragmentSlice(vals []string) []FlowFragmentFlag {
	result := make([]FlowFragmentFlag, 0, len(vals))
	for _, v := range vals {
		result = append(result, parseFlowFragment(v)...)
	}
	return result
}

// parseFlowTCPFlagMatchesSlice parses TCP flag matches from a pre-split slice.
func parseFlowTCPFlagMatchesSlice(vals []string) []FlowMatch {
	result := make([]FlowMatch, 0, len(vals))
	for _, v := range vals {
		result = append(result, parseFlowTCPFlagMatches(v)...)
	}
	return result
}

// parseFlowICMPTypesSlice parses ICMP types from a pre-split slice.
func parseFlowICMPTypesSlice(vals []string) []uint8 {
	result := make([]uint8, 0, len(vals))
	for _, v := range vals {
		result = append(result, parseFlowICMPTypes(v)...)
	}
	return result
}

// parseFlowICMPCodesSlice parses ICMP codes from a pre-split slice.
func parseFlowICMPCodesSlice(vals []string) []uint8 {
	result := make([]uint8, 0, len(vals))
	for _, v := range vals {
		result = append(result, parseFlowICMPCodes(v)...)
	}
	return result
}

// prefixCriterion answers the criterion an operator wrote for one prefix
// component, whichever of the two family spellings they used.
//
// A prefix component is singular, so the two spellings name one slot rather than
// two: a config carrying both is answering the same question twice and the
// second answer is dropped as an unknown criterion by the caller's `seen` sweep.
// The KEY comes back with the value so every refusal quotes the operator.
func prefixCriterion(criteria map[string][]string, v4, v6 string) (key string, values []string, ok bool) {
	if vals, found := criteria[v4]; found {
		return v4, vals, true
	}
	if vals, found := criteria[v6]; found {
		return v6, vals, true
	}
	return "", nil, false
}
