// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI plugin
// RFC: rfc/short/rfc5575.md

package flowspec

import (
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
)

// BuildFlowSpecNLRI builds FlowSpec NLRI bytes from config-format match criteria.
// This implements the InProcessConfigNLRIBuilder signature for the plugin registry.
// matchCriteria keys use config syntax (e.g., "destination", "protocol", "port").
func BuildFlowSpecNLRI(matchCriteria map[string][]string, isIPv6, forVPN bool) []byte {
	fam := IPv4FlowSpec
	if isIPv6 {
		fam = IPv6FlowSpec
	}

	fs := NewFlowSpec(fam)

	// Add destination prefix (first value only - prefix is singular)
	if vals, ok := matchCriteria["destination"]; ok && len(vals) > 0 {
		prefix, offset := parseFlowPrefixWithOffset(vals[0])
		if prefix.IsValid() {
			if prefix.Addr().Is6() && offset > 0 {
				fs.AddComponent(NewFlowDestPrefixComponentWithOffset(prefix, offset))
			} else {
				fs.AddComponent(NewFlowDestPrefixComponent(prefix))
			}
		}
	}

	// Add source prefix (first value only - prefix is singular)
	if vals, ok := matchCriteria["source"]; ok && len(vals) > 0 {
		prefix, offset := parseFlowPrefixWithOffset(vals[0])
		if prefix.IsValid() {
			if prefix.Addr().Is6() && offset > 0 {
				fs.AddComponent(NewFlowSourcePrefixComponentWithOffset(prefix, offset))
			} else {
				fs.AddComponent(NewFlowSourcePrefixComponent(prefix))
			}
		}
	}

	// Add protocol (supports multiple values like [ =tcp =udp ])
	if vals, ok := matchCriteria["protocol"]; ok {
		matches := parseFlowProtocolMatchesSlice(vals)
		if len(matches) > 0 {
			fs.AddComponent(NewFlowNumericComponent(FlowIPProtocol, matches))
		}
	}

	// Add next-header (IPv6 equivalent of protocol)
	if vals, ok := matchCriteria["next-header"]; ok {
		matches := parseFlowProtocolMatchesSlice(vals)
		if len(matches) > 0 {
			fs.AddComponent(NewFlowNumericComponent(FlowIPProtocol, matches))
		}
	}

	// Add port (matches either source or destination)
	if vals, ok := matchCriteria["port"]; ok {
		matches := parseFlowMatchesSlice(vals)
		if len(matches) > 0 {
			fs.AddComponent(NewFlowNumericComponent(FlowPort, matches))
		}
	}

	// Add destination port
	if vals, ok := matchCriteria["destination-port"]; ok {
		matches := parseFlowMatchesSlice(vals)
		if len(matches) > 0 {
			fs.AddComponent(NewFlowNumericComponent(FlowDestPort, matches))
		}
	}

	// Add source port
	if vals, ok := matchCriteria["source-port"]; ok {
		matches := parseFlowMatchesSlice(vals)
		if len(matches) > 0 {
			fs.AddComponent(NewFlowNumericComponent(FlowSourcePort, matches))
		}
	}

	// Add packet length
	if vals, ok := matchCriteria["packet-length"]; ok {
		matches := parseFlowMatchesSlice(vals)
		if len(matches) > 0 {
			fs.AddComponent(NewFlowNumericComponent(FlowPacketLength, matches))
		}
	}

	// Add DSCP
	if vals, ok := matchCriteria["dscp"]; ok {
		octets := parseFlowOctetsSlice(vals)
		if len(octets) > 0 {
			fs.AddComponent(NewFlowDSCPComponent(octets...))
		}
	}

	// Add traffic-class (IPv6)
	if vals, ok := matchCriteria["traffic-class"]; ok {
		octets := parseFlowOctetsSlice(vals)
		if len(octets) > 0 {
			fs.AddComponent(NewFlowDSCPComponent(octets...))
		}
	}

	// Add flow-label (IPv6)
	if vals, ok := matchCriteria["flow-label"]; ok {
		labels := parseFlowLabelsSlice(vals)
		if len(labels) > 0 {
			fs.AddComponent(NewFlowFlowLabelComponent(labels...))
		}
	}

	// Add fragment
	if vals, ok := matchCriteria["fragment"]; ok {
		flags := parseFlowFragmentSlice(vals)
		if len(flags) > 0 {
			fs.AddComponent(NewFlowFragmentComponent(flags...))
		}
	}

	// Add TCP flags
	if vals, ok := matchCriteria["tcp-flags"]; ok {
		matches := parseFlowTCPFlagMatchesSlice(vals)
		if len(matches) > 0 {
			fs.AddComponent(NewFlowNumericComponent(FlowTCPFlags, matches))
		}
	}

	// Add ICMP type
	if vals, ok := matchCriteria["icmp-type"]; ok {
		types := parseFlowICMPTypesSlice(vals)
		if len(types) > 0 {
			fs.AddComponent(NewFlowICMPTypeComponent(types...))
		}
	}

	// Add ICMP code
	if vals, ok := matchCriteria["icmp-code"]; ok {
		codes := parseFlowICMPCodesSlice(vals)
		if len(codes) > 0 {
			fs.AddComponent(NewFlowICMPCodeComponent(codes...))
		}
	}

	// For VPN, return component bytes without length prefix
	if forVPN {
		return fs.ComponentBytes()
	}
	return fs.Bytes()
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

	protoMap := map[string]uint8{
		"icmp": 1, "igmp": 2, "tcp": 6, "udp": 17, "gre": 47, "esp": 50, "ah": 51,
	}

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
		if v, ok := protoMap[p]; ok {
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

	flagMap := map[string]FlowFragmentFlag{
		"dont-fragment":  FlowFragDontFragment,
		"is-fragment":    FlowFragIsFragment,
		"first-fragment": FlowFragFirstFragment,
		"last-fragment":  FlowFragLastFragment,
	}

	for _, p := range parts {
		if f, ok := flagMap[p]; ok {
			result = append(result, f)
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

	flagMap := map[string]uint8{
		"fin": 0x01, "syn": 0x02, "rst": 0x04, "reset": 0x04,
		"psh": 0x08, "push": 0x08,
		"ack": 0x10, "urg": 0x20, "urgent": 0x20,
		"ece": 0x40, "cwr": 0x80,
	}

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
			if f, ok := flagMap[fp]; ok {
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
