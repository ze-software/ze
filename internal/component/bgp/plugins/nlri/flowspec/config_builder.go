// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI plugin
// RFC: rfc/short/rfc5575.md
// Related: config.go -- parseConfigRoute (supplies the match-criteria map)

package flowspec

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

// buildFlowSpecComponents builds the FlowSpec from config-format match criteria
// and returns any criterion keys that were PRESENT in the config but produced no
// component: either an UNRECOGNIZED key (typo) or a known key whose value(s)
// failed to parse (e.g. an invalid prefix or a bad port match). Either case would
// silently widen the filter (worst case an all-match rule), so fail-loud callers
// reject a non-empty dropped list. The `seen` set is populated by the blocks
// themselves, so a future criterion missing its block fails loud rather than
// being silently dropped.
func buildFlowSpecComponents(matchCriteria map[string][]string, isIPv6 bool) (*flowspec.FlowSpec, []string) {
	fam := IPv4FlowSpec
	if isIPv6 {
		fam = IPv6FlowSpec
	}

	fs := flowspec.NewFlowSpec(fam)
	var dropped []string
	seen := make(map[string]bool, len(matchCriteria))

	// add records the criterion as dropped when the component cannot join the ones
	// already built. AddComponent refuses a second component of a type it cannot
	// merge (RFC 8955 Section 4.2), which is the same fail-loud case as a value
	// that would not parse: the criterion is present in the config and produced no
	// match, so the caller must reject the route rather than widen it.
	add := func(key string, c flowspec.FlowComponent) {
		if err := fs.AddComponent(c); err != nil {
			dropped = append(dropped, key)
		}
	}

	// Prefix components are singular. Refuse multiple values rather than
	// silently selecting the first; report the operator's criterion name.
	if key, vals, ok := prefixCriterion(matchCriteria, flowspec.KwDestinationIPv4, flowspec.KwDestinationIPv6); ok {
		seen[key] = true
		switch len(vals) {
		case 1:
			prefix, offset := parseFlowPrefixWithOffset(vals[0])
			if !prefix.IsValid() {
				dropped = append(dropped, key)
				break
			}
			add(key, flowspec.NewFlowDestPrefixComponentWithOffset(prefix, offset))
		default:
			dropped = append(dropped, key)
		}
	}

	// Source prefixes have the same singular-value requirement.
	if key, vals, ok := prefixCriterion(matchCriteria, flowspec.KwSourceIPv4, flowspec.KwSourceIPv6); ok {
		seen[key] = true
		switch len(vals) {
		case 1:
			prefix, offset := parseFlowPrefixWithOffset(vals[0])
			if !prefix.IsValid() {
				dropped = append(dropped, key)
				break
			}
			add(key, flowspec.NewFlowSourcePrefixComponentWithOffset(prefix, offset))
		default:
			dropped = append(dropped, key)
		}
	}

	// Numeric / match criteria: a present key with no parseable value is dropped.
	addNumeric := func(key string, typ flowspec.FlowComponentType, parse func([]string) []flowspec.FlowMatch) {
		if vals, ok := matchCriteria[key]; ok {
			seen[key] = true
			if matches := parse(vals); len(matches) > 0 {
				add(key, flowspec.NewFlowNumericComponent(typ, matches))
			} else {
				dropped = append(dropped, key)
			}
		}
	}
	addNumeric(flowspec.KwProtocol, flowspec.FlowIPProtocol, parseFlowProtocolMatchesSlice)
	addNumeric(flowspec.KwNextHeader, flowspec.FlowIPProtocol, parseFlowProtocolMatchesSlice)
	addNumeric(flowspec.KwPort, flowspec.FlowPort, parseFlowMatchesSlice)
	addNumeric(flowspec.KwDestPort, flowspec.FlowDestPort, parseFlowMatchesSlice)
	addNumeric(flowspec.KwSourcePort, flowspec.FlowSourcePort, parseFlowMatchesSlice)
	addNumeric(flowspec.KwPacketLength, flowspec.FlowPacketLength, parseFlowMatchesSlice)
	addNumeric(flowspec.KwTCPFlags, flowspec.FlowTCPFlags, parseFlowTCPFlagMatchesSlice)

	if vals, ok := matchCriteria[flowspec.KwDSCP]; ok {
		seen[flowspec.KwDSCP] = true
		octets, err := parseFlowDSCPSlice(vals)
		switch {
		case err != nil:
			dropped = append(dropped, flowspec.KwDSCP+" "+err.Error())
		case len(octets) > 0:
			add(flowspec.KwDSCP, flowspec.NewFlowDSCPComponent(octets...))
		default:
			dropped = append(dropped, flowspec.KwDSCP)
		}
	}
	if vals, ok := matchCriteria[flowspec.KwTrafficClass]; ok {
		seen[flowspec.KwTrafficClass] = true
		octets, err := parseFlowDSCPSlice(vals)
		switch {
		case err != nil:
			dropped = append(dropped, flowspec.KwTrafficClass+" "+err.Error())
		case len(octets) > 0:
			add(flowspec.KwTrafficClass, flowspec.NewFlowDSCPComponent(octets...))
		default:
			dropped = append(dropped, flowspec.KwTrafficClass)
		}
	}
	if vals, ok := matchCriteria[flowspec.KwFlowLabel]; ok {
		seen[flowspec.KwFlowLabel] = true
		if labels := parseFlowLabelsSlice(vals); len(labels) > 0 {
			add(flowspec.KwFlowLabel, flowspec.NewFlowFlowLabelComponent(labels...))
		} else {
			dropped = append(dropped, flowspec.KwFlowLabel)
		}
	}
	if vals, ok := matchCriteria[flowspec.KwFragment]; ok {
		seen[flowspec.KwFragment] = true
		if flags := parseFlowFragmentSlice(vals); len(flags) > 0 {
			add(flowspec.KwFragment, flowspec.NewFlowFragmentComponent(flags...))
		} else {
			dropped = append(dropped, flowspec.KwFragment)
		}
	}
	if vals, ok := matchCriteria[flowspec.KwICMPType]; ok {
		seen[flowspec.KwICMPType] = true
		if types := parseFlowICMPTypesSlice(vals); len(types) > 0 {
			add(flowspec.KwICMPType, flowspec.NewFlowICMPTypeComponent(types...))
		} else {
			dropped = append(dropped, flowspec.KwICMPType)
		}
	}
	if vals, ok := matchCriteria[flowspec.KwICMPCode]; ok {
		seen[flowspec.KwICMPCode] = true
		if codes := parseFlowICMPCodesSlice(vals); len(codes) > 0 {
			add(flowspec.KwICMPCode, flowspec.NewFlowICMPCodeComponent(codes...))
		} else {
			dropped = append(dropped, flowspec.KwICMPCode)
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

// parseFlowPrefixWithOffset parses a FlowSpec prefix like "10.0.0.1/32" or "::1/128/120".
// Returns the prefix and offset (0 if no offset).
func parseFlowPrefixWithOffset(s string) (netip.Prefix, uint8) {
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return netip.Prefix{}, 0
	}
	if len(parts) > 3 {
		return netip.Prefix{}, 0
	}
	prefix, err := netip.ParsePrefix(parts[0] + "/" + parts[1])
	if err != nil {
		return netip.Prefix{}, 0
	}
	if len(parts) == 2 {
		return prefix, 0
	}
	value, err := strconv.ParseUint(parts[2], 10, 8)
	if err != nil {
		return netip.Prefix{}, 0
	}
	if value == 0 {
		return prefix, 0
	}
	if !prefix.Addr().Is6() {
		return netip.Prefix{}, 0
	}
	if int(value) >= prefix.Bits() {
		return netip.Prefix{}, 0
	}
	return prefix, uint8(value)
}

// parseFlowProtocolMatches parses protocol values with operators.
func parseFlowProtocolMatches(s string) []flowspec.FlowMatch {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []flowspec.FlowMatch

	for _, p := range parts {
		var op flowspec.FlowOperator

		// Parse operator prefix
		switch {
		case strings.HasPrefix(p, "!="):
			op = flowspec.FlowOpNotEq
			p = strings.TrimPrefix(p, "!=")
		case strings.HasPrefix(p, "="):
			op = flowspec.FlowOpEqual
			p = strings.TrimPrefix(p, "=")
		default: // No operator prefix — bare protocol name/number implies equality
			op = flowspec.FlowOpEqual
		}

		p = strings.ToLower(p)
		if v, ok := flowspec.ProtocolNameToNumber[p]; ok {
			result = append(result, flowspec.FlowMatch{Op: op, Value: uint64(v)})
			continue
		}
		n, err := strconv.ParseUint(p, 10, 8)
		if err != nil {
			return nil
		}
		result = append(result, flowspec.FlowMatch{Op: op, Value: n})
	}
	return result
}

// parseFlowMatches parses FlowSpec match expressions with operators.
// Formats: "=80", ">1024", "[ =80 =8080 ]", ">8080&<8088", "!=443".
func parseFlowMatches(s string) []flowspec.FlowMatch {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []flowspec.FlowMatch

	for _, p := range parts {
		// Handle range operators like ">8080&<8088" by splitting on &
		rangeParts := strings.Split(p, "&")
		for i, rp := range rangeParts {
			var op flowspec.FlowOperator
			isAnd := i > 0 // Parts after & are AND-ed with previous

			// Parse operator prefix
			switch {
			case strings.HasPrefix(rp, "!="):
				op = flowspec.FlowOpNotEq
				rp = strings.TrimPrefix(rp, "!=")
			case strings.HasPrefix(rp, ">="):
				op = flowspec.FlowOpGreater | flowspec.FlowOpEqual
				rp = strings.TrimPrefix(rp, ">=")
			case strings.HasPrefix(rp, "<="):
				op = flowspec.FlowOpLess | flowspec.FlowOpEqual
				rp = strings.TrimPrefix(rp, "<=")
			case strings.HasPrefix(rp, ">"):
				op = flowspec.FlowOpGreater
				rp = strings.TrimPrefix(rp, ">")
			case strings.HasPrefix(rp, "<"):
				op = flowspec.FlowOpLess
				rp = strings.TrimPrefix(rp, "<")
			case strings.HasPrefix(rp, "="):
				op = flowspec.FlowOpEqual
				rp = strings.TrimPrefix(rp, "=")
			default: // No operator prefix — bare number implies equality
				op = flowspec.FlowOpEqual
			}

			n, err := strconv.ParseUint(rp, 10, 64)
			if err != nil {
				return nil
			}
			result = append(result, flowspec.FlowMatch{Op: op, And: isAnd, Value: n})
		}
	}
	return result
}

// parseFlowDSCP parses the DSCP values of a type 11 criterion (`dscp`, and
// `traffic-class`, ExaBGP's spelling of the same component).
//
// RFC 8955 Section 4.2.2.11: "The six least significant bits contain the DSCP
// value." A value above 63 needs a bit the component has no room for, so no
// packet can match it: it is refused with the limit, the bound the text path
// applies through componentMaxValue, never sent.
func parseFlowDSCP(s string) ([]uint8, error) {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []uint8
	dscpMax := componentMaxValue(flowspec.FlowDSCP)

	for _, p := range parts {
		p = strings.TrimPrefix(p, "=")
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a decimal DSCP value: write a number from 0 to %d", p, dscpMax)
		}
		if n > dscpMax {
			return nil, fmt.Errorf("%d is above %d: RFC 8955 Section 4.2.2.11 carries the DSCP in six bits, so write a value from 0 to %d", n, dscpMax, dscpMax)
		}
		result = append(result, uint8(n))
	}
	return result, nil
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
// An unknown token refuses the complete criterion.
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
		return nil
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
// An unknown token refuses the complete criterion.
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
		return nil
	}
	return result
}

// parseFlowFragment parses fragment flags like "[ first-fragment last-fragment ]".
func parseFlowFragment(s string) []flowspec.FlowFragmentFlag {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []flowspec.FlowFragmentFlag

	for _, p := range parts {
		f, ok := flowspec.FragmentFlagNameToValue[p]
		if !ok {
			return nil
		}
		result = append(result, flowspec.FlowFragmentFlag(f))
	}
	return result
}

// parseFlowTCPFlagMatches parses TCP flags with AND and NOT operators.
// TCP flags use bitmask matching:
//   - 0x01 = MATCH (exact match)
//   - 0x02 = NOT (negate)
//   - 0x40 = AND (AND with previous)
func parseFlowTCPFlagMatches(s string) []flowspec.FlowMatch {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []flowspec.FlowMatch

	for _, p := range parts {
		// Handle combined flags like "RST&FIN&!=push"
		flagParts := strings.Split(p, "&")
		for i, fp := range flagParts {
			var op flowspec.FlowOperator
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
			f, ok := flowspec.TCPFlagNameToValue[fp]
			if !ok {
				return nil
			}
			result = append(result, flowspec.FlowMatch{Op: op, And: isAnd, Value: uint64(f)})
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
		if err != nil {
			return nil
		}
		result = append(result, uint32(val))
	}
	return result
}

// --- Slice helpers for map[string][]string NLRI format ---

// parseFlowProtocolMatchesSlice parses protocol values from a pre-split slice.
func parseFlowProtocolMatchesSlice(vals []string) []flowspec.FlowMatch {
	result := make([]flowspec.FlowMatch, 0, len(vals))
	for _, v := range vals {
		matches := parseFlowProtocolMatches(v)
		if len(matches) == 0 {
			return nil
		}
		result = append(result, matches...)
	}
	return result
}

// parseFlowMatchesSlice parses numeric match expressions from a pre-split slice.
func parseFlowMatchesSlice(vals []string) []flowspec.FlowMatch {
	result := make([]flowspec.FlowMatch, 0, len(vals))
	for _, v := range vals {
		matches := parseFlowMatches(v)
		if len(matches) == 0 {
			return nil
		}
		result = append(result, matches...)
	}
	return result
}

// parseFlowDSCPSlice parses DSCP values from a pre-split slice. An empty
// element answers no values and no error, which the caller drops as unparseable.
func parseFlowDSCPSlice(vals []string) ([]uint8, error) {
	result := make([]uint8, 0, len(vals))
	for _, v := range vals {
		octets, err := parseFlowDSCP(v)
		if err != nil {
			return nil, err
		}
		if len(octets) == 0 {
			return nil, nil
		}
		result = append(result, octets...)
	}
	return result, nil
}

// parseFlowLabelsSlice parses flow-label values from a pre-split slice.
func parseFlowLabelsSlice(vals []string) []uint32 {
	result := make([]uint32, 0, len(vals))
	for _, v := range vals {
		labels := parseFlowLabels(v)
		if len(labels) == 0 {
			return nil
		}
		result = append(result, labels...)
	}
	return result
}

// parseFlowFragmentSlice parses fragment flags from a pre-split slice.
func parseFlowFragmentSlice(vals []string) []flowspec.FlowFragmentFlag {
	result := make([]flowspec.FlowFragmentFlag, 0, len(vals))
	for _, v := range vals {
		flags := parseFlowFragment(v)
		if len(flags) == 0 {
			return nil
		}
		result = append(result, flags...)
	}
	return result
}

// parseFlowTCPFlagMatchesSlice parses TCP flag matches from a pre-split slice.
func parseFlowTCPFlagMatchesSlice(vals []string) []flowspec.FlowMatch {
	result := make([]flowspec.FlowMatch, 0, len(vals))
	for _, v := range vals {
		matches := parseFlowTCPFlagMatches(v)
		if len(matches) == 0 {
			return nil
		}
		result = append(result, matches...)
	}
	return result
}

// parseFlowICMPTypesSlice parses ICMP types from a pre-split slice.
func parseFlowICMPTypesSlice(vals []string) []uint8 {
	result := make([]uint8, 0, len(vals))
	for _, v := range vals {
		types := parseFlowICMPTypes(v)
		if len(types) == 0 {
			return nil
		}
		result = append(result, types...)
	}
	return result
}

// parseFlowICMPCodesSlice parses ICMP codes from a pre-split slice.
func parseFlowICMPCodesSlice(vals []string) []uint8 {
	result := make([]uint8, 0, len(vals))
	for _, v := range vals {
		codes := parseFlowICMPCodes(v)
		if len(codes) == 0 {
			return nil
		}
		result = append(result, codes...)
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
