// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI plugin
// RFC: rfc/short/rfc5575.md
// Related: config.go -- parseConfigRoute (supplies the match-criteria map)

package flowspec

import (
	"fmt"
	"maps"
	"net/netip"
	"slices"
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

	// Every numeric criterion takes the update text grammar
	// (parseNumericTermsText), so config and update text read one operator
	// table, one width table (componentMaxValue) and one name table
	// (componentValueNames), and a refusal carries the parser's reason and fix
	// rather than the key alone.
	addNumeric := func(key string, typ flowspec.FlowComponentType) {
		vals, ok := matchCriteria[key]
		if !ok {
			return
		}
		seen[key] = true
		matches, err := parseFlowNumericMatches(vals, typ)
		if err != nil {
			dropped = append(dropped, key+" "+err.Error())
			return
		}
		add(key, flowspec.NewFlowNumericComponent(typ, matches))
	}
	addNumeric(flowspec.KwProtocol, flowspec.FlowIPProtocol)
	addNumeric(flowspec.KwNextHeader, flowspec.FlowIPProtocol)
	addNumeric(flowspec.KwPort, flowspec.FlowPort)
	addNumeric(flowspec.KwDestPort, flowspec.FlowDestPort)
	addNumeric(flowspec.KwSourcePort, flowspec.FlowSourcePort)
	addNumeric(flowspec.KwPacketLength, flowspec.FlowPacketLength)
	addNumeric(flowspec.KwDSCP, flowspec.FlowDSCP)
	addNumeric(flowspec.KwTrafficClass, flowspec.FlowDSCP)
	addNumeric(flowspec.KwFlowLabel, flowspec.FlowFlowLabel)
	addNumeric(flowspec.KwICMPType, flowspec.FlowICMPType)
	addNumeric(flowspec.KwICMPCode, flowspec.FlowICMPCode)

	if vals, ok := matchCriteria[flowspec.KwTCPFlags]; ok {
		seen[flowspec.KwTCPFlags] = true
		matches, err := parseFlowBitmaskMatches(vals, flowspec.TCPFlagNameToValue)
		if err != nil {
			dropped = append(dropped, flowspec.KwTCPFlags+" "+err.Error())
		} else {
			add(flowspec.KwTCPFlags, flowspec.NewFlowTCPFlagsMatchComponent(matches))
		}
	}
	if vals, ok := matchCriteria[flowspec.KwFragment]; ok {
		seen[flowspec.KwFragment] = true
		matches, err := parseFlowBitmaskMatches(vals, flowspec.FragmentFlagNameToValue)
		if err != nil {
			dropped = append(dropped, flowspec.KwFragment+" "+err.Error())
		} else {
			add(flowspec.KwFragment, flowspec.NewFlowFragmentMatchComponent(matches))
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

// parseFlowNumericMatches parses the values of a numeric criterion (protocol,
// next-header, the ports, packet-length, icmp-type, icmp-code, dscp,
// traffic-class, flow-label) with the update text grammar
// (parseNumericTermsText), so config and update text read one operator table:
// `>=1024&<=65535`, `[ >5 <10 ]`, `!=443`, a bare `80`, and a value name where
// componentValueNames has one (`=tcp`, `echo-request`). Each element of vals
// may hold several space-separated tokens and the brackets of a list.
//
// RFC 8955 Section 4.2.1.1: "The operator byte is encoded as: ... a - and bit.
// If unset, the result of the previous {operator, value} pair is ORed with the
// current one. If set, it is ANDed." A value is refused with its reason, never
// skipped or widened: a term missing would match packets the operator
// excluded, and a value wider than its field (componentMaxValue) would reach
// the wire as a rule no packet can satisfy as written.
func parseFlowNumericMatches(vals []string, typ flowspec.FlowComponentType) ([]flowspec.FlowMatch, error) {
	maxValue := componentMaxValue(typ)
	names := componentValueNames(typ)
	var matches []flowspec.FlowMatch
	for _, v := range vals {
		for token := range strings.FieldsSeq(strings.Trim(v, "[]")) {
			// RFC 8955 Section 4.2.1.1
			terms, err := parseNumericTermsText(token, len(matches) > 0, maxValue, names)
			if err != nil {
				return nil, err
			}
			matches = append(matches, terms...)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("has no value: write one or more <op><value> terms with values from 0 to %d, as in =%d or >=1&<=%d", maxValue, maxValue, maxValue)
	}
	return matches, nil
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

// parseFlowBitmaskMatches parses the values of a bitmask criterion (tcp-flags,
// fragment) with the update text grammar (parseBitmaskTermsText), so config
// and update text read one table: `syn&ack` (SYN set AND ACK set),
// `[ syn ack ]` (SYN set OR ACK set), `syn+ack` (one mask of both bits),
// `!rst`, `=syn`, `!=fin`. names maps a flag name to its bit.
//
// RFC 8955 Section 4.2.2.9: "Encoding: <type (1 octet), [bitmask_op,
// bitmask]+>", and Section 4.2.2.12 the same for fragment. A term is refused
// with its reason, never skipped: a criterion with a term missing would match
// packets the operator excluded.
func parseFlowBitmaskMatches(vals []string, names map[string]uint8) ([]flowspec.FlowMatch, error) {
	var matches []flowspec.FlowMatch
	for _, v := range vals {
		for token := range strings.FieldsSeq(strings.ToLower(strings.Trim(v, "[]"))) {
			// RFC 8955 Section 4.2.1.2
			terms, err := parseBitmaskTermsText(token, len(matches) > 0, names)
			if err != nil {
				return nil, err
			}
			matches = append(matches, terms...)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("has no value: write one or more flags (%s), as in [ %s ]", strings.Join(slices.Sorted(maps.Keys(names)), ", "), strings.Join(slices.Sorted(maps.Keys(names))[:2], " "))
	}
	return matches, nil
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
